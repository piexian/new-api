package service

import (
	"context"
	"encoding/base64"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/go-redis/redis/v8"
)

// 滑动窗口与冷却去重的共享基础设施，供探测防护与错误封禁复用。
// Redis 路径为多节点权威实现；Redis 不可用时回退到进程内分片内存（fail-open）。
const (
	riskWindowShardCount      = 16
	riskWindowMaxKeysPerShard = 10000
	riskWindowCleanupInterval = 30 * time.Second
	// riskWindowRetentionSeconds 为内存清理的最长保留窗口，略大于配置允许的最大窗口（24h），
	// 确保清理不会误删仍在窗口期内的事件。
	riskWindowRetentionSeconds = 90000
)

type riskWindowEvent struct {
	ts     int64
	member string
}

type riskWindowShard struct {
	mu   sync.Mutex
	data map[string][]riskWindowEvent
}

// add 将 member 写入 key 的滑动窗口并返回窗口内去重后的成员数。
// 相同 member 仅更新其时间戳（用于“不同模型数”这类去重统计）。
func (s *riskWindowShard) add(key, member string, windowSeconds int, now int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := now - int64(windowSeconds)
	events := s.data[key]
	kept := make([]riskWindowEvent, 0, len(events)+1)
	found := false
	for _, e := range events {
		if e.ts <= cutoff {
			continue
		}
		if e.member == member {
			kept = append(kept, riskWindowEvent{ts: now, member: member})
			found = true
		} else {
			kept = append(kept, e)
		}
	}
	if !found {
		kept = append(kept, riskWindowEvent{ts: now, member: member})
	}
	s.data[key] = kept
	s.evictOverflowLocked(now)
	return len(kept)
}

// evictOverflowLocked 在超过容量上限时淘汰最近最久未活动的 key。
func (s *riskWindowShard) evictOverflowLocked(now int64) {
	if len(s.data) <= riskWindowMaxKeysPerShard {
		return
	}
	var oldestKey string
	oldestTs := now + 1
	for k, events := range s.data {
		last := lastEventTs(events)
		if last < oldestTs {
			oldestTs = last
			oldestKey = k
		}
	}
	if oldestKey != "" {
		delete(s.data, oldestKey)
	}
}

func lastEventTs(events []riskWindowEvent) int64 {
	var max int64
	for _, e := range events {
		if e.ts > max {
			max = e.ts
		}
	}
	return max
}

// cleanup 清理过期事件并删除已清空的 key。
func (s *riskWindowShard) cleanup(now int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := now - riskWindowRetentionSeconds
	for k, events := range s.data {
		kept := events[:0]
		for _, e := range events {
			if e.ts > cutoff {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(s.data, k)
		} else {
			s.data[k] = kept
		}
	}
}

type riskWindowStore struct {
	shards [riskWindowShardCount]*riskWindowShard
}

func newRiskWindowStore() *riskWindowStore {
	s := &riskWindowStore{}
	for i := range s.shards {
		s.shards[i] = &riskWindowShard{data: make(map[string][]riskWindowEvent)}
	}
	return s
}

// members 返回 key 窗口内当前存储的成员（用于审计与通知展示）。
func (s *riskWindowStore) members(key string) []string {
	shard := s.shardFor(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	events := shard.data[key]
	members := make([]string, 0, len(events))
	for _, e := range events {
		members = append(members, e.member)
	}
	return members
}

func fnv32(s string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for _, b := range []byte(s) {
		h ^= uint32(b)
		h *= prime32
	}
	return h
}

func (s *riskWindowStore) shardFor(key string) *riskWindowShard {
	return s.shards[fnv32(key)%riskWindowShardCount]
}

func (s *riskWindowStore) add(key, member string, windowSeconds int) int64 {
	now := common.GetTimestamp()
	return int64(s.shardFor(key).add(key, member, windowSeconds, now))
}

func (s *riskWindowStore) startCleanup() {
	gopool.Go(func() {
		for {
			time.Sleep(riskWindowCleanupInterval)
			now := common.GetTimestamp()
			for _, shard := range s.shards {
				shard.cleanup(now)
			}
		}
	})
}

var (
	globalRiskWindow      = newRiskWindowStore()
	riskWindowCleanupOnce sync.Once
	riskEventSeq          uint64
	riskCooldownStore     sync.Map // key -> 冷却到期 Unix 秒（内存回退路径）
	riskWindowRedisScript = redis.NewScript(riskWindowLuaScript)
)

const riskWindowLuaScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local member = ARGV[3]
local ttl = tonumber(ARGV[4])
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
redis.call('ZADD', key, now, member)
if ttl > 0 then
	redis.call('EXPIRE', key, ttl)
end
return redis.call('ZCARD', key)
`

func ensureRiskWindowCleanup() {
	riskWindowCleanupOnce.Do(globalRiskWindow.startCleanup)
}

// riskIPKey 以 base64 编码 IP 构造 Redis key，避免 IPv6 冒号带来的歧义。
func riskIPKey(prefix, ip string) string {
	return prefix + ":" + base64.RawURLEncoding.EncodeToString([]byte(ip))
}

// riskEventMember 生成窗口内唯一的事件成员，用于“事件总数”类统计。
// 组合时间戳与进程内自增序列，保证多节点共享 Redis 时成员不冲突。
func riskEventMember() string {
	return strconv.FormatInt(common.GetTimestamp(), 10) + "-" + strconv.FormatUint(atomic.AddUint64(&riskEventSeq, 1), 10)
}

// riskWindowAddDistinct 将 member 加入 key 的滑动窗口，返回窗口内去重成员数。
// Redis 失败时回退到内存（fail-open，可能少计）。
func riskWindowAddDistinct(key, member string, windowSeconds int) int64 {
	ensureRiskWindowCleanup()
	if common.RedisEnabled {
		now := common.GetTimestamp()
		ttl := int64(windowSeconds) + 120
		res, err := riskWindowRedisScript.Run(context.Background(), common.RDB, []string{key}, now, windowSeconds, member, ttl).Result()
		if err == nil {
			if count, ok := res.(int64); ok {
				return count
			}
		} else {
			common.SysLog("risk window redis add failed, fallback to memory: " + err.Error())
		}
	}
	return globalRiskWindow.add(key, member, windowSeconds)
}

// riskWindowAddEvent 记录一次事件并返回窗口内事件总数。
func riskWindowAddEvent(key string, windowSeconds int) int64 {
	return riskWindowAddDistinct(key, riskEventMember(), windowSeconds)
}

// riskWindowMembers 返回窗口内当前的成员列表，优先读取 Redis。
func riskWindowMembers(key string) []string {
	if common.RedisEnabled {
		res, err := common.RDB.ZRange(context.Background(), key, 0, -1).Result()
		if err == nil {
			return res
		}
	}
	return globalRiskWindow.members(key)
}

// riskCooldownAcquire 尝试获取冷却锁。返回 true 表示成功获取（可继续处罚），
// false 表示仍处于冷却中（应跳过）。
func riskCooldownAcquire(key string, seconds int) bool {
	if seconds <= 0 {
		return true
	}
	if common.RedisEnabled {
		ok, err := common.RDB.SetNX(context.Background(), key, "1", time.Duration(seconds)*time.Second).Result()
		if err == nil {
			return ok
		}
		common.SysLog("risk cooldown redis set failed, fallback to memory: " + err.Error())
	}
	now := common.GetTimestamp()
	if v, ok := riskCooldownStore.Load(key); ok {
		if expiry, isInt := v.(int64); isInt && expiry > now {
			return false
		}
	}
	riskCooldownStore.Store(key, now+int64(seconds))
	return true
}
