package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// Each request has its own renewable lease, so live streams do not expire and
// crashed requests are not kept alive by other requests in the same group.
const groupConcurrencyCounterTTL = time.Hour

var redisGroupConcurrencyAcquireScript = redis.NewScript(`
local now = tonumber(redis.call('TIME')[1])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[1]) then
    return 0
end
redis.call('ZADD', KEYS[1], now + tonumber(ARGV[2]), ARGV[3])
redis.call('EXPIRE', KEYS[1], ARGV[2])
return 1
`)

var redisGroupConcurrencyReleaseScript = redis.NewScript(`
return redis.call('ZREM', KEYS[1], ARGV[1])
`)

var redisGroupConcurrencyRenewScript = redis.NewScript(`
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) then return 0 end
local now = tonumber(redis.call('TIME')[1])
redis.call('ZADD', KEYS[1], now + tonumber(ARGV[2]), ARGV[1])
redis.call('EXPIRE', KEYS[1], ARGV[2])
return 1
`)

var groupConcurrencyMemory = struct {
	sync.Mutex
	counts map[string]int
}{counts: make(map[string]int)}

// GroupConcurrencyLimit 用户分组并发限制中间件
// 按 账号+分组 限制同时进行中的请求数，限制值来自用户分组配置（0 或未配置表示不限制）。
// 需要挂载在 Distribute 之后，以便 auto 分组已解析为具体分组。
func GroupConcurrencyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		lease := &groupConcurrencyLease{}
		c.Set(groupConcurrencyLeaseKey, lease)
		defer func() {
			if lease.release != nil {
				lease.release()
			}
		}()
		if err := EnsureGroupConcurrency(c); err != nil {
			abortWithOpenAiMessage(c, err.StatusCode, err.Error(), err.GetErrorCode())
			return
		}
		c.Next()
	}
}

const groupConcurrencyLeaseKey = "group_concurrency_lease"

type groupConcurrencyLease struct {
	group       string
	release     func()
	initialized bool
}

// EnsureGroupConcurrency transfers the request's slot when auto retry changes
// groups. Acquire the target before releasing the previous slot.
func EnsureGroupConcurrency(c *gin.Context) *types.NewAPIError {
	value, exists := c.Get(groupConcurrencyLeaseKey)
	if !exists {
		return nil
	}
	lease := value.(*groupConcurrencyLease)
	group := resolveGroupConcurrencyGroup(c)
	if lease.initialized && lease.group == group {
		return nil
	}
	limit := setting.GetUserGroupConcurrencyLimit(group)
	var release func()
	if limit > 0 {
		var allowed bool
		var err error
		release, allowed, err = acquireGroupConcurrency(c.GetInt("id"), group, limit)
		if err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("concurrency_limit_check_failed: %w", err), types.ErrorCode("concurrency_limit_check_failed"), http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
		}
		if !allowed {
			// Slot release depends on in-flight work, so this is a retry suggestion.
			c.Header("Retry-After", "1")
			return types.NewErrorWithStatusCode(fmt.Errorf("该账号在分组 %s 的并发请求数已达上限（最多同时 %d 个请求），请等待进行中的请求完成后再试", group, limit), types.ErrorCode("group_concurrency_limit_exceeded"), http.StatusTooManyRequests, types.ErrOptionWithSkipRetry())
		}
	}
	if lease.release != nil {
		lease.release()
	}
	lease.group, lease.release, lease.initialized = group, release, true
	return nil
}

// resolveGroupConcurrencyGroup 返回请求最终使用的分组：
// auto 分组在选路时已解析为具体分组（ContextKeyAutoGroup），优先使用
func resolveGroupConcurrencyGroup(c *gin.Context) string {
	group := common.GetContextKeyString(c, constant.ContextKeyAutoGroup)
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	}
	return group
}

func acquireGroupConcurrency(userId int, group string, limit int) (release func(), allowed bool, err error) {
	if common.RedisEnabled {
		return groupConcurrencyAcquireRedis(userId, group, limit)
	}
	release, allowed = groupConcurrencyAcquireMemory(userId, group, limit)
	return release, allowed, nil
}

// groupConcurrencyAcquireMemory 基于内存计数器的并发闸门（单机部署精确）
func groupConcurrencyAcquireMemory(userId int, group string, limit int) (func(), bool) {
	key := fmt.Sprintf("%d:%s", userId, group)

	groupConcurrencyMemory.Lock()
	defer groupConcurrencyMemory.Unlock()
	if groupConcurrencyMemory.counts[key] >= limit {
		return nil, false
	}
	groupConcurrencyMemory.counts[key]++

	return sync.OnceFunc(func() {
		groupConcurrencyMemory.Lock()
		defer groupConcurrencyMemory.Unlock()
		if groupConcurrencyMemory.counts[key] <= 1 {
			delete(groupConcurrencyMemory.counts, key)
			return
		}
		groupConcurrencyMemory.counts[key]--
	}), true
}

// groupConcurrencyAcquireRedis 基于 Redis 计数器的并发闸门（多节点部署共享计数）
func groupConcurrencyAcquireRedis(userId int, group string, limit int) (func(), bool, error) {
	// Separate namespace from the old integer counters during rolling upgrades.
	key := fmt.Sprintf("concurrency:group:leases:%d:%s", userId, group)
	requestID := uuid.NewString()
	rdb := common.RDB
	allowed, err := redisGroupConcurrencyAcquireScript.Run(
		context.Background(),
		rdb,
		[]string{key},
		limit,
		int(groupConcurrencyCounterTTL.Seconds()),
		requestID,
	).Int()
	if err != nil {
		return nil, false, err
	}
	if allowed == 0 {
		return nil, false, nil
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go renewGroupConcurrencyLease(rdb, key, requestID, stop, done)
	return sync.OnceFunc(func() {
		close(stop)
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := redisGroupConcurrencyReleaseScript.Run(ctx, rdb, []string{key}, requestID).Err(); err != nil {
			common.SysLog("释放用户分组并发计数失败: " + err.Error())
		}
	}), true, nil
}

func renewGroupConcurrencyLease(rdb *redis.Client, key, requestID string, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(groupConcurrencyCounterTTL / 3)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			result, err := redisGroupConcurrencyRenewScript.Run(ctx, rdb, []string{key}, requestID, int(groupConcurrencyCounterTTL.Seconds())).Int()
			cancel()
			if err != nil {
				common.SysLog("续期用户分组并发租约失败: " + err.Error())
			} else if result == 0 {
				common.SysLog("用户分组并发租约已失效")
				return
			}
		}
	}
}
