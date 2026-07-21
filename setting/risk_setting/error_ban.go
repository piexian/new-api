package risk_setting

import (
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

// 封禁维度。
const (
	DimensionIP   = "ip"
	DimensionUser = "user"
)

// 阶梯处罚动作。
const (
	TierActionTempIPBan   = "temp_ip_ban"  // 临时封禁 IP
	TierActionPermIPBan   = "perm_ip_ban"  // 永久封禁 IP
	TierActionDisableUser = "disable_user" // 禁用账号
	TierActionBoth        = "both"         // 永久封禁 IP 并禁用账号
)

// MaxErrorBanRules 限制规则数量，避免配置膨胀与正则编译开销。
const MaxErrorBanRules = 20

// ErrorBanRule 单条错误匹配规则。
type ErrorBanRule struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Pattern        string `json:"pattern"`
	Enabled        bool   `json:"enabled"`
	Dimension      string `json:"dimension"` // 为空时继承全局 DefaultDimension
	Threshold      int    `json:"threshold"`
	ReasonTemplate string `json:"reason_template"`
}

// ErrorBanTier 按违规次数匹配的阶梯处罚。
type ErrorBanTier struct {
	OffenseCount    int    `json:"offense_count"`
	Action          string `json:"action"` // temp_ip_ban|perm_ip_ban|disable_user|both
	DurationMinutes int    `json:"duration_minutes"`
	ReasonSuffix    string `json:"reason_suffix"`
}

// ErrorBanSetting 错误日志触发自动封禁配置。
type ErrorBanSetting struct {
	Enabled               bool           `json:"enabled"`
	DryRun                bool           `json:"dry_run"`
	WindowSeconds         int            `json:"window_seconds"`
	DefaultDimension      string         `json:"default_dimension"`
	DefaultReasonTemplate string         `json:"default_reason_template"`
	NotifyUserEnabled     bool           `json:"notify_user_enabled"`
	NotifyAdminEnabled    bool           `json:"notify_admin_enabled"`
	AppealHint            string         `json:"appeal_hint"`
	WhitelistUserIDs      string         `json:"whitelist_user_ids"`
	ExcludeStatusCodes    []int          `json:"exclude_status_codes"`
	Rules                 []ErrorBanRule `json:"rules"`
	Tiers                 []ErrorBanTier `json:"tiers"`
}

var errorBanSetting = ErrorBanSetting{
	Enabled:            false,
	DryRun:             true,
	WindowSeconds:      300,
	DefaultDimension:   DimensionIP,
	NotifyUserEnabled:  true,
	NotifyAdminEnabled: true,
	AppealHint:         "如认为误封，请联系管理员。",
	ExcludeStatusCodes: []int{},
	Rules:              []ErrorBanRule{},
	Tiers:              defaultErrorBanTiers(),
}

func init() {
	config.GlobalConfig.Register("error_ban_setting", &errorBanSetting)
}

func defaultErrorBanTiers() []ErrorBanTier {
	return []ErrorBanTier{
		{OffenseCount: 1, Action: TierActionTempIPBan, DurationMinutes: 30, ReasonSuffix: "首次触发"},
		{OffenseCount: 2, Action: TierActionTempIPBan, DurationMinutes: 240, ReasonSuffix: "再次触发"},
		{OffenseCount: 3, Action: TierActionBoth, DurationMinutes: 0, ReasonSuffix: "多次触发"},
	}
}

// GetErrorBanSetting 返回经过归一化的配置副本。
func GetErrorBanSetting() ErrorBanSetting {
	snapshot := errorBanSetting
	snapshot.Normalize()
	return snapshot
}

// Normalize 收敛字段到合法区间，并裁剪超额规则。
func (s *ErrorBanSetting) Normalize() {
	s.WindowSeconds = clampInt(s.WindowSeconds, 10, 86400, 300)
	if s.DefaultDimension != DimensionIP && s.DefaultDimension != DimensionUser {
		s.DefaultDimension = DimensionIP
	}
	if len(s.Rules) > MaxErrorBanRules {
		s.Rules = s.Rules[:MaxErrorBanRules]
	}
	for i := range s.Rules {
		s.Rules[i].Threshold = clampInt(s.Rules[i].Threshold, 1, 100000, 5)
		if s.Rules[i].Dimension != "" &&
			s.Rules[i].Dimension != DimensionIP &&
			s.Rules[i].Dimension != DimensionUser {
			s.Rules[i].Dimension = ""
		}
	}
	if len(s.Tiers) == 0 {
		s.Tiers = defaultErrorBanTiers()
	}
	for i := range s.Tiers {
		s.Tiers[i].OffenseCount = clampInt(s.Tiers[i].OffenseCount, 1, 100000, 1)
		if !isValidTierAction(s.Tiers[i].Action) {
			s.Tiers[i].Action = TierActionTempIPBan
		}
		if s.Tiers[i].DurationMinutes < 0 {
			s.Tiers[i].DurationMinutes = 0
		}
	}
}

func isValidTierAction(action string) bool {
	switch action {
	case TierActionTempIPBan, TierActionPermIPBan, TierActionDisableUser, TierActionBoth:
		return true
	}
	return false
}

// IsUserWhitelisted 判断用户是否在白名单中。
func (s *ErrorBanSetting) IsUserWhitelisted(userId int) bool {
	return whitelistContains(s.WhitelistUserIDs, userId)
}

// IsStatusCodeExcluded 判断状态码是否被排除。
func (s *ErrorBanSetting) IsStatusCodeExcluded(statusCode int) bool {
	for _, code := range s.ExcludeStatusCodes {
		if code == statusCode {
			return true
		}
	}
	return false
}

// ResolveDimension 返回规则维度，空则回退到全局默认维度。
func (s *ErrorBanSetting) ResolveDimension(ruleDimension string) string {
	if ruleDimension == DimensionIP || ruleDimension == DimensionUser {
		return ruleDimension
	}
	return s.DefaultDimension
}

// MatchTier 返回不超过 offenseCount 的最高阶梯。
func (s *ErrorBanSetting) MatchTier(offenseCount int) (ErrorBanTier, bool) {
	var matched *ErrorBanTier
	for i := range s.Tiers {
		tier := &s.Tiers[i]
		if tier.OffenseCount > offenseCount {
			continue
		}
		if matched == nil || tier.OffenseCount > matched.OffenseCount {
			matched = tier
		}
	}
	if matched == nil {
		return ErrorBanTier{}, false
	}
	return *matched, true
}

// CompiledRule 是预编译后的规则快照，供检测路径无锁读取。
type CompiledRule struct {
	Rule ErrorBanRule
	Re   *regexp.Regexp
}

// compiledRules 保存当前已编译规则快照，使用 atomic.Value 支持热更新。
var compiledRules atomic.Value // []CompiledRule

// RebuildRegexCache 依据当前配置重建正则缓存。
// 任一规则编译失败时保留旧快照并返回错误，保证检测路径始终可用。
func RebuildRegexCache() error {
	snapshot := GetErrorBanSetting()
	compiled := make([]CompiledRule, 0, len(snapshot.Rules))
	for _, rule := range snapshot.Rules {
		if !rule.Enabled || strings.TrimSpace(rule.Pattern) == "" {
			continue
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return err
		}
		compiled = append(compiled, CompiledRule{Rule: rule, Re: re})
	}
	compiledRules.Store(compiled)
	return nil
}

// GetCompiledRules 返回当前已编译规则快照。
func GetCompiledRules() []CompiledRule {
	v := compiledRules.Load()
	if v == nil {
		return nil
	}
	rules, ok := v.([]CompiledRule)
	if !ok {
		return nil
	}
	return rules
}
