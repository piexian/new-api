package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// ===== Task 9: 还款按 funding pro-rata 分配测试 =====
// 换算基准：common.QuotaPerUnit = 500000，即 1 USD = 500000 quota。
// 共享内存库中 SQLite 会复用被删用户 id，各用例在开头按 user_id 清理名下残留行。

// setupRepayFundingsTest 开启词元贷 + 创建借款人/放贷人（清理共享库残留行）。
// 放贷人余额与借款人余额由用例自行设置。
func setupRepayFundingsTest(t *testing.T) (*User, *User) {
	t.Helper()
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.RepayFeeRate = 0.0001
		s.LoanTermDays = 30
	})
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	return borrower, lender
}

// createRepayOffer 直接建一条 active pool offer（含不变式 amount_total = available + Σ未还本金）
func createRepayOffer(t *testing.T, lenderId int, total, available int64) *TokenLoanOffer {
	t.Helper()
	now := time.Now()
	offer := &TokenLoanOffer{
		LenderId:        lenderId,
		Mode:            LoanOfferModePool,
		Status:          LoanOfferStatusActive,
		AmountTotal:     total,
		AmountAvailable: available,
		TotalLent:       total - available,
		RateFixed:       0.001,
		MinCreditScore:  -50,
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, DB.Create(offer).Error)
	// 清掉引用该 offer id 的残留 funding 行：SQLite 测试库会复用被删除的 offer id
	// （表无 AUTOINCREMENT），历史用例可能残留指向同一 id 的孤儿 funding，不清理会
	// 污染不变式统计（offer id 刚新建，此时任何指向它的 funding 必是孤儿数据）
	require.NoError(t, DB.Where("offer_id = ?", offer.Id).Delete(&TokenLoanFunding{}).Error)
	t.Cleanup(func() {
		_ = DB.Where("id = ?", offer.Id).Delete(&TokenLoanOffer{}).Error
	})
	return offer
}

// ① 最大余数法：3 条 funding 债务 [333,333,334]（不可整除），还 500 quota。
// Σ 分配 ≡ 500、每条不超过自身债务；余数并列（333 的两条各 .5）按 funding id 升序吃下配额。
func TestRepayLargestRemainderExact(t *testing.T) {
	borrower, _ := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	origDebts := []int64{333, 333, 334}
	var fundings []TokenLoanFunding
	for _, debt := range origDebts {
		f := &TokenLoanFunding{
			LoanUserId:         borrower.Id,
			SourceType:         LoanFundingPlatform,
			Amount:             debt,
			PrincipalRemaining: debt,
			DebtQuota:          debt,
			LastSettledDay:     day,
			Rate:               0.001,
			RepayPlan:          LoanRepayFull,
			Status:             LoanFundingActive,
			DueDay:             day + 30,
			CreatedAt:          now.Unix(),
			UpdatedAt:          now.Unix(),
		}
		require.NoError(t, DB.Create(f).Error)
		fundings = append(fundings, *f)
	}
	acc := &TokenLoanAccount{UserId: borrower.Id}
	info, allocs, _, err := distributeRepayment(DB, acc, fundings, 500, now, "manual", false)
	require.NoError(t, err)
	require.Equal(t, int64(500), info.Amount)
	require.Len(t, allocs, 3)
	var sum int64
	for i, a := range allocs {
		sum += a.Amount
		require.LessOrEqual(t, a.Amount, origDebts[i], "分配不得超过该条 funding 自身债务")
	}
	require.Equal(t, int64(500), sum, "Σ 分配必须 ≡ 还款额")
	// 最大余数 + 并列按 id 升序：floor=[166,166,167]，余数并列的 333 两条中 id 小的 +1
	require.Equal(t, int64(167), allocs[0].Amount)
	require.Equal(t, int64(166), allocs[1].Amount)
	require.Equal(t, int64(167), allocs[2].Amount)
	// 账户投影：总债务 1000 - 500
	require.Equal(t, int64(500), acc.DebtQuota)
	require.Equal(t, int64(500), info.DebtAfter)
	require.Equal(t, int64(500), info.InterestPart+info.PrincipalPart)
}

// ② 高息 funding 多分：同本金 300000、利率 0.001 vs 0.002，结算 3 天后债务分别为
// 300901 / 301804（当前债务比例分配），高息条分配与利息部分均更多。
func TestRepayHigherRateFundingGetsMore(t *testing.T) {
	borrower, lender := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	var fundings []TokenLoanFunding
	for _, rate := range []float64{0.001, 0.002} {
		f := &TokenLoanFunding{
			LoanUserId:         borrower.Id,
			SourceType:         LoanFundingPool,
			LenderId:           lender.Id,
			Amount:             300000,
			PrincipalRemaining: 300000,
			DebtQuota:          300000,
			LastSettledDay:     day - 3,
			Rate:               rate,
			RepayPlan:          LoanRepayFull,
			Status:             LoanFundingActive,
			DueDay:             day + 30,
			CreatedAt:          now.Unix(),
			UpdatedAt:          now.Unix(),
		}
		require.NoError(t, DB.Create(f).Error)
		fundings = append(fundings, *f)
	}
	acc := &TokenLoanAccount{UserId: borrower.Id}
	info, allocs, _, err := distributeRepayment(DB, acc, fundings, 200000, now, "manual", false)
	require.NoError(t, err)
	require.Len(t, allocs, 2)
	require.Equal(t, int64(200000), info.Amount)
	require.Equal(t, int64(200000), allocs[0].Amount+allocs[1].Amount)
	// 同本金不同利息 → 高息债务更大 → 分配更多、利息部分更多
	require.Greater(t, allocs[1].Amount, allocs[0].Amount)
	require.Greater(t, allocs[1].InterestPart, allocs[0].InterestPart)
	// 两条都不超自身债务
	require.LessOrEqual(t, allocs[0].Amount, int64(300901))
	require.LessOrEqual(t, allocs[1].Amount, int64(301804))
	// 落盘后的 funding 债务与结算一致（含未获利息再结算的防回归）
	var low, high TokenLoanFunding
	require.NoError(t, DB.First(&low, fundings[0].Id).Error)
	require.NoError(t, DB.First(&high, fundings[1].Id).Error)
	require.Equal(t, int64(300901), low.DebtQuota+allocs[0].Amount)
	require.Equal(t, int64(301804), high.DebtQuota+allocs[1].Amount)
}

// ③ 每条 funding 内先息后本：债务 100000（本金 90000 + 利息 10000），还 40000
// → 抵息 10000、抵本 30000，剩余债务 60000、本金 60000，status 仍 active。
func TestRepayInterestFirstWithinFunding(t *testing.T) {
	borrower, _ := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	f := &TokenLoanFunding{
		LoanUserId:         borrower.Id,
		SourceType:         LoanFundingPlatform,
		Amount:             90000,
		PrincipalRemaining: 90000,
		DebtQuota:          100000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	acc := &TokenLoanAccount{UserId: borrower.Id}
	info, allocs, _, err := distributeRepayment(DB, acc, []TokenLoanFunding{*f}, 40000, now, "manual", false)
	require.NoError(t, err)
	require.Len(t, allocs, 1)
	a := allocs[0]
	require.Equal(t, int64(40000), a.Amount)
	require.Equal(t, int64(10000), a.InterestPart)
	require.Equal(t, int64(30000), a.PrincipalPart)
	require.Equal(t, int64(10000), info.InterestPart)
	require.Equal(t, int64(30000), info.PrincipalPart)
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, int64(60000), got.DebtQuota)
	require.Equal(t, int64(60000), got.PrincipalRemaining)
	require.Equal(t, LoanFundingActive, got.Status)
	require.Equal(t, int64(60000), acc.DebtQuota)
	require.Equal(t, int64(60000), acc.PrincipalQuota)
}

// ④ 放贷人入账 = 利息；本金回补 offer.AmountAvailable，amount_total 不变（书内划转）：
// 债务 320000（本金 300000 + 利息 20000），还 100000 → 放贷人 +20000，
// offer available 100000+80000=180000、total 保持 400000、TotalInterestEarned=20000，
// 不变式 amount_total = available + Σ(active/overdue principal_remaining) 成立。
func TestRepayLenderInterestOfferAvailableUnchangedTotal(t *testing.T) {
	borrower, lender := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	offer := createRepayOffer(t, lender.Id, 400000, 100000)
	f := &TokenLoanFunding{
		LoanUserId:         borrower.Id,
		SourceType:         LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           lender.Id,
		Amount:             300000,
		PrincipalRemaining: 300000,
		DebtQuota:          320000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 0).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", borrower.Id).Update("quota", 500000).Error)

	acc := &TokenLoanAccount{UserId: borrower.Id}
	_, allocs, _, err := distributeRepayment(DB, acc, []TokenLoanFunding{*f}, 100000, now, "manual", false)
	require.NoError(t, err)
	credits, err := settleRepayAllocations(DB, borrower.Id, allocs, "manual", nil, 0)
	require.NoError(t, err)

	// 放贷人入账 = 利息 20000
	require.Len(t, credits, 1)
	require.Equal(t, lender.Id, credits[0].UserId)
	require.Equal(t, int64(20000), credits[0].Amount)
	var lu User
	require.NoError(t, DB.Select("quota").First(&lu, lender.Id).Error)
	require.Equal(t, 20000, lu.Quota)

	// 本金回补 offer：available += 80000，total 不变
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, int64(180000), got.AmountAvailable)
	require.Equal(t, int64(400000), got.AmountTotal)
	require.Equal(t, int64(20000), got.TotalInterestEarned)
	require.Equal(t, got.AmountTotal, got.AmountAvailable+activeOrOverduePrincipalSum(t, offer.Id))

	// 台账：一条 repay 行挂 funding_id/lender_id
	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", borrower.Id, "repay").First(&rec).Error)
	require.Equal(t, f.Id, rec.FundingId)
	require.Equal(t, lender.Id, rec.LenderId)
	require.Equal(t, int64(220000), rec.DebtAfter)
}

// ⑤ offer 已关闭：本金直接回放贷人余额，amount_total 同步核减（钱离开 offer 账面）。
// 债务 320000（本金 300000 + 利息 20000），还 100000 → 放贷人 +100000（利息 20000 + 本金 80000），
// offer total 300000-80000=220000、available 保持 0。
func TestRepayClosedOfferPrincipalToLender(t *testing.T) {
	borrower, lender := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	offer := &TokenLoanOffer{
		LenderId:        lender.Id,
		Mode:            LoanOfferModePool,
		Status:          LoanOfferStatusClosed,
		AmountTotal:     300000,
		AmountAvailable: 0,
		TotalLent:       300000,
		RateFixed:       0.001,
		MinCreditScore:  -50,
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, DB.Create(offer).Error)
	t.Cleanup(func() {
		_ = DB.Where("id = ?", offer.Id).Delete(&TokenLoanOffer{}).Error
	})
	f := &TokenLoanFunding{
		LoanUserId:         borrower.Id,
		SourceType:         LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           lender.Id,
		Amount:             300000,
		PrincipalRemaining: 300000,
		DebtQuota:          320000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 0).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", borrower.Id).Update("quota", 500000).Error)

	acc := &TokenLoanAccount{UserId: borrower.Id}
	_, allocs, _, err := distributeRepayment(DB, acc, []TokenLoanFunding{*f}, 100000, now, "manual", false)
	require.NoError(t, err)
	credits, err := settleRepayAllocations(DB, borrower.Id, allocs, "manual", nil, 0)
	require.NoError(t, err)

	// 利息 + 本金全部回放贷人余额
	require.Len(t, credits, 1)
	require.Equal(t, lender.Id, credits[0].UserId)
	require.Equal(t, int64(100000), credits[0].Amount)
	var lu User
	require.NoError(t, DB.Select("quota").First(&lu, lender.Id).Error)
	require.Equal(t, 100000, lu.Quota)

	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, int64(220000), got.AmountTotal, "closed offer 本金回退后 amount_total 必须同步核减")
	require.Equal(t, int64(0), got.AmountAvailable)
	require.Equal(t, int64(20000), got.TotalInterestEarned)
}

// ⑥ platform funding：本息归平台（债务销毁），任何账户/offer 均无入账；台账行
// 挂 funding_id 且 lender_id=0。
func TestRepayPlatformNoCredit(t *testing.T) {
	borrower, _ := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	f := &TokenLoanFunding{
		LoanUserId:         borrower.Id,
		SourceType:         LoanFundingPlatform,
		Amount:             90000,
		PrincipalRemaining: 90000,
		DebtQuota:          100000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", borrower.Id).Update("quota", 500000).Error)

	acc := &TokenLoanAccount{UserId: borrower.Id}
	info, allocs, _, err := distributeRepayment(DB, acc, []TokenLoanFunding{*f}, 40000, now, "manual", false)
	require.NoError(t, err)
	credits, err := settleRepayAllocations(DB, borrower.Id, allocs, "manual", nil, 0)
	require.NoError(t, err)
	require.Empty(t, credits, "platform 本息归平台，无任何放贷人入账")

	// 无 offer 行被创建/改动
	var offerCount int64
	require.NoError(t, DB.Model(&TokenLoanOffer{}).Count(&offerCount).Error)
	require.Zero(t, offerCount)

	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", borrower.Id, "repay").First(&rec).Error)
	require.Equal(t, f.Id, rec.FundingId)
	require.Zero(t, rec.LenderId)
	require.Equal(t, int64(40000), rec.Amount)
	require.Equal(t, int64(10000), rec.InterestPart)
	require.Equal(t, int64(60000), rec.DebtAfter)
	require.Equal(t, int64(60000), info.DebtAfter)
}

// ⑦ funding 全额结清：status=repaid，账户投影归零。
func TestRepayFundingFullyClearedStatusRepaid(t *testing.T) {
	borrower, _ := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	f := &TokenLoanFunding{
		LoanUserId:         borrower.Id,
		SourceType:         LoanFundingPlatform,
		Amount:             100000,
		PrincipalRemaining: 100000,
		DebtQuota:          100000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	acc := &TokenLoanAccount{UserId: borrower.Id}
	info, allocs, _, err := distributeRepayment(DB, acc, []TokenLoanFunding{*f}, 100000, now, "manual", false)
	require.NoError(t, err)
	require.Len(t, allocs, 1)
	require.Equal(t, int64(100000), info.Amount)
	require.Zero(t, info.DebtAfter)
	require.Equal(t, int64(0), acc.DebtQuota)
	require.Equal(t, int64(0), acc.PrincipalQuota)
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, LoanFundingRepaid, got.Status)
	require.Zero(t, got.DebtQuota)
	require.Zero(t, got.PrincipalRemaining)
}

// ⑧ RepayLoan 端到端回归（含手续费）：300000 本金/10000 利息债务（债务 310000），还 "0.08"
// USD = 40000 quota → 抵息 10000、抵本 30000、手续费 round(30000*0.0001)=3；借款人余额扣
// 40003，放贷人 +10000（利息），offer available 回补 30000 且 total 不变；台账一条 repay 行。
func TestRepayLoanEndToEndWithFee(t *testing.T) {
	borrower, lender := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	offer := createRepayOffer(t, lender.Id, 400000, 100000)
	f := &TokenLoanFunding{
		LoanUserId:         borrower.Id,
		SourceType:         LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           lender.Id,
		Amount:             300000,
		PrincipalRemaining: 300000,
		DebtQuota:          310000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:         borrower.Id,
		PrincipalQuota: 300000,
		DebtQuota:      310000,
		LastSettledDay: day,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 0).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", borrower.Id).Update("quota", 500000).Error)

	acc, info, _, err := RepayLoan(borrower.Id, "0.08") // 40000 quota
	require.NoError(t, err)
	require.Equal(t, int64(40000), info.Amount)
	require.Equal(t, int64(10000), info.InterestPart)
	require.Equal(t, int64(30000), info.PrincipalPart)
	require.Equal(t, int64(3), info.FeePart)
	require.Equal(t, int64(270000), info.DebtAfter)
	require.Equal(t, int64(270000), acc.DebtQuota)
	require.Equal(t, int64(270000), acc.PrincipalQuota)

	// 借款人余额：500000 - 40000 - 3
	var bu User
	require.NoError(t, DB.Select("quota").First(&bu, borrower.Id).Error)
	require.Equal(t, 459997, bu.Quota)
	// 放贷人余额：利息 10000
	var lu User
	require.NoError(t, DB.Select("quota").First(&lu, lender.Id).Error)
	require.Equal(t, 10000, lu.Quota)
	// offer：available 100000+30000=130000、total 不变、TotalInterestEarned=10000
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, int64(130000), got.AmountAvailable)
	require.Equal(t, int64(400000), got.AmountTotal)
	require.Equal(t, int64(10000), got.TotalInterestEarned)
	// funding：债务 270000/本金 270000，status 仍 active
	var gf TokenLoanFunding
	require.NoError(t, DB.First(&gf, f.Id).Error)
	require.Equal(t, int64(270000), gf.DebtQuota)
	require.Equal(t, int64(270000), gf.PrincipalRemaining)
	require.Equal(t, LoanFundingActive, gf.Status)
	// 台账：一条 repay 行（manual，挂 funding_id/lender_id）
	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", borrower.Id, "repay").First(&rec).Error)
	require.Equal(t, "manual", rec.Source)
	require.Equal(t, f.Id, rec.FundingId)
	require.Equal(t, lender.Id, rec.LenderId)
	require.Equal(t, int64(10000), rec.InterestPart)
	require.Equal(t, int64(30000), rec.PrincipalPart)
	require.Equal(t, int64(3), rec.FeePart, "手续费按抵本部分 pro-rata 落台账（单行 funding 全额归属）")
	require.Equal(t, int64(270000), rec.DebtAfter)
}

// ⑩ 手续费按抵本部分 pro-rata 拆分：两 funding 抵本 [1, 2]，fee=2 → floor [0,1]，
// 余 1 归小数余数最大的 funding1 → [1, 1]；Σ ≡ fee；fee=0 / 全抵息时为空表。
func TestDistributeFeeByPrincipal(t *testing.T) {
	allocs := []RepayAllocation{
		{FundingId: 1, PrincipalPart: 1},
		{FundingId: 2, PrincipalPart: 2},
		{FundingId: 3, PrincipalPart: 0}, // 全抵息行不参与拆分
	}
	parts := distributeFeeByPrincipal(allocs, 2)
	require.Equal(t, map[int64]int64{1: 1, 2: 1}, parts, "floor 后余数归小数余数最大的 funding")

	var total int64
	for _, v := range parts {
		total += v
	}
	require.Equal(t, int64(2), total, "Σ fee_part ≡ fee")

	require.Empty(t, distributeFeeByPrincipal(allocs, 0))
	require.Empty(t, distributeFeeByPrincipal([]RepayAllocation{{FundingId: 9, PrincipalPart: 0}}, 5))
}

// ⑨ 还款额封顶在总债务：两条 funding 债务 [30000,70000]，还 200000（远超 100000）
// → 封顶为 100000，按债务全量分配、两条均结清。
func TestRepayCappedAtTotalDebt(t *testing.T) {
	borrower, _ := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)
	var fundings []TokenLoanFunding
	for _, debt := range []int64{30000, 70000} {
		f := &TokenLoanFunding{
			LoanUserId:         borrower.Id,
			SourceType:         LoanFundingPlatform,
			Amount:             debt,
			PrincipalRemaining: debt,
			DebtQuota:          debt,
			LastSettledDay:     day,
			Rate:               0.001,
			RepayPlan:          LoanRepayFull,
			Status:             LoanFundingActive,
			DueDay:             day + 30,
			CreatedAt:          now.Unix(),
			UpdatedAt:          now.Unix(),
		}
		require.NoError(t, DB.Create(f).Error)
		fundings = append(fundings, *f)
	}
	acc := &TokenLoanAccount{UserId: borrower.Id}
	info, allocs, _, err := distributeRepayment(DB, acc, fundings, 200000, now, "manual", false)
	require.NoError(t, err)
	require.Equal(t, int64(100000), info.Amount, "还款额必须封顶在总债务")
	require.Len(t, allocs, 2)
	require.Equal(t, int64(30000), allocs[0].Amount)
	require.Equal(t, int64(70000), allocs[1].Amount)
	require.Zero(t, acc.DebtQuota)
	for i := range fundings {
		var got TokenLoanFunding
		require.NoError(t, DB.First(&got, fundings[i].Id).Error)
		require.Equal(t, LoanFundingRepaid, got.Status)
	}
}

// 注销清算专用先本后息模式（Task 3）：principalFirst=true 时每条 funding 内先抵本后抵息；
// false 保持既有先息后本。
func TestDistributeRepaymentPrincipalFirst(t *testing.T) {
	borrower, _ := setupRepayFundingsTest(t)
	now := time.Now()
	day := loanDay(now)

	mk := func(t *testing.T) (*TokenLoanAccount, []TokenLoanFunding) {
		t.Helper()
		// debt 40000 = 本金 30000 + 未付利息 10000；今日已结算（LastSettledDay=today 不再计息）
		f := &TokenLoanFunding{
			LoanUserId:         borrower.Id,
			SourceType:         LoanFundingPlatform,
			Amount:             30000,
			PrincipalRemaining: 30000,
			DebtQuota:          40000,
			LastSettledDay:     day,
			Rate:               0.001,
			RepayPlan:          LoanRepayFull,
			Status:             LoanFundingActive,
			DueDay:             day + 30,
			CreatedAt:          now.Unix(),
			UpdatedAt:          now.Unix(),
		}
		require.NoError(t, DB.Create(f).Error)
		t.Cleanup(func() {
			_ = DB.Where("id = ?", f.Id).Delete(&TokenLoanFunding{}).Error
		})
		acc := &TokenLoanAccount{UserId: borrower.Id}
		return acc, []TokenLoanFunding{*f}
	}

	t.Run("principal first", func(t *testing.T) {
		acc, fundings := mk(t)
		info, allocs, _, err := distributeRepayment(DB, acc, fundings, 20000, now, "account_closure", true)
		require.NoError(t, err)
		require.Equal(t, int64(20000), info.Amount)
		require.Equal(t, int64(20000), info.PrincipalPart) // 全部抵本
		require.Equal(t, int64(0), info.InterestPart)
		require.Len(t, allocs, 1)
		require.Equal(t, int64(20000), allocs[0].PrincipalPart)

		var f TokenLoanFunding
		require.NoError(t, DB.First(&f, fundings[0].Id).Error)
		require.Equal(t, int64(10000), f.PrincipalRemaining)
		require.Equal(t, int64(20000), f.DebtQuota) // 利息 10000 未动
	})

	t.Run("interest first unchanged", func(t *testing.T) {
		acc, fundings := mk(t)
		info, _, _, err := distributeRepayment(DB, acc, fundings, 20000, now, "manual", false)
		require.NoError(t, err)
		require.Equal(t, int64(10000), info.InterestPart) // 先抵息
		require.Equal(t, int64(10000), info.PrincipalPart)
	})
}
