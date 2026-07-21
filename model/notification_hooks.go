package model

import "sync"

type TopUpCompletedEvent struct {
	UserId          int
	OrderNo         string
	QuotaAdded      int64
	PaymentAmount   float64
	PaymentMethod   string
	PaymentProvider string
	CompletedAt     int64
}

type SubscriptionCompletedEvent struct {
	UserId             int
	SubscriptionId     int
	PlanId             int
	PlanTitle          string
	AmountTotal        int64
	StartTime          int64
	EndTime            int64
	NextResetTime      int64
	ResetPeriod        string
	PaymentAmount      float64
	PaymentMethod      string
	PaymentProvider    string
	SubscriptionSource string
}

var (
	notificationHooksMu       sync.RWMutex
	topUpCompletedHook        func(TopUpCompletedEvent)
	subscriptionCompletedHook func(SubscriptionCompletedEvent)
)

func SetTopUpCompletedHook(hook func(TopUpCompletedEvent)) {
	notificationHooksMu.Lock()
	defer notificationHooksMu.Unlock()
	topUpCompletedHook = hook
}

func SetSubscriptionCompletedHook(hook func(SubscriptionCompletedEvent)) {
	notificationHooksMu.Lock()
	defer notificationHooksMu.Unlock()
	subscriptionCompletedHook = hook
}

func emitTopUpCompleted(event TopUpCompletedEvent) {
	notificationHooksMu.RLock()
	hook := topUpCompletedHook
	notificationHooksMu.RUnlock()
	if hook != nil {
		hook(event)
	}
}

func emitSubscriptionCompleted(event SubscriptionCompletedEvent) {
	notificationHooksMu.RLock()
	hook := subscriptionCompletedHook
	notificationHooksMu.RUnlock()
	if hook != nil {
		hook(event)
	}
}
