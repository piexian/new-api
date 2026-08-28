package operation_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
)

// NotifySetting 管理员通知分类型开关。
// 这些事件会直达 root 的个人通知渠道（email/webhook/bark/gotify），
// 每类事件独立开关；默认全部开启，保持既有行为不变。
type NotifySetting struct {
	ChannelAutoDisabled  bool `json:"channel_auto_disabled"`  // 通道自动禁用通知
	ChannelAutoEnabled   bool `json:"channel_auto_enabled"`   // 通道自动恢复通知
	ChannelQuotaCooldown bool `json:"channel_quota_cooldown"` // 通道套餐限额冷却通知
	ChannelTestResult    bool `json:"channel_test_result"`    // 通道测试完成汇总通知
	LoanLenderOverflow   bool `json:"loan_lender_overflow"`   // 词元贷放贷人入账溢出通知（资金安全类，建议保持开启）
}

var notifySetting = NotifySetting{
	ChannelAutoDisabled:  true,
	ChannelAutoEnabled:   true,
	ChannelQuotaCooldown: true,
	ChannelTestResult:    true,
	LoanLenderOverflow:   true,
}

func init() {
	config.GlobalConfig.Register("notify_setting", &notifySetting)
}

// GetNotifySetting 获取通知开关配置
func GetNotifySetting() *NotifySetting {
	return &notifySetting
}
