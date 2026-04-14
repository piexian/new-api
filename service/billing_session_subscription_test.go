package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedSubscriptionPlan(t *testing.T, plan *model.SubscriptionPlan) {
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
		plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if plan.DurationValue <= 0 {
		plan.DurationValue = 1
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to create subscription plan: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)
}

func seedSubscriptionWithPlan(t *testing.T, sub *model.UserSubscription) {
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
	require.NoError(t, model.DB.Create(sub).Error)
}

func newBillingTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

func newSubscriptionRelayInfo(userId int, requestId string, preference string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		UserId:          userId,
		RequestId:       requestId,
		OriginModelName: "gpt-4o",
		IsPlayground:    true,
		UserSetting: dto.UserSetting{
			BillingPreference: preference,
		},
	}
}

func TestNewBillingSession_SubscriptionFirstUsesNextAccessibleSubscription(t *testing.T) {
	truncate(t)
	seedUser(t, 1101, 0)

	plan1 := &model.SubscriptionPlan{
		Id:                2101,
		Title:             "restricted",
		ModelRestrictMode: "custom",
		AllowedModels:     `["claude-*"]`,
	}
	plan2 := &model.SubscriptionPlan{
		Id:    2102,
		Title: "fallback",
	}
	seedSubscriptionPlan(t, plan1)
	seedSubscriptionPlan(t, plan2)

	seedSubscriptionWithPlan(t, &model.UserSubscription{
		Id:          3101,
		UserId:      1101,
		PlanId:      plan1.Id,
		AmountTotal: 100,
	})
	seedSubscriptionWithPlan(t, &model.UserSubscription{
		Id:          3102,
		UserId:      1101,
		PlanId:      plan2.Id,
		AmountTotal: 100,
	})

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1101, "req-subscription-first-next-usable", "subscription_first"),
		10,
	)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceSubscription, session.relayInfo.BillingSource)
	assert.Equal(t, plan2.Id, session.relayInfo.SubscriptionPlanId)
	assert.Equal(t, 10, session.GetPreConsumedQuota())

	var firstSub model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3101).First(&firstSub).Error)
	assert.Equal(t, int64(0), firstSub.AmountUsed)

	var secondSub model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3102).First(&secondSub).Error)
	assert.Equal(t, int64(10), secondSub.AmountUsed)
}

func TestNewBillingSession_SubscriptionFirstFallsBackToWalletWhenNoAccessibleSubscription(t *testing.T) {
	truncate(t)
	seedUser(t, 1102, 100)

	plan := &model.SubscriptionPlan{
		Id:                2201,
		Title:             "restricted",
		ModelRestrictMode: "custom",
		AllowedModels:     `["claude-*"]`,
	}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{
		Id:          3201,
		UserId:      1102,
		PlanId:      plan.Id,
		AmountTotal: 100,
	})

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1102, "req-subscription-first-wallet-fallback", "subscription_first"),
		10,
	)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceWallet, session.relayInfo.BillingSource)

	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 1102).First(&user).Error)
	assert.Equal(t, 90, user.Quota)

	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3201).First(&sub).Error)
	assert.Equal(t, int64(0), sub.AmountUsed)
}

func TestNewBillingSession_SubscriptionOnlyDoesNotFallbackToWallet(t *testing.T) {
	truncate(t)
	seedUser(t, 1103, 100)

	plan := &model.SubscriptionPlan{
		Id:                2301,
		Title:             "restricted",
		ModelRestrictMode: "custom",
		AllowedModels:     `["claude-*"]`,
	}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{
		Id:          3301,
		UserId:      1103,
		PlanId:      plan.Id,
		AmountTotal: 100,
	})

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1103, "req-subscription-only-no-wallet-fallback", "subscription_only"),
		10,
	)
	require.Nil(t, session)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())

	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 1103).First(&user).Error)
	assert.Equal(t, 100, user.Quota)

	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3301).First(&sub).Error)
	assert.Equal(t, int64(0), sub.AmountUsed)
}
