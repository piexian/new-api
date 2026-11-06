package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// 单次登录安全审计写入的数据库上限：日志库故障时不能拖住认证流程。
const loginAuditInsertTimeout = 500 * time.Millisecond

// 登录安全审计的固定取值，前端与日志过滤依赖这些字面量。
const (
	LoginAuditOutcomeSuccess   = "success"
	LoginAuditOutcomeFailure   = "failure"
	LoginAuditOutcomeBlocked   = "blocked"
	LoginAuditOutcomeChallenge = "challenge"
	LoginAuditOutcomeError     = "error"
)

const (
	LoginAuditStagePrecheck    = "precheck"
	LoginAuditStageRequest     = "request"
	LoginAuditStageCaptcha     = "captcha"
	LoginAuditStageCredentials = "credentials"
	LoginAuditStageTwoFactor   = "two_factor"
	LoginAuditStageSession     = "session"
)

const (
	LoginAuditReasonInvalidRequest             = "invalid_request"
	LoginAuditReasonPasswordEncryptionRequired = "password_encryption_required"
	LoginAuditReasonPasswordDecryptionFailed   = "password_decryption_failed"
	LoginAuditReasonUserNotFound               = "user_not_found"
	LoginAuditReasonInvalidCredentials         = "invalid_credentials"
	LoginAuditReasonAccountDisabled            = "account_disabled"
	LoginAuditReasonPasswordLoginDisabled      = "password_login_disabled"
	LoginAuditReasonEmailRepairRequired        = "email_repair_required"
	LoginAuditReasonTwoFactorRequired          = "two_factor_required"
	LoginAuditReasonTwoFactorSessionExpired    = "two_factor_session_expired"
	LoginAuditReasonTwoFactorInvalid           = "two_factor_invalid"
	LoginAuditReasonTwoFactorLocked            = "two_factor_locked"
	LoginAuditReasonTwoFactorUnavailable       = "two_factor_unavailable"
	LoginAuditReasonSessionSaveFailed          = "session_save_failed"
	LoginAuditReasonDatabaseError              = "database_error"
	LoginAuditReasonIPBanned                   = "ip_banned"
	LoginAuditReasonRateLimited                = "rate_limited"
	LoginAuditReasonCaptchaMissing             = "captcha_missing"
	LoginAuditReasonCaptchaInvalid             = "captcha_invalid"
	LoginAuditReasonCaptchaError               = "captcha_error"
	LoginAuditReasonInternalError              = "internal_error"
	// LoginAuditReasonUserAgentMissing 表示客户端没有发送 User-Agent，浏览器
	// 环境无法识别，因此拒绝建立新登录会话。
	LoginAuditReasonUserAgentMissing = "user_agent_missing"
)

// 日志字段限长，避免恶意输入膨胀日志。
const (
	loginAuditMaxIdentifierLength  = 128
	loginAuditMaxUserAgentLength   = 512
	loginAuditMaxIPLength          = 64
	loginAuditMaxRequestIdLength   = 64
	loginAuditMaxDiagnosticLength  = 128
	loginAuditMaxRequestPathLength = 256
)

// LoginAuditEvent 是一次登录安全审计事件的完整内容。
type LoginAuditEvent struct {
	Outcome             string
	Stage               string
	Reason              string
	Count               int
	WindowStart         int64
	WindowEnd           int64
	LoginMethod         string
	IP                  string
	UserAgent           string
	RequestId           string
	RequestPath         string
	CloudflareRay       string
	PeerIP              string
	TargetUserId        int
	TargetUsername      string
	AttemptedIdentifier string
	CreatedAt           int64
}

var loginAuditOutcomes = map[string]struct{}{
	LoginAuditOutcomeFailure:   {},
	LoginAuditOutcomeBlocked:   {},
	LoginAuditOutcomeChallenge: {},
	LoginAuditOutcomeError:     {},
}

var loginAuditStages = map[string]struct{}{
	LoginAuditStagePrecheck:    {},
	LoginAuditStageRequest:     {},
	LoginAuditStageCaptcha:     {},
	LoginAuditStageCredentials: {},
	LoginAuditStageTwoFactor:   {},
	LoginAuditStageSession:     {},
}

var loginAuditReasons = map[string]struct{}{
	LoginAuditReasonInvalidRequest:             {},
	LoginAuditReasonPasswordEncryptionRequired: {},
	LoginAuditReasonPasswordDecryptionFailed:   {},
	LoginAuditReasonUserNotFound:               {},
	LoginAuditReasonInvalidCredentials:         {},
	LoginAuditReasonAccountDisabled:            {},
	LoginAuditReasonPasswordLoginDisabled:      {},
	LoginAuditReasonEmailRepairRequired:        {},
	LoginAuditReasonTwoFactorRequired:          {},
	LoginAuditReasonTwoFactorSessionExpired:    {},
	LoginAuditReasonTwoFactorInvalid:           {},
	LoginAuditReasonTwoFactorLocked:            {},
	LoginAuditReasonTwoFactorUnavailable:       {},
	LoginAuditReasonSessionSaveFailed:          {},
	LoginAuditReasonDatabaseError:              {},
	LoginAuditReasonIPBanned:                   {},
	LoginAuditReasonRateLimited:                {},
	LoginAuditReasonCaptchaMissing:             {},
	LoginAuditReasonCaptchaInvalid:             {},
	LoginAuditReasonCaptchaError:               {},
	LoginAuditReasonInternalError:              {},
	LoginAuditReasonUserAgentMissing:           {},
}

var loginAuditReasonTextZH = map[string]string{
	LoginAuditReasonInvalidRequest:             "请求参数无效",
	LoginAuditReasonPasswordEncryptionRequired: "未提供加密密码",
	LoginAuditReasonPasswordDecryptionFailed:   "密码解密失败",
	LoginAuditReasonUserNotFound:               "账号不存在",
	LoginAuditReasonInvalidCredentials:         "账号或密码错误",
	LoginAuditReasonAccountDisabled:            "账号已被禁用",
	LoginAuditReasonPasswordLoginDisabled:      "密码登录已关闭",
	LoginAuditReasonEmailRepairRequired:        "需要补全邮箱",
	LoginAuditReasonTwoFactorRequired:          "需要两步验证",
	LoginAuditReasonTwoFactorSessionExpired:    "两步验证会话已过期",
	LoginAuditReasonTwoFactorInvalid:           "两步验证码错误",
	LoginAuditReasonTwoFactorLocked:            "两步验证已锁定",
	LoginAuditReasonTwoFactorUnavailable:       "无法使用两步验证",
	LoginAuditReasonSessionSaveFailed:          "会话保存失败",
	LoginAuditReasonDatabaseError:              "数据库错误",
	LoginAuditReasonIPBanned:                   "IP 已被封禁",
	LoginAuditReasonRateLimited:                "请求过于频繁",
	LoginAuditReasonCaptchaMissing:             "缺少人机验证令牌",
	LoginAuditReasonCaptchaInvalid:             "人机验证失败",
	LoginAuditReasonCaptchaError:               "人机验证异常",
	LoginAuditReasonInternalError:              "内部错误",
	LoginAuditReasonUserAgentMissing:           "浏览器环境异常",
}

// loginAuditReasonTextEN 与 loginAuditReasonTextZH 一一对应，用于日志本地化。
var loginAuditReasonTextEN = map[string]string{
	LoginAuditReasonInvalidRequest:             "invalid request",
	LoginAuditReasonPasswordEncryptionRequired: "encrypted password missing",
	LoginAuditReasonPasswordDecryptionFailed:   "password decryption failed",
	LoginAuditReasonUserNotFound:               "account not found",
	LoginAuditReasonInvalidCredentials:         "incorrect account or password",
	LoginAuditReasonAccountDisabled:            "account disabled",
	LoginAuditReasonPasswordLoginDisabled:      "password login disabled",
	LoginAuditReasonEmailRepairRequired:        "email repair required",
	LoginAuditReasonTwoFactorRequired:          "two-factor authentication required",
	LoginAuditReasonTwoFactorSessionExpired:    "two-factor session expired",
	LoginAuditReasonTwoFactorInvalid:           "invalid two-factor code",
	LoginAuditReasonTwoFactorLocked:            "two-factor authentication locked",
	LoginAuditReasonTwoFactorUnavailable:       "Two-factor authentication unavailable",
	LoginAuditReasonSessionSaveFailed:          "failed to save session",
	LoginAuditReasonDatabaseError:              "database error",
	LoginAuditReasonIPBanned:                   "IP banned",
	LoginAuditReasonRateLimited:                "rate limited",
	LoginAuditReasonCaptchaMissing:             "missing captcha token",
	LoginAuditReasonCaptchaInvalid:             "captcha validation failed",
	LoginAuditReasonCaptchaError:               "captcha validation error",
	LoginAuditReasonInternalError:              "internal error",
	LoginAuditReasonUserAgentMissing:           "Browser environment error",
}

// loginAuditReasonTextZHToEN 按中文原因文案反查英文，供日志内容本地化使用。
var loginAuditReasonTextZHToEN = func() map[string]string {
	table := make(map[string]string, len(loginAuditReasonTextZH))
	for reason, zhText := range loginAuditReasonTextZH {
		if enText, ok := loginAuditReasonTextEN[reason]; ok {
			table[zhText] = enText
		}
	}
	return table
}()

func loginAuditReasonText(reason string) string {
	if text, ok := loginAuditReasonTextZH[reason]; ok {
		return text
	}
	return reason
}

func loginAuditContent(event LoginAuditEvent) string {
	switch event.Outcome {
	case LoginAuditOutcomeChallenge:
		return "登录需要两步验证：" + loginAuditReasonText(event.Reason)
	case LoginAuditOutcomeBlocked:
		return "登录请求被拦截：" + loginAuditReasonText(event.Reason)
	case LoginAuditOutcomeError:
		return "登录处理异常：" + loginAuditReasonText(event.Reason)
	default:
		return "登录失败：" + loginAuditReasonText(event.Reason)
	}
}

// truncateLoginAuditText 按字符截断，避免破坏多字节字符。
// sanitizeLoginAuditRequestPath 只保留路径，去掉 query 与 fragment，避免把
// 令牌、验证码等查询参数写进日志。
func sanitizeLoginAuditRequestPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		path = path[:idx]
	}
	return path
}

// IsKnownLoginAuditReason 判断原因是否属于固定白名单，用于把聚合状态限制在
// 有限集合内。
func IsKnownLoginAuditReason(reason string) bool {
	_, ok := loginAuditReasons[reason]
	return ok
}

// PostgreSQL 文本与 JSONB 不接受 NUL，统一规范 UTF-8 后再限长。
func truncateLoginAuditText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.ReplaceAll(value, "\x00", "")
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

// SanitizeLoginAuditEvent 统一限长归一化，须在写入或进入聚合状态前调用。
func SanitizeLoginAuditEvent(event LoginAuditEvent) LoginAuditEvent {
	event.Outcome = truncateLoginAuditText(event.Outcome, 32)
	event.Stage = truncateLoginAuditText(event.Stage, 32)
	event.Reason = truncateLoginAuditText(event.Reason, 64)
	event.LoginMethod = truncateLoginAuditText(event.LoginMethod, 64)
	event.IP = truncateLoginAuditText(event.IP, loginAuditMaxIPLength)
	event.UserAgent = truncateLoginAuditText(event.UserAgent, loginAuditMaxUserAgentLength)
	event.RequestId = truncateLoginAuditText(event.RequestId, loginAuditMaxRequestIdLength)
	event.RequestPath = truncateLoginAuditText(sanitizeLoginAuditRequestPath(event.RequestPath), loginAuditMaxRequestPathLength)
	event.CloudflareRay = truncateLoginAuditText(event.CloudflareRay, loginAuditMaxDiagnosticLength)
	event.PeerIP = truncateLoginAuditText(event.PeerIP, loginAuditMaxIPLength)
	event.TargetUsername = truncateLoginAuditText(event.TargetUsername, loginAuditMaxIdentifierLength)
	event.AttemptedIdentifier = truncateLoginAuditText(event.AttemptedIdentifier, loginAuditMaxIdentifierLength)
	if event.TargetUserId < 0 {
		event.TargetUserId = 0
	}
	if event.Count <= 0 {
		event.Count = 1
	}
	return event
}

func (event LoginAuditEvent) adminInfo() map[string]interface{} {
	adminInfo := make(map[string]interface{})
	if event.CloudflareRay != "" {
		adminInfo["cf_ray"] = event.CloudflareRay
	}
	if event.PeerIP != "" {
		adminInfo["peer_ip"] = event.PeerIP
	}
	if event.TargetUserId > 0 {
		adminInfo["target_user_id"] = event.TargetUserId
	}
	if event.TargetUsername != "" {
		adminInfo["target_username"] = event.TargetUsername
	}
	if event.AttemptedIdentifier != "" {
		adminInfo["attempted_identifier"] = event.AttemptedIdentifier
	}
	return adminInfo
}

func (event LoginAuditEvent) otherMap() map[string]interface{} {
	audit := map[string]interface{}{
		"outcome": event.Outcome,
		"stage":   event.Stage,
		"reason":  event.Reason,
		"count":   event.Count,
	}
	if event.WindowStart > 0 {
		audit["window_start"] = event.WindowStart
	}
	if event.WindowEnd > 0 {
		audit["window_end"] = event.WindowEnd
	}

	other := map[string]interface{}{
		"auth_audit": audit,
	}
	if event.LoginMethod != "" {
		other["login_method"] = event.LoginMethod
	}
	if event.RequestPath != "" {
		other["request_path"] = event.RequestPath
	}
	if adminInfo := event.adminInfo(); len(adminInfo) > 0 {
		other["admin_info"] = adminInfo
	}
	return other
}

// Validate 校验事件取值，避免把非法枚举或成功事件写进 type=8。
func (event LoginAuditEvent) Validate() error {
	if event.Outcome == LoginAuditOutcomeSuccess {
		return errors.New("login security audit must not record success")
	}
	if _, ok := loginAuditOutcomes[event.Outcome]; !ok {
		return fmt.Errorf("invalid login audit outcome %q", event.Outcome)
	}
	if _, ok := loginAuditStages[event.Stage]; !ok {
		return fmt.Errorf("invalid login audit stage %q", event.Stage)
	}
	if _, ok := loginAuditReasons[event.Reason]; !ok {
		return fmt.Errorf("invalid login audit reason %q", event.Reason)
	}
	return nil
}

// RecordLoginSecurityEvents 是唯一的 type=8 写入出口：同步逐条插入（各自 500ms 上限），
// ctx 结束即停止，返回已写入条数，失败条目由调用方做结构化备份。
func RecordLoginSecurityEvents(ctx context.Context, events []LoginAuditEvent) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	if LOG_DB == nil {
		return 0, errors.New("log database is not initialized")
	}
	persisted := 0
	for _, raw := range events {
		event := SanitizeLoginAuditEvent(raw)
		if err := event.Validate(); err != nil {
			return persisted, err
		}
		if err := ctx.Err(); err != nil {
			return persisted, err
		}
		insertCtx, cancel := context.WithTimeout(ctx, loginAuditInsertTimeout)
		err := LOG_DB.WithContext(insertCtx).Create(newLoginSecurityLog(event)).Error
		cancel()
		if err != nil {
			return persisted, err
		}
		persisted++
	}
	return persisted, nil
}

func newLoginSecurityLog(event LoginAuditEvent) *Log {
	createdAt := event.CreatedAt
	if createdAt <= 0 {
		createdAt = common.GetTimestamp()
	}
	log := &Log{
		UserId:    0,
		CreatedAt: createdAt,
		Type:      LogTypeLoginSecurity,
		Content:   loginAuditContent(event),
		Ip:        event.IP,
		UserAgent: event.UserAgent,
		RequestId: event.RequestId,
		Other:     common.MapToJsonStr(event.otherMap()),
	}
	ensureLogRequestId(log)
	return log
}

// loginAuditBackupPayload 是日志库不可用时的结构化备份字段白名单。
type loginAuditBackupPayload struct {
	Outcome             string `json:"outcome"`
	Stage               string `json:"stage"`
	Reason              string `json:"reason"`
	Count               int    `json:"count"`
	WindowStart         int64  `json:"window_start,omitempty"`
	WindowEnd           int64  `json:"window_end,omitempty"`
	CreatedAt           int64  `json:"created_at"`
	IP                  string `json:"ip,omitempty"`
	PeerIP              string `json:"peer_ip,omitempty"`
	UserAgent           string `json:"user_agent,omitempty"`
	RequestPath         string `json:"request_path,omitempty"`
	RequestId           string `json:"request_id,omitempty"`
	CloudflareRay       string `json:"cf_ray,omitempty"`
	LoginMethod         string `json:"login_method,omitempty"`
	TargetUserId        int    `json:"target_user_id,omitempty"`
	TargetUsername      string `json:"target_username,omitempty"`
	AttemptedIdentifier string `json:"attempted_identifier,omitempty"`
}

// LoginAuditEventBackupJSON 把事件渲染成可长期追查的结构化备份，只包含白名单
// 字段（无口令、密文、验证码、Cookie、Bearer、原始报错），由 common.Marshal
// 负责转义。
func LoginAuditEventBackupJSON(event LoginAuditEvent) (string, error) {
	event = SanitizeLoginAuditEvent(event)
	createdAt := event.CreatedAt
	if createdAt <= 0 {
		createdAt = common.GetTimestamp()
	}
	payload := loginAuditBackupPayload{
		Outcome:             event.Outcome,
		Stage:               event.Stage,
		Reason:              event.Reason,
		Count:               event.Count,
		WindowStart:         event.WindowStart,
		WindowEnd:           event.WindowEnd,
		CreatedAt:           createdAt,
		IP:                  event.IP,
		PeerIP:              event.PeerIP,
		UserAgent:           event.UserAgent,
		RequestPath:         event.RequestPath,
		RequestId:           event.RequestId,
		CloudflareRay:       event.CloudflareRay,
		LoginMethod:         event.LoginMethod,
		TargetUserId:        event.TargetUserId,
		TargetUsername:      event.TargetUsername,
		AttemptedIdentifier: event.AttemptedIdentifier,
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeleteOldLoginSecurityLogsBefore 清理过期的登录安全审计日志。
// 只删除 type=8，绝不能影响成功登录（type=7）与消费日志。
func DeleteOldLoginSecurityLogsBefore(cutoff int64) (int64, error) {
	if LOG_DB == nil {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		// ClickHouse 的 DELETE 是重写数据分区的 mutation，一次同步删完。
		var total int64
		if err := LOG_DB.WithContext(ctx).Model(&Log{}).
			Where("type = ? AND created_at < ?", LogTypeLoginSecurity, cutoff).
			Count(&total).Error; err != nil {
			return 0, err
		}
		if total == 0 {
			return 0, nil
		}
		if err := LOG_DB.WithContext(ctx).Exec(
			"ALTER TABLE logs DELETE WHERE type = ? AND created_at < ? SETTINGS mutations_sync = 1",
			LogTypeLoginSecurity, cutoff,
		).Error; err != nil {
			return 0, err
		}
		return total, nil
	}

	// 分页删除，避免长事务；先取主键再按主键删除，兼容不支持 DELETE ... LIMIT 的数据库。
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		var ids []int
		if err := LOG_DB.WithContext(ctx).Model(&Log{}).
			Where("type = ? AND created_at < ?", LogTypeLoginSecurity, cutoff).
			Limit(1000).Pluck("id", &ids).Error; err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}
		result := LOG_DB.WithContext(ctx).Where("id IN ?", ids).Delete(&Log{})
		if result.Error != nil {
			return total, result.Error
		}
		total += result.RowsAffected
	}
}
