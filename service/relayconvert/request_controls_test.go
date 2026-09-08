package relayconvert_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRequestConversionPreservesControls(t *testing.T) {
	tests := []struct {
		name, body string
		request    any
		from, to   types.RelayFormat
		fields     map[string]string
	}{
		{"claude tool none", `{"model":"test","messages":[{"role":"user","content":"hi"}],"tool_choice":{"type":"none"}}`, &dto.ClaudeRequest{}, types.RelayFormatClaude, types.RelayFormatOpenAI, map[string]string{"tool_choice": "none"}},
		{"claude forced tool", `{"model":"test","messages":[{"role":"user","content":"hi"}],"tool_choice":{"type":"tool","name":"lookup","disable_parallel_tool_use":true}}`, &dto.ClaudeRequest{}, types.RelayFormatClaude, types.RelayFormatOpenAI, map[string]string{"tool_choice.function.name": "lookup", "parallel_tool_calls": "false"}},
		{"gemini zeros schema and choice", `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"toolConfig":{"functionCallingConfig":{"mode":"NONE"}},"generationConfig":{"topP":0,"topK":0,"seed":0,"thinkingConfig":{"thinkingBudget":0},"responseMimeType":"application/json","responseSchema":{"type":"OBJECT","properties":{"answer":{"type":"STRING","nullable":true}}}}}`, &dto.GeminiChatRequest{}, types.RelayFormatGemini, types.RelayFormatOpenAI, map[string]string{"top_p": "0", "top_k": "0", "seed": "0", "reasoning_effort": "none", "tool_choice": "none", "response_format.json_schema.schema.type": "object", "response_format.json_schema.schema.properties.answer.type.1": "null"}},
		{"chat zeros", `{"model":"test","messages":[{"role":"user","content":"hi"}],"top_p":0,"top_k":0,"seed":0}`, &dto.GeneralOpenAIRequest{}, types.RelayFormatOpenAI, types.RelayFormatGemini, map[string]string{"generationConfig.topP": "0", "generationConfig.topK": "0", "generationConfig.seed": "0"}},
		{"responses schema", `{"model":"test","input":"hi","text":{"format":{"type":"json_schema","name":"result","schema":{"type":"object","properties":{"x":{"type":"string"}},"required":["x"],"additionalProperties":false}}}}`, &dto.OpenAIResponsesRequest{}, types.RelayFormatOpenAIResponses, types.RelayFormatClaude, map[string]string{"output_config.format.type": "json_schema", "output_config.format.schema.required.0": "x"}},
		{"chat no parameter tool", `{"model":"test","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"ping"}}]}`, &dto.GeneralOpenAIRequest{}, types.RelayFormatOpenAI, types.RelayFormatClaude, map[string]string{"tools.0.name": "ping", "tools.0.input_schema.type": "object"}},
		{"responses developer", `{"model":"test","instructions":"first","input":[{"role":"developer","content":"second"},{"role":"user","content":"hi"}]}`, &dto.OpenAIResponsesRequest{}, types.RelayFormatOpenAIResponses, types.RelayFormatGeminiInteractions, map[string]string{"system_instruction": "first\nsecond", "input.#": "1", "input.0.content.0.text": "hi"}},
		{"gemini forced interaction tool", `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"toolConfig":{"functionCallingConfig":{"mode":"ANY","allowedFunctionNames":["lookup"]}}}`, &dto.GeminiChatRequest{}, types.RelayFormatGemini, types.RelayFormatGeminiInteractions, map[string]string{"generation_config.tool_choice.allowed_tools.mode": "any", "generation_config.tool_choice.allowed_tools.tools.0": "lookup"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, common.Unmarshal([]byte(tt.body), tt.request))
			wire, err := common.Marshal(convertRequest(t, tt.request, tt.from, tt.to))
			require.NoError(t, err)
			for path, want := range tt.fields {
				got := gjson.GetBytes(wire, path)
				require.True(t, got.Exists(), path+": "+string(wire))
				require.Equal(t, want, got.String(), path)
			}
		})
	}
}

func TestUnrepresentableRequestContentReturnsError(t *testing.T) {
	tests := []struct {
		name, body string
		request    any
		to         types.RelayFormat
		errorText  string
	}{
		{"claude audio", `{"model":"test","messages":[{"role":"user","content":[{"type":"input_audio","input_audio":{"data":"AAAA","format":"wav"}}]}]}`, &dto.GeneralOpenAIRequest{}, types.RelayFormatClaude, "content type"},
		{"claude file reference", `{"model":"test","input":[{"role":"user","content":[{"type":"input_file","file_id":"file_1"}]}]}`, &dto.OpenAIResponsesRequest{}, types.RelayFormatClaude, "file IDs"},
		{"gemini file reference", `{"model":"test","input":[{"role":"user","content":[{"type":"input_file","file_id":"file_1"}]}]}`, &dto.OpenAIResponsesRequest{}, types.RelayFormatGemini, "file IDs"},
		{"interactions sampling", `{"model":"test","messages":[{"role":"user","content":"hi"}],"temperature":0}`, &dto.GeneralOpenAIRequest{}, types.RelayFormatGeminiInteractions, "does not support temperature"},
		{"responses stop", `{"model":"test","messages":[{"role":"user","content":"hi"}],"stop":["END"]}`, &dto.GeneralOpenAIRequest{}, types.RelayFormatOpenAIResponses, "no Responses equivalent"},
		{"gemini custom tool", `{"model":"test","tools":[{"type":"custom","name":"shell"}],"input":"hi"}`, &dto.OpenAIResponsesRequest{}, types.RelayFormatGemini, "tool type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, common.Unmarshal([]byte(tt.body), tt.request))
			before, err := common.Marshal(tt.request)
			require.NoError(t, err)
			result, err := relayconvert.ConvertRequest(nil, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test"}}, tt.to, tt.request)
			require.ErrorContains(t, err, tt.errorText)
			require.Nil(t, result)
			after, err := common.Marshal(tt.request)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
		})
	}
}

func TestThinkingBudgetsSurviveIntermediateChatRequests(t *testing.T) {
	var claude dto.ClaudeRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"test","thinking":{"type":"enabled","budget_tokens":4096},"messages":[{"role":"user","content":"hi"}]}`), &claude))
	chat := convertRequest(t, &claude, types.RelayFormatClaude, types.RelayFormatOpenAI).(*dto.GeneralOpenAIRequest)
	require.Equal(t, int64(4096), gjson.GetBytes(chat.THINKING, "budget_tokens").Int())
	gemini := convertRequest(t, &claude, types.RelayFormatClaude, types.RelayFormatGemini).(*dto.GeminiChatRequest)
	require.NotNil(t, gemini.GenerationConfig.ThinkingConfig)
	require.Equal(t, 4096, *gemini.GenerationConfig.ThinkingConfig.ThinkingBudget)
	result, err := relayconvert.ConvertRequest(nil, nil, types.RelayFormatOpenAIResponses, chat)
	require.ErrorContains(t, err, "numeric thinking token budget")
	require.Nil(t, result)
	chat = &dto.GeneralOpenAIRequest{Model: "test", ReasoningEffort: "none", Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	gemini = convertRequest(t, chat, types.RelayFormatOpenAI, types.RelayFormatGemini).(*dto.GeminiChatRequest)
	require.Equal(t, 0, *gemini.GenerationConfig.ThinkingConfig.ThinkingBudget)
}
