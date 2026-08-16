package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// setupClosureUser 创建带指定钱包余额的用户（清理共享库按 id 复用残留的贷款/套餐行）
func setupClosureUser(t *testing.T, quota int) *User {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &TokenLoanRecord{},
		&TokenLoanFunding{}, &UserSubscription{}))
	username := fmt.Sprintf("closure-%d", time.Now().UnixNano())
	user := &User{Username: username, Password: "x", Quota: quota,
		AffCode: username + "-aff", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	// SQLite 共享内存库会复用被删用户的 id，先清掉该 id 名下的残留行
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Where("loan_user_id = ?", user.Id).Delete(&TokenLoanFunding{}).Error)
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanRecord{}).Error)
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&UserSubscription{}).Error)
	return user
}

// persistClosureFunding 落盘一条 platform funding（今日已结算、未到期）及对应贷款账户
func persistClosureFunding(t *testing.T, userId int, debt, principal int64) *TokenLoanFunding {
	t.Helper()
	now := time.Now()
	today := loanDay(now)
	f := &TokenLoanFunding{
		LoanUserId:         userId,
		SourceType:         LoanFundingPlatform,
		Amount:             principal,
		PrincipalRemaining: principal,
		DebtQuota:          debt,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		LastSettledDay:     today,
		DueDay:             today + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	acc := &TokenLoanAccount{
		UserId:         userId,
		PrincipalQuota: principal,
		DebtQuota:      debt,
		LastSettledDay: today,
		CreditScore:    50,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}
	require.NoError(t, DB.Create(acc).Error)
	t.Cleanup(func() {
		_ = DB.Where("id = ?", f.Id).Delete(&TokenLoanFunding{}).Error
		_ = DB.Where("user_id = ?", userId).Delete(&TokenLoanAccount{}).Error
	})
	return f
}

func TestCloseAccountNoLoanNoSubZeroesQuota(t *testing.T) {
	user := setupClosureUser(t, 5000)
	require.NoError(t, CloseAccountForDeletion(user.Id))

	var u User
	require.NoError(t, DB.First(&u, user.Id).Error)
	require.Equal(t, 0, u.Quota) // 余额清零
	// 不得建贷款账户
	var count int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Equal(t, int64(0), count)
}

func TestCloseAccountFullRepayFromWallet(t *testing.T) {
	user := setupClosureUser(t, 100000)
	f := persistClosureFunding(t, user.Id, 40000, 40000)

	require.NoError(t, CloseAccountForDeletion(user.Id))

	var updated TokenLoanFunding
	require.NoError(t, DB.First(&updated, f.Id).Error)
	require.Equal(t, LoanFundingRepaid, updated.Status) // 全额还清 → repaid（非核销）
	require.Equal(t, int64(0), updated.DebtQuota)

	var u User
	require.NoError(t, DB.First(&u, user.Id).Error)
	require.Equal(t, 0, u.Quota) // 60000 剩余也清零

	// 台账：source=account_closure，手续费/罚则为 0
	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "repay").First(&rec).Error)
	require.Equal(t, "account_closure", rec.Source)
	require.Equal(t, int64(40000), rec.Amount)
	require.Equal(t, int64(0), rec.FeePart)
	require.Equal(t, int64(0), rec.PenaltyPart)
}

func TestCloseAccountPartialRepayThenWriteoff(t *testing.T) {
	user := setupClosureUser(t, 15000) // 余额不够全还
	f := persistClosureFunding(t, user.Id, 40000, 40000)

	require.NoError(t, CloseAccountForDeletion(user.Id))

	var updated TokenLoanFunding
	require.NoError(t, DB.First(&updated, f.Id).Error)
	require.Equal(t, LoanFundingWrittenOff, updated.Status)    // 残余核销
	require.Equal(t, int64(25000), updated.PrincipalRemaining) // 先本后息：15000 全部抵本（40000-15000）

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(0), acc.DebtQuota) // 投影销毁
	require.Greater(t, acc.BlacklistedUntilDay, 0)
	require.Less(t, acc.CreditScore, 50) // 核销扣分
}

func TestCloseAccountSubscriptionBudgetUsed(t *testing.T) {
	user := setupClosureUser(t, 0) // 钱包为空，套餐有钱
	f := persistClosureFunding(t, user.Id, 50000, 50000)
	now := time.Now().Unix()
	sub := &UserSubscription{
		UserId: user.Id, PlanId: 1, AmountTotal: 100000, AmountUsed: 20000,
		StartTime: now, EndTime: now + 86400*30, Status: "active", Source: "order",
	}
	require.NoError(t, DB.Create(sub).Error)

	require.NoError(t, CloseAccountForDeletion(user.Id))

	var updated TokenLoanFunding
	require.NoError(t, DB.First(&updated, f.Id).Error)
	require.Equal(t, LoanFundingRepaid, updated.Status)    // 套餐预算足够 → 全额还清
	require.Equal(t, int64(0), updated.PrincipalRemaining) // 套餐未消耗 80000 中出资 50000 全部抵本

	var s UserSubscription
	require.NoError(t, DB.First(&s, sub.Id).Error)
	require.Equal(t, "cancelled", s.Status)
	require.Equal(t, int64(70000), s.AmountUsed) // 20000 + 50000
}

func TestDeleteUserByIdRunsClosure(t *testing.T) {
	user := setupClosureUser(t, 100000)
	f := persistClosureFunding(t, user.Id, 40000, 40000)

	require.NoError(t, DeleteUserById(user.Id))

	var updated TokenLoanFunding
	require.NoError(t, DB.First(&updated, f.Id).Error)
	require.Equal(t, LoanFundingRepaid, updated.Status)

	var u User
	require.Error(t, DB.First(&u, user.Id).Error) // 软删后常规查询不可见
	require.NoError(t, DB.Unscoped().First(&u, user.Id).Error)
}
