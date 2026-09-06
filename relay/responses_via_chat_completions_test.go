package relay

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 渠道级自动降级的核心: RelayMode 切到 ChatCompletions 后,
// 原本拒绝 Responses 的渠道(如 Cerebras)其 chat 转换应放行并完成协议清洗
func TestResponsesViaChatCompletionsUnlocksChatOnlyChannels(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "qwen-3.8-27b",
		},
	}
	adaptor := GetAdaptor(constant.APITypeCerebras)
	require.NotNil(t, adaptor)

	responsesReq := dto.OpenAIResponsesRequest{
		Model: "qwen-3.8-27b",
		Input: json.RawMessage(`"hello"`),
		Reasoning: &dto.Reasoning{
			Effort: "high",
		},
	}

	// 原生 Responses 转换在该渠道会失败
	_, nativeErr := adaptor.ConvertOpenAIResponsesRequest(nil, info, responsesReq)
	require.Error(t, nativeErr)

	chatRequest, err := responsescompat.ConvertToOpenAIChatRequest(responsesReq)
	require.NoError(t, err)
	assert.Equal(t, "high", chatRequest.ReasoningEffort)

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	converted, err := adaptor.ConvertOpenAIRequest(nil, info, chatRequest)
	require.NoError(t, err)
	payload, ok := converted.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "qwen-3.8-27b", payload["model"])
	assert.Equal(t, "high", payload["reasoning_effort"])

	// 响应侧: Chat 响应能转回 Responses 协议的 handler 存在且可用
	assert.NotNil(t, openaichannel.ChatCompletionResponsesHandler)
	assert.NotNil(t, openaichannel.ChatCompletionResponsesStreamHandler)
}
