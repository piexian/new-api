package relay

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	kitTypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 渠道级自动降级的核心: 不支持原生 Responses 的渠道(如 Cerebras)应在
// ConvertOpenAIResponsesRequest 内自行降级为 chat 协议并完成清洗
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

	// 该渠道不再直接报错, 而是自行降级为 chat 协议
	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, responsesReq)
	require.NoError(t, err)
	assert.Equal(t, kitTypes.RelayFormatOpenAI, info.GetFinalRequestRelayFormat())
	payload, ok := converted.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "qwen-3.8-27b", payload["model"])
	assert.Equal(t, "high", payload["reasoning_effort"])

	// 响应侧: Chat 响应能转回 Responses 协议的 handler 存在且可用
	assert.NotNil(t, openaichannel.ChatCompletionResponsesHandler)
	assert.NotNil(t, openaichannel.ChatCompletionResponsesStreamHandler)
}
