package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemoryRateLimitWaitAndSuccessCounting(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)
	for i := 0; i < 3; i++ {
		allowed, wait := limiter.CheckWithRetry("success", 1, 60)
		require.True(t, allowed)
		require.Zero(t, wait)
	}
	require.True(t, limiter.Request("success", 1, 60))
	allowed, wait := limiter.CheckWithRetry("success", 1, 60)
	require.False(t, allowed)
	require.InDelta(t, 60, wait, 1)
	*limiter.store["success"] = []int64{time.Now().Unix() - 61}
	allowed, wait = limiter.RequestWithRetry("success", 1, 60)
	require.True(t, allowed)
	require.Zero(t, wait)
	for i := 0; i < 3; i++ {
		require.True(t, limiter.Request("unlimited", 0, 60))
	}
}

func TestMemorySuccessLimitRetainsLatestConcurrentCompletion(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)
	oldCompletion := []int64{time.Now().Unix() - 59}
	limiter.store["success"] = &oldCompletion
	limiter.RecordCompletion("success", 1)
	allowed, wait := limiter.CheckWithRetry("success", 1, 60)
	require.False(t, allowed)
	require.InDelta(t, 60, wait, 1)
	require.Len(t, *limiter.store["success"], 1)
}
