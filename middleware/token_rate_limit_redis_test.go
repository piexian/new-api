package middleware

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func useRateLimitTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	savedRedis, savedEnabled := common.RDB, common.RedisEnabled
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() {
		common.RDB, common.RedisEnabled = savedRedis, savedEnabled
		require.NoError(t, client.Close())
	})
	return srv
}

func TestTokenRedisRPMAndRetryAfter(t *testing.T) {
	for _, perIP := range []bool{false, true} {
		name := "token"
		rate, ipRate := 1, 0
		if perIP {
			name, rate, ipRate = "ip", 0, 1
		}
		t.Run(name, func(t *testing.T) {
			engine := setupTokenRateLimitEngine(t, 910001, rate, ipRate)
			srv := useRateLimitTestRedis(t)
			now := time.Unix(100000, 0)
			srv.SetTime(now)
			first := performTokenRateLimitRequest(t, engine, "192.0.2.1:1234")
			require.Equal(t, http.StatusOK, first.Code)
			require.Empty(t, first.Header().Get("Retry-After"))
			blocked := performTokenRateLimitRequest(t, engine, "192.0.2.1:1235")
			require.Equal(t, http.StatusTooManyRequests, blocked.Code)
			require.Equal(t, "60", blocked.Header().Get("Retry-After"))
			require.Contains(t, blocked.Body.String(), `"new_api_error":true`)
			srv.SetTime(now.Add(time.Second))
			blocked = performTokenRateLimitRequest(t, engine, "192.0.2.1:1236")
			require.Equal(t, http.StatusTooManyRequests, blocked.Code)
			require.Equal(t, "59", blocked.Header().Get("Retry-After"))
			otherIP := performTokenRateLimitRequest(t, engine, "192.0.2.2:1234")
			if perIP {
				require.Equal(t, 200, otherIP.Code)
			} else {
				require.Equal(t, 429, otherIP.Code)
			}
			srv.SetTime(now.Add(time.Minute))
			require.Equal(t, 200, performTokenRateLimitRequest(t, engine, "192.0.2.1:1234").Code)
		})
	}
}

func TestTokenMemoryRetryAfter(t *testing.T) {
	engine := setupTokenRateLimitEngine(t, 910002, 1, 0)
	require.Equal(t, 200, performTokenRateLimitRequest(t, engine, "192.0.2.1:1234").Code)
	blocked := performTokenRateLimitRequest(t, engine, "192.0.2.1:1234")
	require.Equal(t, 429, blocked.Code)
	require.Contains(t, []string{"59", "60"}, blocked.Header().Get("Retry-After"))
	require.Contains(t, blocked.Body.String(), `"new_api_error":true`)
}
