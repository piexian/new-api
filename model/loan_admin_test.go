package model

import (
	"fmt"
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
