package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedBatchPlan(t *testing.T, plan *SubscriptionPlan) {
	t.Helper()
	require.NoError(t, DB.Create(plan).Error)
}

func TestBatchCreateSubscriptionsConcurrentAllStartNow(t *testing.T) {
	truncateTables(t)
	plan := &SubscriptionPlan{
		Id:            7001,
		Title:         "Batch-Concurrent",
		PriceAmount:   10,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   1000,
	}
	seedBatchPlan(t, plan)
	now := GetDBTimestamp()

	var subs []*UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		subs, err = CreateUserSubscriptionsFromPlanWithModeTx(tx, 1, plan, "wallet", SubscriptionPurchaseModeConcurrent, 3)
		return err
	}))
	require.Len(t, subs, 3)
	for _, sub := range subs {
		assert.Equal(t, now, sub.StartTime)
		assert.Equal(t, "active", sub.Status)
	}
	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", 1).Count(&count).Error)
	assert.Equal(t, int64(3), count)
}

func TestBatchCreateSubscriptionsRenewChainsEndTimes(t *testing.T) {
	truncateTables(t)
	plan := &SubscriptionPlan{
		Id:            7002,
		Title:         "Batch-Renew",
		PriceAmount:   10,
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 30,
		TotalAmount:   1000,
	}
	seedBatchPlan(t, plan)

	var subs []*UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		subs, err = CreateUserSubscriptionsFromPlanWithModeTx(tx, 2, plan, "wallet", SubscriptionPurchaseModeRenew, 3)
		return err
	}))
	require.Len(t, subs, 3)
	// 链式：第 i 份起点 = 第 i-1 份终点
	assert.Equal(t, subs[0].EndTime, subs[1].StartTime)
	assert.Equal(t, subs[1].EndTime, subs[2].StartTime)
	assert.Less(t, subs[0].StartTime, subs[1].StartTime)
}

func TestBatchCreateSubscriptionsRejectsOverLimit(t *testing.T) {
	truncateTables(t)
	// 硬顶 100，超硬顶直接拒绝
	plan := &SubscriptionPlan{
		Id:            7003,
		Title:         "Batch-Limit",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
	}
	seedBatchPlan(t, plan)
	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionsFromPlanWithModeTx(tx, 3, plan, "admin", SubscriptionPurchaseModeConcurrent, SubscriptionPurchaseQuantityHardCap+1)
		return err
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "单次最多购买")
}

func TestNormalizeSubscriptionPurchaseQuantity(t *testing.T) {
	// <=0 回落 1 份
	q, err := NormalizeSubscriptionPurchaseQuantity(0)
	require.NoError(t, err)
	assert.Equal(t, 1, q)
	// 合法范围直通
	q, err = NormalizeSubscriptionPurchaseQuantity(5)
	require.NoError(t, err)
	assert.Equal(t, 5, q)
	// 超上限报错（默认上限 10）
	_, err = NormalizeSubscriptionPurchaseQuantity(SubscriptionPurchaseQuantityDefault + 1)
	require.Error(t, err)
}

func TestWalletPurchaseSubscriptionBatchChargesQuotaTimesQuantity(t *testing.T) {
	truncateTables(t)
	plan := &SubscriptionPlan{
		PriceAmount:   1,
		Title:         "Batch-Wallet",
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
		TotalAmount:   500,
	}
	seedBatchPlan(t, plan)
	quotaPerUnit := int64(common.QuotaPerUnit)
	user := &User{Username: "batch-wallet", Quota: int(3 * quotaPerUnit)}
	require.NoError(t, DB.Create(user).Error)

	order, err := WalletPurchaseSubscription(user.Id, plan.Id, SubscriptionPurchaseModeConcurrent, 3, "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, 3, order.Quantity)
	// 金额按份数放大
	assert.InDelta(t, 3.0, order.Money, 0.0001)

	var fresh User
	require.NoError(t, DB.Where("id = ?", user.Id).First(&fresh).Error)
	assert.Equal(t, 0, fresh.Quota)

	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Equal(t, int64(3), count)
}

func TestWalletPurchaseSubscriptionBatchInsufficientQuotaBlocked(t *testing.T) {
	truncateTables(t)
	plan := &SubscriptionPlan{
		Id:            7005,
		Title:         "Batch-Wallet-Poor",
		PriceAmount:   2,
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
	}
	seedBatchPlan(t, plan)
	quotaPerUnit := int64(common.QuotaPerUnit)
	user := &User{Username: "batch-wallet-poor", Quota: int(2 * quotaPerUnit)}
	require.NoError(t, DB.Create(user).Error)

	// 3 份要 6 份的钱，只有 2 份：整单拒绝，不产生订阅、不扣款
	_, err := WalletPurchaseSubscription(user.Id, plan.Id, SubscriptionPurchaseModeConcurrent, 3, "127.0.0.1")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSubscriptionWalletQuotaNotEnough)

	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
	var fresh User
	require.NoError(t, DB.Where("id = ?", user.Id).First(&fresh).Error)
	assert.Equal(t, int(2*quotaPerUnit), fresh.Quota)
}

func TestWalletPurchaseSubscriptionBatchMaxPurchasePerUser(t *testing.T) {
	truncateTables(t)
	plan := &SubscriptionPlan{
		Id:                 7006,
		Title:              "Batch-Cap",
		PriceAmount:        0,
		DurationUnit:       SubscriptionDurationDay,
		DurationValue:      1,
		MaxPurchasePerUser: 2,
	}
	seedBatchPlan(t, plan)
	user := &User{Username: "batch-cap", Quota: 0}
	require.NoError(t, DB.Create(user).Error)

	// count(0)+3 > 上限 2 → 拒绝
	_, err := WalletPurchaseSubscription(user.Id, plan.Id, SubscriptionPurchaseModeConcurrent, 3, "127.0.0.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "购买上限")

	// 恰好 2 份可过
	_, err = WalletPurchaseSubscription(user.Id, plan.Id, SubscriptionPurchaseModeConcurrent, 2, "127.0.0.1")
	require.NoError(t, err)
}
