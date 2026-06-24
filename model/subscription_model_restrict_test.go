package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedModelEnableGroupsForTest(t *testing.T, groups map[string][]string) {
	t.Helper()

	oldPricingMap := pricingMap
	oldLastGetPricingTime := lastGetPricingTime

	modelEnableGroupsLock.Lock()
	oldModelEnableGroups := modelEnableGroups
	clonedGroups := make(map[string][]string, len(groups))
	for modelName, modelGroups := range groups {
		clonedGroups[modelName] = append([]string(nil), modelGroups...)
	}
	modelEnableGroups = clonedGroups
	modelEnableGroupsLock.Unlock()

	pricingMap = []Pricing{{ModelName: "test-model"}}
	lastGetPricingTime = time.Now()

	t.Cleanup(func() {
		pricingMap = oldPricingMap
		lastGetPricingTime = oldLastGetPricingTime
		modelEnableGroupsLock.Lock()
		modelEnableGroups = oldModelEnableGroups
		modelEnableGroupsLock.Unlock()
	})
}

func TestResolveSubscriptionModelRestrictGroup(t *testing.T) {
	tests := []struct {
		name      string
		plan      *SubscriptionPlan
		userGroup string
		expected  string
	}{
		{
			name: "prefer explicit restrict group",
			plan: &SubscriptionPlan{
				ModelRestrictGroup: "vip",
				UpgradeGroup:       "pro",
			},
			userGroup: "default",
			expected:  "vip",
		},
		{
			name: "fallback to upgrade group",
			plan: &SubscriptionPlan{
				UpgradeGroup: "pro",
			},
			userGroup: "default",
			expected:  "pro",
		},
		{
			name:      "fallback to user group",
			plan:      &SubscriptionPlan{},
			userGroup: "default",
			expected:  "default",
		},
		{
			name:      "nil plan",
			plan:      nil,
			userGroup: "default",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolveSubscriptionModelRestrictGroup(tt.plan, tt.userGroup))
		})
	}
}

func TestIsSubscriptionGroupModelAllowed(t *testing.T) {
	seedModelEnableGroupsForTest(t, map[string][]string{
		"gpt-vip":     {"vip"},
		"gpt-pro":     {"pro"},
		"gpt-default": {"default"},
		"gpt-all":     {"all"},
	})

	t.Run("explicit restrict group only allows that group", func(t *testing.T) {
		plan := &SubscriptionPlan{
			ModelRestrictMode:  "group",
			ModelRestrictGroup: "vip",
			UpgradeGroup:       "pro",
		}
		assert.True(t, isSubscriptionGroupModelAllowed(plan, "gpt-vip", "default"))
		assert.False(t, isSubscriptionGroupModelAllowed(plan, "gpt-pro", "default"))
	})

	t.Run("fallback to upgrade group when no restrict group selected", func(t *testing.T) {
		plan := &SubscriptionPlan{
			ModelRestrictMode: "group",
			UpgradeGroup:      "pro",
		}
		assert.True(t, isSubscriptionGroupModelAllowed(plan, "gpt-pro", "default"))
		assert.False(t, isSubscriptionGroupModelAllowed(plan, "gpt-default", "default"))
	})

	t.Run("fallback to user group when no restrict or upgrade group", func(t *testing.T) {
		plan := &SubscriptionPlan{
			ModelRestrictMode: "group",
		}
		assert.True(t, isSubscriptionGroupModelAllowed(plan, "gpt-default", "default"))
		assert.False(t, isSubscriptionGroupModelAllowed(plan, "gpt-pro", "default"))
	})

	t.Run("all group remains allowed", func(t *testing.T) {
		plan := &SubscriptionPlan{
			ModelRestrictMode:  "group",
			ModelRestrictGroup: "vip",
		}
		assert.True(t, isSubscriptionGroupModelAllowed(plan, "gpt-all", "default"))
	})
}

func TestIsSubscriptionGroupRequestAllowedRequiresRequestGroup(t *testing.T) {
	seedModelEnableGroupsForTest(t, map[string][]string{
		"gpt-shared": {"default", "vip"},
	})

	plan := &SubscriptionPlan{
		ModelRestrictMode:  "group",
		ModelRestrictGroup: "vip",
	}

	assert.True(t, isSubscriptionModelAllowedForRequestGroup(plan, "gpt-shared", "default", "vip"))
	assert.False(t, isSubscriptionModelAllowedForRequestGroup(plan, "gpt-shared", "default", "default"))
}

func TestIsSubscriptionModelAllowed(t *testing.T) {
	seedModelEnableGroupsForTest(t, map[string][]string{
		"gpt-vip":     {"vip"},
		"gpt-pro":     {"pro"},
		"gpt-default": {"default"},
		"gpt-all":     {"all"},
	})

	t.Run("default mode allows any model regardless of user group", func(t *testing.T) {
		plan := &SubscriptionPlan{
			ModelRestrictMode: "",
		}
		assert.True(t, isSubscriptionModelAllowed(plan, "gpt-vip", "vip"))
		assert.True(t, isSubscriptionModelAllowed(plan, "gpt-pro", "pro"))
		assert.True(t, isSubscriptionModelAllowed(plan, "unknown-model", ""))
	})

	t.Run("group mode uses propagated user group when no explicit restrict group exists", func(t *testing.T) {
		plan := &SubscriptionPlan{
			ModelRestrictMode: "group",
		}
		assert.True(t, isSubscriptionModelAllowed(plan, "gpt-vip", "vip"))
		assert.False(t, isSubscriptionModelAllowed(plan, "gpt-vip", "pro"))
	})

	t.Run("group mode rejects empty propagated user group unless restrict group is explicit", func(t *testing.T) {
		plan := &SubscriptionPlan{
			ModelRestrictMode: "group",
		}
		assert.False(t, isSubscriptionModelAllowed(plan, "gpt-vip", ""))

		explicitPlan := &SubscriptionPlan{
			ModelRestrictMode:  "group",
			ModelRestrictGroup: "vip",
		}
		assert.True(t, isSubscriptionModelAllowed(explicitPlan, "gpt-vip", ""))
	})

	t.Run("custom mode only enforces allowlist patterns", func(t *testing.T) {
		plan := &SubscriptionPlan{
			ModelRestrictMode: "custom",
			AllowedModels:     `["gpt-vip","gpt-*"]`,
		}
		assert.True(t, isSubscriptionModelAllowed(plan, "gpt-vip", "pro"))
		assert.True(t, isSubscriptionModelAllowed(plan, "gpt-default", "default"))
		assert.False(t, isSubscriptionModelAllowed(plan, "claude-3-5", "vip"))
	})
}

func truncateSubscriptionTables(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM subscription_pre_consume_records")
		DB.Exec("DELETE FROM user_subscriptions")
		DB.Exec("DELETE FROM subscription_plans")
		DB.Exec("DELETE FROM users")
	})
}

func seedSubscriptionTestUser(t *testing.T, id int, group string) {
	t.Helper()
	user := &User{
		Id:       id,
		Username: "subscription-test-user",
		Group:    group,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(user).Error)
}

func seedSubscriptionTestPlan(t *testing.T, plan *SubscriptionPlan) {
	t.Helper()
	if plan == nil {
		t.Fatal("plan is nil")
	}
	if plan.Title == "" {
		plan.Title = "test-plan"
	}
	if plan.Currency == "" {
		plan.Currency = "USD"
	}
	if plan.DurationUnit == "" {
		plan.DurationUnit = SubscriptionDurationMonth
	}
	if plan.DurationValue <= 0 {
		plan.DurationValue = 1
	}
	require.NoError(t, DB.Create(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
}

func seedSubscriptionTestUserSubscription(t *testing.T, sub *UserSubscription) {
	t.Helper()
	if sub == nil {
		t.Fatal("subscription is nil")
	}
	now := time.Now()
	if sub.Status == "" {
		sub.Status = "active"
	}
	if sub.StartTime == 0 {
		sub.StartTime = now.Unix()
	}
	if sub.EndTime == 0 {
		sub.EndTime = now.Add(30 * 24 * time.Hour).Unix()
	}
	require.NoError(t, DB.Create(sub).Error)
}

func TestPreConsumeUserSubscriptionPrefersHigherSortOrderPlan(t *testing.T) {
	truncateSubscriptionTables(t)

	seedModelEnableGroupsForTest(t, map[string][]string{
		"gpt-shared": {"default", "vip"},
	})

	seedSubscriptionTestUser(t, 9101, "default")

	generalPlan := &SubscriptionPlan{
		Id:                 9201,
		Title:              "general-plan",
		SortOrder:          0,
		ModelRestrictMode:  "group",
		ModelRestrictGroup: "default",
	}
	priorityPlan := &SubscriptionPlan{
		Id:                 9202,
		Title:              "priority-plan",
		SortOrder:          1,
		ModelRestrictMode:  "group",
		ModelRestrictGroup: "default",
	}
	seedSubscriptionTestPlan(t, generalPlan)
	seedSubscriptionTestPlan(t, priorityPlan)

	now := time.Now()
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:          9301,
		UserId:      9101,
		PlanId:      generalPlan.Id,
		AmountTotal: 100,
		EndTime:     now.Add(24 * time.Hour).Unix(),
	})
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:          9302,
		UserId:      9101,
		PlanId:      priorityPlan.Id,
		AmountTotal: 100,
		EndTime:     now.Add(48 * time.Hour).Unix(),
	})

	result, err := PreConsumeUserSubscription("req-prefer-higher-sort-order", 9101, "gpt-shared", 0, 10)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 9302, result.UserSubscriptionId)

	var generalSub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9301).First(&generalSub).Error)
	assert.Equal(t, int64(0), generalSub.AmountUsed)

	var prioritySub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9302).First(&prioritySub).Error)
	assert.Equal(t, int64(10), prioritySub.AmountUsed)
}

func TestPreConsumeUserSubscriptionForGroupSkipsRestrictedPlanForOtherRequestGroup(t *testing.T) {
	truncateSubscriptionTables(t)

	seedModelEnableGroupsForTest(t, map[string][]string{
		"gpt-shared": {"default", "vip"},
	})

	seedSubscriptionTestUser(t, 9102, "default")

	vipPlan := &SubscriptionPlan{
		Id:                 9211,
		Title:              "vip-plan",
		SortOrder:          1,
		ModelRestrictMode:  "group",
		ModelRestrictGroup: "vip",
	}
	generalPlan := &SubscriptionPlan{
		Id:                 9212,
		Title:              "general-plan",
		SortOrder:          0,
		ModelRestrictMode:  "group",
		ModelRestrictGroup: "default",
	}
	seedSubscriptionTestPlan(t, vipPlan)
	seedSubscriptionTestPlan(t, generalPlan)

	now := time.Now()
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:          9311,
		UserId:      9102,
		PlanId:      vipPlan.Id,
		AmountTotal: 100,
		EndTime:     now.Add(48 * time.Hour).Unix(),
	})
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:          9312,
		UserId:      9102,
		PlanId:      generalPlan.Id,
		AmountTotal: 100,
		EndTime:     now.Add(24 * time.Hour).Unix(),
	})

	result, err := PreConsumeUserSubscriptionForGroup("req-request-default-group", 9102, "gpt-shared", "default", 0, 10)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 9312, result.UserSubscriptionId)

	var vipSub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9311).First(&vipSub).Error)
	assert.Equal(t, int64(0), vipSub.AmountUsed)

	var generalSub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9312).First(&generalSub).Error)
	assert.Equal(t, int64(10), generalSub.AmountUsed)
}

func getSubscriptionTestUserGroup(t *testing.T, userId int) string {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("group").Where("id = ?", userId).First(&user).Error)
	return user.Group
}

func TestCreateUserSubscriptionFromPlanTxInheritsRollbackGroupForSameUpgradeGroup(t *testing.T) {
	truncateSubscriptionTables(t)

	seedSubscriptionTestUser(t, 9401, "user")
	plan := &SubscriptionPlan{
		Id:            9501,
		Title:         "model-plan",
		UpgradeGroup:  "model",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
	}
	seedSubscriptionTestPlan(t, plan)

	firstSub, err := CreateUserSubscriptionFromPlanTx(DB, 9401, plan, "admin")
	require.NoError(t, err)
	require.NotNil(t, firstSub)
	assert.Equal(t, "user", firstSub.PrevUserGroup)
	assert.Equal(t, "model", getSubscriptionTestUserGroup(t, 9401))

	secondSub, err := CreateUserSubscriptionFromPlanTx(DB, 9401, plan, "admin")
	require.NoError(t, err)
	require.NotNil(t, secondSub)
	assert.Equal(t, "user", secondSub.PrevUserGroup)
	assert.Equal(t, "model", getSubscriptionTestUserGroup(t, 9401))
}

func TestCreateUserSubscriptionWithRenewModeStartsAfterLatestSamePlan(t *testing.T) {
	truncateSubscriptionTables(t)

	seedSubscriptionTestUser(t, 9406, "user")
	plan := &SubscriptionPlan{
		Id:            9506,
		Title:         "renew-plan",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
		TotalAmount:   100,
	}
	seedSubscriptionTestPlan(t, plan)

	firstSub, err := CreateUserSubscriptionFromPlanTx(DB, 9406, plan, "admin")
	require.NoError(t, err)
	require.NotNil(t, firstSub)

	secondSub, err := CreateUserSubscriptionFromPlanWithModeTx(DB, 9406, plan, "admin", SubscriptionPurchaseModeRenew)
	require.NoError(t, err)
	require.NotNil(t, secondSub)
	assert.Equal(t, firstSub.EndTime, secondSub.StartTime)

	thirdSub, err := CreateUserSubscriptionFromPlanWithModeTx(DB, 9406, plan, "admin", SubscriptionPurchaseModeRenew)
	require.NoError(t, err)
	require.NotNil(t, thirdSub)
	assert.Equal(t, secondSub.EndTime, thirdSub.StartTime)
}

func TestFutureRenewalSubscriptionIsNotConsumableBeforeStart(t *testing.T) {
	truncateSubscriptionTables(t)

	now := GetDBTimestamp()
	seedSubscriptionTestUser(t, 9407, "user")
	seedSubscriptionTestPlan(t, &SubscriptionPlan{
		Id:            9507,
		Title:         "future-plan",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
		TotalAmount:   100,
	})
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:          9557,
		UserId:      9407,
		PlanId:      9507,
		Status:      "active",
		AmountTotal: 100,
		StartTime:   now + 3600,
		EndTime:     now + 90000,
	})

	hasActive, err := HasActiveUserSubscription(9407)
	require.NoError(t, err)
	assert.False(t, hasActive)

	activeSubs, err := GetAllActiveUserSubscriptions(9407)
	require.NoError(t, err)
	assert.Empty(t, activeSubs)

	_, err = PreConsumeUserSubscription("req-future-renewal", 9407, "gpt-shared", 0, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active subscription")
}

func TestCompleteSubscriptionOrderUsesStoredRenewMode(t *testing.T) {
	truncateSubscriptionTables(t)

	seedSubscriptionTestUser(t, 9408, "user")
	plan := &SubscriptionPlan{
		Id:            9508,
		Title:         "order-renew-plan",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   100,
	}
	seedSubscriptionTestPlan(t, plan)
	firstSub, err := CreateUserSubscriptionFromPlanTx(DB, 9408, plan, "admin")
	require.NoError(t, err)
	require.NotNil(t, firstSub)

	order := &SubscriptionOrder{
		UserId:          9408,
		PlanId:          plan.Id,
		Money:           1,
		TradeNo:         "sub-renew-order",
		PaymentMethod:   PaymentProviderStripe,
		PaymentProvider: PaymentProviderStripe,
		PurchaseMode:    SubscriptionPurchaseModeRenew,
		Status:          common.TopUpStatusPending,
		CreateTime:      GetDBTimestamp(),
	}
	require.NoError(t, DB.Create(order).Error)

	err = CompleteSubscriptionOrder("sub-renew-order", "", PaymentProviderStripe, "", "127.0.0.1")
	require.NoError(t, err)

	var renewedSub UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ? AND source = ?", 9408, plan.Id, "order").
		Order("id desc").
		First(&renewedSub).Error)
	assert.Equal(t, firstSub.EndTime, renewedSub.StartTime)
}

func TestExpireDueSubscriptionsRollsBackCurrentUpgradeGroupOnly(t *testing.T) {
	truncateSubscriptionTables(t)

	now := GetDBTimestamp()
	seedSubscriptionTestUser(t, 9402, "model")
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:            9511,
		UserId:        9402,
		Status:        "active",
		UpgradeGroup:  "user",
		PrevUserGroup: "user",
		EndTime:       now + 86400,
	})
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:            9512,
		UserId:        9402,
		Status:        "active",
		UpgradeGroup:  "model",
		PrevUserGroup: "user",
		EndTime:       now - 1,
	})

	expired, err := ExpireDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, expired)
	assert.Equal(t, "user", getSubscriptionTestUserGroup(t, 9402))

	var expiredSub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9512).First(&expiredSub).Error)
	assert.Equal(t, "expired", expiredSub.Status)
}

func TestExpireDueSubscriptionsIgnoresHistoricalExpiredRollbackRecords(t *testing.T) {
	truncateSubscriptionTables(t)

	now := GetDBTimestamp()
	seedSubscriptionTestUser(t, 9405, "model")
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:            9541,
		UserId:        9405,
		Status:        "expired",
		UpgradeGroup:  "model",
		PrevUserGroup: "user",
		EndTime:       now - 86400,
	})
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:            9542,
		UserId:        9405,
		Status:        "active",
		UpgradeGroup:  "vip",
		PrevUserGroup: "model",
		EndTime:       now - 1,
	})

	expired, err := ExpireDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, expired)
	assert.Equal(t, "model", getSubscriptionTestUserGroup(t, 9405))

	var expiredSub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9542).First(&expiredSub).Error)
	assert.Equal(t, "expired", expiredSub.Status)
}

func TestAdminInvalidateUserSubscriptionKeepsGroupWhenSameUpgradeStillActive(t *testing.T) {
	truncateSubscriptionTables(t)

	now := GetDBTimestamp()
	seedSubscriptionTestUser(t, 9403, "model")
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:            9521,
		UserId:        9403,
		Status:        "active",
		UpgradeGroup:  "model",
		PrevUserGroup: "user",
		EndTime:       now + 86400,
	})
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:            9522,
		UserId:        9403,
		Status:        "active",
		UpgradeGroup:  "model",
		PrevUserGroup: "user",
		EndTime:       now + 172800,
	})

	msg, err := AdminInvalidateUserSubscription(9521)
	require.NoError(t, err)
	assert.Empty(t, msg)
	assert.Equal(t, "model", getSubscriptionTestUserGroup(t, 9403))

	var cancelledSub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9521).First(&cancelledSub).Error)
	assert.Equal(t, "cancelled", cancelledSub.Status)
}

func TestAdminDeleteUserSubscriptionIgnoresDifferentUpgradeGroupWhenRollingBack(t *testing.T) {
	truncateSubscriptionTables(t)

	now := GetDBTimestamp()
	seedSubscriptionTestUser(t, 9404, "model")
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:            9531,
		UserId:        9404,
		Status:        "active",
		UpgradeGroup:  "user",
		PrevUserGroup: "user",
		EndTime:       now + 86400,
	})
	seedSubscriptionTestUserSubscription(t, &UserSubscription{
		Id:            9532,
		UserId:        9404,
		Status:        "active",
		UpgradeGroup:  "model",
		PrevUserGroup: "user",
		EndTime:       now + 86400,
	})

	msg, err := AdminDeleteUserSubscription(9532)
	require.NoError(t, err)
	assert.Equal(t, "用户分组将回退到 user", msg)
	assert.Equal(t, "user", getSubscriptionTestUserGroup(t, 9404))

	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", 9532).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}
