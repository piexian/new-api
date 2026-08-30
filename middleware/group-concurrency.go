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

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// groupConcurrencyCounterTTL Redis 并发计数器的兜底过期时间。
// 请求结束时一定会释放计数，该 TTL 仅用于进程崩溃等异常场景下的计数自愈，
// 因此需要大于请求（含流式/实时会话）的最大持续时长。
const groupConcurrencyCounterTTL = time.Hour

var redisGroupConcurrencyAcquireScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current > tonumber(ARGV[1]) then
	redis.call('DECR', KEYS[1])
	return 0
end
redis.call('EXPIRE', KEYS[1], ARGV[2])
return 1
`)

var redisGroupConcurrencyReleaseScript = redis.NewScript(`
local current = redis.call('DECR', KEYS[1])
if current <= 0 then
	redis.call('DEL', KEYS[1])
end
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
		group := resolveGroupConcurrencyGroup(c)
		limit := setting.GetUserGroupConcurrencyLimit(group)
		if limit <= 0 {
			c.Next()
			return
		}

		release, allowed, err := acquireGroupConcurrency(c.GetInt("id"), group, limit)
		if err != nil {
			fmt.Println("检查用户分组并发限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "concurrency_limit_check_failed")
			return
		}
		if !allowed {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("该账号在分组 %s 的并发请求数已达上限（最多同时 %d 个请求），请等待进行中的请求完成后再试", group, limit))
			return
		}
		defer release()
		c.Next()
	}
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

	return func() {
		groupConcurrencyMemory.Lock()
		defer groupConcurrencyMemory.Unlock()
		if groupConcurrencyMemory.counts[key] <= 1 {
			delete(groupConcurrencyMemory.counts, key)
			return
		}
		groupConcurrencyMemory.counts[key]--
	}, true
}

// groupConcurrencyAcquireRedis 基于 Redis 计数器的并发闸门（多节点部署共享计数）
func groupConcurrencyAcquireRedis(userId int, group string, limit int) (func(), bool, error) {
	key := fmt.Sprintf("concurrency:group:%d:%s", userId, group)
	allowed, err := redisGroupConcurrencyAcquireScript.Run(
		context.Background(),
		common.RDB,
		[]string{key},
		limit,
		int(groupConcurrencyCounterTTL.Seconds()),
	).Int()
	if err != nil {
		return nil, false, err
	}
	if allowed == 0 {
		return nil, false, nil
	}
	return func() {
		if err := redisGroupConcurrencyReleaseScript.Run(context.Background(), common.RDB, []string{key}).Err(); err != nil {
			common.SysLog("释放用户分组并发计数失败: " + err.Error())
		}
	}, true, nil
}
