package limiter

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/go-redis/redis/v8"
)

//go:embed lua/rate_limit.lua
var rateLimitScript string

var limitScript = redis.NewScript(rateLimitScript)

type RedisLimiter struct {
	client *redis.Client
}

func New(_ context.Context, r *redis.Client) *RedisLimiter {
	return &RedisLimiter{client: r}
}

func (rl *RedisLimiter) Allow(ctx context.Context, key string, opts ...Option) (bool, error) {
	allowed, _, err := rl.AllowWithRetry(ctx, key, opts...)
	return allowed, err
}

// AllowWithRetry returns the seconds until this bucket can serve another request.
// The decision and wait time are calculated atomically from the same bucket state.
func (rl *RedisLimiter) AllowWithRetry(ctx context.Context, key string, opts ...Option) (bool, int64, error) {
	// 默认配置
	config := &Config{
		Capacity:  10,
		Rate:      1,
		Requested: 1,
	}

	// 应用选项模式
	for _, opt := range opts {
		opt(config)
	}

	// 执行限流
	result, err := limitScript.Run(
		ctx,
		rl.client,
		[]string{key},
		config.Requested,
		config.Rate,
		config.Capacity,
	).Int64Slice()

	if err != nil {
		return false, 0, fmt.Errorf("rate limit failed: %w", err)
	}
	return result[0] == 1, result[1], nil
}

// Config 配置选项模式
type Config struct {
	Capacity  int64
	Rate      int64
	Requested int64
}

type Option func(*Config)

func WithCapacity(c int64) Option {
	return func(cfg *Config) { cfg.Capacity = c }
}

func WithRate(r int64) Option {
	return func(cfg *Config) { cfg.Rate = r }
}

func WithRequested(n int64) Option {
	return func(cfg *Config) { cfg.Requested = n }
}
