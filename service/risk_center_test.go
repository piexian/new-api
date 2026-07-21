package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// setupRiskModels 在共享测试数据库上幂等地迁移风控相关表。
func setupRiskModels(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(
		&model.IPBan{},
		&model.User{},
		&model.ProbeIPAbuseState{},
		&model.ProbeUserAbuseState{},
		&model.ErrorBanIPState{},
		&model.ErrorBanUserState{},
		&model.RiskBanLog{},
	))
}

// setProbeGuardConfig 通过真实配置管线写入探测防护配置。
func setProbeGuardConfig(t *testing.T, values map[string]string) {
	t.Helper()
	cfg := config.GlobalConfig.Get("probe_guard_setting")
	require.NotNil(t, cfg)
	require.NoError(t, config.UpdateConfigFromMap(cfg, values))
	t.Cleanup(func() {
		_ = config.UpdateConfigFromMap(cfg, map[string]string{"enabled": "false", "dry_run": "true"})
	})
}

// setErrorBanConfig 通过真实配置管线写入错误封禁配置。
func setErrorBanConfig(t *testing.T, values map[string]string) {
	t.Helper()
	cfg := config.GlobalConfig.Get("error_ban_setting")
	require.NotNil(t, cfg)
	require.NoError(t, config.UpdateConfigFromMap(cfg, values))
	t.Cleanup(func() {
		_ = config.UpdateConfigFromMap(cfg, map[string]string{"enabled": "false", "dry_run": "true", "rules": "[]"})
		_ = risk_setting.RebuildRegexCache()
	})
}

// newRiskTestContext 构造带指定客户端 IP 的 gin 测试上下文。
func newRiskTestContext(t *testing.T, ip string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = ip + ":12345"
	c.Request = req
	return c
}

// TestProbeGuardSlidingWindow 验证滑动窗口的去重计数语义（纯内存路径）。
func TestProbeGuardSlidingWindow(t *testing.T) {
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedis })

	key := riskIPKey("probe_guard:test", "192.0.2.99")
	models := []string{"m1", "m2", "m3", "m4"}
	for i, m := range models {
		got := riskWindowAddDistinct(key, m, 60)
		require.EqualValues(t, i+1, got, "distinct count after adding %s", m)
	}
	// 重复模型不增加去重计数。
	require.EqualValues(t, 4, riskWindowAddDistinct(key, "m1", 60))
	// 第 5 个不同模型使计数达到 5。
	require.EqualValues(t, 5, riskWindowAddDistinct(key, "m5", 60))
}

// TestProbeGuardCooldown 验证冷却锁的去重语义。
func TestProbeGuardCooldown(t *testing.T) {
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedis })

	key := "probe_guard:cooldown:test:192.0.2.120"
	require.True(t, riskCooldownAcquire(key, 60), "first acquire should succeed")
	require.False(t, riskCooldownAcquire(key, 60), "second acquire within cooldown should fail")
}

// TestProbeGuardNormalizeIP 验证客户端 IP 规范化拒绝非公网地址。
func TestProbeGuardNormalizeIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"203.0.113.5", true},
		{"8.8.8.8", true},
		{"10.0.0.1", false},          // 私网
		{"192.168.1.1", false},       // 私网
		{"127.0.0.1", false},         // 环回
		{"100.64.0.1", false},        // CGNAT
		{"169.254.1.1", false},       // 链路本地
		{"::ffff:203.0.113.5", true}, // IPv4 映射 IPv6
		{"not-an-ip", false},
	}
	for _, tc := range cases {
		_, ok := normalizeProbeClientIP(tc.ip)
		require.Equal(t, tc.want, ok, "ip=%s", tc.ip)
	}
}

// TestProbeGuardBanTier 验证违规次数到封禁时长的映射。
func TestProbeGuardBanTier(t *testing.T) {
	setting := risk_setting.ProbeGuardSetting{
		FirstIPBanMinutes:     10,
		SecondIPBanMinutes:    60,
		PermanentOffenseCount: 3,
	}
	duration, permanent := probeBanTier(1, setting)
	require.Equal(t, 10, duration)
	require.False(t, permanent)

	duration, permanent = probeBanTier(2, setting)
	require.Equal(t, 60, duration)
	require.False(t, permanent)

	_, permanent = probeBanTier(3, setting)
	require.True(t, permanent)
}

// TestProbeGuardTrigger 端到端验证：5 个不同模型触发封禁，4 个不触发，冷却防止重复触发，管理员豁免。
func TestProbeGuardTrigger(t *testing.T) {
	setupRiskModels(t)
	setProbeGuardConfig(t, map[string]string{
		"enabled":                 "true",
		"dry_run":                 "false",
		"window_seconds":          "60",
		"distinct_model_count":    "5",
		"first_ip_ban_minutes":    "10",
		"second_ip_ban_minutes":   "60",
		"permanent_offense_count": "3",
		"offense_dedupe_seconds":  "60",
		"user_ban_enabled":        "false",
		"notify_user_enabled":     "false",
		"notify_admin_enabled":    "false",
	})

	// 普通用户。
	require.NoError(t, model.DB.Create(&model.User{
		Id: 91001, Username: "probe_common", AffCode: "PROBECOMMON", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}).Error)
	// 管理员用户。
	require.NoError(t, model.DB.Create(&model.User{
		Id: 92002, Username: "probe_admin", AffCode: "PROBEADMIN", Role: common.RoleAdminUser, Status: common.UserStatusEnabled,
	}).Error)

	ip := "198.51.100.10"
	// 前 4 个不同模型：不触发。
	for i := range 4 {
		c := newRiskTestContext(t, ip)
		info := &relaycommon.RelayInfo{UserId: 91001, OriginModelName: fmt.Sprintf("model-%d", i)}
		require.Nil(t, CheckProbeGuard(c, info), "request %d should not trigger", i)
	}
	// 尚无封禁。
	_, err := model.GetIPBanByTarget(ip)
	require.Error(t, err)

	// 第 5 个不同模型：触发。
	c := newRiskTestContext(t, ip)
	apiErr := CheckProbeGuard(c, &relaycommon.RelayInfo{UserId: 91001, OriginModelName: "model-4"})
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeBulkProbeDetected, apiErr.GetErrorCode())
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)

	// IP 封禁已创建，且为首次临时封禁。
	ban, err := model.GetIPBanByTarget(ip)
	require.NoError(t, err)
	require.NotZero(t, ban.ExpiresAt)

	// 冷却窗口内的第 6 个不同模型：不再重复触发（返回 nil）。
	c2 := newRiskTestContext(t, ip)
	require.Nil(t, CheckProbeGuard(c2, &relaycommon.RelayInfo{UserId: 91001, OriginModelName: "model-5"}))

	// 管理员豁免：发送 6 个不同模型也不触发。
	adminIP := "198.51.100.30"
	for i := range 6 {
		c := newRiskTestContext(t, adminIP)
		info := &relaycommon.RelayInfo{UserId: 92002, OriginModelName: fmt.Sprintf("admin-model-%d", i)}
		require.Nil(t, CheckProbeGuard(c, info))
	}
	_, err = model.GetIPBanByTarget(adminIP)
	require.Error(t, err, "admin IP must not be banned")
}

// TestErrorBan 端到端验证错误封禁：正则匹配累计窗口、阈值触发阶梯、RequestId 去重、配置热更新重建缓存。
func TestErrorBan(t *testing.T) {
	setupRiskModels(t)
	setErrorBanConfig(t, map[string]string{
		"enabled":              "true",
		"dry_run":              "false",
		"window_seconds":       "300",
		"default_dimension":    "ip",
		"notify_user_enabled":  "false",
		"notify_admin_enabled": "false",
		"rules":                `[{"id":"invalid_key","name":"Invalid key","pattern":"invalid_api_key","enabled":true,"threshold":3,"dimension":"ip"}]`,
		"tiers":                `[{"offense_count":1,"action":"temp_ip_ban","duration_minutes":30}]`,
	})
	require.NoError(t, risk_setting.RebuildRegexCache())

	// 正则缓存已构建。
	rules := risk_setting.GetCompiledRules()
	require.Len(t, rules, 1)
	require.True(t, rules[0].Re.MatchString("status_code=401, invalid_api_key: incorrect key"))
	require.False(t, rules[0].Re.MatchString("status_code=500, upstream timeout"))

	ip := "198.51.100.50"
	baseSnap := ErrorBanSnapshot{
		ClientIP:   ip,
		UserId:     0,
		ErrorText:  "status_code=401, invalid_api_key: incorrect api key provided",
		StatusCode: 401,
	}
	// 前两次命中：未达阈值，不封禁。
	for i := range 2 {
		snap := baseSnap
		snap.RequestId = fmt.Sprintf("errban-req-%d", i)
		processErrorBan(snap)
	}
	_, err := model.GetIPBanByTarget(ip)
	require.Error(t, err, "no ban before threshold")

	// 第 3 次命中：达到阈值，触发阶梯 temp_ip_ban。
	snap := baseSnap
	snap.RequestId = "errban-req-2"
	processErrorBan(snap)
	ban, err := model.GetIPBanByTarget(ip)
	require.NoError(t, err, "ban expected after threshold")
	require.NotZero(t, ban.ExpiresAt)

	// 不匹配规则的错误文本：不计数、不封禁。
	otherIP := "198.51.100.51"
	for i := range 5 {
		snap := ErrorBanSnapshot{
			ClientIP:   otherIP,
			ErrorText:  "status_code=500, upstream timeout",
			StatusCode: 500,
			RequestId:  fmt.Sprintf("nomatch-req-%d", i),
		}
		processErrorBan(snap)
	}
	_, err = model.GetIPBanByTarget(otherIP)
	require.Error(t, err, "non-matching errors must not ban")

	// 配置热更新：更换规则后重建缓存，旧规则不再生效。
	setErrorBanConfig(t, map[string]string{
		"rules": `[{"id":"upstream_timeout","name":"Timeout","pattern":"upstream timeout","enabled":true,"threshold":2,"dimension":"ip"}]`,
	})
	require.NoError(t, risk_setting.RebuildRegexCache())
	rules = risk_setting.GetCompiledRules()
	require.Len(t, rules, 1)
	require.Equal(t, "upstream_timeout", rules[0].Rule.Id)
	require.True(t, rules[0].Re.MatchString("status_code=500, upstream timeout"))
	require.False(t, rules[0].Re.MatchString("invalid_api_key"))
}
