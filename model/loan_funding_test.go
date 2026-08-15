package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// mkTestFunding 构造内存 funding（惰性结算测试隔离 DB，直接指定时钟字段）
func mkTestFunding(src, plan string, debt, principal int64, lastSettled int, rate float64) TokenLoanFunding {
	return TokenLoanFunding{
		SourceType:         src,
		PrincipalRemaining: principal,
		DebtQuota:          debt,
		LastSettledDay:     lastSettled,
		Rate:               rate,
		RepayPlan:          plan,
		Status:             LoanFundingActive,
	}
}

// mkTestAcc 构造内存账户（无 DB 落盘，仅提供利率/宽限投影输入）
func mkTestAcc(debt, principal int64, lastSettled int) *TokenLoanAccount {
	return &TokenLoanAccount{
		PrincipalQuota: principal,
		DebtQuota:      debt,
		LastSettledDay: lastSettled,
	}
}

func TestSettleFunding(t *testing.T) {
	withDailyRate(t, 0.001)

	t.Run("normal compound parity with account settle", func(t *testing.T) {
		// 单 platform funding 与既有 settle() 对拍：同起息日、同本金、同利率
		start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
		now := start.AddDate(0, 0, 3)
		acc := mkTestAcc(1000000, 1000000, loanDay(start))
		f := mkTestFunding(LoanFundingPlatform, LoanRepayFull, 1000000, 1000000, loanDay(start), 0.001)
		f.DueDay = loanDay(now) + 30 // 未逾期

		accCopy := *acc
		settle(&accCopy, now)
		settleFunding(&f, acc, now)

		require.Equal(t, accCopy.DebtQuota, f.DebtQuota)
		require.Equal(t, int64(1003003), f.DebtQuota) // round(1000000 * 1.001^3)
		require.Equal(t, loanDay(now), f.LastSettledDay)
		require.Equal(t, int64(1000000), f.PrincipalRemaining) // 本金不动
	})

	t.Run("overdue full plan applies penalty multiplier", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		f := mkTestFunding(LoanFundingPool, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
		f.DueDay = today - 1 // 昨天到期

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		require.Equal(t, int64(1006012), f.DebtQuota) // round(1000000 * 1.002^3)，2 倍罚息
		require.Equal(t, today, f.LastSettledDay)
	})

	t.Run("no_penalty plan not penalized when overdue", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		f := mkTestFunding(LoanFundingPool, LoanRepayNoPenalty, 1000000, 1000000, today-3, 0.001)
		f.DueDay = today - 1 // 已逾期，但 plan=no_penalty 不乘罚息倍率

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		require.Equal(t, int64(1003003), f.DebtQuota) // round(1000000 * 1.001^3)，基础利率
		require.Equal(t, today, f.LastSettledDay)
	})

	t.Run("interest_freeze plan never grows", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		f := mkTestFunding(LoanFundingPool, LoanRepayInterestFreeze, 1000000, 900000, today-5, 0.001)
		f.DueDay = today - 2 // 已逾期

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		require.Equal(t, int64(1000000), f.DebtQuota) // 冻结不增长
		require.Equal(t, today, f.LastSettledDay)     // 时钟仍推进
	})

	t.Run("principal_only plan frozen at principal", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		// 改档时已核销未付利息（Task 14），此处 debt == principal
		f := mkTestFunding(LoanFundingPool, LoanRepayPrincipalOnly, 800000, 800000, today-4, 0.001)
		f.DueDay = today - 1 // 已逾期

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		require.Equal(t, int64(800000), f.DebtQuota) // 冻结不增长
		require.Equal(t, f.PrincipalRemaining, f.DebtQuota)
		require.Equal(t, today, f.LastSettledDay)
	})

	t.Run("platform grace vs p2p no grace", func(t *testing.T) {
		// spec §5 穿透防线：账户宽限期只作用于 platform funding，P2P 照常计息
		today := loanDay(time.Now())
		acc := mkTestAcc(1000000, 1000000, today-3)
		acc.InterestFreeUntil = today + 5 // 宽限期远未结束

		pf := mkTestFunding(LoanFundingPlatform, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
		pf.DueDay = today + 30
		settleFunding(&pf, acc, time.Now())
		require.Equal(t, int64(1000000), pf.DebtQuota) // 宽限期内不计息
		require.Equal(t, today, pf.LastSettledDay)     // 时钟照常推进

		p2p := mkTestFunding(LoanFundingPool, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
		p2p.DueDay = today + 30
		settleFunding(&p2p, acc, time.Now())
		require.Equal(t, int64(1003003), p2p.DebtQuota) // round(1000000 * 1.001^3)，宽限不穿透
		require.Equal(t, today, p2p.LastSettledDay)
	})

	t.Run("platform uses effectiveRate including custom", func(t *testing.T) {
		// 账户自定义利率低于全局时，platform funding 走 effectiveRate 而非自身 rate
		today := loanDay(time.Now())
		acc := mkTestAcc(1000000, 1000000, today-3)
		acc.CustomDailyRate = 0.0005 // < 全局 0.001

		f := mkTestFunding(LoanFundingPlatform, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
		f.DueDay = today + 30
		settleFunding(&f, acc, time.Now())

		require.Equal(t, int64(1001501), f.DebtQuota) // round(1000000 * 1.0005^3)
	})

	t.Run("grace partially elapsed counts remaining days", func(t *testing.T) {
		today := loanDay(time.Now())
		acc := mkTestAcc(1000000, 1000000, today-5)
		acc.InterestFreeUntil = today - 2 // 宽限结束 2 天整，只补算这 2 天

		f := mkTestFunding(LoanFundingPlatform, LoanRepayFull, 1000000, 1000000, today-5, 0.001)
		f.DueDay = today + 30
		settleFunding(&f, acc, time.Now())

		require.Equal(t, int64(1002001), f.DebtQuota) // round(1000000 * 1.001^2)
	})

	t.Run("same day idempotent", func(t *testing.T) {
		today := loanDay(time.Now())
		f := mkTestFunding(LoanFundingPool, LoanRepayFull, 1000000, 1000000, today, 0.001)
		f.DueDay = today - 1

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		require.Equal(t, int64(1000000), f.DebtQuota) // days=0 不增长
		require.Equal(t, today, f.LastSettledDay)
	})
}

func TestProjectFundingDebt(t *testing.T) {
	withDailyRate(t, 0.001)
	today := loanDay(time.Now())
	f := mkTestFunding(LoanFundingPlatform, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
	f.DueDay = today + 30
	acc := mkTestAcc(1000000, 1000000, today-3)

	before := f
	debt := ProjectFundingDebt(&f, acc, time.Now())

	require.Equal(t, int64(1003003), debt) // round(1000000 * 1.001^3)
	require.Equal(t, before, f)            // 只读投影，不修改原 funding
}

func TestSyncAccountFromFundings(t *testing.T) {
	today := loanDay(time.Now())
	acc := mkTestAcc(999, 999, today-10)
	fundings := []TokenLoanFunding{
		mkTestFunding(LoanFundingPool, LoanRepayFull, 300000, 200000, today-2, 0.001),
		mkTestFunding(LoanFundingPlatform, LoanRepayFull, 250000, 150000, today-1, 0.001),
	}

	// Σ(debt)/Σ(principal) 回写账户投影；LastSettledDay 推进到 fundings 最大值
	syncAccountFromFundings(acc, fundings)
	require.Equal(t, int64(550000), acc.DebtQuota)
	require.Equal(t, int64(350000), acc.PrincipalQuota)
	require.Equal(t, today-1, acc.LastSettledDay)

	// 时钟永不倒退：账户已领先时不回拨
	acc.LastSettledDay = today
	syncAccountFromFundings(acc, fundings)
	require.Equal(t, today, acc.LastSettledDay)

	// 空 fundings：债务清零，时钟保持不变
	acc.LastSettledDay = today - 2
	syncAccountFromFundings(acc, nil)
	require.Zero(t, acc.DebtQuota)
	require.Zero(t, acc.PrincipalQuota)
	require.Equal(t, today-2, acc.LastSettledDay)
}
