package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoanSettingDefaults(t *testing.T) {
	s := GetLoanSetting()
	require.NotNil(t, s)
	assert.False(t, s.Enabled)
	assert.Equal(t, int64(2500000), s.MaxTotal) // $5
	assert.Equal(t, 0.001, s.DailyRate)
	assert.Equal(t, int64(10000000), s.AiMaxLimit) // $20
	assert.Equal(t, 0.0005, s.AiMinRate)
	assert.Equal(t, 30, s.AiMaxGraceDays)
	assert.Equal(t, 10, s.AiMaxRounds)
	assert.Equal(t, 3, s.AiDailyLimit)
	assert.Equal(t, 1, s.AiMaxActiveApplications)
	assert.Equal(t, 2048, s.AiMaxOutput)
	assert.True(t, s.CheckinRepayEnabled)
	assert.True(t, s.TermsEnabled)
	assert.NotEmpty(t, s.TermsText)
	assert.NotEmpty(t, s.AiPrompt)
	assert.Equal(t, 0, s.MinRegisterDays)
	assert.Equal(t, int64(0), s.MaxPerBorrow)
	assert.False(t, s.AiEnabled)
}

func TestLoanAiPromptMatchesSpec53(t *testing.T) {
	// 默认 AI prompt 的结案 action 枚举必须与 spec 5.3 白名单一致（只认 "close"）
	p := GetLoanSetting().AiPrompt
	assert.Contains(t, p, `"action":"close"`)
	assert.NotContains(t, p, "approve|reject")
	// 硬边界占位符由 service 层注入
	assert.Contains(t, p, "{{ai_max_limit}}")
	assert.Contains(t, p, "{{ai_min_rate}}")
	assert.Contains(t, p, "{{ai_max_grace_days}}")
}

func TestLoanSettingRegisteredInGlobalConfig(t *testing.T) {
	// 注册后应可通过 GlobalConfig 读取
	val := config.GlobalConfig.Get("loan_setting")
	require.NotNil(t, val, "loan_setting should be registered in GlobalConfig")
	require.Same(t, GetLoanSetting(), val)
}
