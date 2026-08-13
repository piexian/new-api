package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/limiter"

	"github.com/gin-gonic/gin"
)

const tokenRateLimitWindowSeconds int64 = 60

// TokenRateLimit 令牌级限流中间件
// 在用户/分组限流（ModelRequestRateLimit）之外叠加令牌自身的限制，
// 因此令牌限流只能更严格、无法放宽上层限制：
//   - rate_limit：令牌整体每分钟请求数上限，0=不限制
//   - ip_rate_limit：单个来源 IP 每分钟请求数上限，0=不限制
func TokenRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		rateLimit := c.GetInt("token_rate_limit")
		ipRateLimit := c.GetInt("token_ip_rate_limit")
		if rateLimit <= 0 && ipRateLimit <= 0 {
			c.Next()
			return
		}

		tokenId := strconv.Itoa(c.GetInt("token_id"))
		if common.RedisEnabled {
			tokenRateLimitRedis(c, tokenId, rateLimit, ipRateLimit)
		} else {
			tokenRateLimitMemory(c, tokenId, rateLimit, ipRateLimit)
		}
		if c.IsAborted() {
			return
		}
		c.Next()
	}
}

// tokenRateLimitRedis 基于 Redis 令牌桶的令牌限流
func tokenRateLimitRedis(c *gin.Context, tokenId string, rateLimit, ipRateLimit int) {
	ctx := context.Background()
	tb := limiter.New(ctx, common.RDB)

	if rateLimit > 0 {
		allowed, err := tb.Allow(
			ctx,
			fmt.Sprintf("rateLimit:token:%s", tokenId),
			limiter.WithCapacity(int64(rateLimit)),
			limiter.WithRate(int64(rateLimit)),
			limiter.WithRequested(1),
		)
		if err != nil {
			fmt.Println("检查令牌请求数限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("该令牌已达到请求数限制：每分钟最多请求%d次", rateLimit))
			return
		}
	}

	if ipRateLimit > 0 {
		allowed, err := tb.Allow(
			ctx,
			fmt.Sprintf("rateLimit:token:%s:ip:%s", tokenId, c.ClientIP()),
			limiter.WithCapacity(int64(ipRateLimit)),
			limiter.WithRate(int64(ipRateLimit)),
			limiter.WithRequested(1),
		)
		if err != nil {
			fmt.Println("检查令牌IP请求数限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前IP已达到该令牌的请求数限制：每分钟最多请求%d次", ipRateLimit))
			return
		}
	}
}

// tokenRateLimitMemory 基于内存滑动窗口的令牌限流
func tokenRateLimitMemory(c *gin.Context, tokenId string, rateLimit, ipRateLimit int) {
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)

	if rateLimit > 0 &&
		!inMemoryRateLimiter.Request("token:"+tokenId, rateLimit, tokenRateLimitWindowSeconds) {
		abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("该令牌已达到请求数限制：每分钟最多请求%d次", rateLimit))
		return
	}

	if ipRateLimit > 0 &&
		!inMemoryRateLimiter.Request("token:"+tokenId+":ip:"+c.ClientIP(), ipRateLimit, tokenRateLimitWindowSeconds) {
		abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前IP已达到该令牌的请求数限制：每分钟最多请求%d次", ipRateLimit))
		return
	}
}
