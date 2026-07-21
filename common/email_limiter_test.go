package common

import (
	"testing"
	"time"
)

func resetEmailLimiterTestState() {
	emailDailyCountersMu.Lock()
	defer emailDailyCountersMu.Unlock()
	emailSendLocks = make(map[string]time.Time)
}

func TestQuotaNotificationSendLockSeparatesLevelsAndResets(t *testing.T) {
	originalRedisEnabled := RedisEnabled
	RedisEnabled = false
	t.Cleanup(func() {
		RedisEnabled = originalRedisEnabled
		resetEmailLimiterTestState()
	})
	resetEmailLimiterTestState()

	ok, _ := TryAcquireQuotaNotificationSendLock(42, "wallet", 0, "low")
	if !ok {
		t.Fatal("expected first low-balance notification lock to succeed")
	}
	ok, _ = TryAcquireQuotaNotificationSendLock(42, "wallet", 0, "low")
	if ok {
		t.Fatal("expected duplicate low-balance notification lock to be denied")
	}
	ok, _ = TryAcquireQuotaNotificationSendLock(42, "wallet", 0, "exhausted")
	if !ok {
		t.Fatal("expected exhausted notification to use an independent lock")
	}

	ResetQuotaNotificationSendLocks(42, "wallet", 0)
	ok, _ = TryAcquireQuotaNotificationSendLock(42, "wallet", 0, "low")
	if !ok {
		t.Fatal("expected replenishment reset to allow a new notification")
	}
}

func TestPasswordResetSendLockIsPerReceiver(t *testing.T) {
	originalRedisEnabled := RedisEnabled
	RedisEnabled = false
	t.Cleanup(func() {
		RedisEnabled = originalRedisEnabled
		resetEmailLimiterTestState()
	})
	resetEmailLimiterTestState()

	ok, _ := TryAcquirePasswordResetSendLock("User@example.com")
	if !ok {
		t.Fatal("expected first password-reset lock to succeed")
	}
	ok, _ = TryAcquirePasswordResetSendLock(" user@example.com ")
	if ok {
		t.Fatal("expected normalized duplicate receiver to be denied")
	}
	ok, _ = TryAcquirePasswordResetSendLock("other@example.com")
	if !ok {
		t.Fatal("expected another receiver to have an independent lock")
	}
}
