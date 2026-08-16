package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
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
	sub.AllowWalletOverflow = true
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

func seedModelGroups(t *testing.T, modelName string, groups ...string) {
	t.Helper()
	channel := &model.Channel{
		Id:     9000 + len(groups),
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "test-channel",
		Group:  "default",
		Models: modelName,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	for _, group := range groups {
		require.NoError(t, model.DB.Create(&model.Ability{
			Group:     group,
			Model:     modelName,
			ChannelId: channel.Id,
			Enabled:   true,
		}).Error)
	}
	model.InvalidatePricingCache()
	t.Cleanup(model.InvalidatePricingCache)
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

func TestNewBillingSession_SubscriptionFirstFallsBackWhenRequestGroupDoesNotMatchRestrictedSubscription(t *testing.T) {
	truncate(t)
	seedUser(t, 1104, 100)
	seedModelGroups(t, "gpt-4o", "default", "vip")

	plan := &model.SubscriptionPlan{
		Id:                 2401,
		Title:              "vip-only",
		ModelRestrictMode:  "group",
		ModelRestrictGroup: "vip",
	}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{
		Id:          3401,
		UserId:      1104,
		PlanId:      plan.Id,
		AmountTotal: 100,
	})

	relayInfo := newSubscriptionRelayInfo(1104, "req-subscription-default-group-wallet-fallback", "subscription_first")
	relayInfo.UsingGroup = "default"

	session, apiErr := NewBillingSession(newBillingTestContext(), relayInfo, 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceWallet, session.relayInfo.BillingSource)

	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 1104).First(&user).Error)
	assert.Equal(t, 90, user.Quota)

	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3401).First(&sub).Error)
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

func TestNewBillingSession_SubscriptionFirstSplitsAcrossSubscriptions(t *testing.T) {
	truncate(t)
	seedUser(t, 1110, 0)

	plan := &model.SubscriptionPlan{Id: 2110, Title: "split"}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{Id: 3110, UserId: 1110, PlanId: plan.Id, AmountTotal: 30})
	seedSubscriptionWithPlan(t, &model.UserSubscription{Id: 3111, UserId: 1110, PlanId: plan.Id, AmountTotal: 100})

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1110, "req-split-across-subs", "subscription_first"),
		50,
	)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceSubscription, session.relayInfo.BillingSource)
	assert.Equal(t, 3110, session.relayInfo.SubscriptionId)
	assert.Equal(t, 50, session.GetPreConsumedQuota())

	var sub1 model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3110).First(&sub1).Error)
	assert.Equal(t, int64(30), sub1.AmountUsed)

	var sub2 model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3111).First(&sub2).Error)
	assert.Equal(t, int64(20), sub2.AmountUsed)

	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 1110).First(&user).Error)
	assert.Equal(t, 0, user.Quota)
}

func TestNewBillingSession_SubscriptionFirstSplitRemainderFallsToWallet(t *testing.T) {
	truncate(t)
	seedUser(t, 1111, 100)

	plan := &model.SubscriptionPlan{Id: 2111, Title: "partial"}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{Id: 3112, UserId: 1111, PlanId: plan.Id, AmountTotal: 30})

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1111, "req-split-remainder-wallet", "subscription_first"),
		50,
	)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	assert.Equal(t, BillingSourceSubscription, session.relayInfo.BillingSource)
	assert.Equal(t, 3112, session.relayInfo.SubscriptionId)

	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3112).First(&sub).Error)
	assert.Equal(t, int64(30), sub.AmountUsed)

	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 1111).First(&user).Error)
	assert.Equal(t, 80, user.Quota)
}

func TestNewBillingSession_SubscriptionFirstStrictSubscriptionBlocksWalletRemainder(t *testing.T) {
	truncate(t)
	seedUser(t, 1112, 100)

	plan := &model.SubscriptionPlan{Id: 2112, Title: "strict"}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{Id: 3113, UserId: 1112, PlanId: plan.Id, AmountTotal: 30})
	// seedSubscriptionWithPlan 默认允许超支，这里显式改为禁止
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", 3113).
		Update("allow_wallet_overflow", false).Error)

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1112, "req-strict-blocks-wallet", "subscription_first"),
		50,
	)
	require.Nil(t, session)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())

	// 预扣必须整体回滚：订阅不残留扣减，钱包不动
	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3113).First(&sub).Error)
	assert.Equal(t, int64(0), sub.AmountUsed)

	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 1112).First(&user).Error)
	assert.Equal(t, 100, user.Quota)
}

func TestNewBillingSession_SubscriptionOnlySplitsAcrossSubscriptions(t *testing.T) {
	truncate(t)
	seedUser(t, 1113, 100)

	plan := &model.SubscriptionPlan{Id: 2113, Title: "split-only"}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{Id: 3114, UserId: 1113, PlanId: plan.Id, AmountTotal: 30})
	seedSubscriptionWithPlan(t, &model.UserSubscription{Id: 3115, UserId: 1113, PlanId: plan.Id, AmountTotal: 100})

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1113, "req-subscription-only-split", "subscription_only"),
		50,
	)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceSubscription, session.relayInfo.BillingSource)

	var sub1 model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3114).First(&sub1).Error)
	assert.Equal(t, int64(30), sub1.AmountUsed)

	var sub2 model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3115).First(&sub2).Error)
	assert.Equal(t, int64(20), sub2.AmountUsed)

	// subscription_only 不动钱包
	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 1113).First(&user).Error)
	assert.Equal(t, 100, user.Quota)
}

func TestBillingSession_SettleExceedingSubscriptionFallsToWallet(t *testing.T) {
	truncate(t)
	seedUser(t, 1114, 1000)

	plan := &model.SubscriptionPlan{Id: 2114, Title: "settle-overflow"}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{Id: 3116, UserId: 1114, PlanId: plan.Id, AmountTotal: 100})

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1114, "req-settle-overflow-wallet", "subscription_first"),
		10,
	)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	// 实际消耗 150，超出预扣：订阅补扣到满（100），剩余 50 落钱包
	require.NoError(t, session.Settle(150))

	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("id = ?", 3116).First(&sub).Error)
	assert.Equal(t, int64(100), sub.AmountUsed)

	var user model.User
	require.NoError(t, model.DB.Where("id = ?", 1114).First(&user).Error)
	assert.Equal(t, 950, user.Quota)
}

func TestBillingSession_RefundReturnsSplitLegsAndWallet(t *testing.T) {
	truncate(t)
	seedUser(t, 1115, 100)

	plan := &model.SubscriptionPlan{Id: 2115, Title: "refund-split"}
	seedSubscriptionPlan(t, plan)
	seedSubscriptionWithPlan(t, &model.UserSubscription{Id: 3117, UserId: 1115, PlanId: plan.Id, AmountTotal: 30})

	session, apiErr := NewBillingSession(
		newBillingTestContext(),
		newSubscriptionRelayInfo(1115, "req-refund-split-legs", "subscription_first"),
		50,
	)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	ctx := newBillingTestContext()
	session.Refund(ctx)

	// Refund 异步执行，轮询等待退款落库
	require.Eventually(t, func() bool {
		var user model.User
		if err := model.DB.Where("id = ?", 1115).First(&user).Error; err != nil {
			return false
		}
		var sub model.UserSubscription
		if err := model.DB.Where("id = ?", 3117).First(&sub).Error; err != nil {
			return false
		}
		return user.Quota == 100 && sub.AmountUsed == 0
	}, 3*time.Second, 20*time.Millisecond)
}
