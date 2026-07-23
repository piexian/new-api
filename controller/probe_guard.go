package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/gin-gonic/gin"
)

// saveRiskConfig 将配置结构体的每个字段以 "<prefix>.<jsonKey>" 持久化到 options，
// 并通过 handleConfigUpdate 同步刷新内存配置。
func saveRiskConfig(prefix string, cfg interface{}) error {
	configMap, err := config.ConfigToMap(cfg)
	if err != nil {
		return err
	}
	values := make(map[string]string, len(configMap))
	for key, value := range configMap {
		values[prefix+key] = value
	}
	return model.UpdateOptionsBulk(values)
}

// GetProbeGuardConfig 返回探测防护配置。
func GetProbeGuardConfig(c *gin.Context) {
	common.ApiSuccess(c, risk_setting.GetProbeGuardSetting())
}

// UpdateProbeGuardConfig 校验并保存探测防护配置。
func UpdateProbeGuardConfig(c *gin.Context) {
	var req risk_setting.ProbeGuardSetting
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Normalize()
	riskConfigUpdateMu.Lock()
	defer riskConfigUpdateMu.Unlock()
	if err := saveRiskConfig("probe_guard_setting.", &req); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, risk_setting.GetProbeGuardSetting())
}

// ListProbeIPOffenses 分页查询 IP 违规记录。
func ListProbeIPOffenses(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	states, total, err := model.ListProbeIPAbuseStates(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(states)
	common.ApiSuccess(c, pageInfo)
}

// ListProbeUserOffenses 分页查询用户违规记录。
func ListProbeUserOffenses(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	states, total, err := model.ListProbeUserAbuseStates(c.Query("keyword"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(states)
	common.ApiSuccess(c, pageInfo)
}

// ResetProbeIPOffense 删除某 IP 的探测违规记录。
func ResetProbeIPOffense(c *gin.Context) {
	ip := strings.TrimSpace(c.Param("ip"))
	if ip == "" {
		common.ApiErrorMsg(c, "IP 不能为空")
		return
	}
	if err := model.ResetProbeIPAbuse(ip); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ClearRiskLiveProgress(service.RiskLiveSourceProbeGuard, risk_setting.DimensionIP, ip)
	common.ApiSuccess(c, gin.H{"ip": ip})
}

// isRiskBanReason 判断封禁原因是否由风控自动封禁写入。
func isRiskBanReason(reason string) bool {
	return strings.Contains(reason, "自动封禁")
}

// UnbanProbeUser 解封被风控自动封禁的用户，并清理缓存与违规计数。
func UnbanProbeUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Status != common.UserStatusDisabled {
		common.ApiErrorMsg(c, "该用户当前未被禁用")
		return
	}
	// 仅解封当前禁用原因与风控记录匹配的账号，避免误解除管理员手动封禁。
	hasLog, _ := model.HasCurrentRiskBanLogForUser(id, user.DisableReason)
	if !isRiskBanReason(user.DisableReason) && !hasLog {
		common.ApiErrorMsg(c, "该用户不是被风控自动封禁的，请通过用户管理手动解禁")
		return
	}
	if _, err := model.EnableUserByRiskBan(id); err != nil {
		common.ApiError(c, err)
		return
	}
	_ = model.InvalidateUserCache(id)
	_ = model.InvalidateUserTokensCache(id)
	_ = model.ResetProbeUserAbuse(id)
	service.ClearRiskLiveProgress(service.RiskLiveSourceProbeGuard, risk_setting.DimensionUser, strconv.Itoa(id))
	common.ApiSuccess(c, gin.H{"id": id})
}

// ProbeGuardStats 返回探测防护统计数据。
func ProbeGuardStats(c *gin.Context) {
	stats, err := model.GetProbeGuardStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stats)
}
