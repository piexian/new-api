package limiter

import (
	"context"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	return client
}

func TestAllowLoadsMissingScript(t *testing.T) {
	ctx := context.Background()
	client := newTestRedisClient(t)
	script := redis.NewScript(rateLimitScript)

	exists, err := client.ScriptExists(ctx, script.Hash()).Result()
	require.NoError(t, err)
	require.Equal(t, []bool{false}, exists)

	allowed, err := New(ctx, client).Allow(ctx, "rateLimit:1")
	require.NoError(t, err)
	require.True(t, allowed)

	exists, err = client.ScriptExists(ctx, script.Hash()).Result()
	require.NoError(t, err)
	require.Equal(t, []bool{true}, exists)
}

func TestAllowRecoversAfterScriptFlush(t *testing.T) {
	ctx := context.Background()
	client := newTestRedisClient(t)
	limiter := New(ctx, client)

	allowed, err := limiter.Allow(ctx, "rateLimit:2")
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, client.ScriptFlush(ctx).Err())

	allowed, err = limiter.Allow(ctx, "rateLimit:2")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestAllowConcurrentCacheMissPreservesCapacity(t *testing.T) {
	ctx := context.Background()
	client := newTestRedisClient(t)
	limiter := New(ctx, client)
	require.NoError(t, client.ScriptFlush(ctx).Err())

	const (
		capacity = 5
		requests = 20
	)
	start := make(chan struct{})
	results := make(chan bool, requests)
	errs := make(chan error, requests)
	var waitGroup sync.WaitGroup

	for range requests {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			allowed, err := limiter.Allow(
				ctx,
				"rateLimit:3",
				WithCapacity(capacity),
				WithRate(0),
			)
			results <- allowed
			errs <- err
		}()
	}

	close(start)
	waitGroup.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	allowedCount := 0
	for allowed := range results {
		if allowed {
			allowedCount++
		}
	}
	require.Equal(t, capacity, allowedCount)
}

func TestAllowDoesNotMaskWrongType(t *testing.T) {
	ctx := context.Background()
	client := newTestRedisClient(t)
	key := "rateLimit:4"
	require.NoError(t, limitScript.Load(ctx, client).Err())
	require.NoError(t, client.LPush(ctx, key, "legacy").Err())

	allowed, err := New(ctx, client).Allow(ctx, key)
	require.False(t, allowed)
	require.ErrorContains(t, err, "WRONGTYPE")
	require.Equal(t, "list", client.Type(ctx, key).Val())
}
