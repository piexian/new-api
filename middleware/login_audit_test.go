package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	gorillasessions "github.com/gorilla/sessions"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLoginAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "logs.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.IPBan{}))

	previousDB, previousLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		clearLoginAuditAggregates()
		model.DB, model.LOG_DB = previousDB, previousLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func clearLoginAuditAggregates() {
	loginAuditAggMu.Lock()
	loginAuditAggBuckets = make(map[loginAuditAggKey]*loginAuditBucket)
	loginAuditOverflow = make(map[string]*loginAuditOverflowBucket)
	loginAuditOverflowTotal = 0
	loginAuditAggMu.Unlock()
}

// loginAuditFailingStore 只让会话保存失败：gin 的 session.Save 走
// gorilla Session.Save → store.Save，因此在这里返回错误即可。
type loginAuditFailingStore struct {
	sessions.Store
}

func (store loginAuditFailingStore) Get(_ *http.Request, name string) (*gorillasessions.Session, error) {
	return gorillasessions.NewSession(store, name), nil
}

func (store loginAuditFailingStore) New(_ *http.Request, name string) (*gorillasessions.Session, error) {
	return gorillasessions.NewSession(store, name), nil
}

func (store loginAuditFailingStore) Save(_ *http.Request, _ http.ResponseWriter, _ *gorillasessions.Session) error {
	return errors.New("session store unavailable")
}

// captureSysErrors 收集 common.SysError 的输出，用于断言结构化备份内容。
func captureSysErrors(t *testing.T) *safeBuffer {
	t.Helper()
	buffer := &safeBuffer{}
	previous := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = buffer
	t.Cleanup(func() { gin.DefaultErrorWriter = previous })
	return buffer
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func loginAuditLogRows(t *testing.T, db *gorm.DB) []model.Log {
	t.Helper()
	var rows []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeLoginSecurity).Order("id ASC").Find(&rows).Error)
	return rows
}

// countLoginAuditRows 不依赖 *testing.T，可安全用于 Eventually 的条件。
func countLoginAuditRows(db *gorm.DB) int64 {
	var total int64
	if err := db.Model(&model.Log{}).Where("type = ?", model.LogTypeLoginSecurity).Count(&total).Error; err != nil {
		return -1
	}
	return total
}

func loginAuditAuthAudit(t *testing.T, log model.Log) map[string]interface{} {
	t.Helper()
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	audit, ok := other["auth_audit"].(map[string]interface{})
	require.True(t, ok)
	return audit
}

func loginAuditOtherField(t *testing.T, log model.Log, key string) string {
	t.Helper()
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	value, _ := other[key].(string)
	return value
}

func loginAuditAdminInfo(t *testing.T, log model.Log) map[string]interface{} {
	t.Helper()
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	return adminInfo
}

func serveLoginAuditRequest(t *testing.T, router *gin.Engine, method string, path string, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	req.Header.Set("User-Agent", "LoginAuditTest/1.0")
	req.Header.Set("CF-Ray", "9ab12cd34ef56789")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestLoginAuditIgnoresNonLoginRequests(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/v1/chat/completions", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	router.GET("/api/user/login", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	router.POST("/api/user/login/2fa/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	serveLoginAuditRequest(t, router, http.MethodPost, "/v1/chat/completions", "203.0.113.30:1234")
	serveLoginAuditRequest(t, router, http.MethodGet, "/api/user/login", "203.0.113.30:1234")
	serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login/2fa/", "203.0.113.30:1234")

	require.Empty(t, loginAuditLogRows(t, db))
}

// 真实认证事件在请求结束前已同步落库，无需任何后台 flush。
func TestLoginAuditRecordsFailureSynchronouslyWithRequestCorrelation(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditIdentifier(c, strings.Repeat("u", 400))
		MarkLoginAuditTarget(c, 42, "target_user")
		MarkLoginAuditFailure(c, model.LoginAuditStageCredentials, model.LoginAuditReasonInvalidCredentials)
		// 后续标记与成功标记都不能覆盖首个结果，也不会产生第二条日志。
		MarkLoginAuditFailure(c, model.LoginAuditStageTwoFactor, model.LoginAuditReasonTwoFactorInvalid)
		MarkLoginAuditSuccess(c)
		c.JSON(http.StatusOK, gin.H{"success": false})
	})

	recorder := serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.31:1234")
	require.Equal(t, http.StatusOK, recorder.Code)
	requestId := recorder.Header().Get(common.RequestIdKey)
	require.NotEmpty(t, requestId)

	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 1)

	log := rows[0]
	require.Equal(t, 0, log.UserId)
	require.Equal(t, requestId, log.RequestId)
	require.Equal(t, "203.0.113.31", log.Ip)
	require.Equal(t, "LoginAuditTest/1.0", log.UserAgent)

	audit := loginAuditAuthAudit(t, log)
	require.Equal(t, "failure", audit["outcome"])
	require.Equal(t, "credentials", audit["stage"])
	require.Equal(t, "invalid_credentials", audit["reason"])
	require.EqualValues(t, 1, audit["count"])

	adminInfo := loginAuditAdminInfo(t, log)
	require.Equal(t, float64(42), adminInfo["target_user_id"])
	require.Equal(t, "target_user", adminInfo["target_username"])
	require.Equal(t, "9ab12cd34ef56789", adminInfo["cf_ray"])
	require.Equal(t, "203.0.113.31", adminInfo["peer_ip"])
	require.LessOrEqual(t, len([]rune(adminInfo["attempted_identifier"].(string))), 128)
	require.LessOrEqual(t, len([]rune(log.UserAgent)), 512)

	require.Equal(t, "/api/user/login", loginAuditOtherField(t, log, "request_path"))
}

// 限长与事件时间在写入前完成，落库行与内存事件一致。
func TestLoginAuditLimitsFieldsBeforePersist(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditIdentifier(c, strings.Repeat("u", 400))
		MarkLoginAuditFailure(c, model.LoginAuditStageCredentials, model.LoginAuditReasonUserNotFound)
		c.JSON(http.StatusOK, gin.H{"success": false})
	})

	before := time.Now().Unix()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login?turnstile=secret", nil)
	req.RemoteAddr = "203.0.113.92:1234"
	req.Header.Set("User-Agent", "Agent/1.0 "+strings.Repeat("z", 4000))
	router.ServeHTTP(httptest.NewRecorder(), req)

	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 1)
	require.LessOrEqual(t, len([]rune(rows[0].UserAgent)), 512)
	require.InDelta(t, before, rows[0].CreatedAt, 5)

	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(rows[0].Other, &other))
	require.Equal(t, "/api/user/login", other["request_path"])
	require.NotContains(t, rows[0].Other, "secret")
	adminInfo := loginAuditAdminInfo(t, rows[0])
	require.LessOrEqual(t, len([]rune(adminInfo["attempted_identifier"].(string))), 128)
}

func TestLoginAuditPrecheckAggregationKeepsExactCounts(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditBlocked(c, model.LoginAuditStagePrecheck, model.LoginAuditReasonRateLimited)
		c.String(http.StatusTooManyRequests, "blocked")
	})

	// 首条立即落库，随后 3 条只累加计数。
	for i := 0; i < 4; i++ {
		recorder := serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.32:1234")
		require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	}

	require.Len(t, loginAuditLogRows(t, db), 1)
	FlushLoginAudit()
	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 2)

	first := loginAuditAuthAudit(t, rows[0])
	second := loginAuditAuthAudit(t, rows[1])
	require.Equal(t, "blocked", first["outcome"])
	require.Equal(t, "rate_limited", first["reason"])
	require.EqualValues(t, 1, first["count"])
	require.EqualValues(t, 3, second["count"])
	require.Equal(t, first["window_start"], second["window_start"])
	require.Equal(t, first["window_end"], second["window_end"])

	// 再次 flush 不会重复写入。
	FlushLoginAudit()
	require.Len(t, loginAuditLogRows(t, db), 2)
}

// 聚合状态满后命中进入按原因归并的溢出桶：状态数量有界，但计数守恒且可查。
func TestLoginAuditAggregateOverflowKeepsCountsConserved(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	previousLimit := loginAuditBucketLimit
	loginAuditBucketLimit = 2
	t.Cleanup(func() { loginAuditBucketLimit = previousLimit })

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditBlocked(c, model.LoginAuditStagePrecheck, model.LoginAuditReasonIPBanned)
		c.String(http.StatusForbidden, "blocked")
	})

	ips := []string{"203.0.113.41", "203.0.113.42", "203.0.113.43", "203.0.113.44", "203.0.113.45"}
	for _, ip := range ips {
		serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", ip+":1234")
	}

	loginAuditAggMu.Lock()
	bucketCount := len(loginAuditAggBuckets)
	overflowCount := loginAuditOverflowTotal
	loginAuditAggMu.Unlock()
	require.Equal(t, 2, bucketCount)
	require.Equal(t, int64(len(ips)-2), overflowCount)

	FlushLoginAudit()
	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 3)

	var totalHits int
	for _, row := range rows {
		audit := loginAuditAuthAudit(t, row)
		require.Equal(t, "blocked", audit["outcome"])
		require.Equal(t, "ip_banned", audit["reason"])
		totalHits += int(audit["count"].(float64))
	}
	require.Equal(t, len(ips), totalHits)

	// 溢出桶只按原因归并，不保存每个溢出 IP。
	overflowRow := rows[2]
	require.Empty(t, overflowRow.Ip)
	require.NotContains(t, overflowRow.Other, "203.0.113.43")
}

// 慢成功多次后遇上整体 deadline：已插入的部分落库，剩余计数全部进入完整白名单备份。
func TestLoginAuditFlushTimeoutBacksUpRemainingCounts(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()
	logs := captureSysErrors(t)

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditBlocked(c, model.LoginAuditStagePrecheck, model.LoginAuditReasonIPBanned)
		c.String(http.StatusForbidden, "blocked")
	})

	// 10 个文档 IP 各命中 3 次：首条立即落库，其余形成 count=2 的 pending 计数。
	const hitsPerIP = 3
	ips := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		ips = append(ips, fmt.Sprintf("203.0.113.%d", 51+i))
	}
	for _, ip := range ips {
		for hit := 0; hit < hitsPerIP; hit++ {
			serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", ip+":1234")
		}
	}
	initialRows := len(ips)
	require.Len(t, loginAuditLogRows(t, db), initialRows)

	// 每次插入先等 300ms 再真正写入；整体 deadline 到期后的插入直接失败。
	const slowCreate = "test:slow_login_audit_create"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(slowCreate, func(tx *gorm.DB) {
		select {
		case <-tx.Statement.Context.Done():
			_ = tx.AddError(tx.Statement.Context.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(slowCreate) })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	FlushLoginAuditContext(ctx)
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 1500*time.Millisecond, "flush must run into the overall deadline")
	require.Less(t, elapsed, 3*time.Second, "flush must not exceed the overall budget")

	// 聚合状态已被取空，不会重复落库。
	loginAuditAggMu.Lock()
	pending := 0
	for _, bucket := range loginAuditAggBuckets {
		pending += bucket.Pending
	}
	loginAuditAggMu.Unlock()
	require.Equal(t, 0, pending)

	rows := loginAuditLogRows(t, db)
	require.Greater(t, len(rows), initialRows)
	require.Less(t, len(rows), 2*initialRows)

	// 落库部分累加 auth_audit.count（首条为 1、聚合的 pending 为 2），
	// 剩余部分逐条解析备份 JSON 的 count。
	dbCount := 0
	for _, row := range rows {
		dbCount += int(loginAuditAuthAudit(t, row)["count"].(float64))
	}

	output := logs.String()
	backupPayloads := loginAuditBackupPayloads(output)
	require.NotEmpty(t, backupPayloads, "remaining counts must be backed up")
	backupCount := 0
	for _, payload := range backupPayloads {
		count, ok := payload["count"].(float64)
		require.True(t, ok, payload)
		backupCount += int(count)
		// 备份保留完整白名单字段与 count=2 的聚合计数。
		require.EqualValues(t, 2, count)
		require.Equal(t, "blocked", payload["outcome"])
		require.Equal(t, "precheck", payload["stage"])
		require.Equal(t, "ip_banned", payload["reason"])
		require.NotZero(t, payload["created_at"])
		require.NotEmpty(t, payload["request_path"])
		require.NotEmpty(t, payload["login_method"])
	}

	// 两种途径的计数之和等于总命中数。
	require.Equal(t, initialRows*hitsPerIP, dbCount+backupCount)
}

// loginAuditBackupPayloads 解析 common.SysError 输出里的白名单备份 JSON。
func loginAuditBackupPayloads(output string) []map[string]interface{} {
	const marker = "login audit event backup: "
	payloads := make([]map[string]interface{}, 0, 8)
	for _, line := range strings.Split(output, "\n") {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		var payload map[string]interface{}
		if err := common.UnmarshalJsonStr(strings.TrimSpace(line[index+len(marker):]), &payload); err != nil {
			continue
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func TestLoginAuditLogFailureDoesNotAffectResponse(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()
	logs := captureSysErrors(t)

	// 日志表不可用时写入失败，但认证响应不受影响。
	require.NoError(t, db.Migrator().DropTable(&model.Log{}))

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditTarget(c, 7, "blocked_target")
		MarkLoginAuditFailure(c, model.LoginAuditStageCredentials, model.LoginAuditReasonInvalidCredentials)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid credentials"})
	})

	recorder := serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.81:1234")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "invalid credentials")

	// 失败事件被完整备份，可用于追查。
	output := logs.String()
	require.Contains(t, output, `"reason":"invalid_credentials"`)
	require.Contains(t, output, `"target_user_id":7`)
	require.Contains(t, output, `"target_username":"blocked_target"`)
	require.Contains(t, output, `"ip":"203.0.113.81"`)
	require.Contains(t, output, `"request_path":"/api/user/login"`)
	require.False(t, db.Migrator().HasTable(&model.Log{}))
}

// handler 内部 panic 时，defer 收尾会在外层 Recovery 之前留下 error 事件，
// 且中间件自身不 recover、异常继续上抛。
func TestLoginAuditRecordsErrorEventWhenHandlerPanics(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	router := gin.New()
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.String(http.StatusInternalServerError, "recovered")
	}))
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		panic("boom in login handler")
	})

	recorder := serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.91:1234")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, recorder.Body.String(), "recovered")

	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditAuthAudit(t, rows[0])
	require.Equal(t, "error", audit["outcome"])
	require.Equal(t, "session", audit["stage"])
	require.Equal(t, "internal_error", audit["reason"])
}

func TestLoginAuditRateLimitMiddlewareMarksBlocked(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	previousEnable := common.CriticalRateLimitEnable
	previousNum := common.CriticalRateLimitNum
	previousDuration := common.CriticalRateLimitDuration
	previousRedis := common.RedisEnabled
	t.Cleanup(func() {
		common.CriticalRateLimitEnable = previousEnable
		common.CriticalRateLimitNum = previousNum
		common.CriticalRateLimitDuration = previousDuration
		common.RedisEnabled = previousRedis
	})
	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 1
	common.CriticalRateLimitDuration = 60
	common.RedisEnabled = false

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", CriticalRateLimit(), func(c *gin.Context) {
		MarkLoginAuditFailure(c, model.LoginAuditStageCredentials, model.LoginAuditReasonInvalidCredentials)
		c.JSON(http.StatusOK, gin.H{"success": false})
	})

	serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.61:1234")
	recorder := serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.61:1234")
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)

	FlushLoginAudit()
	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 2)
	require.Equal(t, "failure", loginAuditAuthAudit(t, rows[0])["outcome"])
	blocked := loginAuditAuthAudit(t, rows[1])
	require.Equal(t, "blocked", blocked["outcome"])
	require.Equal(t, "precheck", blocked["stage"])
	require.Equal(t, "rate_limited", blocked["reason"])
}

// Redis 限流器的拒绝与故障都要留下 blocked / error 事件。
func TestLoginAuditRedisRateLimitRejectAndFailure(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	previousEnable := common.CriticalRateLimitEnable
	previousNum := common.CriticalRateLimitNum
	previousDuration := common.CriticalRateLimitDuration
	t.Cleanup(func() {
		common.CriticalRateLimitEnable = previousEnable
		common.CriticalRateLimitNum = previousNum
		common.CriticalRateLimitDuration = previousDuration
	})

	redisServer := useRateLimitTestRedis(t)
	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 1
	common.CriticalRateLimitDuration = 60

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", CriticalRateLimit(), func(c *gin.Context) {
		// 该请求已通过限流：显式标记成功，避免落成 internal_error。
		MarkLoginAuditSuccess(c)
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.62:1234")
	recorder := serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.62:1234")
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)

	blockedRows := loginAuditLogRows(t, db)
	require.Len(t, blockedRows, 1)
	blocked := loginAuditAuthAudit(t, blockedRows[0])
	require.Equal(t, "blocked", blocked["outcome"])
	require.Equal(t, "precheck", blocked["stage"])
	require.Equal(t, "rate_limited", blocked["reason"])

	// Redis 不可用 → error 而不是 blocked。
	redisServer.Close()
	clearLoginAuditAggregates()
	require.NoError(t, db.Exec("DELETE FROM logs").Error)

	recorder = serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.63:1234")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)

	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditAuthAudit(t, rows[0])
	require.Equal(t, "error", audit["outcome"])
	require.Equal(t, "precheck", audit["stage"])
	require.Equal(t, "internal_error", audit["reason"])
}

func TestLoginAuditIPBanMiddlewareMarksBlocked(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	require.NoError(t, model.CreateIPBan(&model.IPBan{Target: "203.0.113.71", Reason: "abuse"}))
	model.InitIPBanCache()
	t.Cleanup(func() {
		model.DB.Where("target = ?", "203.0.113.71").Delete(&model.IPBan{})
		model.InitIPBanCache()
	})

	router := gin.New()
	router.Use(RequestId(), LoginAudit(), IPBan())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditSuccess(c)
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.71:1234")
	require.Equal(t, http.StatusForbidden, recorder.Code)

	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditAuthAudit(t, rows[0])
	require.Equal(t, "blocked", audit["outcome"])
	require.Equal(t, "ip_banned", audit["reason"])
}

// Turnstile 的四种结果：缺少令牌(blocked)、校验失败(blocked)、网络故障(error)、
// 会话保存失败(error)。用 FakeRoundTripper 注入，不访问外网。
func TestLoginAuditTurnstileOutcomes(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	previousEnabled := common.TurnstileLoginEnabled
	previousSecret := common.TurnstileSecretKey
	previousClient := turnstileSiteVerifyClient
	previousDirect := turnstileDirectSiteVerifyClient
	common.TurnstileLoginEnabled = true
	common.TurnstileSecretKey = "test-secret"
	t.Cleanup(func() {
		common.TurnstileLoginEnabled = previousEnabled
		common.TurnstileSecretKey = previousSecret
		turnstileSiteVerifyClient = previousClient
		turnstileDirectSiteVerifyClient = previousDirect
		clearLoginAuditAggregates()
	})

	newRouter := func() *gin.Engine {
		router := gin.New()
		router.Use(RequestId(), LoginAudit())
		router.POST("/api/user/login", TurnstileCheckNoSessionForScope(TurnstileScopeLogin), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"success": true})
		})
		return router
	}

	setSiteVerifyResponse := func(body string, err error) {
		client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		})}
		turnstileSiteVerifyClient = client
		turnstileDirectSiteVerifyClient = client
	}

	// 1) 缺少令牌 → blocked/captcha_missing
	serveLoginAuditRequest(t, newRouter(), http.MethodPost, "/api/user/login", "203.0.113.81:1234")

	// 2) 上游判定失败 → blocked/captcha_invalid
	clearLoginAuditAggregates()
	setSiteVerifyResponse(`{"success":false}`, nil)
	serveLoginAuditRequest(t, newRouter(), http.MethodPost, "/api/user/login?turnstile=token", "203.0.113.82:1234")

	// 3) 网络故障 → error/captcha_error
	clearLoginAuditAggregates()
	setSiteVerifyResponse("", errors.New("siteverify unreachable"))
	serveLoginAuditRequest(t, newRouter(), http.MethodPost, "/api/user/login?turnstile=token", "203.0.113.83:1234")

	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 3)

	missing := loginAuditAuthAudit(t, rows[0])
	require.Equal(t, "blocked", missing["outcome"])
	require.Equal(t, "captcha", missing["stage"])
	require.Equal(t, "captcha_missing", missing["reason"])

	invalid := loginAuditAuthAudit(t, rows[1])
	require.Equal(t, "blocked", invalid["outcome"])
	require.Equal(t, "captcha_invalid", invalid["reason"])

	network := loginAuditAuthAudit(t, rows[2])
	require.Equal(t, "error", network["outcome"])
	require.Equal(t, "captcha", network["stage"])
	require.Equal(t, "captcha_error", network["reason"])
}

// 验证通过但会话保存失败 → error/session_save_failed，不是 blocked。
func TestLoginAuditTurnstileSessionSaveFailureIsError(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	previousEnabled := common.TurnstileLoginEnabled
	previousSecret := common.TurnstileSecretKey
	previousClient := turnstileSiteVerifyClient
	previousDirect := turnstileDirectSiteVerifyClient
	common.TurnstileLoginEnabled = true
	common.TurnstileSecretKey = "test-secret"
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	turnstileSiteVerifyClient = client
	turnstileDirectSiteVerifyClient = client
	t.Cleanup(func() {
		common.TurnstileLoginEnabled = previousEnabled
		common.TurnstileSecretKey = previousSecret
		turnstileSiteVerifyClient = previousClient
		turnstileDirectSiteVerifyClient = previousDirect
	})

	store := cookie.NewStore([]byte("login-audit-turnstile"))
	router := gin.New()
	router.Use(RequestId(), LoginAudit(), sessions.Sessions("session", loginAuditFailingStore{Store: store}))
	router.POST("/api/user/login", TurnstileCheckForScope(TurnstileScopeLogin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login?turnstile=token", "203.0.113.84:1234")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)

	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 1)
	audit := loginAuditAuthAudit(t, rows[0])
	require.Equal(t, "error", audit["outcome"])
	require.Equal(t, "captcha", audit["stage"])
	require.Equal(t, "session_save_failed", audit["reason"])
}

// 验证码前置拒绝与 IPBan/限流一样按预拦截策略聚合，真实口令失败仍逐条。
func TestLoginAuditCaptchaRejectionsAreAggregated(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	previousEnabled := common.TurnstileLoginEnabled
	common.TurnstileLoginEnabled = true
	t.Cleanup(func() { common.TurnstileLoginEnabled = previousEnabled })

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", TurnstileCheckNoSessionForScope(TurnstileScopeLogin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// 同一 IP、同一原因、同一分钟窗口连打 3 次。
	for i := 0; i < 3; i++ {
		serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.93:1234")
	}

	FlushLoginAudit()
	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 2)
	first := loginAuditAuthAudit(t, rows[0])
	second := loginAuditAuthAudit(t, rows[1])
	require.Equal(t, "captcha", first["stage"])
	require.Equal(t, "captcha_missing", first["reason"])
	require.EqualValues(t, 1, first["count"])
	require.EqualValues(t, 2, second["count"])
}

// 周期任务由显式启动/停止控制，测试不会留下永驻 goroutine。
func TestLoginAuditFlusherLifecycle(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditBlocked(c, model.LoginAuditStagePrecheck, model.LoginAuditReasonIPBanned)
		c.String(http.StatusForbidden, "blocked")
	})

	serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.95:1234")
	serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.95:1234")
	require.Len(t, loginAuditLogRows(t, db), 1)

	StartLoginAuditFlusher(20 * time.Millisecond)
	t.Cleanup(StopLoginAuditFlusher)

	require.Eventually(t, func() bool {
		return countLoginAuditRows(db) == 2
	}, 2*time.Second, 20*time.Millisecond)

	StopLoginAuditFlusher()
	// 停止后再有命中不会被自动落库，只能靠显式 flush。
	serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.96:1234")
	time.Sleep(80 * time.Millisecond)
	loginAuditAggMu.Lock()
	pendingBuckets := len(loginAuditAggBuckets)
	loginAuditAggMu.Unlock()
	require.GreaterOrEqual(t, pendingBuckets, 1)
}

// 周期批次进行中退出：Stop 必须等该批次写完或转备份后才返回，计数不丢。
func TestLoginAuditStopWaitsForInFlightPeriodicFlush(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()
	logs := captureSysErrors(t)

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditBlocked(c, model.LoginAuditStagePrecheck, model.LoginAuditReasonIPBanned)
		c.String(http.StatusForbidden, "blocked")
	})

	// 首条立即落库，随后两次命中形成 count=2 的 pending 计数。
	for hit := 0; hit < 3; hit++ {
		serveLoginAuditRequest(t, router, http.MethodPost, "/api/user/login", "203.0.113.97:1234")
	}
	require.Len(t, loginAuditLogRows(t, db), 1)

	entered := make(chan struct{})
	release := make(chan struct{})
	createFinished := make(chan struct{})
	var blockedOnce sync.Once
	const blockingCreate = "test:block_periodic_login_audit_create"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(blockingCreate, func(tx *gorm.DB) {
		// 只阻塞周期批次里的这次写入，让 Stop 与进行中的批次重叠。
		blockedOnce.Do(func() {
			defer close(createFinished)
			close(entered)
			select {
			case <-release:
			case <-tx.Statement.Context.Done():
			}
		})
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(blockingCreate) })

	StartLoginAuditFlusher(20 * time.Millisecond)
	t.Cleanup(StopLoginAuditFlusher)

	<-entered // 周期批次已进入写入阶段
	stopped := make(chan struct{})
	stopStarted := time.Now()
	go func() {
		StopLoginAuditFlusher()
		close(stopped)
	}()

	// Stop 应等待进行中的批次：要么放开写入让它落库，要么由 deadline 取消。
	go func() {
		select {
		case <-time.After(300 * time.Millisecond):
			close(release)
		case <-stopped:
		}
	}()
	<-stopped
	select {
	case <-createFinished:
	default:
		t.Fatal("stop returned before the in-flight write completed")
	}
	require.Less(t, time.Since(stopStarted), 3*time.Second, "stop must stay within the flush budget")

	// Stop 返回时该批次已完成：聚合状态清空，落库行 + 备份计数 = 命中总数。
	loginAuditAggMu.Lock()
	pending := 0
	for _, bucket := range loginAuditAggBuckets {
		pending += bucket.Pending
	}
	overflowPending := len(loginAuditOverflow)
	loginAuditAggMu.Unlock()
	require.Equal(t, 0, pending)
	require.Equal(t, 0, overflowPending)

	dbCount := 0
	for _, row := range loginAuditLogRows(t, db) {
		dbCount += int(loginAuditAuthAudit(t, row)["count"].(float64))
	}
	backupCount := 0
	for _, payload := range loginAuditBackupPayloads(logs.String()) {
		backupCount += int(payload["count"].(float64))
	}
	require.Equal(t, 3, dbCount+backupCount)
}

// 已知目标的前置拦截必须逐条保留：同一 IP、同一分钟、同一原因也不会并进同一个桶。
func TestLoginAuditKeepsSeparateEventsForKnownTargets(t *testing.T) {
	db := setupLoginAuditTestDB(t)
	clearLoginAuditAggregates()

	router := gin.New()
	router.Use(RequestId(), LoginAudit())
	router.POST("/api/oauth/ownership/confirm", func(c *gin.Context) {
		// 共享登录出口拒绝时按需创建状态并带上已知目标。
		EnsureLoginAuditState(c, "oauth_ownership_transfer")
		targetID, _ := strconv.Atoi(c.Query("uid"))
		MarkLoginAuditTarget(c, targetID, c.Query("target"))
		MarkLoginAuditBlocked(c, model.LoginAuditStagePrecheck, model.LoginAuditReasonUserAgentMissing)
		c.String(http.StatusOK, "blocked")
	})

	for _, target := range []struct {
		uid  string
		name string
	}{{"41", "target_a"}, {"42", "target_b"}} {
		req := httptest.NewRequest(http.MethodPost, "/api/oauth/ownership/confirm?uid="+target.uid+"&target="+target.name, nil)
		req.RemoteAddr = "203.0.113.97:1234"
		router.ServeHTTP(httptest.NewRecorder(), req)
	}

	rows := loginAuditLogRows(t, db)
	require.Len(t, rows, 2)
	first := loginAuditAdminInfo(t, rows[0])
	second := loginAuditAdminInfo(t, rows[1])
	require.Equal(t, float64(41), first["target_user_id"])
	require.Equal(t, float64(42), second["target_user_id"])
	require.EqualValues(t, 1, loginAuditAuthAudit(t, rows[0])["count"])
	require.EqualValues(t, 1, loginAuditAuthAudit(t, rows[1])["count"])
	require.Equal(t, "oauth_ownership_transfer", loginAuditOtherField(t, rows[0], "login_method"))

	// 未知目标的前置拦截仍按 IP+入口+原因+分钟窗口聚合。
	clearLoginAuditAggregates()
	require.NoError(t, db.Exec("DELETE FROM logs").Error)
	unknownRouter := gin.New()
	unknownRouter.Use(RequestId(), LoginAudit())
	unknownRouter.POST("/api/user/login", func(c *gin.Context) {
		MarkLoginAuditBlocked(c, model.LoginAuditStagePrecheck, model.LoginAuditReasonUserAgentMissing)
		c.String(http.StatusOK, "blocked")
	})
	for i := 0; i < 3; i++ {
		serveLoginAuditRequest(t, unknownRouter, http.MethodPost, "/api/user/login", "203.0.113.98:1234")
	}
	FlushLoginAudit()
	unknownRows := loginAuditLogRows(t, db)
	require.Len(t, unknownRows, 2)
	require.EqualValues(t, 1, loginAuditAuthAudit(t, unknownRows[0])["count"])
	require.EqualValues(t, 2, loginAuditAuthAudit(t, unknownRows[1])["count"])
}
