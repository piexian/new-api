package risk_setting

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// ProbeGuardSetting 批量模型探测防护配置。
// 当同一 IP 在滑动窗口内请求了过多不同模型时，判定为探测行为并逐级处罚。
type ProbeGuardSetting struct {
	Enabled                bool     `json:"enabled"`
	DryRun                 bool     `json:"dry_run"`
	WindowSeconds          int      `json:"window_seconds"`
	DistinctModelCount     int      `json:"distinct_model_count"`
	BanDimension           string   `json:"ban_dimension"`
	FirstIPBanMinutes      int      `json:"first_ip_ban_minutes"`  // 兼容键：当前作为所有封禁维度的首次处罚时长。
	SecondIPBanMinutes     int      `json:"second_ip_ban_minutes"` // 兼容键：当前作为所有封禁维度的再次处罚时长。
	PermanentOffenseCount  int      `json:"permanent_offense_count"`
	OffenseDedupeSeconds   int      `json:"offense_dedupe_seconds"`
	WhitelistUserIDs       string   `json:"whitelist_user_ids"`
	WhitelistGroups        []string `json:"whitelist_groups"`
	UserBanEnabled         bool     `json:"user_ban_enabled"`          // Deprecated: 仅用于迁移旧配置。
	UserBanThreshold       int      `json:"user_ban_threshold"`        // Deprecated: 仅保留配置兼容性。
	UserBanDurationMinutes int      `json:"user_ban_duration_minutes"` // Deprecated: 账号处罚改用统一阶梯。
	UserBanReason          string   `json:"user_ban_reason"`
	NotifyUserEnabled      bool     `json:"notify_user_enabled"`
	NotifyAdminEnabled     bool     `json:"notify_admin_enabled"`
	AppealHint             string   `json:"appeal_hint"`
}

// 默认配置：默认关闭且开启 dry_run，避免误伤。
var probeGuardSetting = ProbeGuardSetting{
	Enabled:                false,
	DryRun:                 true,
	WindowSeconds:          60,
	DistinctModelCount:     5,
	BanDimension:           "",
	FirstIPBanMinutes:      10,
	SecondIPBanMinutes:     60,
	PermanentOffenseCount:  3,
	OffenseDedupeSeconds:   60,
	WhitelistUserIDs:       "",
	WhitelistGroups:        []string{},
	UserBanEnabled:         false,
	UserBanThreshold:       2,
	UserBanDurationMinutes: 0,
	UserBanReason:          "触发批量模型探测自动封禁",
	NotifyUserEnabled:      true,
	NotifyAdminEnabled:     true,
	AppealHint:             "如认为误封，请联系管理员。",
}

func init() {
	config.GlobalConfig.Register("probe_guard_setting", &probeGuardSetting)
}

// GetProbeGuardSetting 返回经过归一化的配置副本，避免读路径修改共享状态。
func GetProbeGuardSetting() ProbeGuardSetting {
	snapshot := probeGuardSetting
	snapshot.WhitelistGroups = append([]string{}, probeGuardSetting.WhitelistGroups...)
	snapshot.Normalize()
	return snapshot
}

// Normalize 将各字段收敛到合法区间，防止异常配置导致服务不可用。
func (s *ProbeGuardSetting) Normalize() {
	legacyUserBanEnabled := s.UserBanEnabled
	s.BanDimension = strings.ToLower(strings.TrimSpace(s.BanDimension))
	switch s.BanDimension {
	case DimensionIP, DimensionUser, ProbeBanDimensionBoth:
	default:
		if legacyUserBanEnabled {
			s.BanDimension = ProbeBanDimensionBoth
		} else {
			s.BanDimension = DimensionIP
		}
	}
	// 继续向旧客户端返回兼容字段，但运行时只读取 BanDimension。
	s.UserBanEnabled = s.BansUser()
	s.WindowSeconds = clampInt(s.WindowSeconds, 5, 3600, 60)
	s.DistinctModelCount = clampInt(s.DistinctModelCount, 2, 100, 5)
	s.FirstIPBanMinutes = clampInt(s.FirstIPBanMinutes, 1, 525600, 10)
	s.SecondIPBanMinutes = clampInt(s.SecondIPBanMinutes, 1, 525600, 60)
	s.PermanentOffenseCount = clampInt(s.PermanentOffenseCount, 1, 100, 3)
	s.OffenseDedupeSeconds = clampInt(s.OffenseDedupeSeconds, 0, 3600, 60)
	s.UserBanThreshold = clampInt(s.UserBanThreshold, 1, 100, 2)
	s.WhitelistGroups = normalizeStringList(s.WhitelistGroups)
	if strings.TrimSpace(s.UserBanReason) == "" {
		s.UserBanReason = "触发批量模型探测自动封禁"
	}
}

const ProbeBanDimensionBoth = "both"

// BansIP 判断本次探测处罚是否包含 IP 封禁。
func (s ProbeGuardSetting) BansIP() bool {
	return s.BanDimension == DimensionIP || s.BanDimension == ProbeBanDimensionBoth
}

// BansUser 判断本次探测处罚是否包含账号封禁。
func (s ProbeGuardSetting) BansUser() bool {
	return s.BanDimension == DimensionUser || s.BanDimension == ProbeBanDimensionBoth
}

// IsUserWhitelisted 判断用户是否在白名单中（逗号分隔的用户 ID 列表）。
func (s *ProbeGuardSetting) IsUserWhitelisted(userId int) bool {
	return whitelistContains(s.WhitelistUserIDs, userId)
}

// IsGroupWhitelisted 判断请求分组是否在白名单中。
func (s *ProbeGuardSetting) IsGroupWhitelisted(group string) bool {
	return stringListContains(s.WhitelistGroups, group)
}

func normalizeStringList(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func stringListContains(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

// whitelistContains 解析逗号分隔的用户 ID 列表并判断是否包含目标用户。
func whitelistContains(raw string, userId int) bool {
	if userId <= 0 {
		return false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && id == userId {
			return true
		}
	}
	return false
}

// clampInt 将 v 收敛到 [min, max]；当 v <= 0 时使用 def。
func clampInt(v, min, max, def int) int {
	if v <= 0 {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
