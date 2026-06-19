package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func insertRedemptionForTest(t *testing.T, redemption *Redemption) {
	t.Helper()
	if redemption.Status == 0 {
		redemption.Status = common.RedemptionCodeStatusEnabled
	}
	if redemption.Name == "" {
		redemption.Name = "test redemption"
	}
	if redemption.Type == "" {
		redemption.Type = RedemptionTypeQuota
	}
	require.NoError(t, DB.Create(redemption).Error)
}

func TestRedeemQuotaCodeReturnsStructuredResultAndAddsQuota(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 9101, 10)
	insertRedemptionForTest(t, &Redemption{
		Key:   "quota-code",
		Type:  RedemptionTypeQuota,
		Quota: 50,
	})

	result, err := Redeem("quota-code", 9101)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, RedemptionTypeQuota, result.Type)
	require.Equal(t, 50, result.Quota)

	var user User
	require.NoError(t, DB.First(&user, "id = ?", 9101).Error)
	require.Equal(t, 60, user.Quota)

	var redemption Redemption
	require.NoError(t, DB.First(&redemption, "`key` = ?", "quota-code").Error)
	require.Equal(t, common.RedemptionCodeStatusUsed, redemption.Status)
	require.Equal(t, 9101, redemption.UsedUserId)
}

func TestRedeemSubscriptionCodeCreatesSubscriptionAndConsumesCode(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 9102, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 9202)
	insertRedemptionForTest(t, &Redemption{
		Key:                "subscription-code",
		Type:               RedemptionTypeSubscription,
		SubscriptionPlanId: plan.Id,
	})

	result, err := Redeem("subscription-code", 9102)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, RedemptionTypeSubscription, result.Type)
	require.Equal(t, plan.Id, result.SubscriptionPlanId)
	require.NotNil(t, result.Subscription)
	require.NotNil(t, result.SubscriptionPlan)
	require.Equal(t, "redemption", result.Subscription.Source)

	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ? AND plan_id = ?", 9102, plan.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)

	var redemption Redemption
	require.NoError(t, DB.First(&redemption, "`key` = ?", "subscription-code").Error)
	require.Equal(t, common.RedemptionCodeStatusUsed, redemption.Status)
	require.Equal(t, 9102, redemption.UsedUserId)
}

func TestRedeemSubscriptionCodeWithInvalidPlanDoesNotConsumeCode(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 9103, 0)
	insertRedemptionForTest(t, &Redemption{
		Key:                "invalid-plan-code",
		Type:               RedemptionTypeSubscription,
		SubscriptionPlanId: 9999,
	})

	result, err := Redeem("invalid-plan-code", 9103)
	require.Nil(t, result)
	require.True(t, errors.Is(err, ErrRedeemFailed))

	var redemption Redemption
	require.NoError(t, DB.First(&redemption, "`key` = ?", "invalid-plan-code").Error)
	require.Equal(t, common.RedemptionCodeStatusEnabled, redemption.Status)
	require.Zero(t, redemption.UsedUserId)

	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", 9103).Count(&count).Error)
	require.Zero(t, count)
}

func TestRedeemSubscriptionCodeHonorsPlanPurchaseLimit(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 9104, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 9204)
	require.NoError(t, DB.Model(plan).Update("max_purchase_per_user", 1).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId: 9104,
		PlanId: plan.Id,
		Status: "active",
		Source: "wallet",
	}).Error)
	insertRedemptionForTest(t, &Redemption{
		Key:                "limited-plan-code",
		Type:               RedemptionTypeSubscription,
		SubscriptionPlanId: plan.Id,
	})

	result, err := Redeem("limited-plan-code", 9104)
	require.Nil(t, result)
	require.True(t, errors.Is(err, ErrRedeemFailed))

	var redemption Redemption
	require.NoError(t, DB.First(&redemption, "`key` = ?", "limited-plan-code").Error)
	require.Equal(t, common.RedemptionCodeStatusEnabled, redemption.Status)
	require.Zero(t, redemption.UsedUserId)
}
