package controller

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/gin-gonic/gin"
)

// 规则测试端点的输入长度上限，防止 ReDoS 与超大请求。
const (
	errorBanTestPatternMaxLen = 1024
	errorBanTestSampleMaxLen  = 4096
	errorBanTestMatcherMaxLen = 256
)

// GetErrorBanConfig 返回错误封禁配置。
func GetErrorBanConfig(c *gin.Context) {
	common.ApiSuccess(c, risk_setting.GetErrorBanSetting())
}

// validateErrorBanSetting 校验规则正则、维度与阶梯动作，返回首个错误。
func validateErrorBanSetting(req *risk_setting.ErrorBanSetting) error {
	if req.WindowSeconds < 10 || req.WindowSeconds > 86400 {
		return fmt.Errorf("窗口时间必须在 10 到 86400 秒之间")
	}
	if req.DefaultDimension != risk_setting.DimensionIP && req.DefaultDimension != risk_setting.DimensionUser {
		return fmt.Errorf("默认封禁维度无效")
	}
	if len(req.Rules) > risk_setting.MaxErrorBanRules {
		return fmt.Errorf("规则数量不能超过 %d 条", risk_setting.MaxErrorBanRules)
	}
	seenRuleIDs := make(map[string]struct{}, len(req.Rules))
	for _, rule := range req.Rules {
		ruleID := strings.TrimSpace(rule.Id)
		if ruleID == "" {
			return fmt.Errorf("规则 ID 不能为空")
		}
		if _, exists := seenRuleIDs[ruleID]; exists {
			return fmt.Errorf("规则 ID %s 重复", ruleID)
		}
		seenRuleIDs[ruleID] = struct{}{}
		if len([]rune(rule.Pattern)) > errorBanTestPatternMaxLen {
			return fmt.Errorf("规则 %s 的正则表达式过长", rule.Id)
		}
		if rule.Enabled && strings.TrimSpace(rule.Pattern) == "" && len(rule.Keywords) == 0 && len(rule.ErrorCodes) == 0 {
			return fmt.Errorf("规则 %s 至少需要正则、关键词或错误码中的一种匹配条件", rule.Id)
		}
		if strings.TrimSpace(rule.Pattern) != "" {
			if _, err := regexp.Compile(rule.Pattern); err != nil {
				return fmt.Errorf("规则 %s 的正则无效: %s", rule.Id, err.Error())
			}
		}
		if rule.Threshold < 1 || rule.Threshold > 100000 {
			return fmt.Errorf("规则 %s 的阈值必须在 1 到 100000 之间", rule.Id)
		}
		if rule.Dimension != "" && rule.Dimension != risk_setting.DimensionIP && rule.Dimension != risk_setting.DimensionUser {
			return fmt.Errorf("规则 %s 的封禁维度无效", rule.Id)
		}
		if len(rule.Keywords) > risk_setting.MaxErrorBanMatchersPerRule || len(rule.ErrorCodes) > risk_setting.MaxErrorBanMatchersPerRule {
			return fmt.Errorf("规则 %s 的关键词和错误码分别不能超过 %d 个", rule.Id, risk_setting.MaxErrorBanMatchersPerRule)
		}
		for _, keyword := range rule.Keywords {
			if strings.TrimSpace(keyword) == "" || len([]rune(keyword)) > errorBanTestMatcherMaxLen {
				return fmt.Errorf("规则 %s 的关键词无效或过长", rule.Id)
			}
		}
		for _, code := range rule.ErrorCodes {
			if strings.TrimSpace(code) == "" || len([]rune(code)) > errorBanTestMatcherMaxLen {
				return fmt.Errorf("规则 %s 的错误码无效或过长", rule.Id)
			}
		}
		if err := validateErrorBanTiers(rule.Id, rule.Tiers); err != nil {
			return err
		}
	}
	return validateErrorBanTiers("旧版全局配置", req.Tiers)
}

func validateErrorBanTiers(ruleID string, tiers []risk_setting.ErrorBanTier) error {
	for _, tier := range tiers {
		if tier.OffenseCount < 1 || tier.OffenseCount > 100000 {
			return fmt.Errorf("规则 %s 的阶梯违规次数必须在 1 到 100000 之间", ruleID)
		}
		switch tier.Action {
		case risk_setting.TierActionTempIPBan, risk_setting.TierActionPermIPBan,
			risk_setting.TierActionDisableUser, risk_setting.TierActionBoth:
		default:
			return fmt.Errorf("规则 %s 的阶梯处罚动作无效", ruleID)
		}
		if tier.DurationMinutes < 0 || tier.DurationMinutes > 525600 {
			return fmt.Errorf("规则 %s 的阶梯封禁时长必须在 0 到 525600 分钟之间", ruleID)
		}
		if tier.Action == risk_setting.TierActionTempIPBan && tier.DurationMinutes == 0 {
			return fmt.Errorf("规则 %s 的临时 IP 封禁时长必须大于 0", ruleID)
		}
	}
	return nil
}

// UpdateErrorBanConfig 校验并保存错误封禁配置，随后重建正则缓存。
func UpdateErrorBanConfig(c *gin.Context) {
	var req risk_setting.ErrorBanSetting
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateErrorBanSetting(&req); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	req.Normalize()
	riskConfigUpdateMu.Lock()
	defer riskConfigUpdateMu.Unlock()
	if err := saveRiskConfig("error_ban_setting.", &req); err != nil {
		common.ApiError(c, err)
		return
	}
	// handleConfigUpdate 会在加载时重建缓存，这里显式重建以保证即时生效。
	if err := risk_setting.RebuildRegexCache(); err != nil {
		common.SysError("failed to rebuild error ban regex cache after update: " + err.Error())
	}
	common.ApiSuccess(c, risk_setting.GetErrorBanSetting())
}

// TestErrorBanRuleRequest 规则测试请求体。
type TestErrorBanRuleRequest struct {
	Pattern    string   `json:"pattern"`
	Keywords   []string `json:"keywords"`
	ErrorCodes []string `json:"error_codes"`
	SampleText string   `json:"sample_text"`
	ErrorCode  string   `json:"error_code"`
}

// TestErrorBanRule 编译并测试正则匹配，无任何副作用。
func TestErrorBanRule(c *gin.Context) {
	var req TestErrorBanRuleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len([]rune(req.Pattern)) > errorBanTestPatternMaxLen {
		common.ApiErrorMsg(c, "正则表达式过长")
		return
	}
	if len([]rune(req.SampleText)) > errorBanTestSampleMaxLen {
		common.ApiErrorMsg(c, "测试文本过长")
		return
	}
	if len([]rune(req.ErrorCode)) > errorBanTestMatcherMaxLen {
		common.ApiErrorMsg(c, "测试错误码过长")
		return
	}
	if len(req.Keywords) > risk_setting.MaxErrorBanMatchersPerRule || len(req.ErrorCodes) > risk_setting.MaxErrorBanMatchersPerRule {
		common.ApiErrorMsg(c, fmt.Sprintf("关键词和错误码分别不能超过 %d 个", risk_setting.MaxErrorBanMatchersPerRule))
		return
	}
	for _, matcher := range append(append([]string{}, req.Keywords...), req.ErrorCodes...) {
		if len([]rune(matcher)) > errorBanTestMatcherMaxLen {
			common.ApiErrorMsg(c, "关键词或错误码过长")
			return
		}
	}
	rule := risk_setting.ErrorBanRule{Pattern: req.Pattern, Keywords: req.Keywords, ErrorCodes: req.ErrorCodes}
	testSetting := risk_setting.ErrorBanSetting{WindowSeconds: 300, DefaultDimension: risk_setting.DimensionIP, Rules: []risk_setting.ErrorBanRule{rule}}
	testSetting.Normalize()
	rule = testSetting.Rules[0]
	if strings.TrimSpace(rule.Pattern) == "" && len(rule.Keywords) == 0 && len(rule.ErrorCodes) == 0 {
		common.ApiSuccess(c, gin.H{"valid": false, "matched": false, "error": "至少需要一种匹配条件"})
		return
	}
	var re *regexp.Regexp
	if strings.TrimSpace(rule.Pattern) != "" {
		var err error
		re, err = regexp.Compile(rule.Pattern)
		if err != nil {
			common.ApiSuccess(c, gin.H{"valid": false, "matched": false, "error": err.Error()})
			return
		}
	}
	compiled := risk_setting.CompiledRule{Rule: rule, Re: re}
	common.ApiSuccess(c, gin.H{
		"valid":   true,
		"matched": compiled.Matches(req.SampleText, strings.TrimSpace(req.ErrorCode)),
	})
}

// ListErrorBanIPStates 分页查询 IP 错误封禁状态。
func ListErrorBanIPStates(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	states, total, err := model.ListErrorBanIPStates(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(states)
	common.ApiSuccess(c, pageInfo)
}

// ListErrorBanUserStates 分页查询用户错误封禁状态。
func ListErrorBanUserStates(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	states, total, err := model.ListErrorBanUserStates(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(states)
	common.ApiSuccess(c, pageInfo)
}

// ResetErrorBanIPState 删除某 IP 的错误封禁状态。
func ResetErrorBanIPState(c *gin.Context) {
	ip := strings.TrimSpace(c.Param("ip"))
	if ip == "" {
		common.ApiErrorMsg(c, "IP 不能为空")
		return
	}
	if err := model.ResetErrorBanIPStatesByIP(ip); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ClearRiskLiveProgress(service.RiskLiveSourceErrorBan, risk_setting.DimensionIP, ip)
	common.ApiSuccess(c, gin.H{"ip": ip})
}

// ResetErrorBanUserState 删除某用户的错误封禁状态。
func ResetErrorBanUserState(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	if err := model.ResetErrorBanUserStatesByUser(id); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ClearRiskLiveProgress(service.RiskLiveSourceErrorBan, risk_setting.DimensionUser, strconv.Itoa(id))
	common.ApiSuccess(c, gin.H{"id": id})
}

// ErrorBanStats 返回错误封禁统计数据（含启用规则数）。
func ErrorBanStats(c *gin.Context) {
	stats, err := model.GetErrorBanStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setting := risk_setting.GetErrorBanSetting()
	var active int64
	for _, rule := range setting.Rules {
		if rule.Enabled {
			active++
		}
	}
	stats.ActiveRules = active
	common.ApiSuccess(c, stats)
}
