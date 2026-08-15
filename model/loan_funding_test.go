package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

	t.Run("overdue full plan splits penalty at due day", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		f := mkTestFunding(LoanFundingPool, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
		f.DueDay = today - 1 // 昨天到期，到期前 2 天按 base 利率、之后 1 天按罚息利率

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		// Round(Round(1000000 * 1.001^2) * 1.002^1) = Round(1002001 * 1.002) = 1004005
		require.Equal(t, int64(1004005), f.DebtQuota)
		require.Equal(t, today, f.LastSettledDay)
	})

	t.Run("mid-span due day splits base and penalty segments", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		f := mkTestFunding(LoanFundingPool, LoanRepayFull, 1000000, 1000000, today-5, 0.001)
		f.DueDay = today - 2 // 区间中段的 due_day

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		// Round(Round(1000000 * 1.001^3) * 1.002^2) = Round(1003003 * 1.004004) = 1007019
		require.Equal(t, int64(1007019), f.DebtQuota)
		require.Equal(t, today, f.LastSettledDay)
	})

	t.Run("whole span past due single penalty segment", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		f := mkTestFunding(LoanFundingPool, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
		f.DueDay = today - 6 // 整个未结算区间都在 due_day 之后

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		require.Equal(t, int64(1006012), f.DebtQuota) // round(1000000 * 1.002^3)，整段罚息
		require.Equal(t, today, f.LastSettledDay)
	})

	t.Run("not yet due single base segment", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		f := mkTestFunding(LoanFundingPool, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
		f.DueDay = today + 2 // 尚未到期

		settleFunding(&f, mkTestAcc(0, 0, 0), time.Now())

		require.Equal(t, int64(1003003), f.DebtQuota) // round(1000000 * 1.001^3)，不乘罚息
		require.Equal(t, today, f.LastSettledDay)
	})

	t.Run("platform grace lift past due day no double penalty", func(t *testing.T) {
		withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
		today := loanDay(time.Now())
		acc := mkTestAcc(1000000, 1000000, today-5)
		acc.InterestFreeUntil = today - 1 // 宽限把起算日上提到 due_day 之后
		f := mkTestFunding(LoanFundingPlatform, LoanRepayFull, 1000000, 1000000, today-5, 0.001)
		f.DueDay = today - 4

		settleFunding(&f, acc, time.Now())

		// base 上提到 today-1：seg1=0，仅宽限结束后的 1 天按罚息计息，宽限期不双重计息
		require.Equal(t, int64(1002000), f.DebtQuota) // round(1000000 * 1.002^1)
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

// ===== Task 11: 逾期状态机（flipOverdueFundingsTx） =====

// createFlipFunding 直接建一条 status=active 的 funding 行（时钟字段由 now 决定，
// 翻转判定与结算解耦：LastSettledDay=loanDay(now)，结算分段长为 0、债务数值确定）。
// DueDay = loanDay(now) + dueDayOffset；debt <= 0 时 Amount/本金/债务同取 0。
func createFlipFunding(t *testing.T, userId int, now time.Time, dueDayOffset int, debt int64) TokenLoanFunding {
	t.Helper()
	f := TokenLoanFunding{
		LoanUserId:         userId,
		SourceType:         LoanFundingPlatform,
		Amount:             debt,
		PrincipalRemaining: debt,
		DebtQuota:          debt,
		LastSettledDay:     loanDay(now),
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             loanDay(now) + dueDayOffset,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(&f).Error)
	return f
}

func TestFlipOverdueFundingsTx(t *testing.T) {
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.Local)
	today := loanDay(now)

	// ① 到期未清（active + 已过期 + 债务>0）→ 翻转，penalty_started_day 落账，
	//    内存切片与库内行同步更新；返回本次新翻转的列表（含 SourceType，供 Task 15 过滤）
	t.Run("active past due with debt flips", func(t *testing.T) {
		user := createLoanTestUser(t)
		cleanupLoanBorrowData(t, user.Id, 0)
		f := createFlipFunding(t, user.Id, now, -1, 1000)

		slice := []TokenLoanFunding{f}
		var flipped []TokenLoanFunding
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			var err error
			flipped, err = flipOverdueFundingsTx(tx, user.Id, slice, now)
			return err
		}))

		require.Len(t, flipped, 1)
		require.Equal(t, f.Id, flipped[0].Id)
		require.Equal(t, LoanFundingOverdue, flipped[0].Status)
		require.Equal(t, today, flipped[0].PenaltyStartedDay)
		require.Equal(t, LoanFundingPlatform, flipped[0].SourceType, "新翻转列表保留 SourceType 供平台处置过滤")
		// 内存切片同步
		require.Equal(t, LoanFundingOverdue, slice[0].Status)
		require.Equal(t, today, slice[0].PenaltyStartedDay)
		// 库内落盘
		var got TokenLoanFunding
		require.NoError(t, DB.First(&got, f.Id).Error)
		require.Equal(t, LoanFundingOverdue, got.Status)
		require.Equal(t, today, got.PenaltyStartedDay)
	})

	// ② 幂等：同事务二次调用返回空（切片已 overdue）；新事务重读再翻也返回空
	//    （条件更新命中 0 行），penalty_started_day 两次均不变
	t.Run("idempotent second call returns empty", func(t *testing.T) {
		user := createLoanTestUser(t)
		cleanupLoanBorrowData(t, user.Id, 0)
		f := createFlipFunding(t, user.Id, now, -1, 1000)
		slice := []TokenLoanFunding{f}

		var first, second []TokenLoanFunding
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			var err error
			first, err = flipOverdueFundingsTx(tx, user.Id, slice, now)
			if err != nil {
				return err
			}
			second, err = flipOverdueFundingsTx(tx, user.Id, slice, now)
			return err
		}))
		require.Len(t, first, 1)
		require.Empty(t, second)

		// 新事务重读（切片状态仍为库内 overdue）再翻 → 空，penalty_started_day 不变
		var reload []TokenLoanFunding
		require.NoError(t, DB.Where("loan_user_id = ? AND status IN ?",
			user.Id, []string{LoanFundingActive, LoanFundingOverdue}).Find(&reload).Error)
		require.Len(t, reload, 1)
		require.Equal(t, LoanFundingOverdue, reload[0].Status)
		var third []TokenLoanFunding
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			var err error
			third, err = flipOverdueFundingsTx(tx, user.Id, reload, now)
			return err
		}))
		require.Empty(t, third)

		var got TokenLoanFunding
		require.NoError(t, DB.First(&got, f.Id).Error)
		require.Equal(t, today, got.PenaltyStartedDay)
	})

	// ③ 未到期不翻：今天到期（today == DueDay）尚不逾期，翻转判定要求 today > DueDay
	t.Run("due today not yet overdue does not flip", func(t *testing.T) {
		user := createLoanTestUser(t)
		cleanupLoanBorrowData(t, user.Id, 0)
		f := createFlipFunding(t, user.Id, now, 0, 1000)

		slice := []TokenLoanFunding{f}
		var flipped []TokenLoanFunding
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			var err error
			flipped, err = flipOverdueFundingsTx(tx, user.Id, slice, now)
			return err
		}))
		require.Empty(t, flipped)

		var got TokenLoanFunding
		require.NoError(t, DB.First(&got, f.Id).Error)
		require.Equal(t, LoanFundingActive, got.Status)
		require.Zero(t, got.PenaltyStartedDay)
	})

	// ④ 已过期但债务为 0 不翻：保持 active，等结清清账路径（debt=0 → repaid）处理
	t.Run("past due with zero debt does not flip", func(t *testing.T) {
		user := createLoanTestUser(t)
		cleanupLoanBorrowData(t, user.Id, 0)
		f := createFlipFunding(t, user.Id, now, -1, 0)

		slice := []TokenLoanFunding{f}
		var flipped []TokenLoanFunding
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			var err error
			flipped, err = flipOverdueFundingsTx(tx, user.Id, slice, now)
			return err
		}))
		require.Empty(t, flipped)

		var got TokenLoanFunding
		require.NoError(t, DB.First(&got, f.Id).Error)
		require.Equal(t, LoanFundingActive, got.Status)
		require.Zero(t, got.PenaltyStartedDay)
	})

	// ⑦ 并发翻转模拟：他事务已把行置为 overdue（本切片为过期读 active）→ 条件更新
	//    命中 0 行 → 不视为新翻转，重读 status/penalty_started_day 回填切片
	t.Run("concurrent flip wins and is not counted", func(t *testing.T) {
		user := createLoanTestUser(t)
		cleanupLoanBorrowData(t, user.Id, 0)
		f := createFlipFunding(t, user.Id, now, -1, 1000)
		// 模拟并发翻转已提交（penalty_started_day 由他事务落账）
		require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", f.Id).
			Updates(map[string]interface{}{
				"status":              LoanFundingOverdue,
				"penalty_started_day": today - 2,
			}).Error)

		stale := []TokenLoanFunding{f} // 切片仍是创建时的 active
		var flipped []TokenLoanFunding
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			var err error
			flipped, err = flipOverdueFundingsTx(tx, user.Id, stale, now)
			return err
		}))
		require.Empty(t, flipped)
		// 切片回填库内最新状态
		require.Equal(t, LoanFundingOverdue, stale[0].Status)
		require.Equal(t, today-2, stale[0].PenaltyStartedDay)
		// 库内保持不变（他事务的值不被覆盖）
		var got TokenLoanFunding
		require.NoError(t, DB.First(&got, f.Id).Error)
		require.Equal(t, LoanFundingOverdue, got.Status)
		require.Equal(t, today-2, got.PenaltyStartedDay)
	})
}

// ⑤ 利息数学与 status 无关：同参数 funding 仅 status 不同（active vs overdue），
//
//	settleFunding 结果完全一致——罚息由 today > DueDay 纯计算驱动，翻转不改变利息
func TestSettleFundingUnaffectedByStatus(t *testing.T) {
	withDailyRate(t, 0.001)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.OverduePenaltyMultiplier = 2.0 })
	today := loanDay(time.Now())

	active := mkTestFunding(LoanFundingPool, LoanRepayFull, 1000000, 1000000, today-3, 0.001)
	active.DueDay = today - 1
	overdue := active
	overdue.Status = LoanFundingOverdue
	overdue.PenaltyStartedDay = today - 1

	settleFunding(&active, nil, time.Now())
	settleFunding(&overdue, nil, time.Now())

	require.Equal(t, active.DebtQuota, overdue.DebtQuota)
	require.Equal(t, active.LastSettledDay, overdue.LastSettledDay)
	// round(Round(1000000 * 1.001^2) * 1.002) = round(1002001 * 1.002) = 1004005
	require.Equal(t, int64(1004005), active.DebtQuota)
	// 翻转后再次结算（同日）不二次计息：与翻转前结果一致
	settleFunding(&overdue, nil, time.Now())
	require.Equal(t, int64(1004005), overdue.DebtQuota)
}

// ⑥ 借款闸门：今天刚过期的 active funding 在 BorrowLoan 结算+翻转后阻断借款
//
//	（ErrLoanHasOverdue，杜绝借新还旧）；拒绝路径整体回滚，翻转不落痕
func TestBorrowLoanGateBlocksJustOverdueFunding(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MarketEnabled = false
		s.MaxTotal = 1_000_000
		s.LoanTermDays = 30
	})
	user := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, 0)
	now := time.Now()
	// 昨天到期、今天尚未结清的 active funding：本次借款会在结算后翻转为 overdue，
	// 闸门用翻转后的列表拒绝借款（此前闸门只拦已翻转的 overdue 行）
	f := createFlipFunding(t, user.Id, now, -1, 100_000)

	_, _, err := BorrowLoan(user.Id, "0.10", 0, nil)
	require.ErrorIs(t, err, ErrLoanHasOverdue)

	// 拒绝路径整体回滚：funding 仍 active（翻转幂等，下次写路径再翻），无新借款
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, LoanFundingActive, got.Status)
	require.Zero(t, got.PenaltyStartedDay)
	var n int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).Where("user_id = ?", user.Id).Count(&n).Error)
	require.Zero(t, n)
}

// 逾期 → repaid 流转（spec 状态机）：active 的 funding 在签到路径先翻转 overdue，
// 再被本次签到全额结清，最终状态必须是 repaid（distributeRepayment 在 debt 归零时
// 无论先前 active/overdue 一律置 repaid，翻转挂接不得破坏该流转）
func TestCheckinRepayClearsJustOverdueFundingToRepaid(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	// 昨天到期、今天仍 active 的 funding：签到结算后翻转 overdue，随后被 5000 奖励结清
	f := createFlipFunding(t, user.Id, time.Now(), -1, 3000)
	createLoanDebtAccount(t, user.Id, 3000, 3000)

	_, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.NotNil(t, repay)
	require.Equal(t, int64(3000), repay.Amount)

	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, LoanFundingRepaid, got.Status)
	require.Zero(t, got.DebtQuota)
	require.Zero(t, got.PrincipalRemaining)
}
