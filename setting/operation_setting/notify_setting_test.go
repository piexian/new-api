package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

// NotifySetting 默认值：所有管理端通知默认开启，保持既有行为不变。
func TestNotifySettingDefaults(t *testing.T) {
	s := GetNotifySetting()
	require.True(t, s.ChannelAutoDisabled)
	require.True(t, s.ChannelAutoEnabled)
	require.True(t, s.ChannelQuotaCooldown)
	require.True(t, s.ChannelTestResult)
	require.True(t, s.LoanLenderOverflow)
}

// UpdateConfigFromMap 直写路径能正确落地布尔开关（与保存 API 同链路）。
func TestNotifySettingUpdateFromMap(t *testing.T) {
	old := *GetNotifySetting()
	t.Cleanup(func() { *GetNotifySetting() = old })

	config.UpdateConfigFromMap(GetNotifySetting(), map[string]string{
		"channel_auto_disabled": "false",
		"loan_lender_overflow":  "false",
	})
	require.False(t, GetNotifySetting().ChannelAutoDisabled)
	require.False(t, GetNotifySetting().LoanLenderOverflow)
	require.True(t, GetNotifySetting().ChannelAutoEnabled)
}
