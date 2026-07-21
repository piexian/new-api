package risk_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
)

// RiskCenterSetting 风控中心全局开关，目前控制 IP 连坐封禁的通知行为。
type RiskCenterSetting struct {
	NotifyIPCollateralUser  bool `json:"notify_ip_collateral_user"`
	NotifyIPCollateralAdmin bool `json:"notify_ip_collateral_admin"`
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

// GetRiskCenterAppealHint 返回风控封禁通知使用的默认申诉提示。
func GetRiskCenterAppealHint() string {
	return "如认为误封，请联系管理员。"
}
