package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// ===== Task 8: BorrowLoan 按 funding 放款测试 =====
// 换算基准：common.QuotaPerUnit = 500000，即 1 USD = 500000 quota。
// 共享内存库中 SQLite 会复用被删用户 id，各用例在开头按 user_id 清理名下残留行。

// cleanupLoanBorrowData 清掉 borrower/lender 名下可能残留的贷款行（账户/台账/funding）
// 与放贷人 offer 行，保证用例从干净状态开始；lenderId<=0 时跳过 offer 清理
func cleanupLoanBorrowData(t *testing.T, borrowerId, lenderId int) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &TokenLoanRecord{}, &TokenLoanOffer{}, &TokenLoanFunding{}))
	if lenderId > 0 {
		require.NoError(t, DB.Where("lender_id = ?", lenderId).Delete(&TokenLoanOffer{}).Error)
		require.NoError(t, DB.Where("lender_id = ?", lenderId).Delete(&TokenLoanFunding{}).Error)
		require.NoError(t, DB.Where("user_id = ?", lenderId).Delete(&TokenLoanAccount{}).Error)
		require.NoError(t, DB.Where("user_id = ?", lenderId).Delete(&TokenLoanRecord{}).Error)
	}
	require.NoError(t, DB.Where("loan_user_id = ?", borrowerId).Delete(&TokenLoanFunding{}).Error)
	require.NoError(t, DB.Where("user_id = ?", borrowerId).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Where("user_id = ?", borrowerId).Delete(&TokenLoanRecord{}).Error)
}

// setupBorrowMarketTest 开启词元贷 + 放贷市场并创建借款人/放贷人（共享库残留行已清理）
func setupBorrowMarketTest(t *testing.T, maxTotal int64) (*User, *User) {
	t.Helper()
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MarketEnabled = true
		s.MaxTotal = maxTotal
		s.MaxPerBorrow = 0
		s.LoanTermDays = 30
		s.MaxFundingsPerBorrow = 5
	})
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	return borrower, lender
}

// createBorrowOffer 直接建一条 active pool offer（跳过 CreateLoanOffer 的余额扣款路径，
// 测试聚焦放款侧）；测试结束按 id 删除，避免残留污染其他用例
func createBorrowOffer(t *testing.T, lenderId int, available int64, rate float64) *TokenLoanOffer {
	t.Helper()
	now := time.Now()
	offer := &TokenLoanOffer{
		LenderId:        lenderId,
		Mode:            LoanOfferModePool,
		Status:          LoanOfferStatusActive,
		AmountTotal:     available,
		AmountAvailable: available,
		RateFixed:       rate,
		MinCreditScore:  -50,
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, DB.Create(offer).Error)
	t.Cleanup(func() {
		_ = DB.Where("id = ?", offer.Id).Delete(&TokenLoanOffer{}).Error
	})
	return offer
}

// ① 借款闸门：blacklisted_until_day 未过的用户拒绝借款（ErrLoanBlacklisted），
//
//	且拒绝后不落任何行
func TestBorrowLoanBlacklistedGate(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 1_000_000
	})
	user := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, 0)
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:              user.Id,
		BlacklistedUntilDay: loanDay(now) + 5,
		LastSettledDay:      loanDay(now),
		CreatedAt:           now.Unix(),
		UpdatedAt:           now.Unix(),
	}).Error)

	_, _, err := BorrowLoan(user.Id, "0.10", 0, nil)
	require.ErrorIs(t, err, ErrLoanBlacklisted)

	var n int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).Where("user_id = ?", user.Id).Count(&n).Error)
	require.Zero(t, n)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("loan_user_id = ?", user.Id).Count(&n).Error)
	require.Zero(t, n)
}

// ② 借款闸门：存在 overdue funding 的用户拒绝借款（ErrLoanHasOverdue），
//
//	事务整体回滚（闸门路径不建账户）
func TestBorrowLoanOverdueGate(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 1_000_000
	})
	user := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, 0)
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanFunding{
		LoanUserId:         user.Id,
		SourceType:         LoanFundingPlatform,
		Amount:             100_000,
		PrincipalRemaining: 100_000,
		DebtQuota:          100_000,
		LastSettledDay:     loanDay(now),
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingOverdue,
		DueDay:             loanDay(now) - 1,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}).Error)

	_, _, err := BorrowLoan(user.Id, "0.10", 0, nil)
	require.ErrorIs(t, err, ErrLoanHasOverdue)

	// 闸门拒绝发生在账户建行之后：整个事务回滚，账户行不得残留
	var accCount int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", user.Id).Count(&accCount).Error)
	require.Zero(t, accCount)
}

// ③ 纯官方路径：市场关闭 → 整笔借款生成一条 platform funding，
//
//	BorrowEventId = 台账 borrow 行 id、DueDay = 当天 + LoanTermDays（>0）、
//	OfferId/LenderId = 0、RepayPlan=full、Status=active
func TestBorrowLoanPurePlatformFunding(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MarketEnabled = false
		s.MaxTotal = 1_000_000
		s.LoanTermDays = 30
	})
	user := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, 0)

	acc, _, err := BorrowLoan(user.Id, "1.00", 0, nil) // 500000 quota
	require.NoError(t, err)
	require.Equal(t, int64(500_000), acc.DebtQuota)
	require.Equal(t, int64(500_000), acc.PrincipalQuota)
	require.Equal(t, int64(500_000), acc.TotalBorrowed)

	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "borrow").First(&rec).Error)
	require.Equal(t, int64(500_000), rec.Amount)
	require.Equal(t, int64(500_000), rec.PrincipalPart)
	require.Equal(t, "manual", rec.Source)
	require.Equal(t, int64(500_000), rec.DebtAfter)

	var fundings []TokenLoanFunding
	require.NoError(t, DB.Where("loan_user_id = ?", user.Id).Find(&fundings).Error)
	require.Len(t, fundings, 1)
	f := fundings[0]
	require.Equal(t, LoanFundingPlatform, f.SourceType)
	require.Equal(t, int64(rec.Id), f.BorrowEventId, "funding 必须挂到台账 borrow 事件 id")
	require.Equal(t, int64(500_000), f.Amount)
	require.Equal(t, int64(500_000), f.PrincipalRemaining)
	require.Equal(t, int64(500_000), f.DebtQuota)
	require.Equal(t, LoanRepayFull, f.RepayPlan)
	require.Equal(t, LoanFundingActive, f.Status)
	require.Greater(t, f.DueDay, 0, "DueDay 必须 > 0")
	require.Equal(t, loanDay(time.Now())+30, f.DueDay)
	require.Zero(t, f.OfferId)
	require.Zero(t, f.LenderId)
}

// ④ 混合投放：市场开启 + 预置 pool offer → 借款拆成两条 funding（pool + platform 兜底），
//
//	offer.AmountAvailable 扣减、TotalLent 累加，且 offer 不变式
//	amount_total = amount_available + Σ 未还本金 成立
func TestBorrowLoanMixedMarketFunding(t *testing.T) {
	borrower, lender := setupBorrowMarketTest(t, 1_000_000)
	offer := createBorrowOffer(t, lender.Id, 400_000, 0.001)

	acc, fundings, err := BorrowLoan(borrower.Id, "1.00", 0, nil) // 500000：pool 400000 + 平台 100000
	require.NoError(t, err)
	require.Equal(t, int64(500_000), acc.DebtQuota)
	require.Len(t, fundings, 2)

	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", borrower.Id, "borrow").First(&rec).Error)

	var rows []TokenLoanFunding
	require.NoError(t, DB.Where("loan_user_id = ?", borrower.Id).Order("id ASC").Find(&rows).Error)
	require.Len(t, rows, 2)
	pool, platform := rows[0], rows[1]
	require.Equal(t, LoanFundingPool, pool.SourceType)
	require.Equal(t, offer.Id, pool.OfferId)
	require.Equal(t, lender.Id, pool.LenderId)
	require.Equal(t, int64(400_000), pool.Amount)
	require.Equal(t, int64(rec.Id), pool.BorrowEventId)
	require.Equal(t, 0.001, pool.Rate)
	require.Equal(t, LoanFundingPlatform, platform.SourceType)
	require.Equal(t, int64(100_000), platform.Amount)
	require.Equal(t, int64(rec.Id), platform.BorrowEventId, "同一借款事件的全部 funding 共享 BorrowEventId")

	// offer 扣减与不变式
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Zero(t, got.AmountAvailable)
	require.Equal(t, int64(400_000), got.TotalLent)
	require.Equal(t, got.AmountTotal, got.AmountAvailable+activeOrOverduePrincipalSum(t, offer.Id))
}

// ⑤ 市场关闭时忽略池 offer：即使有 active pool offer，借款也整笔走平台，
//
//	offer 不被触碰
func TestBorrowLoanMarketDisabledIgnoresOffers(t *testing.T) {
	borrower, lender := setupBorrowMarketTest(t, 1_000_000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.MarketEnabled = false })
	offer := createBorrowOffer(t, lender.Id, 500_000, 0.001)

	_, fundings, err := BorrowLoan(borrower.Id, "1.00", 0, nil)
	require.NoError(t, err)
	require.Len(t, fundings, 1)
	require.Equal(t, LoanFundingPlatform, fundings[0].SourceType)
	require.Equal(t, int64(500_000), fundings[0].Amount)

	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, int64(500_000), got.AmountAvailable, "市场关闭时 offer 不得被扣减")
	require.Zero(t, got.TotalLent)
}

// ⑥ 定向挂单意向透传：intendedOrderId 指向的 order offer 优先出资，
//
//	funding 的 SourceType=order、OfferId/LenderId/Rate 均来自该 offer
func TestBorrowLoanIntendedOrderFlowsThrough(t *testing.T) {
	borrower, lender := setupBorrowMarketTest(t, 1_000_000)
	now := time.Now()
	offer := &TokenLoanOffer{
		LenderId:        lender.Id,
		Mode:            LoanOfferModeOrder,
		Status:          LoanOfferStatusActive,
		AmountTotal:     500_000,
		AmountAvailable: 500_000,
		RateFixed:       0.002,
		MinCreditScore:  -50,
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, DB.Create(offer).Error)
	t.Cleanup(func() {
		_ = DB.Where("id = ?", offer.Id).Delete(&TokenLoanOffer{}).Error
	})

	_, fundings, err := BorrowLoan(borrower.Id, "0.80", offer.Id, nil) // 400000，定向单全部覆盖
	require.NoError(t, err)
	require.Len(t, fundings, 1)
	f := fundings[0]
	require.Equal(t, LoanFundingOrder, f.SourceType)
	require.Equal(t, offer.Id, f.OfferId)
	require.Equal(t, lender.Id, f.LenderId)
	require.Equal(t, int64(400_000), f.Amount)
	require.Equal(t, 0.002, f.Rate)

	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, int64(100_000), got.AmountAvailable)
}

// ⑦ dropReasons 不阻断借款：意向挂单不存在（过期意向）时撮合记录 drop 原因，
//
//	借款照常成功，缺额由平台兜底
func TestBorrowLoanDropReasonsDoNotFail(t *testing.T) {
	borrower, _ := setupBorrowMarketTest(t, 1_000_000)

	_, fundings, err := BorrowLoan(borrower.Id, "1.00", 987654321, nil)
	require.NoError(t, err)
	require.Len(t, fundings, 1)
	require.Equal(t, LoanFundingPlatform, fundings[0].SourceType)
	require.Equal(t, int64(500_000), fundings[0].Amount)
}
