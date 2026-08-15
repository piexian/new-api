package model

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// createAdminLoanTestAccount 建行造债并返回用户名，供管理端查询测试使用
func createAdminLoanTestAccount(t *testing.T, debt int64) (*User, *TokenLoanAccount) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &TokenLoanRecord{}, &TokenLoanApplication{}))
	user := createLoanTestUser(t)
	// SQLite 会复用被删用户的 id，其名下可能残留贷款账户，先清掉再建行
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanAccount{}).Error)
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
