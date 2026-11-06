package model

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLoginSecurityLogDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "logs.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	previousLogDB := LOG_DB
	previousLogDatabaseType := common.LogDatabaseType()
	LOG_DB = db
	// 登录 scope 的账号筛选会按方言生成 JSON 提取表达式，这里固定为 SQLite。
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		common.SetLogDatabaseType(previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func countLoginSecurityLogs(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var total int64
	require.NoError(t, db.Model(&Log{}).Where("type = ?", LogTypeLoginSecurity).Count(&total).Error)
	return total
}

func TestRecordLoginSecurityEventWritesType8WithAdminInfo(t *testing.T) {
	db := setupLoginSecurityLogDB(t)

	event := LoginAuditEvent{
		Outcome:             LoginAuditOutcomeFailure,
		Stage:               LoginAuditStageCredentials,
		Reason:              LoginAuditReasonInvalidCredentials,
		LoginMethod:         "password",
		IP:                  "203.0.113.9",
		UserAgent:           strings.Repeat("A", 900),
		RequestId:           "req-login-audit-1",
		CloudflareRay:       "8f1c9b0a1b2c3d4e",
		PeerIP:              "10.1.2.3",
		TargetUserId:        42,
		TargetUsername:      "target_user",
		AttemptedIdentifier: "target_user@example.com",
		RequestPath:         "/api/user/login?turnstile=secret-token",
	}
	_, err := RecordLoginSecurityEvents(context.Background(), []LoginAuditEvent{event})
	require.NoError(t, err)

	var log Log
	require.NoError(t, db.Where("type = ?", LogTypeLoginSecurity).First(&log).Error)
	require.Equal(t, 0, log.UserId)
	require.Empty(t, log.Username)
	require.Equal(t, "203.0.113.9", log.Ip)
	require.Equal(t, "req-login-audit-1", log.RequestId)
	require.LessOrEqual(t, len([]rune(log.UserAgent)), 512)
	require.Contains(t, log.Content, "账号或密码错误")

	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))

	audit, ok := other["auth_audit"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "failure", audit["outcome"])
	require.Equal(t, "credentials", audit["stage"])
	require.Equal(t, "invalid_credentials", audit["reason"])
	require.EqualValues(t, 1, audit["count"])

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(42), adminInfo["target_user_id"])
	require.Equal(t, "target_user", adminInfo["target_username"])
	require.Equal(t, "target_user@example.com", adminInfo["attempted_identifier"])
	require.Equal(t, "8f1c9b0a1b2c3d4e", adminInfo["cf_ray"])
	require.Equal(t, "10.1.2.3", adminInfo["peer_ip"])
	// 请求路径只保留路径本身，query 不会落库。
	require.Equal(t, "/api/user/login", other["request_path"])

	// 敏感字段绝不落库：事件结构里没有口令、验证码、Cookie 或 Bearer 字段，
	// 请求路径的 query 也不会被写入。
	for _, forbidden := range []string{"password", "password_encrypted", "turnstile", "cookie", "authorization", "bearer", "verification_code"} {
		require.NotContains(t, other, forbidden)
	}
	require.NotContains(t, log.Other, "secret-token")
}

func TestRecordLoginSecurityEventRejectsSuccessAndUnknownValues(t *testing.T) {
	db := setupLoginSecurityLogDB(t)

	_, err := RecordLoginSecurityEvents(context.Background(), []LoginAuditEvent{{
		Outcome: LoginAuditOutcomeSuccess,
		Stage:   LoginAuditStageSession,
		Reason:  LoginAuditReasonInternalError,
	}})
	require.Error(t, err)
	_, err = RecordLoginSecurityEvents(context.Background(), []LoginAuditEvent{{
		Outcome: LoginAuditOutcomeFailure,
		Stage:   LoginAuditStageCredentials,
		Reason:  "made_up_reason",
	}})
	require.Error(t, err)
	require.EqualValues(t, 0, countLoginSecurityLogs(t, db))

	// 预拦截汇总事件携带窗口与真实命中次数。
	_, err = RecordLoginSecurityEvents(context.Background(), []LoginAuditEvent{{
		Outcome:     LoginAuditOutcomeBlocked,
		Stage:       LoginAuditStagePrecheck,
		Reason:      LoginAuditReasonRateLimited,
		Count:       7,
		WindowStart: 1700000000,
		WindowEnd:   1700000059,
		IP:          "198.51.100.7",
	}})
	require.NoError(t, err)
	var log Log
	require.NoError(t, db.Where("type = ?", LogTypeLoginSecurity).First(&log).Error)
	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	audit := other["auth_audit"].(map[string]interface{})
	require.EqualValues(t, 7, audit["count"])
	require.EqualValues(t, 1700000000, audit["window_start"])
	require.EqualValues(t, 1700000059, audit["window_end"])
	require.Contains(t, log.Content, "请求过于频繁")
}

// 账号筛选的字段提取表达式必须只指向 admin_info 的字段值，且不含跨字段的
// 百分号拼接；各受支持方言都要有对应写法。
func TestLoginAuditAdminFieldExprPerDialect(t *testing.T) {
	previous := common.LogDatabaseType()
	t.Cleanup(func() { common.SetLogDatabaseType(previous) })

	cases := []struct {
		dialect       common.DatabaseType
		wantFragments []string
	}{
		{common.DatabaseTypeSQLite, []string{"json_extract", "$.admin_info.target_username", "logs.type ="}},
		{common.DatabaseTypeMySQL, []string{"JSON_EXTRACT", "JSON_VALID", "$.admin_info.target_username"}},
		{common.DatabaseTypePostgreSQL, []string{"::jsonb #>>", "{admin_info,target_username}"}},
		{common.DatabaseTypeClickHouse, []string{"JSONExtractString", "'admin_info'", "'target_username'"}},
	}
	for _, tc := range cases {
		common.SetLogDatabaseType(tc.dialect)
		expr := loginAuditAdminFieldExpr("target_username")
		for _, fragment := range tc.wantFragments {
			require.Contains(t, expr, fragment, "dialect %s", tc.dialect)
		}
		require.NotContains(t, expr, "%target_username%")
	}
}

func TestValidateLogScope(t *testing.T) {
	require.NoError(t, ValidateLogScope(""))
	require.NoError(t, ValidateLogScope(LogScopeLogin))
	require.Error(t, ValidateLogScope("consume"))
	require.Error(t, ValidateLogScope("LOGIN"))
}

func insertLoginAuditFixtureLogs(t *testing.T, db *gorm.DB) (now int64) {
	t.Helper()
	now = common.GetTimestamp()
	rows := []*Log{
		{UserId: 7, Username: "self_user", CreatedAt: now - 30, Type: LogTypeLogin, Content: "登录成功"},
		{UserId: 8, Username: "other_user", CreatedAt: now - 20, Type: LogTypeLogin, Content: "登录成功"},
		{UserId: 0, CreatedAt: now - 10, Type: LogTypeLoginSecurity, Content: "登录失败：账号或密码错误", Ip: "203.0.113.9"},
		{UserId: 7, Username: "self_user", CreatedAt: now - 5, Type: LogTypeConsume, Content: "消费"},
	}
	for _, row := range rows {
		require.NoError(t, db.Create(row).Error)
	}
	return now
}

func TestGetAllLogsLoginScopeReturnsOnlyLoginEvents(t *testing.T) {
	db := setupLoginSecurityLogDB(t)
	insertLoginAuditFixtureLogs(t, db)

	logs, total, err := GetAllLogs(LogTypeUnknown, LogScopeLogin, 0, 0, "", "", "", 0, 50, 0, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, logs, 3)
	for _, log := range logs {
		require.Contains(t, []int{LogTypeLogin, LogTypeLoginSecurity}, log.Type)
	}

	// scope 与 type 筛选取交集：type=2 落在登录审计视图之外。
	logs, total, err = GetAllLogs(LogTypeConsume, LogScopeLogin, 0, 0, "", "", "", 0, 50, 0, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, logs)

	// 缺省 scope 保持原查询语义。
	_, total, err = GetAllLogs(LogTypeUnknown, "", 0, 0, "", "", "", 0, 50, 0, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
}

func TestGetUserLogsLoginScopeKeepsFailuresHidden(t *testing.T) {
	db := setupLoginSecurityLogDB(t)
	insertLoginAuditFixtureLogs(t, db)

	logs, total, err := GetUserLogs(7, LogTypeUnknown, LogScopeLogin, 0, 0, "", "", 0, 50, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, 7, logs[0].UserId)
	require.Equal(t, LogTypeLogin, logs[0].Type)

	// 其他用户与失败事件都不会出现在自助视图里。
	_, total, err = GetUserLogs(8, LogTypeUnknown, LogScopeLogin, 0, 0, "", "", 0, 50, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
}

func TestDeleteOldLoginSecurityLogsBeforeOnlyRemovesType8(t *testing.T) {
	db := setupLoginSecurityLogDB(t)
	now := common.GetTimestamp()
	cutoff := now - 90*86400

	old := []*Log{
		{UserId: 0, CreatedAt: cutoff - 60, Type: LogTypeLoginSecurity, Content: "登录失败：账号或密码错误"},
		{UserId: 9, CreatedAt: cutoff - 60, Type: LogTypeLogin, Content: "登录成功"},
		{UserId: 9, CreatedAt: cutoff - 60, Type: LogTypeConsume, Content: "消费"},
	}
	recent := &Log{UserId: 0, CreatedAt: now - 60, Type: LogTypeLoginSecurity, Content: "登录失败：账号或密码错误"}
	for _, row := range old {
		require.NoError(t, db.Create(row).Error)
	}
	require.NoError(t, db.Create(recent).Error)

	deleted, err := DeleteOldLoginSecurityLogsBefore(cutoff)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	// 只删掉过期的 type=8；其余行按主键集合逐一核对，不依赖结果顺序。
	var remaining []Log
	require.NoError(t, db.Find(&remaining).Error)
	remainingIds := make(map[int]bool, len(remaining))
	for _, row := range remaining {
		remainingIds[row.Id] = true
	}
	require.False(t, remainingIds[old[0].Id], "expired type=8 must be deleted")
	require.True(t, remainingIds[old[1].Id], "old type=7 login log must be kept")
	require.True(t, remainingIds[old[2].Id], "old type=2 consume log must be kept")
	require.True(t, remainingIds[recent.Id], "recent type=8 log must be kept")
	require.Len(t, remaining, 3)

	// 保留下来的新事件仍是 type=8，且时间戳未被清理改动。
	var keptRecent Log
	require.NoError(t, db.Where("id = ?", recent.Id).First(&keptRecent).Error)
	require.Equal(t, LogTypeLoginSecurity, keptRecent.Type)
	require.Equal(t, recent.CreatedAt, keptRecent.CreatedAt)
}

// 登录审计视图的账号筛选必须同时命中失败事件记录的目标账号与输入账号，
// 这些事件的 user_id 恒为 0、username 列为空；筛选只看字段值，不能被键名、
// login_method 或 user_agent 污染。
func TestGetAllLogsLoginScopeMatchesTargetAndAttemptedIdentifier(t *testing.T) {
	db := setupLoginSecurityLogDB(t)
	now := common.GetTimestamp()

	targetLog := &Log{
		UserId: 0, CreatedAt: now - 30, Type: LogTypeLoginSecurity, Content: "登录失败：账号已被禁用",
		Ip: "203.0.113.9",
		Other: `{"login_method":"password","user_agent":"Mozilla/5.0 (X11) AppleWebKit",` +
			`"auth_audit":{"outcome":"failure","stage":"credentials","reason":"account_disabled","count":1},` +
			`"admin_info":{"target_user_id":5,"target_username":"blocked_target","cf_ray":"9ab"}}`,
	}
	attemptedLog := &Log{
		UserId: 0, CreatedAt: now - 20, Type: LogTypeLoginSecurity, Content: "登录失败：账号不存在",
		Other: `{"login_method":"2fa","user_agent":"Mozilla/5.0 Firefox",` +
			`"auth_audit":{"outcome":"failure","stage":"credentials","reason":"user_not_found","count":1},` +
			`"admin_info":{"attempted_identifier":"ghost_user"}}`,
	}
	successLog := &Log{
		UserId: 7, Username: "self_user", CreatedAt: now - 15, Type: LogTypeLogin, Content: "登录成功",
		UserAgent: "Mozilla/5.0 LoginAuditTest",
		Other:     `{"login_method":"password","user_agent":"Mozilla/5.0 LoginAuditTest","admin_info":{"cf_ray":"abc"}}`,
	}
	adminActorLog := &Log{UserId: 9, Username: "admin_ops", CreatedAt: now - 10, Type: LogTypeLogin, Content: "登录成功"}
	// 历史成功登录记录：other 为空或不是 JSON，CASE 隔离后查询不得报错。
	legacyEmptyLog := &Log{UserId: 11, Username: "legacy_user", CreatedAt: now - 8, Type: LogTypeLogin, Content: "登录成功"}
	legacyBrokenLog := &Log{UserId: 12, Username: "broken_user", CreatedAt: now - 5, Type: LogTypeLogin, Content: "登录成功", Other: "not-json"}
	consumeLog := &Log{
		UserId: 0, CreatedAt: now - 3, Type: LogTypeConsume, Content: "消费",
		Other: `{"admin_info":{"target_username":"blocked_target"}}`,
	}
	for _, row := range []*Log{targetLog, attemptedLog, successLog, adminActorLog, legacyEmptyLog, legacyBrokenLog, consumeLog} {
		require.NoError(t, db.Create(row).Error)
	}

	query := func(username string) []*Log {
		t.Helper()
		logs, _, err := GetAllLogs(LogTypeUnknown, LogScopeLogin, 0, 0, "", username, "", 0, 50, 0, "", "", "", "", "", LogSessionFilter{})
		require.NoError(t, err)
		return logs
	}

	// 正例：目标账号、输入账号与 actor 用户名。
	found := query("blocked_target")
	require.Len(t, found, 1)
	require.Equal(t, targetLog.Id, found[0].Id)

	found = query("ghost_user")
	require.Len(t, found, 1)
	require.Equal(t, attemptedLog.Id, found[0].Id)

	found = query("self_user")
	require.Len(t, found, 1)
	require.Equal(t, successLog.Id, found[0].Id)

	// 模糊模式与 actor 筛选语义一致：%admin% 命中 actor，字段值不含 admin 的行不受影响。
	found = query("%admin%")
	require.Len(t, found, 1)
	require.Equal(t, adminActorLog.Id, found[0].Id)

	// 历史 type=7 记录（other 为空 / 非 JSON）不参与字段提取，也不会让查询失败。
	found = query("legacy_user")
	require.Len(t, found, 1)
	require.Equal(t, legacyEmptyLog.Id, found[0].Id)
	found = query("broken_user")
	require.Len(t, found, 1)
	require.Equal(t, legacyBrokenLog.Id, found[0].Id)

	// 负例：键名、login_method 与 UA 都不能命中毒账号筛选。
	for _, needle := range []string{"admin_info", "password", "login_method", "Mozilla", "cf_ray", "auth_audit"} {
		require.Empty(t, query(needle), "unexpected match for %q", needle)
	}

	// OR 组必须与其他筛选条件取交集：type=7 时不得因为字段命中而返回 type=8。
	logs, total, err := GetAllLogs(LogTypeLogin, LogScopeLogin, 0, 0, "", "blocked_target", "", 0, 50, 0, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, logs)

	// 时间筛选同样不能被 OR 组绕过。
	_, total, err = GetAllLogs(LogTypeUnknown, LogScopeLogin, 0, now-40, "", "blocked_target", "", 0, 50, 0, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 0, total)

	// 非登录 scope 保持原有语义：只匹配 username 列。
	logs, total, err = GetAllLogs(LogTypeUnknown, "", 0, 0, "", "blocked_target", "", 0, 50, 0, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, logs)
}

// 批次写入受 ctx 与失败条数约束：已结束的 ctx 一条也不写，中途失败则返回已写条数。
func TestRecordLoginSecurityEventsStopsOnCancelledContextAndFailure(t *testing.T) {
	db := setupLoginSecurityLogDB(t)

	events := []LoginAuditEvent{
		{Outcome: LoginAuditOutcomeFailure, Stage: LoginAuditStageCredentials, Reason: LoginAuditReasonInvalidCredentials},
		{Outcome: LoginAuditOutcomeFailure, Stage: LoginAuditStageCredentials, Reason: LoginAuditReasonInvalidCredentials},
		{Outcome: LoginAuditOutcomeFailure, Stage: LoginAuditStageCredentials, Reason: LoginAuditReasonInvalidCredentials},
	}

	// (a) 已经结束的 ctx：一条都不写，调用方据此转备份。
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	persisted, err := RecordLoginSecurityEvents(cancelled, events)
	require.Error(t, err)
	require.Equal(t, 0, persisted)
	require.EqualValues(t, 0, countLoginSecurityLogs(t, db))

	// (b) 第 3 条插入失败：返回已写入的 2 条，不再继续写。
	const failThirdCreate = "test:fail_third_login_audit_create"
	creates := 0
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(failThirdCreate, func(tx *gorm.DB) {
		creates++
		if creates == 3 {
			_ = tx.AddError(errors.New("log db unavailable in test"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(failThirdCreate) })

	persisted, err = RecordLoginSecurityEvents(context.Background(), events)

	require.Error(t, err)
	require.Equal(t, 2, persisted)
	require.EqualValues(t, 2, countLoginSecurityLogs(t, db))
}

// 真实 500ms 上限：让 INSERT 等待 Statement.Context 到期，验证插入不会拖住调用方。
func TestRecordLoginSecurityEventsInsertTimeoutIsBounded(t *testing.T) {
	db := setupLoginSecurityLogDB(t)
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test:block_until_ctx_done") })

	// 让 INSERT 真正等待 ctx 结束，验证 500ms 上限生效。
	blockRegistered := db.Callback().Create().Before("gorm:create").Register("test:block_until_ctx_done", func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if ctx == nil {
			return
		}
		<-ctx.Done()
		_ = tx.AddError(ctx.Err())
	})
	require.NoError(t, blockRegistered)

	start := time.Now()
	persisted, err := RecordLoginSecurityEvents(context.Background(), []LoginAuditEvent{{
		Outcome: LoginAuditOutcomeFailure,
		Stage:   LoginAuditStageCredentials,
		Reason:  LoginAuditReasonInvalidCredentials,
	}})
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Equal(t, 0, persisted)
	require.Less(t, elapsed, 2*time.Second, "insert must be bounded by the 500ms cap")
	require.GreaterOrEqual(t, elapsed, 400*time.Millisecond)
	require.EqualValues(t, 0, countLoginSecurityLogs(t, db))
}

// 备份 JSON 必须保留可追查的完整白名单字段，且不含任何敏感值。
func TestLoginAuditEventBackupJSONKeepsWhitelistedFields(t *testing.T) {
	event := LoginAuditEvent{
		Outcome:             LoginAuditOutcomeFailure,
		Stage:               LoginAuditStageCredentials,
		Reason:              LoginAuditReasonInvalidCredentials,
		Count:               2,
		WindowStart:         1700000000,
		WindowEnd:           1700000059,
		CreatedAt:           1700000030,
		IP:                  "203.0.113.9",
		PeerIP:              "10.0.0.9",
		UserAgent:           "Agent/1.0 LoginAudit",
		RequestPath:         "/api/user/login?turnstile=secret",
		RequestId:           "req-backup-1",
		CloudflareRay:       "9ab12cd34ef56789",
		LoginMethod:         "password",
		TargetUserId:        42,
		TargetUsername:      "target_user",
		AttemptedIdentifier: "target_user@example.com",
	}

	payload, err := LoginAuditEventBackupJSON(event)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(payload, &decoded))
	require.Equal(t, "failure", decoded["outcome"])
	require.Equal(t, "credentials", decoded["stage"])
	require.Equal(t, "invalid_credentials", decoded["reason"])
	require.EqualValues(t, 2, decoded["count"])
	require.EqualValues(t, 1700000000, decoded["window_start"])
	require.EqualValues(t, 1700000059, decoded["window_end"])
	require.EqualValues(t, 1700000030, decoded["created_at"])
	require.Equal(t, "203.0.113.9", decoded["ip"])
	require.Equal(t, "10.0.0.9", decoded["peer_ip"])
	require.Equal(t, "Agent/1.0 LoginAudit", decoded["user_agent"])
	require.Equal(t, "req-backup-1", decoded["request_id"])
	require.Equal(t, "9ab12cd34ef56789", decoded["cf_ray"])
	require.Equal(t, "password", decoded["login_method"])
	require.EqualValues(t, 42, decoded["target_user_id"])
	require.Equal(t, "target_user", decoded["target_username"])
	require.Equal(t, "target_user@example.com", decoded["attempted_identifier"])
	// 请求路径去掉 query，且不含敏感字段。
	require.Equal(t, "/api/user/login", decoded["request_path"])
	for _, forbidden := range []string{"secret", "password_hash", "cookie", "bearer"} {
		require.NotContains(t, strings.ToLower(payload), forbidden)
	}

	// 超长输入被限长后再备份。
	longEvent := LoginAuditEvent{
		Outcome:             LoginAuditOutcomeFailure,
		Stage:               LoginAuditStageCredentials,
		Reason:              LoginAuditReasonUserNotFound,
		UserAgent:           strings.Repeat("z", 4000),
		AttemptedIdentifier: strings.Repeat("a", 400),
	}
	longPayload, err := LoginAuditEventBackupJSON(longEvent)
	require.NoError(t, err)
	var longDecoded map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(longPayload, &longDecoded))
	require.LessOrEqual(t, len([]rune(longDecoded["user_agent"].(string))), 512)
	require.LessOrEqual(t, len([]rune(longDecoded["attempted_identifier"].(string))), 128)
}

// 成功登录日志（type=7）同样在落库前统一限长 IP/UA/诊断字段，并去掉路径 query。
func TestRecordLoginLogSanitizesFieldsBeforePersist(t *testing.T) {
	db := setupLoginSecurityLogDB(t)

	RecordLoginLog(
		7,
		strings.Repeat("u", 400),
		"Logged in successfully via password",
		strings.Repeat("9", 100),
		strings.Repeat("r", 200),
		"login",
		map[string]interface{}{"method": "password"},
		map[string]interface{}{
			"login_method": "password",
			"user_agent":   strings.Repeat("z", 4000),
			"request_path": "/api/user/login?turnstile=secret",
			"admin_info": map[string]interface{}{
				"cf_ray":  strings.Repeat("c", 300),
				"peer_ip": strings.Repeat("7", 100),
			},
		},
	)

	var log Log
	require.NoError(t, db.Where("type = ?", LogTypeLogin).First(&log).Error)
	require.Equal(t, 7, log.UserId)
	require.LessOrEqual(t, len([]rune(log.Username)), 128)
	require.LessOrEqual(t, len([]rune(log.Ip)), 64)
	require.LessOrEqual(t, len([]rune(log.UserAgent)), 512)
	require.LessOrEqual(t, len([]rune(log.RequestId)), 64)

	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	require.Equal(t, "/api/user/login", other["request_path"])
	require.NotContains(t, log.Other, "secret")
	require.LessOrEqual(t, len([]rune(other["user_agent"].(string))), 512)

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.LessOrEqual(t, len([]rune(adminInfo["cf_ray"].(string))), 128)
	require.LessOrEqual(t, len([]rune(adminInfo["peer_ip"].(string))), 64)
}

// 成功登录日志写入失败时留结构化备份，且不改变调用方的行为。
func TestRecordLoginLogBacksUpOnPersistFailure(t *testing.T) {
	db := setupLoginSecurityLogDB(t)

	var buffer strings.Builder
	previousWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &buffer
	t.Cleanup(func() { gin.DefaultErrorWriter = previousWriter })

	require.NoError(t, db.Migrator().DropTable(&Log{}))
	RecordLoginLog(11, "backup_user", "Logged in successfully via password", "203.0.113.11", "req-backup-2", "login",
		nil, map[string]interface{}{"user_agent": "Agent/1.0"})

	output := buffer.String()
	require.Contains(t, output, `"user_id":11`)
	require.Contains(t, output, `"username":"backup_user"`)
	require.Contains(t, output, `"ip":"203.0.113.11"`)
	require.Contains(t, output, `"request_id":"req-backup-2"`)
	require.NotContains(t, output, "password_hash")
}

// NUL 文本归一化：移除真实 NUL、按 rune 限长，字面 "\u0000" 原样保留。
func TestTruncateLoginAuditTextRemovesNulAndKeepsLiteralEscape(t *testing.T) {
	require.Equal(t, "user", truncateLoginAuditText("us\x00er", 128))
	require.Equal(t, "ab  cd", truncateLoginAuditText("  ab\x00  cd  ", 128))
	require.Equal(t, "", truncateLoginAuditText("a\x00b", 0))
	require.Equal(t, "abcd", truncateLoginAuditText("abcdef", 4))
	require.Equal(t, "中文", truncateLoginAuditText("中文测试", 2))
	// 字面反斜杠 u0000 不是真实 NUL，必须原样保留。
	require.Equal(t, `a\u0000b`, truncateLoginAuditText(`a\u0000b`, 128))
	// 无效 UTF-8 也要返回规范文本，不能原样透传。
	invalid := "bad\xff"
	require.False(t, utf8.ValidString(invalid))
	require.Equal(t, "bad\uFFFD", truncateLoginAuditText(invalid, 128))
}

// 真实写入后 other 不含 NUL，账号筛选仍可命中目标/输入账号。
func TestRecordLoginSecurityEventStripsNulAndKeepsAccountFilterWorking(t *testing.T) {
	db := setupLoginSecurityLogDB(t)

	persisted, err := RecordLoginSecurityEvents(context.Background(), []LoginAuditEvent{{
		Outcome:             LoginAuditOutcomeFailure,
		Stage:               LoginAuditStageCredentials,
		Reason:              LoginAuditReasonInvalidCredentials,
		LoginMethod:         "password",
		IP:                  "203.0.113.9",
		UserAgent:           "UA\x00/1.0",
		RequestPath:         "/api/user/login\x00?token=secret",
		CloudflareRay:       "ray\x00id",
		TargetUserId:        42,
		TargetUsername:      "tar\x00get",
		AttemptedIdentifier: "scan\x00@example.com",
	}})
	require.NoError(t, err)
	require.Equal(t, 1, persisted)

	var log Log
	require.NoError(t, db.Where("type = ?", LogTypeLoginSecurity).First(&log).Error)
	require.NotContains(t, log.Other, "\x00")
	require.NotContains(t, log.UserAgent, "\x00")
	require.Equal(t, "UA/1.0", log.UserAgent)

	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	adminInfo := other["admin_info"].(map[string]interface{})
	require.Equal(t, "target", adminInfo["target_username"])
	require.Equal(t, "scan@example.com", adminInfo["attempted_identifier"])
	require.Equal(t, "rayid", adminInfo["cf_ray"])
	require.Equal(t, "/api/user/login", other["request_path"])

	// 账号筛选在 SQLite 上正常返回；NUL 已被移除，PG 的 other::jsonb 也不会再报错。
	logs, total, err := GetAllLogs(LogTypeUnknown, LogScopeLogin, 0, 0, "", "%scan@example.com%", "", 0, 20, 0, "", "", "", "", "", LogSessionFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)

	// 成功登录日志与结构化备份共用同一文本归一化。
	RecordLoginLog(
		7,
		"user\x00name",
		"Logged in successfully via password",
		"203.0.113.9",
		"req\x00id",
		"login",
		map[string]interface{}{"method": "password"},
		map[string]interface{}{
			"login_method": "password",
			"user_agent":   "UA\x00/2.0",
			"request_path": "/api/user/login\x00",
			"admin_info":   map[string]interface{}{"cf_ray": "ray\x00two"},
		},
	)
	var successLog Log
	require.NoError(t, db.Where("type = ?", LogTypeLogin).First(&successLog).Error)
	require.NotContains(t, successLog.Other, "\x00")
	require.NotContains(t, successLog.Username, "\x00")
	require.Equal(t, "username", successLog.Username)
	require.Equal(t, "UA/2.0", successLog.UserAgent)

	payload, err := LoginAuditEventBackupJSON(LoginAuditEvent{
		Outcome:             LoginAuditOutcomeFailure,
		Stage:               LoginAuditStageCredentials,
		Reason:              LoginAuditReasonInvalidCredentials,
		UserAgent:           "UA\x00/1.0",
		AttemptedIdentifier: "scan\x00@example.com",
	})
	require.NoError(t, err)
	require.NotContains(t, payload, "\x00")
	var backup map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(payload, &backup))
	require.Equal(t, "scan@example.com", backup["attempted_identifier"])

	// 字面 "\u0000" 在备份 JSON 里仍按原文保留（解码后为字面反斜杠序列）。
	literalPayload, err := LoginAuditEventBackupJSON(LoginAuditEvent{
		Outcome:             LoginAuditOutcomeFailure,
		Stage:               LoginAuditStageCredentials,
		Reason:              LoginAuditReasonInvalidCredentials,
		AttemptedIdentifier: `user\u0000name`,
	})
	require.NoError(t, err)
	var literalBackup map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(literalPayload, &literalBackup))
	require.Equal(t, `user\u0000name`, literalBackup["attempted_identifier"])
}
