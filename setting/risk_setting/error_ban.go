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

// 配置数量上限，避免配置膨胀与匹配开销失控。
const (
	MaxErrorBanRules           = 20
	MaxErrorBanMatchersPerRule = 50
)

// ErrorBanRule 单条错误匹配规则。
type ErrorBanRule struct {
	Id             string         `json:"id"`
	Name           string         `json:"name"`
	Pattern        string         `json:"pattern"`
	Keywords       []string       `json:"keywords"`
	ErrorCodes     []string       `json:"error_codes"`
	Enabled        bool           `json:"enabled"`
	CountRetries   bool           `json:"count_retries"`
	Dimension      string         `json:"dimension"` // 为空时继承全局 DefaultDimension
	Threshold      int            `json:"threshold"`
	ReasonTemplate string         `json:"reason_template"`
	Tiers          []ErrorBanTier `json:"tiers"`
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
	WhitelistGroups       []string       `json:"whitelist_groups"`
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
	WhitelistGroups:    []string{},
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
	snapshot.WhitelistGroups = append([]string{}, errorBanSetting.WhitelistGroups...)
	snapshot.ExcludeStatusCodes = append([]int{}, errorBanSetting.ExcludeStatusCodes...)
	snapshot.Rules = make([]ErrorBanRule, len(errorBanSetting.Rules))
	for i, rule := range errorBanSetting.Rules {
		snapshot.Rules[i] = rule
		snapshot.Rules[i].Keywords = append([]string{}, rule.Keywords...)
		snapshot.Rules[i].ErrorCodes = append([]string{}, rule.ErrorCodes...)
		snapshot.Rules[i].Tiers = append([]ErrorBanTier{}, rule.Tiers...)
	}
	snapshot.Tiers = append([]ErrorBanTier{}, errorBanSetting.Tiers...)
	snapshot.Normalize()
	return snapshot
}

// Normalize 收敛字段到合法区间，并裁剪超额规则。
func (s *ErrorBanSetting) Normalize() {
	s.WindowSeconds = clampInt(s.WindowSeconds, 10, 86400, 300)
	s.WhitelistGroups = normalizeStringList(s.WhitelistGroups)
	if s.DefaultDimension != DimensionIP && s.DefaultDimension != DimensionUser {
		s.DefaultDimension = DimensionIP
	}
	if s.ExcludeStatusCodes == nil {
		s.ExcludeStatusCodes = []int{}
	}
	if len(s.Tiers) == 0 {
		s.Tiers = defaultErrorBanTiers()
	}
	normalizeErrorBanTiers(s.Tiers)
	if s.Rules == nil {
		s.Rules = []ErrorBanRule{}
	}
	if len(s.Rules) > MaxErrorBanRules {
		s.Rules = s.Rules[:MaxErrorBanRules]
	}
	for i := range s.Rules {
		rule := &s.Rules[i]
		rule.Id = strings.TrimSpace(rule.Id)
		rule.Threshold = clampInt(rule.Threshold, 1, 100000, 5)
		rule.Keywords = normalizeStringList(rule.Keywords)
		rule.ErrorCodes = normalizeStringList(rule.ErrorCodes)
		if len(rule.Keywords) > MaxErrorBanMatchersPerRule {
			rule.Keywords = rule.Keywords[:MaxErrorBanMatchersPerRule]
		}
		if len(rule.ErrorCodes) > MaxErrorBanMatchersPerRule {
			rule.ErrorCodes = rule.ErrorCodes[:MaxErrorBanMatchersPerRule]
		}
		if rule.Dimension != "" && rule.Dimension != DimensionIP && rule.Dimension != DimensionUser {
			rule.Dimension = ""
		}
		if len(rule.Tiers) == 0 {
			rule.Tiers = append([]ErrorBanTier{}, s.Tiers...)
		}
		normalizeErrorBanTiers(rule.Tiers)
	}
}

func normalizeErrorBanTiers(tiers []ErrorBanTier) {
	for i := range tiers {
		tiers[i].OffenseCount = clampInt(tiers[i].OffenseCount, 1, 100000, 1)
		if !isValidTierAction(tiers[i].Action) {
			tiers[i].Action = TierActionTempIPBan
		}
		if tiers[i].Action == TierActionTempIPBan && tiers[i].DurationMinutes <= 0 {
			tiers[i].DurationMinutes = 1
		} else if tiers[i].DurationMinutes < 0 {
			tiers[i].DurationMinutes = 0
		} else if tiers[i].DurationMinutes > 525600 {
			tiers[i].DurationMinutes = 525600
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

// IsGroupWhitelisted 判断请求分组是否在白名单中。
func (s *ErrorBanSetting) IsGroupWhitelisted(group string) bool {
	return stringListContains(s.WhitelistGroups, group)
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
func (r *ErrorBanRule) MatchTier(offenseCount int) (ErrorBanTier, bool) {
	var matched *ErrorBanTier
	for i := range r.Tiers {
		tier := &r.Tiers[i]
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

// Matches 对所有已配置条件执行 AND 匹配；错误码列表内任一精确命中或 * 通配即可。
func (r *CompiledRule) Matches(errorText, errorCode string) bool {
	if r.Re != nil && !r.Re.MatchString(errorText) {
		return false
	}
	for _, keyword := range r.Rule.Keywords {
		if !strings.Contains(errorText, keyword) {
			return false
		}
	}
	if len(r.Rule.ErrorCodes) > 0 &&
		!stringListContains(r.Rule.ErrorCodes, "*") &&
		!stringListContains(r.Rule.ErrorCodes, errorCode) {
		return false
	}
	return r.Re != nil || len(r.Rule.Keywords) > 0 || len(r.Rule.ErrorCodes) > 0
}

// compiledRules 保存当前已编译规则快照，使用 atomic.Value 支持热更新。
var compiledRules atomic.Value // []CompiledRule

// RebuildRegexCache 依据当前配置重建正则缓存。
// 任一规则编译失败时保留旧快照并返回错误，保证检测路径始终可用。
func RebuildRegexCache() error {
	snapshot := GetErrorBanSetting()
	compiled := make([]CompiledRule, 0, len(snapshot.Rules))
	for _, rule := range snapshot.Rules {
		if !rule.Enabled {
			continue
		}
		var re *regexp.Regexp
		if strings.TrimSpace(rule.Pattern) != "" {
			var err error
			re, err = regexp.Compile(rule.Pattern)
			if err != nil {
				return err
			}
		}
		if re == nil && len(rule.Keywords) == 0 && len(rule.ErrorCodes) == 0 {
			continue
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
