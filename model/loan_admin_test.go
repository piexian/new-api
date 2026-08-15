package model

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// createAdminLoanTestAccount 建行造债并返回用户名，供管理端查询测试使用。
// 同步建一条等额 platform funding（LastSettledDay=今天，当日投影无利息），
// 使账户级债务与逐 funding 投影口径一致（spec §4.5 不变式）。
func createAdminLoanTestAccount(t *testing.T, debt int64) (*User, *TokenLoanAccount) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &TokenLoanRecord{}, &TokenLoanApplication{}, &TokenLoanFunding{}))
	user := createLoanTestUser(t)
	// SQLite 会复用被删用户的 id，其名下可能残留贷款账户/投放记录，先清掉再建行
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Where("loan_user_id = ?", user.Id).Delete(&TokenLoanFunding{}).Error)
	now := time.Now()
	acc := &TokenLoanAccount{
		UserId:         user.Id,
		PrincipalQuota: debt,
		DebtQuota:      debt,
		LastSettledDay: loanDay(now),
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}
	require.NoError(t, DB.Create(acc).Error)
	require.NoError(t, DB.Create(&TokenLoanFunding{
		LoanUserId:         user.Id,
		SourceType:         LoanFundingPlatform,
		Amount:             debt,
		PrincipalRemaining: debt,
		DebtQuota:          debt,
		LastSettledDay:     loanDay(now),
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             loanDay(now) + 10,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}).Error)
	// 清理本行：offer_id=0 的 platform funding 不影响按 offer 聚合的用例，
	// 但共享库中残留 funding 行仍会累积，删净保持用例自洽
	t.Cleanup(func() {
		require.NoError(t, DB.Where("loan_user_id = ?", user.Id).Delete(&TokenLoanFunding{}).Error)
	})
	return user, acc
}

func TestAdminGetLoanAccounts(t *testing.T) {
	user, acc := createAdminLoanTestAccount(t, 100000)
	other, _ := createAdminLoanTestAccount(t, 50000)

	items, total, err := AdminGetLoanAccounts(1, 100, "")
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(2))
	require.GreaterOrEqual(t, len(items), 2)

	// 找到刚创建的账户，校验用户名回联与投影字段
	var found *AdminLoanAccountItem
	for i := range items {
		if items[i].UserId == user.Id {
			found = &items[i]
			break
		}
	}
	require.NotNil(t, found)
	require.Equal(t, user.Username, found.Username)
	require.Equal(t, acc.DebtQuota, found.DebtNow)
	require.Equal(t, acc.DebtQuota-acc.PrincipalQuota, found.InterestNow)

	// 用户名模糊匹配
	items, total, err = AdminGetLoanAccounts(1, 100, user.Username)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, user.Id, items[0].UserId)

	// 纯数字 keyword 同时匹配 user_id 与用户名（用户名含时间戳数字，可能同命中），
	// 只断言目标用户必在结果中
	items, _, err = AdminGetLoanAccounts(1, 100, fmt.Sprintf("%d", other.Id))
	require.NoError(t, err)
	foundOther := false
	for _, item := range items {
		if item.UserId == other.Id {
			foundOther = true
			break
		}
	}
	require.True(t, foundOther)
}

func TestAdminGetLoanRecords(t *testing.T) {
	user, _ := createAdminLoanTestAccount(t, 100000)
	// 回收 id 名下可能残留历史台账，先清掉保证计数准确
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanRecord{}).Error)
	now := time.Now()
	for i, typ := range []string{"borrow", "repay"} {
		require.NoError(t, DB.Create(&TokenLoanRecord{
			UserId:    user.Id,
			Type:      typ,
			Amount:    int64(1000 * (i + 1)),
			DebtAfter: 100000,
			Source:    "manual",
			CreatedAt: now.Unix(),
		}).Error)
	}

	items, total, err := AdminGetLoanRecords(user.Id, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, items, 2)
	// id 倒序：repay 在前
	require.Equal(t, "repay", items[0].Type)
	require.Equal(t, user.Username, items[0].Username)

	// 不过滤用户时至少包含这两条
	_, totalAll, err := AdminGetLoanRecords(0, 1, 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, totalAll, int64(2))
}

func TestAdminGetLoanApplications(t *testing.T) {
	user, _ := createAdminLoanTestAccount(t, 100000)
	// 同上：清理回收 id 名下可能残留的工单
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanApplication{}).Error)
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanApplication{
		UserId:    user.Id,
		Topic:     "credit",
		Status:    LoanAppStatusOpen,
		ModelUsed: "test-model",
		CreatedAt: now.Unix(),
		UpdatedAt: now.Unix(),
	}).Error)
	require.NoError(t, DB.Create(&TokenLoanApplication{
		UserId:    user.Id,
		Topic:     "rate",
		Status:    LoanAppStatusClosed,
		ModelUsed: "test-model",
		CreatedAt: now.Unix(),
		UpdatedAt: now.Unix(),
	}).Error)

	// 按用户 + 状态过滤
	items, total, err := AdminGetLoanApplications(user.Id, LoanAppStatusOpen, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "credit", items[0].Topic)
	require.Equal(t, user.Username, items[0].Username)

	// 非法状态值被忽略（不按状态过滤）
	_, total, err = AdminGetLoanApplications(user.Id, "bogus", 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
}

// seedAdminLoanOffer 直接造一条放贷挂单（管理端查询测试用）
func seedAdminLoanOffer(t *testing.T, lenderId int, mode, status string, available, interest int64) {
	t.Helper()
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanOffer{
		LenderId:            lenderId,
		Mode:                mode,
		Status:              status,
		AmountTotal:         available,
		AmountAvailable:     available,
		RateFixed:           0.001,
		TotalInterestEarned: interest,
		CreatedAt:           now.Unix(),
		UpdatedAt:           now.Unix(),
	}).Error)
}

func TestAdminGetLoanOffers(t *testing.T) {
	// 表可能尚未迁移（-run 过滤单测时无前置测试建表），先迁移再清理保证精确计数
	require.NoError(t, DB.AutoMigrate(&TokenLoanOffer{}, &TokenLoanFunding{}))
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanOffer{}).Error)
	lenderA := createLoanTestUser(t)
	lenderB := createLoanTestUser(t)
	seedAdminLoanOffer(t, lenderA.Id, LoanOfferModeOrder, LoanOfferStatusActive, 500000, 0)
	seedAdminLoanOffer(t, lenderA.Id, LoanOfferModeOrder, LoanOfferStatusActive, 300000, 0)
	seedAdminLoanOffer(t, lenderA.Id, LoanOfferModePool, LoanOfferStatusPaused, 200000, 0)
	seedAdminLoanOffer(t, lenderB.Id, LoanOfferModeOrder, LoanOfferStatusActive, 800000, 0)

	// 分页 + id 倒序：pageSize 2 取最新两单
	items, total, err := AdminGetLoanOffers(1, 2, "")
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, items, 2)
	require.Greater(t, items[0].Id, items[1].Id)
	require.Equal(t, lenderB.Username, items[0].Username) // 最新一单属于 lenderB

	// 用户名模糊匹配
	items, total, err = AdminGetLoanOffers(1, 100, lenderA.Username)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	for _, item := range items {
		require.Equal(t, lenderA.Username, item.Username)
		require.Equal(t, lenderA.Id, item.LenderId)
	}

	// 纯数字 keyword → 按 lender_id 精确匹配
	items, total, err = AdminGetLoanOffers(1, 100, strconv.Itoa(lenderB.Id))
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, lenderB.Id, items[0].LenderId)
	require.Equal(t, lenderB.Username, items[0].Username)

	// 第二页
	items, total, err = AdminGetLoanOffers(2, 2, "")
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, items, 2)
	require.Equal(t, lenderA.Username, items[1].Username) // 最早一单属于 lenderA
}

// seedAdminLoanFunding 直接造一条投放记录（管理端查询测试用）
func seedAdminLoanFunding(t *testing.T, lenderId, borrowerId int, sourceType, status string, amount int64) {
	t.Helper()
	now := time.Now()
	day := loanDay(now)
	require.NoError(t, DB.Create(&TokenLoanFunding{
		LoanUserId:         borrowerId,
		SourceType:         sourceType,
		OfferId:            1,
		LenderId:           lenderId,
		Amount:             amount,
		PrincipalRemaining: amount,
		DebtQuota:          amount,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             status,
		DueDay:             day,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}).Error)
}

func TestAdminGetLoanFundings(t *testing.T) {
	// 表可能尚未迁移（-run 过滤单测时无前置测试建表），先迁移再清理保证精确计数
	require.NoError(t, DB.AutoMigrate(&TokenLoanOffer{}, &TokenLoanFunding{}))
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanFunding{}).Error)
	lender := createLoanTestUser(t)
	borrower := createLoanTestUser(t)
	other := createLoanTestUser(t)
	seedAdminLoanFunding(t, lender.Id, borrower.Id, LoanFundingOrder, LoanFundingActive, 100000)
	seedAdminLoanFunding(t, lender.Id, borrower.Id, LoanFundingOrder, LoanFundingOverdue, 200000)
	seedAdminLoanFunding(t, lender.Id, borrower.Id, LoanFundingOrder, LoanFundingRepaid, 300000)
	seedAdminLoanFunding(t, lender.Id, borrower.Id, LoanFundingOrder, LoanFundingWrittenOff, 400000)
	seedAdminLoanFunding(t, other.Id, other.Id, LoanFundingPool, LoanFundingActive, 500000)

	// 全量（id 倒序）
	items, total, err := AdminGetLoanFundings(0, 0, "", 1, 100)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Greater(t, items[0].Id, items[1].Id)

	// 按放贷人过滤：用户名回联正确（lender 名下 4 笔）
	items, total, err = AdminGetLoanFundings(lender.Id, 0, "", 1, 100)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	for _, item := range items {
		require.Equal(t, lender.Username, item.LenderUsername)
		require.Equal(t, borrower.Username, item.BorrowerUsername)
	}

	// 按借款人过滤
	_, total, err = AdminGetLoanFundings(0, borrower.Id, "", 1, 100)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)

	// 按状态过滤（白名单）
	items, total, err = AdminGetLoanFundings(0, 0, LoanFundingOverdue, 1, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, LoanFundingOverdue, items[0].Status)

	// 非法状态值被忽略（等同全量）
	_, total, err = AdminGetLoanFundings(0, 0, "bogus", 1, 100)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)

	// 分页
	items, total, err = AdminGetLoanFundings(0, 0, "", 1, 2)
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Len(t, items, 2)

	// 平台资金（lender_id = 0）：放贷人用户名为空串，不因 JOIN 缺失报错
	seedAdminLoanFunding(t, 0, borrower.Id, LoanFundingPlatform, LoanFundingActive, 60000)
	items, _, err = AdminGetLoanFundings(0, borrower.Id, "", 1, 100)
	require.NoError(t, err)
	foundPlatform := false
	for _, item := range items {
		if item.SourceType == LoanFundingPlatform {
			foundPlatform = true
			require.Equal(t, "", item.LenderUsername)
		}
	}
	require.True(t, foundPlatform)
}

func TestAdminLoanMarketOverview(t *testing.T) {
	// 表可能尚未迁移（-run 过滤单测时无前置测试建表），先迁移再清理保证精确计数
	require.NoError(t, DB.AutoMigrate(&TokenLoanOffer{}, &TokenLoanFunding{}))
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanOffer{}).Error)
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanFunding{}).Error)
	lender := createLoanTestUser(t)
	borrower := createLoanTestUser(t)

	// 挂单：2 active（闲置 500000+300000、利息 100000+0）、1 closed（闲置已清零）
	seedAdminLoanOffer(t, lender.Id, LoanOfferModeOrder, LoanOfferStatusActive, 500000, 100000)
	seedAdminLoanOffer(t, lender.Id, LoanOfferModePool, LoanOfferStatusActive, 300000, 0)
	seedAdminLoanOffer(t, lender.Id, LoanOfferModeOrder, LoanOfferStatusClosed, 0, 0)

	// funding：active 100000、overdue 200000（在贷本金 300000）、repaid 300000、
	// written_off 400000 → 逾期 1 笔
	seedAdminLoanFunding(t, lender.Id, borrower.Id, LoanFundingOrder, LoanFundingActive, 100000)
	seedAdminLoanFunding(t, lender.Id, borrower.Id, LoanFundingOrder, LoanFundingOverdue, 200000)
	seedAdminLoanFunding(t, lender.Id, borrower.Id, LoanFundingOrder, LoanFundingRepaid, 300000)
	seedAdminLoanFunding(t, lender.Id, borrower.Id, LoanFundingOrder, LoanFundingWrittenOff, 400000)

	overview, err := AdminLoanMarketOverview()
	require.NoError(t, err)
	require.Equal(t, int64(2), overview.OffersByStatus[LoanOfferStatusActive])
	require.Equal(t, int64(1), overview.OffersByStatus[LoanOfferStatusClosed])
	require.Equal(t, int64(800000), overview.FrozenIdle)      // 500000+300000+0
	require.Equal(t, int64(300000), overview.InLoanPrincipal) // 100000+200000
	require.Equal(t, int64(100000), overview.TotalInterestEarned)
	require.Equal(t, int64(1), overview.OverdueFundings)
	require.Equal(t, int64(2), overview.ActiveOffers)

	// 空表：全零不报错（COALESCE 兜底）
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanOffer{}).Error)
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanFunding{}).Error)
	overview, err = AdminLoanMarketOverview()
	require.NoError(t, err)
	require.Empty(t, overview.OffersByStatus)
	require.Zero(t, overview.FrozenIdle)
	require.Zero(t, overview.InLoanPrincipal)
	require.Zero(t, overview.TotalInterestEarned)
	require.Zero(t, overview.OverdueFundings)
	require.Zero(t, overview.ActiveOffers)
}

// 管理端账户债务按逐 funding 投影求和（P2-5）：同一用户两条不同利率的 P2P funding，
// DebtNow = Σ ProjectFundingDebt（各自利率，不穿透账户）；platform funding 用账户行
// 的有效利率/宽限输入。账户级混合利率投影会失真，故不再使用 ProjectLoanStatus。
func TestAdminGetLoanAccountsDebtNowSumsPerFunding(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.DailyRate = 0.001 })
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &TokenLoanFunding{}))
	borrower := createLoanTestUser(t)
	// 清掉复用 id 可能残留的账户行与投放行
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Where("loan_user_id = ?", borrower.Id).Delete(&TokenLoanFunding{}).Error)
	// 测试结束清理本用例的账户行与投放行：共享内存库中残留的 offer_id 行会污染
	// 后续用例（SQLite rowid 复用），必须删净
	t.Cleanup(func() {
		require.NoError(t, DB.Where("loan_user_id = ?", borrower.Id).Delete(&TokenLoanFunding{}).Error)
		require.NoError(t, DB.Where("user_id = ?", borrower.Id).Delete(&TokenLoanAccount{}).Error)
	})
	now := time.Now()
	day := loanDay(now)
	// 账户行：自定义日利率 0.0005（低于全局），platform funding 投影时使用
	acc := &TokenLoanAccount{
		UserId:          borrower.Id,
		CustomDailyRate: 0.0005,
		LastSettledDay:  day,
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, DB.Create(acc).Error)

	seed := func(sourceType string, principal int64, rate float64, lastSettledDay int) *TokenLoanFunding {
		t.Helper()
		f := &TokenLoanFunding{
			LoanUserId:         borrower.Id,
			SourceType:         sourceType,
			OfferId:            1,
			LenderId:           999,
			Amount:             principal,
			PrincipalRemaining: principal,
			DebtQuota:          principal,
			LastSettledDay:     lastSettledDay,
			Rate:               rate,
			RepayPlan:          LoanRepayFull,
			Status:             LoanFundingActive,
			DueDay:             day + 10,
			CreatedAt:          now.Unix(),
			UpdatedAt:          now.Unix(),
		}
		require.NoError(t, DB.Create(f).Error)
		return f
	}
	// 两条不同利率的 P2P funding（各自利率复利，不穿透账户）
	f1 := seed(LoanFundingPool, 100000, 0.001, day-3)
	f2 := seed(LoanFundingOrder, 200000, 0.003, day-5)
	// 一条 platform funding（用账户有效利率 0.0005，无视自身 Rate）
	seed(LoanFundingPlatform, 100000, 0.002, day-2)

	items, total, err := AdminGetLoanAccounts(1, 100, fmt.Sprintf("%d", borrower.Id))
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	var item *AdminLoanAccountItem
	for i := range items {
		if items[i].UserId == borrower.Id {
			item = &items[i]
			break
		}
	}
	require.NotNil(t, item)

	// 期望 = 逐 funding 投影求和（platform 传账户行，P2P 传 nil）
	var wantDebt, wantPrincipal int64
	wantDebt += ProjectFundingDebt(f1, nil, time.Now())
	wantDebt += ProjectFundingDebt(f2, nil, time.Now())
	wantDebt += ProjectFundingDebt(&TokenLoanFunding{
		LoanUserId:         borrower.Id,
		SourceType:         LoanFundingPlatform,
		Amount:             100000,
		PrincipalRemaining: 100000,
		DebtQuota:          100000,
		LastSettledDay:     day - 2,
		Rate:               0.002,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 10,
	}, acc, time.Now())
	wantPrincipal = 100000 + 200000 + 100000
	require.Equal(t, wantDebt, item.DebtNow)
	require.Equal(t, wantDebt-wantPrincipal, item.InterestNow)

	// 与账户级投影区分：账户 DebtQuota 恒 0（未回写 funding 汇总），
	// 若仍用 ProjectLoanStatus 会得到 0 而非逐 funding 求和——证明走的是 per-funding 口径
	var accRow TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).First(&accRow).Error)
	require.NotEqual(t, accRow.DebtQuota, item.DebtNow)
}
