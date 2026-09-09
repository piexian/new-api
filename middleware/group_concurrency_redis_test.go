package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupConcurrencyAtomicAdmission(t *testing.T) {
	for _, redisMode := range []bool{false, true} {
		t.Run(fmt.Sprintf("redis=%t", redisMode), func(t *testing.T) {
			saveConcurrencySetting(t)
			if redisMode {
				useRateLimitTestRedis(t)
			}
			start := make(chan struct{})
			releases := make(chan func(), 20)
			errs := make(chan error, 20)
			var wg sync.WaitGroup
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					release, allowed, err := acquireGroupConcurrency(51, "concurrent", 3)
					errs <- err
					if allowed {
						releases <- release
					}
				}()
			}
			close(start)
			wg.Wait()
			close(releases)
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
			count := 0
			for release := range releases {
				count++
				release()
			}
			require.Equal(t, 3, count)
		})
	}
}

func TestGroupConcurrencyRedisExpiredReleaseCannotRemoveNewLease(t *testing.T) {
	srv := useRateLimitTestRedis(t)
	now := time.Unix(100000, 0)
	srv.SetTime(now)
	oldRelease, allowed, err := groupConcurrencyAcquireRedis(1, "vip", 1)
	require.NoError(t, err)
	require.True(t, allowed)
	t.Cleanup(oldRelease)
	_, allowed, err = groupConcurrencyAcquireRedis(1, "vip", 1)
	require.NoError(t, err)
	require.False(t, allowed)
	srv.SetTime(now.Add(groupConcurrencyCounterTTL + time.Second))
	newRelease, allowed, err := groupConcurrencyAcquireRedis(1, "vip", 1)
	require.NoError(t, err)
	require.True(t, allowed)
	t.Cleanup(newRelease)
	oldRelease()
	oldRelease()
	_, allowed, err = groupConcurrencyAcquireRedis(1, "vip", 1)
	require.NoError(t, err)
	require.False(t, allowed)
	newRelease()
	release, allowed, err := groupConcurrencyAcquireRedis(1, "vip", 1)
	require.NoError(t, err)
	require.True(t, allowed)
	release()
}

func TestGroupConcurrencyRedisRenewOnlyKeepsLiveRequest(t *testing.T) {
	srv := useRateLimitTestRedis(t)
	now := time.Unix(100000, 0)
	srv.SetTime(now)
	ctx := context.Background()
	key := "concurrency:group:leases:2:vip"
	acquire := func(id string) int {
		n, err := redisGroupConcurrencyAcquireScript.Run(ctx, common.RDB, []string{key}, 2, int(groupConcurrencyCounterTTL.Seconds()), id).Int()
		require.NoError(t, err)
		return n
	}
	require.Equal(t, 1, acquire("live"))
	require.Equal(t, 1, acquire("crashed"))
	for _, elapsed := range []time.Duration{20 * time.Minute, 40 * time.Minute, 60 * time.Minute, 80 * time.Minute} {
		srv.SetTime(now.Add(elapsed))
		n, err := redisGroupConcurrencyRenewScript.Run(ctx, common.RDB, []string{key}, "live", int(groupConcurrencyCounterTTL.Seconds())).Int()
		require.NoError(t, err)
		require.Equal(t, 1, n)
	}
	require.Equal(t, 1, acquire("replacement"))
	require.Equal(t, 0, acquire("overflow"))
	members, err := common.RDB.ZRange(ctx, key, 0, -1).Result()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"live", "replacement"}, members)
}

func TestGroupConcurrencyTransferChecksTargetAndReleasesBoth(t *testing.T) {
	for _, redisMode := range []bool{false, true} {
		t.Run(fmt.Sprintf("redis=%t", redisMode), func(t *testing.T) {
			saveConcurrencySetting(t)
			if redisMode {
				useRateLimitTestRedis(t)
			}
			setting.UserGroupConcurrencyLimitMutex.Lock()
			setting.UserGroupConcurrencyLimit = map[string]int{"a": 1, "b": 1}
			setting.UserGroupConcurrencyLimitMutex.Unlock()
			occupied, allowed, err := acquireGroupConcurrency(42, "b", 1)
			require.NoError(t, err)
			require.True(t, allowed)
			t.Cleanup(occupied)
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("id", 42)
				common.SetContextKey(c, constant.ContextKeyAutoGroup, "a")
			}, GroupConcurrencyLimit())
			r.GET("/", func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyAutoGroup, "b")
				err := EnsureGroupConcurrency(c)
				require.NotNil(t, err)
				require.Equal(t, 429, err.StatusCode)
				require.True(t, err.IsLocalError())
				occupied()
				require.Nil(t, EnsureGroupConcurrency(c))
				// Successful transfer releases A while B remains held.
				releaseA, allowed, err2 := acquireGroupConcurrency(42, "a", 1)
				require.NoError(t, err2)
				require.True(t, allowed)
				releaseA()
				_, allowed, err2 = acquireGroupConcurrency(42, "b", 1)
				require.NoError(t, err2)
				require.False(t, allowed)
				c.Status(http.StatusOK)
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			require.Equal(t, 200, w.Code)
			releaseB, allowed, err := acquireGroupConcurrency(42, "b", 1)
			require.NoError(t, err)
			require.True(t, allowed)
			releaseB()
		})
	}
}
