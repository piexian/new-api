package controller

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/gin-gonic/gin"
)

// 规则测试端点的输入长度上限，防止 ReDoS 与超大请求。
const (
	errorBanTestPatternMaxLen = 1024
	errorBanTestSampleMaxLen  = 4096
)

// GetErrorBanConfig 返回错误封禁配置。
func GetErrorBanConfig(c *gin.Context) {
	common.ApiSuccess(c, risk_setting.GetErrorBanSetting())
}

// validateErrorBanSetting 校验规则正则、维度与阶梯动作，返回首个错误。
func validateErrorBanSetting(req *risk_setting.ErrorBanSetting) error {
	if len(req.Rules) > risk_setting.MaxErrorBanRules {
		return fmt.Errorf("规则数量不能超过 %d 条", risk_setting.MaxErrorBanRules)
	}
	for _, rule := range req.Rules {
		if strings.TrimSpace(rule.Id) == "" {
			return fmt.Errorf("规则 ID 不能为空")
		}
		if rule.Enabled && strings.TrimSpace(rule.Pattern) != "" {
			if _, err := regexp.Compile(rule.Pattern); err != nil {
				return fmt.Errorf("规则 %s 的正则无效: %s", rule.Id, err.Error())
			}
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
	Pattern    string `json:"pattern"`
	SampleText string `json:"sample_text"`
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
	re, err := regexp.Compile(req.Pattern)
	if err != nil {
		common.ApiSuccess(c, gin.H{"valid": false, "matched": false, "error": err.Error()})
		return
	}
	common.ApiSuccess(c, gin.H{
		"valid":   true,
		"matched": re.MatchString(req.SampleText),
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
