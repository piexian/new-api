package geminithought

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Gemini 3.x 的 thoughtSignature 是跨轮延续推理链的凭据,但 OpenAI 兼容格式没有
// 承载字段。响应转换时把真实签名暂存在 tool_call_id 下(该 id 会被客户端原样带回),
// 下一轮请求转换按 id 取回真签名,未命中再退回 bypass 值。
// Redis 优先;未启用 Redis 时退化为进程内 map(单实例有效,重启丢失)。
const signatureTTL = 6 * time.Hour

type signatureEntry struct {
	signature string
	expiresAt time.Time
}

var memoryStore sync.Map // toolCallID -> signatureEntry

func redisKey(toolCallID string) string {
	return "gemini:thought_signature:" + toolCallID
}

// redisEnabled 开关开着但客户端未初始化时(如单测环境)同样走进程内 map,避免空指针
func redisEnabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

// Save 保存上游下发的 thoughtSignature 与客户端可见 tool_call_id 的映射
func Save(toolCallID string, signature json.RawMessage) {
	sig := string(signature)
	if toolCallID == "" || len(sig) == 0 {
		return
	}
	if redisEnabled() {
		if err := common.RDB.Set(context.Background(), redisKey(toolCallID), sig, signatureTTL).Err(); err != nil {
			common.SysError("save gemini thought signature failed: " + err.Error())
		}
		return
	}
	memoryStore.Store(toolCallID, signatureEntry{
		signature: sig,
		expiresAt: time.Now().Add(signatureTTL),
	})
}

// Get 按 tool_call_id 读取真实签名,未命中返回 nil
func Get(toolCallID string) json.RawMessage {
	if toolCallID == "" {
		return nil
	}
	if redisEnabled() {
		data, err := common.RDB.Get(context.Background(), redisKey(toolCallID)).Bytes()
		if err != nil || len(data) == 0 {
			return nil
		}
		return json.RawMessage(data)
	}
	value, ok := memoryStore.Load(toolCallID)
	if !ok {
		return nil
	}
	entry := value.(signatureEntry)
	if time.Now().After(entry.expiresAt) {
		memoryStore.Delete(toolCallID)
		return nil
	}
	return json.RawMessage(entry.signature)
}
