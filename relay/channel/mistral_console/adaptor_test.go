package mistralconsole

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
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
	aboveDefault := uint(defaultBoraMaxTokens + 100)
	aboveCeiling := uint(maxBoraMaxTokens + 100)
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
			// 客户端显式请求的预算不能被压回默认值，否则思考链会挤掉正文。
			name:     "above default passes through",
			request:  &dto.GeneralOpenAIRequest{MaxTokens: &aboveDefault, Messages: []dto.Message{{Role: "user", Content: "hi"}}},
			expected: aboveDefault,
		},
		{
			name:     "above ceiling clamped",
			request:  &dto.GeneralOpenAIRequest{MaxTokens: &aboveCeiling, Messages: []dto.Message{{Role: "user", Content: "hi"}}},
			expected: maxBoraMaxTokens,
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
	headers := http.Header{
		"Authorization": {"Bearer stale"},
		"User-Agent":    {"browser"},
		"X-Api-Key":     {"unrelated-key"},
		"Origin":        {"https://unrelated.example"},
	}

	err := (&Adaptor{}).SetupRequestHeader(ctx, &headers, info)
	require.NoError(t, err)
	require.Equal(t, info.ApiKey, headers.Get("Cookie"))
	require.Equal(t, "text/event-stream", headers.Get("Accept"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Empty(t, headers.Get("Authorization"))
	require.Equal(t, http.Header{
		"Accept":       {"text/event-stream"},
		"Content-Type": {"application/json"},
		"Cookie":       {info.ApiKey},
		"User-Agent":   {""},
	}, headers)
	require.NotContains(t, info.ToString(), info.ApiKey)
}

func TestSetupRequestHeaderRejectsInvalidCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	for _, cookie := range []string{"", "Cookie: session=value", "session=value\r\nX-Test: bad", `""`, `"unclosed`, "session\x00value"} {
		info := testRelayInfo(false)
		info.ApiKey = cookie
		err := (&Adaptor{}).SetupRequestHeader(ctx, &http.Header{}, info)
		require.Error(t, err)
	}
}

func TestSessionCookieInputFormats(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
	}{
		{"bare", "c2Vzc2lvbg==", boraSessionCookieName + `="c2Vzc2lvbg=="`},
		{"quoted", `"c2Vzc2lvbg=="`, boraSessionCookieName + `="c2Vzc2lvbg=="`},
		{"whitespace", "  c2Vzc2lvbg==  ", boraSessionCookieName + `="c2Vzc2lvbg=="`},
		{"named session", `ory_session_test="session"`, `ory_session_test="session"`},
		{"full cookie", `csrf=test; ory_session_test="session"`, `csrf=test; ory_session_test="session"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			info := testRelayInfo(false)
			info.ApiKey = test.input
			adaptor := &Adaptor{}
			_, err := adaptor.ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{
				Messages: []dto.Message{{Role: "user", Content: "Hi"}},
			})
			require.NoError(t, err)
			headers := make(http.Header)
			require.NoError(t, adaptor.SetupRequestHeader(nil, &headers, info))
			require.Equal(t, test.want, headers.Get("Cookie"))
		})
	}
}

func TestDoRequestDoesNotForwardClientOrOverrideHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	t.Cleanup(service.GetHttpClient().CloseIdleConnections)
	const body = `{"model":"mistral-medium-latest"}`
	seen := make(chan *http.Request, 1)
	seenBody := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		seen <- r.Clone(r.Context())
		seenBody <- string(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	info := testRelayInfo(false)
	info.ApiKey = "c2Vzc2lvbg=="
	info.ChannelBaseUrl = server.URL
	info.UpstreamRequestBodySize = int64(len(body))
	info.HeadersOverride = map[string]interface{}{"Authorization": "Bearer {api_key}", "User-Agent": "browser"}
	info.UseRuntimeHeadersOverride = true
	info.RuntimeHeadersOverride = map[string]interface{}{"Cookie": "wrong", "X-Api-Key": "wrong", "Host": "wrong.example"}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	ctx.Request.Header.Set("Authorization", "Bearer client-secret")
	ctx.Request.Header.Set("X-Client-Header", "unrelated")
	// Match the reader-only body wrapper used by the relay in production.
	response, err := (&Adaptor{}).DoRequest(ctx, info, struct{ io.Reader }{strings.NewReader(body)})
	require.NoError(t, err)
	defer response.(*http.Response).Body.Close()
	r := <-seen
	require.Equal(t, conversationsURL, r.URL.Path)
	require.Equal(t, http.MethodPost, r.Method)
	require.Equal(t, int64(len(body)), r.ContentLength)
	require.Equal(t, body, <-seenBody)
	require.Equal(t, boraSessionCookieName+`="c2Vzc2lvbg=="`, r.Header.Get("Cookie"))
	require.Equal(t, "text/event-stream", r.Header.Get("Accept"))
	require.Equal(t, "application/json", r.Header.Get("Content-Type"))
	for _, name := range []string{"Authorization", "User-Agent", "X-Api-Key", "X-Client-Header"} {
		require.NotContains(t, r.Header, name)
	}
	require.NotEqual(t, "wrong.example", r.Host)
	require.Equal(t, "Bearer {api_key}", info.HeadersOverride["Authorization"])
	require.Equal(t, "wrong", info.RuntimeHeadersOverride["Cookie"])
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

func TestConvertOpenAIRequestReasoningEffortMapping(t *testing.T) {
	tests := []struct {
		name     string
		request  *dto.GeneralOpenAIRequest
		expected string
	}{
		{name: "unset keeps thinking", request: &dto.GeneralOpenAIRequest{}, expected: boraMaxReasoningEffort},
		{name: "high", request: &dto.GeneralOpenAIRequest{ReasoningEffort: "high"}, expected: boraMaxReasoningEffort},
		{name: "low normalized to high", request: &dto.GeneralOpenAIRequest{ReasoningEffort: "low"}, expected: boraMaxReasoningEffort},
		{name: "none", request: &dto.GeneralOpenAIRequest{ReasoningEffort: "none"}, expected: boraNoReasoningEffort},
		{name: "none with case and spaces", request: &dto.GeneralOpenAIRequest{ReasoningEffort: " NONE "}, expected: boraNoReasoningEffort},
		{name: "reasoning object effort none", request: &dto.GeneralOpenAIRequest{Reasoning: []byte(`{"effort":"none"}`)}, expected: boraNoReasoningEffort},
		{name: "reasoning object disabled", request: &dto.GeneralOpenAIRequest{Reasoning: []byte(`{"enabled":false,"effort":"low"}`)}, expected: boraNoReasoningEffort},
		{name: "reasoning string none", request: &dto.GeneralOpenAIRequest{Reasoning: []byte(`"none"`)}, expected: boraNoReasoningEffort},
		{name: "reasoning object high", request: &dto.GeneralOpenAIRequest{Reasoning: []byte(`{"effort":"high"}`)}, expected: boraMaxReasoningEffort},
		{name: "thinking disabled", request: &dto.GeneralOpenAIRequest{THINKING: []byte(`{"type":"disabled"}`)}, expected: boraNoReasoningEffort},
		{name: "thinking enabled with budget", request: &dto.GeneralOpenAIRequest{THINKING: []byte(`{"type":"enabled","budget_tokens":1024}`)}, expected: boraMaxReasoningEffort},
		{name: "enable_thinking false", request: &dto.GeneralOpenAIRequest{EnableThinking: []byte(`false`)}, expected: boraNoReasoningEffort},
		{name: "enable_thinking string false", request: &dto.GeneralOpenAIRequest{EnableThinking: []byte(`"false"`)}, expected: boraNoReasoningEffort},
		{name: "enable_thinking true", request: &dto.GeneralOpenAIRequest{EnableThinking: []byte(`true`)}, expected: boraMaxReasoningEffort},
		{name: "explicit disable wins over high", expected: boraNoReasoningEffort, request: &dto.GeneralOpenAIRequest{
			ReasoningEffort: "high",
			THINKING:        []byte(`{"type":"disabled"}`),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := testRelayInfo(false)
			test.request.Messages = []dto.Message{{Role: "user", Content: "hi"}}
			adaptor := &Adaptor{}
			converted, err := adaptor.ConvertOpenAIRequest(nil, info, test.request)
			require.NoError(t, err)
			payload := converted.(*boraConversationRequest)
			require.Equal(t, test.expected, payload.CompletionArgs.ReasoningEffort)
			// RelayInfo 记录实际生效等级，供消费日志输出映射。
			require.Equal(t, test.expected, info.ReasoningEffort)
			require.Equal(t, defaultBoraMaxTokens, adaptor.sentMaxTokens)
		})
	}
}

func TestConvertOpenAIRequestRecordsSentMaxTokens(t *testing.T) {
	requested := uint(16384)
	info := testRelayInfo(false)
	adaptor := &Adaptor{}
	converted, err := adaptor.ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{
		MaxTokens:       &requested,
		ReasoningEffort: "none",
		Messages:        []dto.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	payload := converted.(*boraConversationRequest)
	require.Equal(t, requested, *payload.CompletionArgs.MaxTokens)
	require.Equal(t, requested, adaptor.sentMaxTokens)
	require.Equal(t, boraNoReasoningEffort, info.ReasoningEffort)
}
