package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestQuotaNotificationLevelForRemaining(t *testing.T) {
	tests := []struct {
		name      string
		remaining int64
		threshold int64
		want      string
	}{
		{name: "negative is exhausted", remaining: -1, threshold: 100, want: quotaNotificationLevelExhausted},
		{name: "zero is exhausted", remaining: 0, threshold: 100, want: quotaNotificationLevelExhausted},
		{name: "positive below threshold is low", remaining: 99, threshold: 100, want: quotaNotificationLevelLow},
		{name: "threshold boundary is healthy", remaining: 100, threshold: 100, want: ""},
		{name: "disabled threshold ignores positive balance", remaining: 1, threshold: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, quotaNotificationLevelForRemaining(tt.remaining, tt.threshold))
		})
	}
}

func TestQuotaNotificationTransition(t *testing.T) {
	tests := []struct {
		name      string
		before    int64
		remaining int64
		threshold int64
		want      string
	}{
		{name: "healthy crosses low threshold", before: 100, remaining: 99, threshold: 100, want: quotaNotificationLevelLow},
		{name: "already low stays low", before: 99, remaining: 50, threshold: 100, want: ""},
		{name: "low balance becomes exhausted", before: 50, remaining: 0, threshold: 100, want: quotaNotificationLevelExhausted},
		{name: "healthy balance becomes exhausted", before: 100, remaining: -1, threshold: 100, want: quotaNotificationLevelExhausted},
		{name: "already exhausted stays exhausted", before: 0, remaining: -1, threshold: 100, want: ""},
		{name: "positive balance with disabled threshold stays silent", before: 10, remaining: 1, threshold: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, quotaNotificationTransition(tt.before, tt.remaining, tt.threshold))
		})
	}
}

func TestFormatEmailDuration(t *testing.T) {
	duration := 26*time.Hour + 5*time.Minute
	assert.Equal(t, "1 day 2 hours 5 minutes", formatEmailDuration(duration, "en"))
	assert.Equal(t, "1 天 2 小时 5 分钟", formatEmailDuration(duration, "zh-CN"))
	assert.Equal(t, "1 天 2 小時 5 分鐘", formatEmailDuration(duration, "zh-TW"))
	assert.Equal(t, "1 minute", formatEmailDuration(time.Second, "en"))
}

func TestShouldNotifySubscriptionCompletedFiltersRegistrationGift(t *testing.T) {
	base := model.SubscriptionCompletedEvent{UserId: 1, SubscriptionId: 2, SubscriptionSource: "order"}
	assert.True(t, shouldNotifySubscriptionCompleted(base))

	base.SubscriptionSource = "auto"
	assert.False(t, shouldNotifySubscriptionCompleted(base))
}
