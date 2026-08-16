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
	// 信用分档位提额上限默认值：[-50,$2] [0,$5] [60,$10] [80,$20]（QuotaPerUnit=500000）
	assert.Equal(t, []CreditTierLimit{
		{MinScore: -50, MaxTotal: 1000000},
		{MinScore: 0, MaxTotal: 2500000},
		{MinScore: 60, MaxTotal: 5000000},
		{MinScore: 80, MaxTotal: 10000000},
	}, s.CreditTierLimits)
}

func TestGetCreditTierMaxTotal(t *testing.T) {
	s := &LoanSetting{CreditTierLimits: []CreditTierLimit{
		{MinScore: -50, MaxTotal: 1000000}, // $2
		{MinScore: 0, MaxTotal: 2500000},   // $5
		{MinScore: 60, MaxTotal: 5000000},  // $10
		{MinScore: 80, MaxTotal: 10000000}, // $20
	}}
	cases := []struct {
		score int
		want  int64
	}{
		{-50, 1000000},  // 命中 -50 档（最低档）
		{-1, 1000000},   // 未到 0 档，仍取 -50 档
		{0, 2500000},    // 0 档
		{59, 2500000},   // 未到 60 档
		{60, 5000000},   // 60 档
		{80, 10000000},  // 80 档
		{100, 10000000}, // 超过最高档，取最高档
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.want, GetCreditTierMaxTotal(s, tc.score), "score %d", tc.score)
	}
}

func TestGetCreditTierMaxTotalEmptyFallsBackToAiMaxLimit(t *testing.T) {
	s := &LoanSetting{AiMaxLimit: 10000000}
	assert.Equal(t, int64(10000000), GetCreditTierMaxTotal(s, 0))
	assert.Equal(t, int64(10000000), GetCreditTierMaxTotal(s, 100))
	// nil 配置同样回退全局默认 AiMaxLimit（历史行为）
	assert.Equal(t, GetLoanSetting().AiMaxLimit, GetCreditTierMaxTotal(nil, 0))
}

func TestGetCreditTierMaxTotalUnsortedAndBelowAll(t *testing.T) {
	// 未排序配置：先按 MinScore 升序再查档
	s := &LoanSetting{CreditTierLimits: []CreditTierLimit{
		{MinScore: 60, MaxTotal: 5000000},
		{MinScore: 0, MaxTotal: 2500000},
	}}
	// score 低于全部档位 → 升序后的第一档（min_score=0 档）
	assert.Equal(t, int64(2500000), GetCreditTierMaxTotal(s, -10))
	assert.Equal(t, int64(5000000), GetCreditTierMaxTotal(s, 70))
	// 原配置顺序不被改动（排序作用于副本）
	assert.Equal(t, 60, s.CreditTierLimits[0].MinScore)
}

func TestLoanAiPromptMatchesSpec53(t *testing.T) {
	// 默认 AI prompt 的结案 action 枚举必须与 spec 5.3 白名单一致（只认 "close"）
	p := GetLoanSetting().AiPrompt
	assert.Contains(t, p, `"action":"close"`)
	assert.NotContains(t, p, "approve|reject")
	// 硬边界占位符由 service 层注入（USD/百分比/天数文案），模板自身不带单位字样
	assert.Contains(t, p, "{{ai_max_limit}}")
	assert.Contains(t, p, "{{ai_min_rate}}")
	assert.Contains(t, p, "{{ai_max_grace_days}}")
	assert.NotContains(t, p, "{{ai_max_limit}} quota")
}

func TestLoanSettingRegisteredInGlobalConfig(t *testing.T) {
	// 注册后应可通过 GlobalConfig 读取
	val := config.GlobalConfig.Get("loan_setting")
	require.NotNil(t, val, "loan_setting should be registered in GlobalConfig")
	require.Same(t, GetLoanSetting(), val)
}

func TestLoanMarketSettingDefaults(t *testing.T) {
	// 市场配置默认值，见 docs/specs/2026-08-15-loan-marketplace-design.md 第 12 节
	s := GetLoanSetting()
	require.NotNil(t, s)
	assert.False(t, s.MarketEnabled)
	assert.Equal(t, int64(50000), s.LenderMinAmount) // 50000 quota = 0.10 USD @ QuotaPerUnit=500000
	assert.Equal(t, 0.0005, s.LenderRateMin)
	assert.Equal(t, 0.003, s.LenderRateMax)
	assert.Equal(t, int64(0), s.PerLoanCapDefault) // 0 = 不限
	assert.Equal(t, 5, s.MaxFundingsPerBorrow)
	assert.Equal(t, 30, s.LoanTermDays)
	assert.Equal(t, 30, s.BlacklistDaysOnDefault)
	assert.Equal(t, 2.0, s.OverduePenaltyMultiplier)
	assert.Equal(t, 50, s.CreditInitial)
	assert.Equal(t, 5, s.CreditRepayBonus)
	assert.Equal(t, 2, s.CreditFastRepayPenalty)
	assert.Equal(t, 20, s.CreditDefaultPenalty)
	assert.Equal(t, 3, s.CreditMinHoldDays)
	assert.Equal(t, 1.0, s.CreditMinBorrowUsd)
}

func TestValidateLoanMarketSetting(t *testing.T) {
	valid := func() *LoanSetting {
		return &LoanSetting{
			DailyRate:            0.001,
			LenderRateMin:        0.0005,
			LenderRateMax:        0.003,
			MaxFundingsPerBorrow: 5,
		}
	}

	// 默认配置（含市场字段）与全部合法的配置应总是通过
	assert.NoError(t, ValidateLoanMarketSetting(GetLoanSetting()))
	assert.NoError(t, ValidateLoanMarketSetting(valid()))

	// lender_rate_min <= 0 拒绝
	bad := valid()
	bad.LenderRateMin = 0
	assert.Error(t, ValidateLoanMarketSetting(bad))
	bad = valid()
	bad.LenderRateMin = -0.0005
	assert.Error(t, ValidateLoanMarketSetting(bad))

	// lender_rate_min >= daily_rate 拒绝（必须严格低于官方日利率）
	bad = valid()
	bad.LenderRateMin = 0.001
	assert.Error(t, ValidateLoanMarketSetting(bad))
	bad = valid()
	bad.LenderRateMin = 0.002
	assert.Error(t, ValidateLoanMarketSetting(bad))

	// lender_rate_min > lender_rate_max 拒绝；相等时允许（单一利率定价）
	bad = valid()
	bad.LenderRateMin = 0.005
	assert.Error(t, ValidateLoanMarketSetting(bad))
	equal := valid()
	equal.LenderRateMin = 0.0008
	equal.LenderRateMax = 0.0008
	assert.NoError(t, ValidateLoanMarketSetting(equal))

	// max_fundings_per_borrow 必须在 1~10 之间（默认 5 合法）
	min := valid()
	min.MaxFundingsPerBorrow = 1
	assert.NoError(t, ValidateLoanMarketSetting(min))
	max := valid()
	max.MaxFundingsPerBorrow = 10
	assert.NoError(t, ValidateLoanMarketSetting(max))
	bad = valid()
	bad.MaxFundingsPerBorrow = 0
	assert.Error(t, ValidateLoanMarketSetting(bad))
	bad = valid()
	bad.MaxFundingsPerBorrow = 11
	assert.Error(t, ValidateLoanMarketSetting(bad))

	// nil 指针不应 panic，返回错误
	assert.Error(t, ValidateLoanMarketSetting(nil))
}

func TestValidateLoanMarketSettingRateBounds(t *testing.T) {
	valid := func() *LoanSetting {
		return &LoanSetting{
			DailyRate:            0.001,
			LenderRateMin:        0.0005,
			LenderRateMax:        0.003,
			MaxFundingsPerBorrow: 5,
		}
	}

	// 边界值：10%/天 合法
	edge := valid()
	edge.DailyRate = 0.1
	assert.NoError(t, ValidateLoanMarketSetting(edge))

	// 官方日利率：0 / 负值 / 超上界一律拒绝
	bad := valid()
	bad.DailyRate = 0
	assert.Error(t, ValidateLoanMarketSetting(bad))
	bad = valid()
	bad.DailyRate = 0.2
	assert.Error(t, ValidateLoanMarketSetting(bad))

	// 放贷利率上限超界拒绝
	bad = valid()
	bad.LenderRateMax = 0.5
	assert.Error(t, ValidateLoanMarketSetting(bad))

	// AI 最低日利率 / 提前还款手续费率：0 允许（不收），超上界拒绝
	ok := valid()
	ok.AiMinRate = 0
	ok.RepayFeeRate = 0
	assert.NoError(t, ValidateLoanMarketSetting(ok))
	bad = valid()
	bad.AiMinRate = 0.2
	assert.Error(t, ValidateLoanMarketSetting(bad))
	bad = valid()
	bad.RepayFeeRate = -0.001
	assert.Error(t, ValidateLoanMarketSetting(bad))
}
