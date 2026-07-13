package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetRankingUserQuotaTotalsGroupsByUserID(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&QuotaData{}))

	user := User{
		Username: "ranking-current-name",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "ranking-current-name-aff",
	}
	other := User{
		Username: "ranking-other-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "ranking-other-user-aff",
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&other).Error)
	t.Cleanup(func() {
		DB.Where("user_id IN ?", []int{user.Id, other.Id}).Delete(&QuotaData{})
		DB.Unscoped().Where("id IN ?", []int{user.Id, other.Id}).Delete(&User{})
	})

	rows := []QuotaData{
		{UserID: user.Id, Username: "ranking-old-name", CreatedAt: 100, TokenUsed: 40, Quota: 4, Count: 1},
		{UserID: user.Id, Username: "ranking-current-name", CreatedAt: 200, TokenUsed: 60, Quota: 6, Count: 2},
		{UserID: other.Id, Username: "ranking-other-user", CreatedAt: 150, TokenUsed: 50, Quota: 5, Count: 1},
	}
	require.NoError(t, DB.Create(&rows).Error)

	totals, err := GetRankingUserQuotaTotals(0, 300)
	require.NoError(t, err)
	require.Len(t, totals, 2)
	require.Equal(t, user.Id, totals[0].UserID)
	require.Equal(t, "ranking-current-name", totals[0].Username)
	require.Equal(t, int64(100), totals[0].TotalTokens)
	require.Equal(t, int64(10), totals[0].TotalQuota)
	require.Equal(t, int64(3), totals[0].RequestCount)
	require.Equal(t, other.Id, totals[1].UserID)
}
