package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
			name: "fallback to user group",
			plan: &SubscriptionPlan{},
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
