package risk_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
)

// RiskCenterSetting 风控中心全局开关，控制 IP 连坐封禁的通知行为与会话标识采集。
type RiskCenterSetting struct {
	NotifyIPCollateralUser  bool `json:"notify_ip_collateral_user"`
	NotifyIPCollateralAdmin bool `json:"notify_ip_collateral_admin"`
	// SessionRecordingEnabled 为 true 时才从原始请求采集会话标识/缓存关联标识写入日志。
	SessionRecordingEnabled bool `json:"session_recording_enabled"`
}

var riskCenterSetting = RiskCenterSetting{
	NotifyIPCollateralUser:  true,
	NotifyIPCollateralAdmin: true,
}

func init() {
	config.GlobalConfig.Register("risk_center_setting", &riskCenterSetting)
}

// GetRiskCenterSetting 返回风控中心全局配置副本。
func GetRiskCenterSetting() RiskCenterSetting {
	return riskCenterSetting
}


// SetSessionRecordingEnabled 切换会话标识采集开关，返回原值（主要用于测试）。
func SetSessionRecordingEnabled(enabled bool) bool {
	original := riskCenterSetting.SessionRecordingEnabled
	riskCenterSetting.SessionRecordingEnabled = enabled
	return original
}
// GetRiskCenterAppealHint 返回风控封禁通知使用的默认申诉提示。
func GetRiskCenterAppealHint() string {
	return "如认为误封，请联系管理员。"
}
