package service

import (
	"context"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Gemini Interactions API 状态路由映射。
// interaction 状态按上游 API key 隔离存储,get/cancel/delete 及
// previous_interaction_id 链式请求必须路由回创建时所用的渠道与 key。
// Redis 优先;未启用 Redis 时退化为进程内 map(单实例有效,重启丢失)。

const geminiInteractionStateTTL = 60 * 24 * time.Hour // 对齐上游最长保留期(55 天)留余量

const (
	geminiInteractionKeyPrefix       = "gemini:interaction:"
	geminiInteractionBilledKeySuffix = ":billed"
)

// GeminiInteractionState 创建时记录的路由与计费上下文
type GeminiInteractionState struct {
	ChannelID  int    `json:"channel_id"`
	Key        string `json:"key"`
	UserID     int    `json:"user_id"`
	TokenID    int    `json:"token_id"`
	Model      string `json:"model"` // 创建时的 OriginModelName
	Background bool   `json:"background"`
	Billed     bool   `json:"billed"`
	CreatedAt  int64  `json:"created_at"`
}

type geminiInteractionMemoryEntry struct {
	state     GeminiInteractionState
	expiresAt time.Time
}

var geminiInteractionMemoryStore sync.Map // key -> geminiInteractionMemoryEntry

func geminiInteractionRedisKey(id string) string {
	return geminiInteractionKeyPrefix + id
}

// SaveGeminiInteractionState 保存 interaction 路由映射
func SaveGeminiInteractionState(id string, state *GeminiInteractionState) {
	if id == "" || state == nil {
		return
	}
	if state.CreatedAt == 0 {
		state.CreatedAt = time.Now().Unix()
	}
	if common.RedisEnabled {
		data, err := common.Marshal(state)
		if err != nil {
			common.SysError("marshal gemini interaction state failed: " + err.Error())
			return
		}
		if err := common.RDB.Set(context.Background(), geminiInteractionRedisKey(id), data, geminiInteractionStateTTL).Err(); err != nil {
			common.SysError("save gemini interaction state to redis failed: " + err.Error())
		}
		return
	}
	geminiInteractionMemoryStore.Store(id, geminiInteractionMemoryEntry{
		state:     *state,
		expiresAt: time.Now().Add(geminiInteractionStateTTL),
	})
}

// GetGeminiInteractionState 读取 interaction 路由映射
func GetGeminiInteractionState(id string) (*GeminiInteractionState, bool) {
	if id == "" {
		return nil, false
	}
	if common.RedisEnabled {
		data, err := common.RDB.Get(context.Background(), geminiInteractionRedisKey(id)).Bytes()
		if err != nil {
			return nil, false
		}
		var state GeminiInteractionState
		if err := common.Unmarshal(data, &state); err != nil {
			common.SysError("unmarshal gemini interaction state failed: " + err.Error())
			return nil, false
		}
		return &state, true
	}
	value, ok := geminiInteractionMemoryStore.Load(id)
	if !ok {
		return nil, false
	}
	entry := value.(geminiInteractionMemoryEntry)
	if time.Now().After(entry.expiresAt) {
		geminiInteractionMemoryStore.Delete(id)
		return nil, false
	}
	state := entry.state
	return &state, true
}

// DeleteGeminiInteractionState 删除映射(DELETE 端点成功后调用)
func DeleteGeminiInteractionState(id string) {
	if id == "" {
		return
	}
	if common.RedisEnabled {
		if err := common.RDB.Del(context.Background(), geminiInteractionRedisKey(id), geminiInteractionRedisKey(id)+geminiInteractionBilledKeySuffix).Err(); err != nil {
			common.SysError("delete gemini interaction state failed: " + err.Error())
		}
		return
	}
	geminiInteractionMemoryStore.Delete(id)
	geminiInteractionMemoryStore.Delete(id + geminiInteractionBilledKeySuffix)
}

// ClaimGeminiInteractionBilling 原子认领计费权,首个成功者返回 true(防重复结算)
func ClaimGeminiInteractionBilling(id string) bool {
	if id == "" {
		return false
	}
	if common.RedisEnabled {
		ok, err := common.RDB.SetNX(context.Background(), geminiInteractionRedisKey(id)+geminiInteractionBilledKeySuffix, "1", geminiInteractionStateTTL).Result()
		if err != nil {
			common.SysError("claim gemini interaction billing failed: " + err.Error())
			return false
		}
		return ok
	}
	_, loaded := geminiInteractionMemoryStore.LoadOrStore(id+geminiInteractionBilledKeySuffix, geminiInteractionMemoryEntry{
		expiresAt: time.Now().Add(geminiInteractionStateTTL),
	})
	return !loaded
}

// IsChannelKeyUsable 校验映射记录的 key 当前仍属于该渠道且处于启用状态(多 key 渠道检查状态列表)
func IsChannelKeyUsable(channel *model.Channel, key string) bool {
	if channel == nil || key == "" {
		return false
	}
	keys := channel.GetKeys()
	for i, k := range keys {
		if k != key {
			continue
		}
		if !channel.ChannelInfo.IsMultiKey {
			return true
		}
		if channel.ChannelInfo.MultiKeyStatusList == nil {
			return true
		}
		status, ok := channel.ChannelInfo.MultiKeyStatusList[i]
		if !ok {
			return true
		}
		return status == common.ChannelStatusEnabled
	}
	return false
}
