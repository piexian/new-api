package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var ErrDuplicateEmailSuppressed = errors.New("duplicate email suppressed")

var emailDailyCounters = make(map[string]int64)
var emailDailyCountersMu sync.Mutex
var emailSendLocks = make(map[string]time.Time)

const (
	emailSendDedupWindow          = 2 * time.Minute
	emailVerificationSendLockTime = 5 * time.Minute
)

func emailDailyKey() string {
	return "email:daily:" + time.Now().Format("20060102")
}

func emailVerificationDailyKey(email string) string {
	return "email:verification:daily:" + time.Now().Format("20060102") + ":" + strings.ToLower(email)
}

func emailVerificationSendLockKey(email string) string {
	return "email:verification:send-lock:" + strings.ToLower(strings.TrimSpace(email))
}

func emailSendDedupKey(subject string, receiver string, content string) string {
	hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(receiver)) + "\x00" + subject + "\x00" + content))
	return "email:send:dedup:" + hex.EncodeToString(hash[:])
}

func secondsUntilMidnight() time.Duration {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	return midnight.Sub(now)
}

func tryAcquireEmailLock(key string, ttl time.Duration) (bool, func()) {
	if ttl <= 0 {
		ttl = time.Minute
	}
	if RedisEnabled {
		ctx := context.Background()
		ok, err := RDB.SetNX(ctx, key, "1", ttl).Result()
		if err != nil {
			SysError(fmt.Sprintf("failed to acquire email lock %s: %v", key, err))
			return true, func() {}
		}
		return ok, func() {
			_ = RDB.Del(ctx, key).Err()
		}
	}

	emailDailyCountersMu.Lock()
	defer emailDailyCountersMu.Unlock()
	now := time.Now()
	if expiresAt, ok := emailSendLocks[key]; ok && expiresAt.After(now) {
		return false, func() {}
	}
	emailSendLocks[key] = now.Add(ttl)
	return true, func() {
		emailDailyCountersMu.Lock()
		defer emailDailyCountersMu.Unlock()
		delete(emailSendLocks, key)
	}
}

// TryAcquireEmailVerificationSendLock prevents rapid duplicate verification-code sends to one address.
func TryAcquireEmailVerificationSendLock(email string) (bool, func()) {
	return tryAcquireEmailLock(emailVerificationSendLockKey(email), emailVerificationSendLockTime)
}

// TryAcquireEmailSendDedupLock prevents sending the exact same email multiple times in a short window.
func TryAcquireEmailSendDedupLock(subject string, receiver string, content string) (bool, func()) {
	return tryAcquireEmailLock(emailSendDedupKey(subject, receiver, content), emailSendDedupWindow)
}

// CheckEmailDailyLimit returns an error if the global daily email limit has been reached.
func CheckEmailDailyLimit() error {
	if EmailDailyLimit <= 0 {
		return nil
	}
	if RedisEnabled {
		return checkEmailDailyLimitRedis()
	}
	return checkEmailDailyLimitMemory()
}

func checkEmailDailyLimitRedis() error {
	ctx := context.Background()
	key := emailDailyKey()
	count, err := RDB.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		count = 0
	} else if err != nil {
		return fmt.Errorf("failed to check daily email limit: %w", err)
	}
	if count >= int64(EmailDailyLimit) {
		return fmt.Errorf("daily email sending limit reached (%d/%d)", count, EmailDailyLimit)
	}
	return nil
}

func checkEmailDailyLimitMemory() error {
	emailDailyCountersMu.Lock()
	defer emailDailyCountersMu.Unlock()
	key := emailDailyKey()
	count := emailDailyCounters[key]
	if count >= int64(EmailDailyLimit) {
		return fmt.Errorf("daily email sending limit reached (%d/%d)", count, EmailDailyLimit)
	}
	return nil
}

// IncrEmailDailyCount increments the daily email counter after a successful send.
func IncrEmailDailyCount() {
	if EmailDailyLimit <= 0 {
		return
	}
	if RedisEnabled {
		ctx := context.Background()
		key := emailDailyKey()
		count, err := RDB.Incr(ctx, key).Result()
		if err == nil && count == 1 {
			_ = RDB.Expire(ctx, key, secondsUntilMidnight()).Err()
		}
		return
	}
	emailDailyCountersMu.Lock()
	defer emailDailyCountersMu.Unlock()
	key := emailDailyKey()
	emailDailyCounters[key]++
}

// CheckEmailVerificationDailyLimit returns an error if the per-email daily verification limit has been reached.
func CheckEmailVerificationDailyLimit(email string) error {
	if EmailVerificationDailyLimitPerUser <= 0 {
		return nil
	}
	if RedisEnabled {
		return checkEmailVerificationDailyLimitRedis(email)
	}
	return checkEmailVerificationDailyLimitMemory(email)
}

func checkEmailVerificationDailyLimitRedis(email string) error {
	ctx := context.Background()
	key := emailVerificationDailyKey(email)
	count, err := RDB.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		count = 0
	} else if err != nil {
		return fmt.Errorf("failed to check email verification limit: %w", err)
	}
	if count >= int64(EmailVerificationDailyLimitPerUser) {
		return fmt.Errorf("verification code daily limit reached for this email (%d/%d)", count, EmailVerificationDailyLimitPerUser)
	}
	return nil
}

func checkEmailVerificationDailyLimitMemory(email string) error {
	emailDailyCountersMu.Lock()
	defer emailDailyCountersMu.Unlock()
	key := emailVerificationDailyKey(email)
	count := emailDailyCounters[key]
	if count >= int64(EmailVerificationDailyLimitPerUser) {
		return fmt.Errorf("verification code daily limit reached for this email (%d/%d)", count, EmailVerificationDailyLimitPerUser)
	}
	return nil
}

// IncrEmailVerificationDailyCount increments the per-email daily verification counter after a successful send.
func IncrEmailVerificationDailyCount(email string) {
	if EmailVerificationDailyLimitPerUser <= 0 {
		return
	}
	if RedisEnabled {
		ctx := context.Background()
		key := emailVerificationDailyKey(email)
		count, err := RDB.Incr(ctx, key).Result()
		if err == nil && count == 1 {
			_ = RDB.Expire(ctx, key, secondsUntilMidnight()).Err()
		}
		return
	}
	emailDailyCountersMu.Lock()
	defer emailDailyCountersMu.Unlock()
	key := emailVerificationDailyKey(email)
	emailDailyCounters[key]++
}

// CleanupExpiredEmailCounters removes expired email counters. Called periodically.
func CleanupExpiredEmailCounters() {
	emailDailyCountersMu.Lock()
	defer emailDailyCountersMu.Unlock()
	todayKey := emailDailyKey()
	now := time.Now()
	for key := range emailDailyCounters {
		if key != todayKey && !strings.HasPrefix(key, "email:verification:daily:"+time.Now().Format("20060102")) {
			delete(emailDailyCounters, key)
		}
	}
	for key, expiresAt := range emailSendLocks {
		if !expiresAt.After(now) {
			delete(emailSendLocks, key)
		}
	}
}
