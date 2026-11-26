package mistral

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	globalconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestPassesThroughOpenAIBody(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
	}
	request := &dto.GeneralOpenAIRequest{
		Model:           "mistral-small-latest",
		ReasoningEffort: "high",
		Messages:        []dto.Message{{Role: "user", Content: "hello"}},
		Tools:           []dto.ToolCallRequest{{Type: "function", Function: dto.FunctionRequest{Name: "lookup"}}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	require.NoError(t, err)
	require.Same(t, request, converted)
	require.Equal(t, types.RelayFormatOpenAI, info.FinalRequestRelayFormat)
	require.Equal(t, "high", info.ReasoningEffort)
}

func TestNormalizeMistralNonStreamThinkingContent(t *testing.T) {
	input := []byte(`{
		"id":"response-id",
		"choices":[{
			"message":{
				"role":"assistant",
				"content":[
					{"type":"thinking","thinking":[{"type":"text","text":"先分析"},{"type":"text","text":"，再作答"}],"closed":true},
					{"type":"text","text":"最终答案"}
				]
			},
			"finish_reason":"stop"
		}],
		"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
	}`)

	output, err := normalizeMistralResponseData(input)
	require.NoError(t, err)

	var response map[string]any
	require.NoError(t, common.Unmarshal(output, &response))
	choice := response["choices"].([]any)[0].(map[string]any)
	message := choice["message"].(map[string]any)
	require.Equal(t, "最终答案", message["content"])
	require.Equal(t, "先分析，再作答", message["reasoning_content"])
	require.Equal(t, "stop", choice["finish_reason"])
	require.Equal(t, "response-id", response["id"])
}

func TestNormalizeMistralStreamThinkingContent(t *testing.T) {
	input := `{"id":"response-id","choices":[{"index":0,"delta":{"index":0,"content":[{"type":"thinking","thinking":[{"type":"text","text":"思考片段"}],"closed":true}]},"finish_reason":null}]}`

	output, err := normalizeMistralStreamData(input)
	require.NoError(t, err)

	var response map[string]any
	require.NoError(t, common.UnmarshalJsonStr(output, &response))
	choice := response["choices"].([]any)[0].(map[string]any)
	delta := choice["delta"].(map[string]any)
	require.Equal(t, "", delta["content"])
	require.Equal(t, "思考片段", delta["reasoning_content"])
}

func TestNormalizeMistralMixedAndStringContent(t *testing.T) {
	mixedInput := []byte(`{"choices":[{"delta":{"content":["前缀",{"type":"thinking","thinking":"推理"},{"type":"text","text":"正文"},{"type":"custom","text":"保留文本"}]}}]}`)

	mixedOutput, err := normalizeMistralResponseData(mixedInput)
	require.NoError(t, err)

	var response map[string]any
	require.NoError(t, common.Unmarshal(mixedOutput, &response))
	delta := response["choices"].([]any)[0].(map[string]any)["delta"].(map[string]any)
	require.Equal(t, "前缀正文保留文本", delta["content"])
	require.Equal(t, "推理", delta["reasoning_content"])

	stringInput := []byte(`{"choices":[{"delta":{"content":"普通文本"}}]}`)
	stringOutput, err := normalizeMistralResponseData(stringInput)
	require.NoError(t, err)
	require.Equal(t, stringInput, stringOutput)
}

func TestNormalizeMistralResponseRejectsInvalidJSON(t *testing.T) {
	_, err := normalizeMistralResponseData([]byte(`{"choices":`))
	require.Error(t, err)
}

func TestConvertOpenAIRequestModerationUsesClassificationRequest(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeModerations,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    globalconstant.ChannelTypeMistral,
			ChannelBaseUrl: "https://api.mistral.ai",
		},
	}

	converted, err := adaptor.ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{
		Model: "mistral-moderation-latest",
		Input: "hello world",
	})
	require.NoError(t, err)

	body, err := common.Marshal(converted)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"mistral-moderation-latest","input":"hello world"}`, string(body))

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.mistral.ai/v1/moderations", url)
}
