package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"sync/atomic"
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
		_ = config.UpdateConfigFromMap(cfg, map[string]string{"enabled": "false", "dry_run": "true", "ban_dimension": "ip", "user_ban_enabled": "false", "whitelist_groups": "[]"})
	})
}

// setErrorBanConfig 通过真实配置管线写入错误封禁配置。
func setErrorBanConfig(t *testing.T, values map[string]string) {
	t.Helper()
	cfg := config.GlobalConfig.Get("error_ban_setting")
	require.NotNil(t, cfg)
	require.NoError(t, config.UpdateConfigFromMap(cfg, values))
	t.Cleanup(func() {
		_ = config.UpdateConfigFromMap(cfg, map[string]string{"enabled": "false", "dry_run": "true", "whitelist_groups": "[]", "rules": "[]"})
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

func TestErrorBanLiveProgressBeforeThreshold(t *testing.T) {
	setupRiskModels(t)
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	resetRiskLiveProgressForTest()
	t.Cleanup(func() {
		common.RedisEnabled = oldRedis
		resetRiskLiveProgressForTest()
	})
	setErrorBanConfig(t, map[string]string{
		"enabled":              "true",
		"dry_run":              "true",
		"window_seconds":       "300",
		"default_dimension":    "ip",
		"notify_user_enabled":  "false",
		"notify_admin_enabled": "false",
		"rules":                `[{"id":"live_before_threshold","name":"Live before threshold","pattern":"live_progress_error","enabled":true,"threshold":3,"dimension":"ip","tiers":[{"offense_count":1,"action":"temp_ip_ban","duration_minutes":1}]}]`,
	})
	require.NoError(t, risk_setting.RebuildRegexCache())

	ip := "198.51.100.171"
	processErrorBan(ErrorBanSnapshot{
		ClientIP:  ip,
		ErrorText: "live_progress_error",
		RequestId: "live-progress-before-threshold",
	})

	targets, total := GetRiskLiveTargets(RiskLiveSourceErrorBan, "live_before_threshold", "ip", "", 0, 10)
	require.Equal(t, 1, total)
	require.Len(t, targets, 1)
	require.Equal(t, ip, targets[0].Target)
	require.EqualValues(t, 1, targets[0].CurrentCount)
	require.Equal(t, 3, targets[0].Threshold)
	require.Equal(t, "observing", targets[0].Status)

	states, stateTotal, err := model.ListErrorBanIPStates(ip, 0, 10)
	require.NoError(t, err)
	require.Zero(t, stateTotal, "threshold前不应增加累计违规状态")
	require.Empty(t, states)

	ClearRiskLiveProgress(RiskLiveSourceErrorBan, risk_setting.DimensionIP, ip)
	_, total = GetRiskLiveTargets(RiskLiveSourceErrorBan, "live_before_threshold", "ip", "", 0, 10)
	require.Zero(t, total)
}

func TestProbeGuardLiveProgressUsesIndependentUserWindows(t *testing.T) {
	setupRiskModels(t)
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	resetRiskLiveProgressForTest()
	t.Cleanup(func() {
		common.RedisEnabled = oldRedis
		resetRiskLiveProgressForTest()
	})
	setProbeGuardConfig(t, map[string]string{
		"enabled":                "true",
		"dry_run":                "true",
		"window_seconds":         "60",
		"distinct_model_count":   "3",
		"offense_dedupe_seconds": "60",
		"ban_dimension":          "user",
		"notify_user_enabled":    "false",
		"notify_admin_enabled":   "false",
	})

	users := []model.User{
		{Id: 97101, Username: "live_probe_a", AffCode: "LIVEPROBEA", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Id: 97102, Username: "live_probe_b", AffCode: "LIVEPROBEB", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}
	for i := range users {
		require.NoError(t, model.DB.Create(&users[i]).Error)
		userId := users[i].Id
		t.Cleanup(func() { model.DB.Unscoped().Delete(&model.User{}, userId) })
	}

	ip := "198.51.100.172"
	require.Nil(t, CheckProbeGuard(newRiskTestContext(t, ip), &relaycommon.RelayInfo{UserId: users[0].Id, OriginModelName: "model-a"}))
	require.Nil(t, CheckProbeGuard(newRiskTestContext(t, ip), &relaycommon.RelayInfo{UserId: users[1].Id, OriginModelName: "model-b"}))

	targets, total := GetRiskLiveTargets(RiskLiveSourceProbeGuard, RiskLiveProbeGuardRuleID, risk_setting.DimensionUser, "", 0, 10)
	require.Equal(t, 2, total)
	require.Len(t, targets, 2)
	for _, target := range targets {
		require.EqualValues(t, 1, target.CurrentCount, "每个用户应维护独立的不同模型窗口")
		require.Equal(t, 3, target.Threshold)
		require.Equal(t, "observing", target.Status)
	}

	_, offenseTotal, err := model.ListProbeUserAbuseStates(ip, 0, 10)
	require.NoError(t, err)
	require.Zero(t, offenseTotal, "threshold前不应创建探针累计违规状态")
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

func TestRiskCooldownConcurrentAcquire(t *testing.T) {
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedis })

	key := fmt.Sprintf("risk:cooldown:concurrent:%d", common.GetTimestamp())
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if riskCooldownAcquire(key, 60) {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, successes.Load())
}

func TestRiskEventMemberUniqueAcrossCalls(t *testing.T) {
	seen := make(map[string]struct{}, 128)
	for range 128 {
		member := riskEventMember()
		if _, exists := seen[member]; exists {
			t.Fatalf("duplicate risk event member: %s", member)
		}
		seen[member] = struct{}{}
	}
}

func TestErrorBanRuleCombinedMatchers(t *testing.T) {
	rule := risk_setting.ErrorBanRule{
		Pattern:    `(?i)quota`,
		Keywords:   []string{"exceeded", "request"},
		ErrorCodes: []string{"insufficient_quota", "rate_limit_exceeded"},
	}
	compiled := risk_setting.CompiledRule{Rule: rule, Re: regexp.MustCompile(rule.Pattern)}
	require.True(t, compiled.Matches("Quota exceeded for this request", "insufficient_quota"))
	require.False(t, compiled.Matches("Quota exceeded", "insufficient_quota"), "all keywords must match")
	require.False(t, compiled.Matches("Quota exceeded for this request", "invalid_request_error"), "error code must match")
	require.False(t, compiled.Matches("Capacity exceeded for this request", "insufficient_quota"), "regex must match")

	wildcard := risk_setting.CompiledRule{Rule: risk_setting.ErrorBanRule{ErrorCodes: []string{"*"}}}
	require.True(t, wildcard.Matches("any upstream failure", "any_error_code"))
	require.True(t, wildcard.Matches("error without a structured code", ""))

	wildcardWithKeyword := risk_setting.CompiledRule{Rule: risk_setting.ErrorBanRule{
		Keywords:   []string{"timeout"},
		ErrorCodes: []string{"*"},
	}}
	require.True(t, wildcardWithKeyword.Matches("upstream timeout", "gateway_error"))
	require.False(t, wildcardWithKeyword.Matches("upstream rejected", "gateway_error"), "configured keyword must still match")
}

func TestErrorBanRetryFailureRequiresRuleOptIn(t *testing.T) {
	baseRule := risk_setting.CompiledRule{Rule: risk_setting.ErrorBanRule{
		ErrorCodes: []string{"upstream_error"},
	}}
	retrySnapshot := ErrorBanSnapshot{ErrorCode: "upstream_error", RetryFailure: true}
	finalSnapshot := ErrorBanSnapshot{ErrorCode: "upstream_error"}

	require.False(t, shouldProcessErrorBanRule(retrySnapshot, baseRule))
	require.True(t, shouldProcessErrorBanRule(finalSnapshot, baseRule))

	baseRule.Rule.CountRetries = true
	require.True(t, shouldProcessErrorBanRule(retrySnapshot, baseRule))
}

func TestErrorBanWildcardCountsAllErrors(t *testing.T) {
	setupRiskModels(t)
	setErrorBanConfig(t, map[string]string{
		"enabled":              "true",
		"dry_run":              "false",
		"window_seconds":       "300",
		"default_dimension":    "ip",
		"notify_user_enabled":  "false",
		"notify_admin_enabled": "false",
		"rules": `[{
			"id":"all_errors",
			"name":"All errors",
			"pattern":"",
			"keywords":[],
			"error_codes":["*"],
			"enabled":true,
			"threshold":2,
			"dimension":"ip",
			"tiers":[{"offense_count":1,"action":"temp_ip_ban","duration_minutes":1}]
		}]`,
	})
	require.NoError(t, risk_setting.RebuildRegexCache())

	ip := "198.51.100.133"
	processErrorBan(ErrorBanSnapshot{
		ClientIP: ip, ErrorText: "first arbitrary failure", StatusCode: http.StatusBadGateway, RequestId: "wildcard-error-1",
	})
	_, err := model.GetIPBanByTarget(ip)
	require.Error(t, err, "the first error must remain below the threshold")

	processErrorBan(ErrorBanSnapshot{
		ClientIP: ip, ErrorText: "second unrelated failure", ErrorCode: "upstream_error", StatusCode: http.StatusServiceUnavailable, RequestId: "wildcard-error-2",
	})
	ban, err := model.GetIPBanByTarget(ip)
	require.NoError(t, err)
	require.NotZero(t, ban.ExpiresAt)
}

func TestErrorBanRuleUsesIndependentTiers(t *testing.T) {
	rule := risk_setting.ErrorBanRule{Tiers: []risk_setting.ErrorBanTier{
		{OffenseCount: 1, Action: risk_setting.TierActionTempIPBan, DurationMinutes: 5},
		{OffenseCount: 3, Action: risk_setting.TierActionDisableUser, DurationMinutes: 10},
	}}
	tier, ok := rule.MatchTier(3)
	require.True(t, ok)
	require.Equal(t, risk_setting.TierActionDisableUser, tier.Action)
	require.Equal(t, 10, tier.DurationMinutes)
}

func TestErrorBanLegacyTiersMigratePerRule(t *testing.T) {
	setting := risk_setting.ErrorBanSetting{
		WindowSeconds:    300,
		DefaultDimension: risk_setting.DimensionIP,
		Rules: []risk_setting.ErrorBanRule{{
			Id: "legacy", Pattern: "legacy", Enabled: true, Threshold: 1,
		}},
		Tiers: []risk_setting.ErrorBanTier{{
			OffenseCount: 1, Action: risk_setting.TierActionTempIPBan, DurationMinutes: 15,
		}},
	}
	setting.Normalize()
	require.Len(t, setting.Rules[0].Tiers, 1)
	require.Equal(t, 15, setting.Rules[0].Tiers[0].DurationMinutes)
	setting.Rules[0].Tiers[0].DurationMinutes = 99
	require.Equal(t, 15, setting.Tiers[0].DurationMinutes, "migrated tiers must not share backing storage")
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

func TestRiskRequestGroup(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		UsingGroup: " active ",
		TokenGroup: "token",
		UserGroup:  "user",
	}
	require.Equal(t, "active", riskRequestGroup(relayInfo))

	relayInfo.UsingGroup = ""
	require.Equal(t, "token", riskRequestGroup(relayInfo))

	relayInfo.TokenGroup = ""
	require.Equal(t, "user", riskRequestGroup(relayInfo))
	require.Empty(t, riskRequestGroup(nil))
}

func TestRiskGroupWhitelist(t *testing.T) {
	setupRiskModels(t)
	setProbeGuardConfig(t, map[string]string{
		"enabled":                "true",
		"dry_run":                "false",
		"distinct_model_count":   "2",
		"offense_dedupe_seconds": "60",
		"whitelist_groups":       `["trusted"]`,
		"notify_user_enabled":    "false",
		"notify_admin_enabled":   "false",
	})

	userID := 94004
	require.NoError(t, model.DB.Create(&model.User{
		Id: userID, Username: "risk_group_whitelist", AffCode: "RISKGROUP", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}).Error)

	ip := "198.51.100.90"
	for i := range 2 {
		require.Nil(t, CheckProbeGuard(
			newRiskTestContext(t, ip),
			&relaycommon.RelayInfo{UserId: userID, UsingGroup: "trusted", OriginModelName: fmt.Sprintf("trusted-model-%d", i)},
		))
	}
	states, total, err := model.ListProbeIPAbuseStates(ip, 0, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, states)

	setErrorBanConfig(t, map[string]string{
		"enabled":              "true",
		"dry_run":              "false",
		"whitelist_groups":     `["trusted"]`,
		"notify_user_enabled":  "false",
		"notify_admin_enabled": "false",
		"rules":                `[{"id":"blocked_error","name":"Blocked error","pattern":"blocked_error","enabled":true,"threshold":1,"dimension":"ip"}]`,
		"tiers":                `[{"offense_count":1,"action":"temp_ip_ban","duration_minutes":30}]`,
	})
	require.NoError(t, risk_setting.RebuildRegexCache())
	processErrorBan(ErrorBanSnapshot{
		ClientIP:   ip,
		UserId:     userID,
		ErrorText:  "blocked_error",
		StatusCode: http.StatusBadRequest,
		RequestId:  "trusted-error-request",
		Group:      "trusted",
	})
	_, err = model.GetIPBanByTarget(ip)
	require.Error(t, err, "whitelisted group must not be banned")
}

func TestProbeGuardUserBanUsesSharedSecondTier(t *testing.T) {
	setupRiskModels(t)
	userID := 94021
	require.NoError(t, model.DB.Create(&model.User{
		Id: userID, Username: "probe_temp_user_ban", AffCode: "PROBETMP", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}).Error)
	t.Cleanup(func() { model.DB.Unscoped().Delete(&model.User{}, userID) })

	now := common.GetTimestamp()
	setting := risk_setting.ProbeGuardSetting{
		FirstIPBanMinutes:     1,
		SecondIPBanMinutes:    5,
		PermanentOffenseCount: 3,
		UserBanReason:         "probe temporary ban",
		NotifyUserEnabled:     false,
	}
	userBase := (&model.User{Id: userID, Username: "probe_temp_user_ban"}).ToBaseUser()
	applyProbeGuardUserBan(setting, "198.51.100.121", userBase, "model-a,model-b", 2, now)

	var banned model.User
	require.NoError(t, model.DB.First(&banned, userID).Error)
	require.Equal(t, common.UserStatusDisabled, banned.Status)
	require.Equal(t, now+300, banned.DisabledUntil)
	var log model.RiskBanLog
	require.NoError(t, model.DB.Where("user_id = ? AND source = ? AND dimension = ?", userID, model.RiskBanSourceProbeGuard, model.RiskBanDimensionUser).Last(&log).Error)
	require.False(t, log.IsPermanent)
	require.Equal(t, 5, log.DurationMinutes)
	require.Equal(t, now+300, log.UnbanAt)
}

func TestErrorBanTemporaryUserBan(t *testing.T) {
	setupRiskModels(t)
	userID := 94022
	require.NoError(t, model.DB.Create(&model.User{
		Id: userID, Username: "error_temp_user_ban", AffCode: "ERRORTMP", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}).Error)
	t.Cleanup(func() { model.DB.Unscoped().Delete(&model.User{}, userID) })

	now := common.GetTimestamp()
	applyErrorBanUserBan(
		risk_setting.ErrorBanSetting{NotifyUserEnabled: false},
		ErrorBanSnapshot{UserId: userID, Username: "error_temp_user_ban", ClientIP: "198.51.100.122", ErrorText: "test error"},
		RiskBanInfo{
			Source: model.RiskBanSourceErrorBan, UserId: userID, Username: "error_temp_user_ban", Reason: "error temporary ban", DurationMinutes: 2,
			BannedAt: now, RuleId: "temporary-user", RuleName: "Temporary user", OffenseCount: 1,
		},
	)

	var banned model.User
	require.NoError(t, model.DB.First(&banned, userID).Error)
	require.Equal(t, common.UserStatusDisabled, banned.Status)
	require.Equal(t, now+120, banned.DisabledUntil)
	var log model.RiskBanLog
	require.NoError(t, model.DB.Where("user_id = ? AND source = ? AND dimension = ?", userID, model.RiskBanSourceErrorBan, model.RiskBanDimensionUser).Last(&log).Error)
	require.False(t, log.IsPermanent)
	require.Equal(t, 2, log.DurationMinutes)
	require.Equal(t, now+120, log.UnbanAt)
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
		"ban_dimension":           "ip",
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

func TestProbeGuardDryRunRecordsStateWithoutBan(t *testing.T) {
	setupRiskModels(t)
	setProbeGuardConfig(t, map[string]string{
		"enabled":                "true",
		"dry_run":                "true",
		"window_seconds":         "60",
		"distinct_model_count":   "2",
		"offense_dedupe_seconds": "60",
		"ban_dimension":          "ip",
		"notify_user_enabled":    "false",
		"notify_admin_enabled":   "false",
	})

	userID := 93003
	require.NoError(t, model.DB.Create(&model.User{
		Id: userID, Username: "probe_dry_run", AffCode: "PROBEDRYRUN", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}).Error)

	ip := "198.51.100.80"
	require.Nil(t, CheckProbeGuard(
		newRiskTestContext(t, ip),
		&relaycommon.RelayInfo{UserId: userID, OriginModelName: "dry-model-1"},
	))
	require.Nil(t, CheckProbeGuard(
		newRiskTestContext(t, ip),
		&relaycommon.RelayInfo{UserId: userID, OriginModelName: "dry-model-2"},
	))

	states, total, err := model.ListProbeIPAbuseStates(ip, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, states, 1)
	require.Equal(t, 1, states[0].OffenseCount)
	_, err = model.GetIPBanByTarget(ip)
	require.Error(t, err, "dry-run mode must not create an active IP ban")
}

func TestProbeGuardUserOnlyDimensionDoesNotBanIP(t *testing.T) {
	setupRiskModels(t)
	setProbeGuardConfig(t, map[string]string{
		"enabled":                 "true",
		"dry_run":                 "false",
		"ban_dimension":           "user",
		"window_seconds":          "60",
		"distinct_model_count":    "2",
		"offense_dedupe_seconds":  "60",
		"first_ip_ban_minutes":    "1",
		"second_ip_ban_minutes":   "5",
		"permanent_offense_count": "3",
		"user_ban_reason":         "probe user-only ban",
		"notify_user_enabled":     "false",
		"notify_admin_enabled":    "false",
	})

	userID := 94031
	require.NoError(t, model.DB.Create(&model.User{
		Id: userID, Username: "probe_user_only", AffCode: "PROBEUSERONLY", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}).Error)
	t.Cleanup(func() { model.DB.Unscoped().Delete(&model.User{}, userID) })

	ip := "198.51.100.131"
	require.Nil(t, CheckProbeGuard(
		newRiskTestContext(t, ip),
		&relaycommon.RelayInfo{UserId: userID, OriginModelName: "user-only-model-1"},
	))
	apiErr := CheckProbeGuard(
		newRiskTestContext(t, ip),
		&relaycommon.RelayInfo{UserId: userID, OriginModelName: "user-only-model-2"},
	)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)

	_, err := model.GetIPBanByTarget(ip)
	require.Error(t, err, "user-only dimension must not create an IP ban")
	ipStates, ipTotal, err := model.ListProbeIPAbuseStates(ip, 0, 10)
	require.NoError(t, err)
	require.Zero(t, ipTotal, "user-only dimension must not create IP offense state")
	require.Empty(t, ipStates)
	userStates, userTotal, err := model.ListProbeUserAbuseStates(ip, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, userTotal)
	require.Len(t, userStates, 1)
	require.Equal(t, 1, userStates[0].OffenseCount)
	var user model.User
	require.NoError(t, model.DB.First(&user, userID).Error)
	require.Equal(t, common.UserStatusDisabled, user.Status)
	require.NotZero(t, user.DisabledUntil)

	var ipLogs int64
	require.NoError(t, model.DB.Model(&model.RiskBanLog{}).
		Where("user_id = ? AND source = ? AND dimension = ?", userID, model.RiskBanSourceProbeGuard, model.RiskBanDimensionIP).
		Count(&ipLogs).Error)
	require.Zero(t, ipLogs)
	var userLogs int64
	require.NoError(t, model.DB.Model(&model.RiskBanLog{}).
		Where("user_id = ? AND source = ? AND dimension = ?", userID, model.RiskBanSourceProbeGuard, model.RiskBanDimensionUser).
		Count(&userLogs).Error)
	require.EqualValues(t, 1, userLogs)
	var userLog model.RiskBanLog
	require.NoError(t, model.DB.Where("user_id = ? AND source = ? AND dimension = ?", userID, model.RiskBanSourceProbeGuard, model.RiskBanDimensionUser).Last(&userLog).Error)
	require.Equal(t, 1, userLog.DurationMinutes)
	require.False(t, userLog.IsPermanent)
}

func TestProbeGuardBothDimensionRecordsBothTierStates(t *testing.T) {
	setupRiskModels(t)
	setProbeGuardConfig(t, map[string]string{
		"enabled":                 "true",
		"dry_run":                 "true",
		"ban_dimension":           "both",
		"window_seconds":          "60",
		"distinct_model_count":    "2",
		"offense_dedupe_seconds":  "60",
		"first_ip_ban_minutes":    "1",
		"second_ip_ban_minutes":   "5",
		"permanent_offense_count": "3",
		"notify_user_enabled":     "false",
		"notify_admin_enabled":    "false",
	})

	userID := 94032
	require.NoError(t, model.DB.Create(&model.User{
		Id: userID, Username: "probe_both_dimensions", AffCode: "PROBEBOTH", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}).Error)
	t.Cleanup(func() { model.DB.Unscoped().Delete(&model.User{}, userID) })

	ip := "198.51.100.132"
	for _, modelName := range []string{"both-model-1", "both-model-2"} {
		require.Nil(t, CheckProbeGuard(
			newRiskTestContext(t, ip),
			&relaycommon.RelayInfo{UserId: userID, OriginModelName: modelName},
		))
	}

	ipStates, ipTotal, err := model.ListProbeIPAbuseStates(ip, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, ipTotal)
	require.Equal(t, 1, ipStates[0].OffenseCount)
	userStates, userTotal, err := model.ListProbeUserAbuseStates(ip, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, userTotal)
	require.Equal(t, 1, userStates[0].OffenseCount)

	var logs []model.RiskBanLog
	require.NoError(t, model.DB.Where("user_id = ? AND source = ?", userID, model.RiskBanSourceProbeGuard).Order("dimension").Find(&logs).Error)
	require.Len(t, logs, 2)
	for _, log := range logs {
		require.True(t, log.DryRun)
		require.Equal(t, 1, log.DurationMinutes)
		require.False(t, log.IsPermanent)
	}
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
