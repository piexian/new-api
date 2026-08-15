package mistralconsole

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestBuildsBoraPayload(t *testing.T) {
	name := "alice"
	temperature := 0.0
	topP := 1.0
	maxTokens := uint(2048)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ApiKey:            `ory_session_test="session"`,
		UpstreamModelName: "glm-5-2",
	}}
	request := &dto.GeneralOpenAIRequest{
		Model:           "client-model",
		Temperature:     &temperature,
		TopP:            &topP,
		MaxTokens:       &maxTokens,
		ReasoningEffort: "max",
		Messages: []dto.Message{
			{Role: "system", Content: "Follow instructions."},
			{Role: "user", Name: &name, Content: []any{
				map[string]any{"type": "text", "text": "Hello"},
				map[string]any{"type": "text", "text": " world"},
			}},
			{Role: "assistant", Content: "Hi!"},
		},
		Tools: []dto.ToolCallRequest{
			{Type: "code_interpreter"},
			{Type: "image_generation"},
			{Type: "web_search"},
			{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        "get_time",
					Description: "Get current time",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"timezone": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	require.NoError(t, err)
	payload, ok := converted.(*boraConversationRequest)
	require.True(t, ok)
	require.Equal(t, "glm-5-2", payload.Model)
	require.True(t, payload.Stream)
	require.Equal(t, "high", payload.CompletionArgs.ReasoningEffort)
	require.Equal(t, uint(2048), *payload.CompletionArgs.MaxTokens)

	data, err := common.Marshal(payload)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"glm-5-2",
		"instructions":"[system]\nFollow instructions.",
		"completion_args":{
			"temperature":0,
			"max_tokens":2048,
			"top_p":1,
			"reasoning_effort":"high"
		},
		"tools":[
			{"type":"code_interpreter"},
			{"type":"image_generation"},
			{"type":"web_search_premium"},
			{"type":"function","function":{
				"name":"get_time",
				"description":"Get current time",
				"parameters":{"type":"object","properties":{"timezone":{"type":"string"}}}
			}}
		],
		"stream":true,
		"inputs":[
			{"object":"entry","type":"message.input","role":"user","content":"[user:alice]\nHello world","prefix":false},
			{"object":"entry","type":"message.output","role":"assistant","content":"Hi!"},
			{"object":"entry","type":"message.input","role":"user","content":"Please continue.","prefix":false}
		]
	}`, string(data))
}

func TestConvertOpenAIRequestMapsFunctionCallHistory(t *testing.T) {
	toolCalls, err := common.Marshal([]dto.ToolCallRequest{{
		ID:   "call-1",
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      "get_time",
			Arguments: `{"timezone":"Asia/Shanghai"}`,
		},
	}})
	require.NoError(t, err)
	request := &dto.GeneralOpenAIRequest{Messages: []dto.Message{
		{Role: "user", Content: "What time is it?"},
		{Role: "assistant", Content: nil, ToolCalls: toolCalls},
		{Role: "tool", ToolCallId: "call-1", Content: `{"time":"17:30"}`},
	}}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, testRelayInfo(false), request)
	require.NoError(t, err)
	payload := converted.(*boraConversationRequest)
	data, err := common.Marshal(payload.Inputs)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"object":"entry","type":"message.input","role":"user","content":"What time is it?","prefix":false},
		{"object":"entry","type":"function.call","name":"get_time","tool_call_id":"call-1","arguments":"{\"timezone\":\"Asia/Shanghai\"}"},
		{"object":"entry","type":"function.result","tool_call_id":"call-1","result":"{\"time\":\"17:30\"}"}
	]`, string(data))
}

func TestConvertOpenAIRequestMaxTokensAndToolChoice(t *testing.T) {
	zero := uint(0)
	tooLarge := uint(defaultBoraMaxTokens + 100)
	tests := []struct {
		name     string
		request  *dto.GeneralOpenAIRequest
		expected uint
		tools    int
	}{
		{
			name:     "large default",
			request:  &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: "hi"}}},
			expected: defaultBoraMaxTokens,
		},
		{
			name:     "explicit zero preserved",
			request:  &dto.GeneralOpenAIRequest{MaxCompletionTokens: &zero, Messages: []dto.Message{{Role: "user", Content: "hi"}}},
			expected: 0,
		},
		{
			name:     "oversized value clamped",
			request:  &dto.GeneralOpenAIRequest{MaxTokens: &tooLarge, Messages: []dto.Message{{Role: "user", Content: "hi"}}},
			expected: defaultBoraMaxTokens,
		},
		{
			name: "none disables tools",
			request: &dto.GeneralOpenAIRequest{
				Messages:   []dto.Message{{Role: "user", Content: "hi"}},
				Tools:      []dto.ToolCallRequest{{Type: "code_interpreter"}},
				ToolChoice: "none",
			},
			expected: defaultBoraMaxTokens,
			tools:    0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, testRelayInfo(false), test.request)
			require.NoError(t, err)
			payload := converted.(*boraConversationRequest)
			require.Equal(t, test.expected, *payload.CompletionArgs.MaxTokens)
			require.Equal(t, test.tools, len(payload.Tools))
		})
	}
}

func TestConvertOpenAIRequestEmptyToolParametersGetDefaultSchema(t *testing.T) {
	// 回归：客户端传 "parameters": {} 时，空 map 在 omitempty 序列化下会整个丢失，
	// 上游 bora 要求 parameters 必填，缺失会返回 422（生产 mistral_cookie 渠道大量
	// 422 的根因）。空 map 与缺省都必须归一化为默认 object schema。
	request := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "no_params"}},
			{Type: "function", Function: dto.FunctionRequest{Name: "empty_params", Parameters: map[string]any{}}},
		},
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, testRelayInfo(false), request)
	require.NoError(t, err)
	payload := converted.(*boraConversationRequest)
	require.Len(t, payload.Tools, 2)
	for i, tool := range payload.Tools {
		require.NotNil(t, tool.Function, "tool %d", i)
		params, ok := tool.Function.Parameters.(map[string]any)
		require.True(t, ok, "tool %d parameters must be a map", i)
		require.Equal(t, "object", params["type"], "tool %d", i)
	}
	// 序列化后 parameters 键必须真实存在（bora DTO 已无 omitempty）
	raw, err := common.Marshal(payload)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"parameters":{`)
}

func TestConvertOpenAIRequestRejectsUnsupportedContent(t *testing.T) {
	info := testRelayInfo(false)
	tests := []struct {
		name    string
		request *dto.GeneralOpenAIRequest
	}{
		{
			name: "image content",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/cat.png"}},
			}}}},
		},
		{
			name: "unknown tool",
			request: &dto.GeneralOpenAIRequest{
				Messages: []dto.Message{{Role: "user", Content: "hello"}},
				Tools:    []dto.ToolCallRequest{{Type: "file_search"}},
			},
		},
		{
			name:    "function role",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "function", Content: "result"}}},
		},
		{
			name:    "tool without id",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "tool", Content: "result"}}},
		},
		{
			name:    "empty",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: ""}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, test.request)
			require.Error(t, err)
			var apiErr *types.NewAPIError
			require.True(t, errors.As(err, &apiErr))
			require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			require.Equal(t, types.ErrorCodeInvalidRequest, apiErr.GetErrorCode())
		})
	}
}

func TestSetupRequestHeaderUsesCookieOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := testRelayInfo(false)
	headers := http.Header{"Authorization": []string{"Bearer stale"}}

	err := (&Adaptor{}).SetupRequestHeader(ctx, &headers, info)
	require.NoError(t, err)
	require.Equal(t, info.ApiKey, headers.Get("Cookie"))
	require.Equal(t, "text/event-stream", headers.Get("Accept"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Empty(t, headers.Get("Authorization"))
	require.NotContains(t, info.ToString(), info.ApiKey)
}

func TestSetupRequestHeaderRejectsInvalidCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	for _, cookie := range []string{"", "Cookie: session=value", "session=value\r\nX-Test: bad"} {
		info := testRelayInfo(false)
		info.ApiKey = cookie
		err := (&Adaptor{}).SetupRequestHeader(ctx, &http.Header{}, info)
		require.Error(t, err)
	}
}

func testRelayInfo(stream bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		IsStream: stream,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            `ory_session_test="session"`,
			UpstreamModelName: "glm-5-2",
		},
	}
}

func TestBoraFunctionNameMapping(t *testing.T) {
	tests := []struct {
		name     string
		upstream string
		client   string
	}{
		// 内置工具类型名重命名/逆映射
		{name: "web_search", upstream: "web_search_fn", client: "web_search"},
		{name: "code_interpreter", upstream: "code_interpreter_fn", client: "code_interpreter"},
		{name: "image_generation", upstream: "image_generation_fn", client: "image_generation"},
		{name: "web_search_premium", upstream: "web_search_premium_fn", client: "web_search_premium"},
		// 普通函数名原样透传
		{name: "get_time", upstream: "get_time", client: "get_time"},
		// 后缀结尾但基础名不是内置类型名：不裁剪
		{name: "my_search_fn", upstream: "my_search_fn", client: "my_search_fn"},
		// 内置类型名但没有后缀：不裁剪
		{name: "web_search_extra", upstream: "web_search_extra", client: "web_search_extra"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.upstream, boraFunctionNameForUpstream(test.name))
			require.Equal(t, test.client, boraFunctionNameForClient(test.upstream))
		})
	}
}

func TestConvertOpenAIRequestRenamesProtectedFunctionTools(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "web_search"}},
			{Type: "function", Function: dto.FunctionRequest{Name: "get_time"}},
		},
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, testRelayInfo(false), request)
	require.NoError(t, err)
	payload := converted.(*boraConversationRequest)
	require.Len(t, payload.Tools, 2)
	require.Equal(t, "web_search_fn", payload.Tools[0].Function.Name)
	require.Equal(t, "get_time", payload.Tools[1].Function.Name)
}

func TestAppendFunctionCallMapsProtectedNameBack(t *testing.T) {
	state := &boraResponseState{toolCallIndexes: make(map[string]int)}
	call, err := state.appendFunctionCall(boraStreamEvent{
		Type:       "function.call.delta",
		ID:         "fc-1",
		ToolCallID: "call-1",
		Name:       "web_search_fn",
		Arguments:  `{"query":"hello"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "web_search", call.Function.Name)
	require.Equal(t, "call-1", call.ID)

	// 非内置名的函数调用原样保留
	state2 := &boraResponseState{toolCallIndexes: make(map[string]int)}
	call2, err := state2.appendFunctionCall(boraStreamEvent{
		Type:       "function.call.delta",
		ID:         "fc-2",
		ToolCallID: "call-2",
		Name:       "get_time",
		Arguments:  `{}`,
	})
	require.NoError(t, err)
	require.Equal(t, "get_time", call2.Function.Name)

	// 非流式聚合路径同样逆映射
	require.Len(t, state.toolCalls, 1)
	require.Equal(t, "web_search", state.toolCalls[0].Function.Name)
}

func TestConvertOpenAIRequestAppendsContinuationForTrailingAssistant(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{Messages: []dto.Message{
		{Role: "system", Content: "Write a story."},
		{Role: "user", Content: "Begin."},
		{Role: "assistant", Content: "Once upon a time"},
	}}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, testRelayInfo(false), request)
	require.NoError(t, err)
	payload := converted.(*boraConversationRequest)
	data, err := common.Marshal(payload.Inputs)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"object":"entry","type":"message.input","role":"user","content":"Begin.","prefix":false},
		{"object":"entry","type":"message.output","role":"assistant","content":"Once upon a time"},
		{"object":"entry","type":"message.input","role":"user","content":"Please continue.","prefix":false}
	]`, string(data))
}

func TestConvertOpenAIRequestNoContinuationForTrailingUser(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{Messages: []dto.Message{
		{Role: "user", Content: "Begin."},
		{Role: "assistant", Content: "Once upon a time"},
		{Role: "user", Content: "Continue."},
	}}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, testRelayInfo(false), request)
	require.NoError(t, err)
	payload := converted.(*boraConversationRequest)
	data, err := common.Marshal(payload.Inputs)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"object":"entry","type":"message.input","role":"user","content":"Begin.","prefix":false},
		{"object":"entry","type":"message.output","role":"assistant","content":"Once upon a time"},
		{"object":"entry","type":"message.input","role":"user","content":"Continue.","prefix":false}
	]`, string(data))
}

func TestNormalizeBoraSamplingParams(t *testing.T) {
	onePointFive := 1.5
	negative := -0.5
	topTwo := 2.0
	topZero := 0.0
	normal := 0.7

	require.Equal(t, 1.0, *normalizeBoraTemperature(&onePointFive))
	require.Equal(t, 0.0, *normalizeBoraTemperature(&negative))
	require.Equal(t, 0.7, *normalizeBoraTemperature(&normal))
	require.Nil(t, normalizeBoraTemperature(nil))

	require.Equal(t, 1.0, *normalizeBoraTopP(&topTwo))
	require.Equal(t, 0.0001, *normalizeBoraTopP(&topZero))
	require.Equal(t, 0.7, *normalizeBoraTopP(&normal))
	require.Nil(t, normalizeBoraTopP(nil))
}

func TestConvertOpenAIRequestPassesStrictFlag(t *testing.T) {
	strict := true
	nonStrict := false
	request := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "strict_tool", Strict: &strict}},
			{Type: "function", Function: dto.FunctionRequest{Name: "loose_tool", Strict: &nonStrict}},
			{Type: "function", Function: dto.FunctionRequest{Name: "unset_tool"}},
		},
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, testRelayInfo(false), request)
	require.NoError(t, err)
	payload := converted.(*boraConversationRequest)
	require.Len(t, payload.Tools, 3)
	require.NotNil(t, payload.Tools[0].Function.Strict)
	require.True(t, *payload.Tools[0].Function.Strict)
	require.NotNil(t, payload.Tools[1].Function.Strict)
	require.False(t, *payload.Tools[1].Function.Strict)
	require.Nil(t, payload.Tools[2].Function.Strict)

	data, err := common.Marshal(payload)
	require.NoError(t, err)
	require.Contains(t, string(data), `"strict":true`)
	require.Contains(t, string(data), `"strict":false`)
}
