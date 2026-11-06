package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// 登录安全审计只覆盖密码登录与登录两步验证两个 POST 入口。中间件注册在
// RequestId/I18n 之后、IPBan 与限流之前，只读取已解析的数据，不抓取请求体
// 或响应体；handler 用 MarkLoginAudit* 给出明确结果，本中间件在请求结束前
// 同步落库一次，失败则写结构化备份。
const (
	loginAuditEntryPassword  = "password"
	loginAuditEntryTwoFactor = "2fa"

	// 单次请求内的写入上限；与 model 的插入上限一致，客户端断开也不影响留档。
	loginAuditRequestTimeout  = 500 * time.Millisecond
	loginAuditFlushInterval   = 10 * time.Second
	loginAuditFlushTimeout    = 2 * time.Second
	loginAuditBucketRetention = 10 * time.Minute
)

// loginAuditBucketLimit 限制预拦截聚合状态的数量，避免内存无界增长。
var loginAuditBucketLimit = 2048

// loginAuditState 是单次请求的审计状态，挂在 gin context 上供 handler 标记。
type loginAuditState struct {
	Entry               string
	Decided             bool
	Outcome             string
	Stage               string
	Reason              string
	TargetUserId        int
	TargetUsername      string
	AttemptedIdentifier string
}

// loginAuditEntryForRequest 返回该请求对应的审计入口，非登录入口返回空串。
func loginAuditEntryForRequest(method string, path string) string {
	if !strings.EqualFold(strings.TrimSpace(method), http.MethodPost) {
		return ""
	}
	switch strings.TrimSpace(path) {
	case "/api/user/login":
		return loginAuditEntryPassword
	case "/api/user/login/2fa":
		return loginAuditEntryTwoFactor
	}
	return ""
}

// LoginAuditRequestPath 返回不含 query 的请求路径：优先用路由模板，回退到
// URL 路径。调用方据此写入 other.request_path，避免记录查询参数。
func LoginAuditRequestPath(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if path := c.FullPath(); path != "" {
		return path
	}
	if c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	return ""
}

func loginAuditStateFrom(c *gin.Context) *loginAuditState {
	value, ok := common.GetContextKey(c, constant.ContextKeyLoginAudit)
	if !ok {
		return nil
	}
	state, ok := value.(*loginAuditState)
	if !ok {
		return nil
	}
	return state
}

// LoginAudit 捕获登录入口的认证结果并写入登录安全审计日志（type=8）。
//
// 只有显式登录路径会在入口创建审计状态；OAuth/Passkey/归属转移等共享登录出口
// 由明确的登录拒绝点调用 EnsureLoginAuditState 按需创建。没有状态时本中间件
// 不记录任何东西，普通 API 失败不会被误记成登录事件。
func LoginAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if entry := loginAuditEntryForRequest(c.Request.Method, LoginAuditRequestPath(c)); entry != "" {
			storeLoginAuditState(c, &loginAuditState{Entry: entry})
		}

		// 用 defer 收尾：handler 内部 panic 时本函数会先于外层 Recovery 执行，
		// 未标记结果的请求因此会留下 error 事件；这里不做 recover，异常继续上抛。
		defer finishLoginAudit(c)

		c.Next()
	}
}

func storeLoginAuditState(c *gin.Context, state *loginAuditState) {
	if c == nil || state == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyLoginAudit, state)
}

// EnsureLoginAuditState 在明确的登录拒绝点按需创建审计状态（例如共享会话出口
// 拒绝建立会话时），供 LoginAudit 的 defer 收尾使用。
func EnsureLoginAuditState(c *gin.Context, method string) {
	if c == nil || loginAuditStateFrom(c) != nil {
		return
	}
	entry := strings.TrimSpace(method)
	if entry == "" {
		entry = "unknown"
	}
	storeLoginAuditState(c, &loginAuditState{Entry: entry})
}

// MarkLoginAuditOutcome 标记本次登录请求的认证结果；只有第一个标记生效。
func MarkLoginAuditOutcome(c *gin.Context, outcome string, stage string, reason string) {
	state := loginAuditStateFrom(c)
	if state == nil || state.Decided {
		return
	}
	state.Decided = true
	state.Outcome = outcome
	state.Stage = stage
	state.Reason = reason
}

func MarkLoginAuditSuccess(c *gin.Context) {
	MarkLoginAuditOutcome(c, model.LoginAuditOutcomeSuccess, model.LoginAuditStageSession, "")
}

func MarkLoginAuditFailure(c *gin.Context, stage string, reason string) {
	MarkLoginAuditOutcome(c, model.LoginAuditOutcomeFailure, stage, reason)
}

func MarkLoginAuditBlocked(c *gin.Context, stage string, reason string) {
	MarkLoginAuditOutcome(c, model.LoginAuditOutcomeBlocked, stage, reason)
}

func MarkLoginAuditChallenge(c *gin.Context, stage string, reason string) {
	MarkLoginAuditOutcome(c, model.LoginAuditOutcomeChallenge, stage, reason)
}

func MarkLoginAuditError(c *gin.Context, stage string, reason string) {
	MarkLoginAuditOutcome(c, model.LoginAuditOutcomeError, stage, reason)
}

// MarkLoginAuditTarget 记录被尝试登录的账号；它只作为目标，永远不会成为 actor。
func MarkLoginAuditTarget(c *gin.Context, userId int, username string) {
	state := loginAuditStateFrom(c)
	if state == nil || userId <= 0 {
		return
	}
	state.TargetUserId = userId
	state.TargetUsername = username
}

// MarkLoginAuditIdentifier 记录输入但未匹配到账号的登录名（限长后落库）。
func MarkLoginAuditIdentifier(c *gin.Context, identifier string) {
	state := loginAuditStateFrom(c)
	if state == nil {
		return
	}
	state.AttemptedIdentifier = identifier
}

// LoginAuditRequestDiagnostics 返回仅管理员可见的请求诊断字段。
func LoginAuditRequestDiagnostics(c *gin.Context) map[string]interface{} {
	diagnostics := make(map[string]interface{}, 2)
	if ray := loginAuditCloudflareRay(c); ray != "" {
		diagnostics["cf_ray"] = ray
	}
	if peerIP := loginAuditPeerIP(c); peerIP != "" {
		diagnostics["peer_ip"] = peerIP
	}
	return diagnostics
}

func loginAuditCloudflareRay(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetHeader(common.CloudflareRayHeader))
}

// loginAuditPeerIP 取 TCP 对端地址，便于与客户端声明 IP 交叉排查。
func loginAuditPeerIP(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	remote := strings.TrimSpace(c.Request.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		return host
	}
	return remote
}

// loginAuditRequestContext 给单次写入一个自带时限的独立 context。
// 不用请求 context 的取消（客户端断开也要留档），但会尊重更早的请求截止时间。
func loginAuditRequestContext(c *gin.Context) (context.Context, context.CancelFunc) {
	timeout := loginAuditRequestTimeout
	if c != nil && c.Request != nil {
		if deadline, ok := c.Request.Context().Deadline(); ok {
			if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
				timeout = remaining
			}
		}
	}
	return context.WithTimeout(context.Background(), timeout)
}

func finishLoginAudit(c *gin.Context) {
	state := loginAuditStateFrom(c)
	if state == nil {
		return
	}
	if !state.Decided {
		// handler 没有给出明确结果：记录为内部错误，避免事件静默丢失。
		state.Decided = true
		state.Outcome = model.LoginAuditOutcomeError
		state.Stage = model.LoginAuditStageSession
		state.Reason = model.LoginAuditReasonInternalError
	}
	if state.Outcome == model.LoginAuditOutcomeSuccess {
		// 成功登录由 controller 以 type=7 记录，这里不重复写。
		return
	}

	event := buildLoginAuditEvent(c, state)
	ctx, cancel := loginAuditRequestContext(c)
	defer cancel()

	// 前置拦截（IPBan/限流/人机验证/缺 UA）可能成批出现，按
	// IP+入口+原因+分钟窗口聚合；但已知目标账号的事件必须逐条保留，
	// 不能把不同目标并进同一个 IP 桶。真实口令与 2FA 校验同样逐条记录。
	if aggregateLoginAuditStage(state.Stage) && state.TargetUserId == 0 {
		aggregateLoginAuditEvent(ctx, event, state.Entry)
		return
	}
	persistLoginAuditEvents(ctx, []model.LoginAuditEvent{event})
}

// aggregateLoginAuditStage 判断该阶段是否按预拦截策略聚合。
func aggregateLoginAuditStage(stage string) bool {
	switch stage {
	case model.LoginAuditStagePrecheck, model.LoginAuditStageCaptcha:
		// 验证码拒绝同样位于认证之前，与 IPBan/限流一样按窗口聚合。
		return true
	default:
		return false
	}
}

func buildLoginAuditEvent(c *gin.Context, state *loginAuditState) model.LoginAuditEvent {
	event := model.LoginAuditEvent{
		Outcome:             state.Outcome,
		Stage:               state.Stage,
		Reason:              state.Reason,
		LoginMethod:         state.Entry,
		TargetUserId:        state.TargetUserId,
		TargetUsername:      state.TargetUsername,
		AttemptedIdentifier: state.AttemptedIdentifier,
		CloudflareRay:       loginAuditCloudflareRay(c),
		PeerIP:              loginAuditPeerIP(c),
		RequestPath:         LoginAuditRequestPath(c),
	}
	if c != nil {
		event.IP = c.ClientIP()
		event.RequestId = c.GetString(common.RequestIdKey)
		if c.Request != nil {
			event.UserAgent = c.Request.UserAgent()
		}
	}
	// 事件时间在写入之前固定，落库延迟不会改变它。
	event.CreatedAt = time.Now().Unix()
	// 限长在写入之前完成，内存里不保留原始超长字段。
	return model.SanitizeLoginAuditEvent(event)
}

type loginAuditAggKey struct {
	IP          string
	Entry       string
	Reason      string
	WindowStart int64
}

type loginAuditBucket struct {
	Base     model.LoginAuditEvent
	Pending  int
	LastSeen int64
}

// loginAuditOverflowBucket 汇总因聚合表满而无法单独建桶的预拦截命中。
// 只按原因归并、不保存每个溢出 IP，因此状态数量被原因白名单限制在有限集合内。
type loginAuditOverflowBucket struct {
	Base        model.LoginAuditEvent
	Count       int
	WindowStart int64
}

var (
	loginAuditAggMu          sync.Mutex
	loginAuditAggBuckets     = make(map[loginAuditAggKey]*loginAuditBucket)
	loginAuditOverflow       = make(map[string]*loginAuditOverflowBucket)
	loginAuditOverflowTotal  int64
	loginAuditCapacityWarnAt int64
)

// aggregateLoginAuditEvent 首条事件同步落库，后续命中只累加计数，由 flush 以
// 差额写入。聚合状态满时命中并入按原因归并的溢出桶，计数不会静默丢失。
func aggregateLoginAuditEvent(ctx context.Context, event model.LoginAuditEvent, entry string) {
	now := time.Now()
	windowSeconds := int64(time.Minute / time.Second)
	windowStart := now.Unix() - now.Unix()%windowSeconds
	windowEnd := windowStart + windowSeconds - 1

	key := loginAuditAggKey{IP: event.IP, Entry: entry, Reason: event.Reason, WindowStart: windowStart}

	var immediate []model.LoginAuditEvent
	var capacityWarning string

	loginAuditAggMu.Lock()
	if bucket, ok := loginAuditAggBuckets[key]; ok {
		bucket.Pending++
		bucket.LastSeen = now.Unix()
		loginAuditAggMu.Unlock()
		return
	}

	switch {
	case !model.IsKnownLoginAuditReason(event.Reason):
		// 白名单之外的原因不建状态，直接单条落库。
		immediate = append(immediate, event)
	case len(loginAuditAggBuckets)+len(loginAuditOverflow) >= loginAuditBucketLimit:
		overflow, ok := loginAuditOverflow[event.Reason]
		if ok && overflow.WindowStart != windowStart {
			// 上一窗口的溢出计数先落库，保证窗口字段自洽。
			stale := overflow.Base
			stale.Count = overflow.Count
			stale.WindowStart = overflow.WindowStart
			immediate = append(immediate, stale)
			overflow = nil
		}
		if overflow == nil {
			overflow = &loginAuditOverflowBucket{
				Base: model.LoginAuditEvent{
					Outcome:     event.Outcome,
					Stage:       event.Stage,
					Reason:      event.Reason,
					LoginMethod: event.LoginMethod,
					RequestPath: event.RequestPath,
					CreatedAt:   event.CreatedAt,
				},
				WindowStart: windowStart,
			}
			loginAuditOverflow[event.Reason] = overflow
		}
		overflow.Count++
		overflow.Base.WindowEnd = windowEnd
		loginAuditOverflowTotal++
		if now.Unix()-loginAuditCapacityWarnAt >= 60 {
			loginAuditCapacityWarnAt = now.Unix()
			capacityWarning = fmt.Sprintf(
				"login audit aggregate cache full (%d buckets), folding precheck hits into per-reason overflow (total overflow %d)",
				loginAuditBucketLimit, loginAuditOverflowTotal,
			)
		}
	default:
		event.Count = 1
		event.WindowStart = windowStart
		event.WindowEnd = windowEnd
		loginAuditAggBuckets[key] = &loginAuditBucket{
			Base:     event,
			LastSeen: now.Unix(),
		}
		immediate = append(immediate, event)
	}
	loginAuditAggMu.Unlock()

	if capacityWarning != "" {
		common.SysError(capacityWarning)
	}
	if len(immediate) > 0 {
		persistLoginAuditEvents(ctx, immediate)
	}
}

// drainLoginAuditAggregates 取走当前的差额计数并清理过期状态。
func drainLoginAuditAggregates() []model.LoginAuditEvent {
	now := common.GetTimestamp()
	events := make([]model.LoginAuditEvent, 0, 8)

	loginAuditAggMu.Lock()
	for key, bucket := range loginAuditAggBuckets {
		if bucket.Pending > 0 {
			event := bucket.Base
			event.Count = bucket.Pending
			events = append(events, event)
			bucket.Pending = 0
		}
		if now-bucket.LastSeen >= int64(loginAuditBucketRetention/time.Second) {
			delete(loginAuditAggBuckets, key)
		}
	}
	for reason, overflow := range loginAuditOverflow {
		if overflow.Count > 0 {
			event := overflow.Base
			event.Count = overflow.Count
			event.WindowStart = overflow.WindowStart
			if event.WindowEnd == 0 {
				event.WindowEnd = overflow.WindowStart + int64(time.Minute/time.Second) - 1
			}
			events = append(events, event)
		}
		delete(loginAuditOverflow, reason)
	}
	loginAuditAggMu.Unlock()

	return events
}

// persistLoginAuditEvents 是中间件侧唯一的写入出口：同步写库，失败后把尚未
// 写入的事件转成结构化备份日志，绝不改变认证响应。
func persistLoginAuditEvents(ctx context.Context, events []model.LoginAuditEvent) {
	if len(events) == 0 {
		return
	}
	persisted, err := model.RecordLoginSecurityEvents(ctx, events)
	if err == nil {
		return
	}
	common.SysError(fmt.Sprintf("failed to persist %d login audit event(s): %v", len(events)-persisted, err))
	backupLoginAuditEvents(events[persisted:])
}

// backupLoginAuditEvents 逐条写结构化备份；只做内存序列化与日志输出，不再访问
// 数据库，因此超时后不会逐条等待。
func backupLoginAuditEvents(events []model.LoginAuditEvent) {
	for _, event := range events {
		backupLoginAuditEvent(event)
	}
}

func backupLoginAuditEvent(event model.LoginAuditEvent) {
	payload, err := model.LoginAuditEventBackupJSON(event)
	if err != nil {
		common.SysError(fmt.Sprintf(
			"failed to marshal login audit backup: %v; outcome=%s stage=%s reason=%s",
			err, event.Outcome, event.Stage, event.Reason,
		))
		return
	}
	common.SysError("login audit event backup: " + payload)
}

// FlushLoginAuditContext 在给定总时限内落库聚合计数；超时后剩余事件转结构化
// 备份，不再逐条等待。
func FlushLoginAuditContext(ctx context.Context) {
	events := drainLoginAuditAggregates()
	if len(events) == 0 {
		return
	}
	persistLoginAuditEvents(ctx, events)
}

// FlushLoginAudit 用默认的 2 秒总时限落库聚合计数。
func FlushLoginAudit() {
	ctx, cancel := context.WithTimeout(context.Background(), loginAuditFlushTimeout)
	defer cancel()
	FlushLoginAuditContext(ctx)
}

var (
	loginAuditFlusherMu     sync.Mutex
	loginAuditFlusherCancel context.CancelFunc
	loginAuditFlusherDone   chan struct{}
)

// StartLoginAuditFlusher 启动周期落库任务，由 main 显式调用；重复调用无副作用。
func StartLoginAuditFlusher(interval time.Duration) {
	if interval <= 0 {
		interval = loginAuditFlushInterval
	}
	loginAuditFlusherMu.Lock()
	defer loginAuditFlusherMu.Unlock()
	if loginAuditFlusherCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	loginAuditFlusherCancel = cancel
	loginAuditFlusherDone = done
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 周期批次受 worker 生命周期约束：退出时取消未写完的批次，
				// 未写入的计数会在该批次返回前转结构化备份。
				flushCtx, flushCancel := context.WithTimeout(ctx, loginAuditFlushTimeout)
				FlushLoginAuditContext(flushCtx)
				flushCancel()
			}
		}
	}()
}

// StopLoginAuditFlusher 停止周期落库任务并等待进行中的批次结束，避免退出时丢计数。
func StopLoginAuditFlusher() {
	loginAuditFlusherMu.Lock()
	defer loginAuditFlusherMu.Unlock()
	cancel := loginAuditFlusherCancel
	if cancel == nil {
		return
	}
	cancel()
	// worker 从不获取该互斥锁，因此在锁内等待其收尾是安全的。
	<-loginAuditFlusherDone
	loginAuditFlusherCancel = nil
	loginAuditFlusherDone = nil
}
