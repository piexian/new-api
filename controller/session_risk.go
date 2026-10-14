package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/gin-gonic/gin"
)

// GetSessionRiskConfig 返回会话风控配置（含会话标识采集开关）。
func GetSessionRiskConfig(c *gin.Context) {
	common.ApiSuccess(c, risk_setting.GetRiskCenterSetting())
}

// UpdateSessionRiskConfig 更新会话标识采集开关，其余风控中心字段保持原值。
func UpdateSessionRiskConfig(c *gin.Context) {
	var req struct {
		SessionRecordingEnabled *bool `json:"session_recording_enabled"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.SessionRecordingEnabled == nil {
		common.ApiError(c, errors.New("session_recording_enabled is required"))
		return
	}
	merged := risk_setting.GetRiskCenterSetting()
	merged.SessionRecordingEnabled = *req.SessionRecordingEnabled
	riskConfigUpdateMu.Lock()
	defer riskConfigUpdateMu.Unlock()
	if err := saveRiskConfig("risk_center_setting.", &merged); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, risk_setting.GetRiskCenterSetting())
}

// ListSharedSessions 分页列出被多个用户共用的会话标识/缓存关联标识。
func ListSharedSessions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	start, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	result, err := service.ListSharedSessions(c.DefaultQuery("kind", service.SessionRiskKindSession),
		pageInfo.GetPage(), pageInfo.GetPageSize(), c.Query("keyword"), start, end,
		c.Query("sort"), c.Query("order"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// BanSharedSessionUsers 批量封禁共用标识关联用户，封禁前在请求携带的时间窗内复核关联关系。
func BanSharedSessionUsers(c *gin.Context) {
	var req service.SessionRiskBanRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.BanSharedSessionUsers(req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}
