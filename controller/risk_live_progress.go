package controller

import (
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/gin-gonic/gin"
)

var riskConfigUpdateMu sync.Mutex

type RiskLiveRuleToggleRequest struct {
	Source  string `json:"source"`
	RuleId  string `json:"rule_id"`
	Enabled bool   `json:"enabled"`
}

func ListRiskLiveRules(c *gin.Context) {
	common.ApiSuccess(c, service.GetRiskLiveRuleSummaries())
}

func ListRiskLiveTargets(c *gin.Context) {
	source := strings.TrimSpace(c.Query("source"))
	ruleId := strings.TrimSpace(c.Query("rule_id"))
	dimension := strings.TrimSpace(c.Query("dimension"))
	if source != service.RiskLiveSourceProbeGuard && source != service.RiskLiveSourceErrorBan {
		common.ApiErrorMsg(c, "无效的风控来源")
		return
	}
	if ruleId == "" {
		common.ApiErrorMsg(c, "规则 ID 不能为空")
		return
	}
	if dimension != "" && dimension != risk_setting.DimensionIP && dimension != risk_setting.DimensionUser {
		common.ApiErrorMsg(c, "无效的目标维度")
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total := service.GetRiskLiveTargets(
		source,
		ruleId,
		dimension,
		c.Query("keyword"),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	pageInfo.SetTotal(total)
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func saveRiskConfigField(prefix, key string, cfg interface{}) error {
	configMap, err := config.ConfigToMap(cfg)
	if err != nil {
		return err
	}
	value, ok := configMap[key]
	if !ok {
		return nil
	}
	return model.UpdateOptionsBulk(map[string]string{prefix + key: value})
}

func ToggleRiskLiveRule(c *gin.Context) {
	var req RiskLiveRuleToggleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Source = strings.TrimSpace(req.Source)
	req.RuleId = strings.TrimSpace(req.RuleId)

	riskConfigUpdateMu.Lock()
	defer riskConfigUpdateMu.Unlock()

	switch req.Source {
	case service.RiskLiveSourceProbeGuard:
		if req.RuleId != service.RiskLiveProbeGuardRuleID {
			common.ApiErrorMsg(c, "探针防护规则 ID 无效")
			return
		}
		setting := risk_setting.GetProbeGuardSetting()
		setting.Enabled = req.Enabled
		if err := saveRiskConfigField("probe_guard_setting.", "enabled", &setting); err != nil {
			common.ApiError(c, err)
			return
		}
	case service.RiskLiveSourceErrorBan:
		setting := risk_setting.GetErrorBanSetting()
		found := false
		for i := range setting.Rules {
			if setting.Rules[i].Id == req.RuleId {
				setting.Rules[i].Enabled = req.Enabled
				found = true
				break
			}
		}
		if !found {
			common.ApiErrorMsg(c, "风控规则不存在")
			return
		}
		if err := saveRiskConfigField("error_ban_setting.", "rules", &setting); err != nil {
			common.ApiError(c, err)
			return
		}
		if err := risk_setting.RebuildRegexCache(); err != nil {
			common.SysError("failed to rebuild error ban regex cache after toggle: " + err.Error())
		}
	default:
		common.ApiErrorMsg(c, "无效的风控来源")
		return
	}

	common.ApiSuccess(c, gin.H{
		"source":  req.Source,
		"rule_id": req.RuleId,
		"enabled": req.Enabled,
	})
}
