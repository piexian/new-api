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
	vipPlan := &SubscriptionPlan{
		Id:                 9202,
		Title:              "vip-plan",
		SortOrder:          1,
		ModelRestrictMode:  "group",
		ModelRestrictGroup: "vip",
	}
	seedSubscriptionTestPlan(t, generalPlan)
	seedSubscriptionTestPlan(t, vipPlan)

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
		PlanId:      vipPlan.Id,
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

	var vipSub UserSubscription
	require.NoError(t, DB.Where("id = ?", 9302).First(&vipSub).Error)
	assert.Equal(t, int64(10), vipSub.AmountUsed)
}
