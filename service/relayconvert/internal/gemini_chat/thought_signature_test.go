package geminichat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/geminithought"

	"github.com/stretchr/testify/require"
)

// 签名缓存在未启用 Redis 时走进程内 map;测试环境没有 RDB 客户端,必须显式关闭
func disableRedisForTest(t *testing.T) {
	t.Helper()
	prev := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = prev })
}

func TestGeminiResponseToolCallStoresThoughtSignature(t *testing.T) {
	disableRedisForTest(t)

	part := &dto.GeminiPart{
		FunctionCall:     &dto.FunctionCall{FunctionName: "get_weather", Arguments: map[string]any{"city": "sf"}},
		ThoughtSignature: json.RawMessage(`"sig-abc"`),
	}
	call := geminiResponseToolCall(part)
	require.NotNil(t, call)
	require.True(t, dto.IsFallbackToolCallID(call.ID), "upstream 未下发 id 时应生成可识别的兜底 id")
	require.Equal(t, json.RawMessage(`"sig-abc"`), geminithought.Get(call.ID))
}

func TestGeminiResponseToolCallKeepsUpstreamCallID(t *testing.T) {
	disableRedisForTest(t)

	part := &dto.GeminiPart{
		FunctionCall:     &dto.FunctionCall{FunctionName: "get_weather", ID: "fc_upstream_1"},
		ThoughtSignature: json.RawMessage(`"sig-xyz"`),
	}
	call := geminiResponseToolCall(part)
	require.NotNil(t, call)
	require.Equal(t, "fc_upstream_1", call.ID)
	require.Equal(t, json.RawMessage(`"sig-xyz"`), geminithought.Get("fc_upstream_1"))
}

func TestGeminiResponseToolCallWithoutSignatureStoresNothing(t *testing.T) {
	disableRedisForTest(t)

	part := &dto.GeminiPart{
		FunctionCall: &dto.FunctionCall{FunctionName: "get_weather"},
	}
	call := geminiResponseToolCall(part)
	require.NotNil(t, call)
	require.Nil(t, geminithought.Get(call.ID))
}
