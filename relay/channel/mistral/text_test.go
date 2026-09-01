package mistral

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	globalconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestRequestOpenAI2MistralPreservesReasoningEffort(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:           "mistral-small-latest",
		ReasoningEffort: "max",
	}

	converted := requestOpenAI2Mistral(request)

	require.Equal(t, "max", converted.ReasoningEffort)
}

func TestRequestOpenAI2MistralPreservesEmptyTextContent(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "mistral-small-latest",
		Messages: []dto.Message{
			{Role: "user", Content: ""},
			{Role: "tool", Content: "", ToolCallId: "123456789"},
			{Role: "assistant", Content: "", ToolCalls: json.RawMessage(`[{"id":"123456789"}]`)},
		},
	}

	converted := requestOpenAI2Mistral(request)

	if got := converted.Messages[0].Content; got != "" {
		t.Fatalf("user content = %#v, want empty string", got)
	}
	if got := converted.Messages[1].Content; got != "" {
		t.Fatalf("tool content = %#v, want empty string", got)
	}
	if got := converted.Messages[2].Content; got == "" {
		t.Fatalf("assistant tool-call content = %#v, want an empty array", got)
	}

	body, err := common.Marshal(converted)
	require.NoError(t, err)
	require.NotContains(t, string(body), `{"type":"text"}`)
	require.Contains(t, string(body), `"content":""`)
	require.Contains(t, string(body), `"content":[]`)
}

func TestRequestOpenAI2MistralNormalizesEmptyTextBlocks(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "mistral-small-latest",
		Messages: []dto.Message{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": ""},
			}},
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": ""},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}},
			}},
		},
	}

	converted := requestOpenAI2Mistral(request)
	require.Equal(t, "", converted.Messages[0].Content)
	content := converted.Messages[1].ParseContent()
	require.Len(t, content, 1)
	require.Equal(t, dto.ContentTypeImageURL, content[0].Type)
	require.NotContains(t, string(mustMarshal(t, converted)), `{"type":"text"}`)
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	body, err := common.Marshal(value)
	require.NoError(t, err)
	return body
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

func TestRequestOpenAI2MistralNormalizesBuiltInToolTypes(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "mistral-large-latest",
		Tools: []dto.ToolCallRequest{
			{Type: "web_search_preview"},
			{Type: "web_search_preview_2025_03_11"},
			{Type: "web_search_20250305"},
			{Type: "web_search"},
			{Type: "web_search_premium"},
			{Type: "function", Function: dto.FunctionRequest{Name: "get_weather"}},
		},
	}

	converted := requestOpenAI2Mistral(request)

	got := make([]string, len(converted.Tools))
	for i, tool := range converted.Tools {
		got[i] = tool.Type
	}
	require.Equal(t, []string{
		"web_search",
		"web_search",
		"web_search",
		"web_search",
		"web_search_premium",
		"function",
	}, got)

	// 转换不应改写调用方的 request.Tools
	require.Equal(t, "web_search_preview", request.Tools[0].Type)
	require.Equal(t, "web_search_20250305", request.Tools[2].Type)
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
