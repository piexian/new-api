package mistral

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	globalconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestPassesThroughNatively(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:              "mistral-large-4",
		Input:              []byte(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"look"},{"type":"input_image","image_url":"https://example.com/a.png","detail":"low"}]}]`),
		Instructions:       []byte(`"be concise"`),
		Tools:              []byte(`[{"type":"function","name":"get_weather","parameters":{"type":"object"}}]`),
		PromptCacheKey:     []byte(`"cache-key-1"`),
		Reasoning:          &dto.Reasoning{Effort: "high"},
		Conversation:       []byte(`"conv-1"`),
		Prompt:             []byte(`"system-prompt"`),
		Moderation:         []byte(`{"type":"omni-moderation-latest"}`),
		Store:              []byte(`true`),
		Truncation:         []byte(`"auto"`),
		PreviousResponseID: "resp_prev",
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	convertedResp, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want dto.OpenAIResponsesRequest", converted)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatOpenAIResponses {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatOpenAIResponses)
	}
	if convertedResp.Model != "mistral-large-4" ||
		len(convertedResp.Input) == 0 ||
		string(convertedResp.Instructions) != `"be concise"` ||
		len(convertedResp.Tools) == 0 ||
		convertedResp.Reasoning == nil || convertedResp.Reasoning.Effort != "high" ||
		string(convertedResp.PromptCacheKey) != `"cache-key-1"` {
		t.Fatalf("native passthrough fields were modified: %#v", convertedResp)
	}
	// 无状态端点：状态字段一律清空，携带非法值会被上游 422 拒绝
	if convertedResp.Conversation != nil || convertedResp.Prompt != nil || convertedResp.Moderation != nil ||
		convertedResp.Store != nil || convertedResp.Truncation != nil || convertedResp.PreviousResponseID != "" {
		t.Fatalf("stateful fields must be cleared: %#v", convertedResp)
	}
}

func TestConvertClaudeRequestUsesMistralChatFormat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
	}
	maxTokens := uint(128)

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, &dto.ClaudeRequest{
		Model:     "mistral-small-latest",
		MaxTokens: &maxTokens,
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "tool_use", "id": "123456789", "name": "lookup", "input": map[string]any{}},
			}},
			{Role: "user", Content: []any{
				map[string]any{"type": "tool_result", "tool_use_id": "123456789", "content": ""},
			}},
		},
	})
	if err != nil {
		t.Fatalf("ConvertClaudeRequest returned error: %v", err)
	}

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertClaudeRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatOpenAI {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatOpenAI)
	}
	if len(chatReq.Messages) != 3 || chatReq.Messages[0].StringContent() != "hello" {
		t.Fatalf("messages = %#v, want converted user message", chatReq.Messages)
	}
	if got := chatReq.Messages[2].Content; got != "" {
		t.Fatalf("tool result content = %#v, want empty string", got)
	}
}

func TestGetRequestURLRoutesByEndpoint(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	tests := []struct {
		name        string
		relayMode   int
		requestPath string
		baseURL     string
		finalFormat types.RelayFormat
		want        string
	}{
		{
			name:        "chat uses the OpenAI-compatible endpoint",
			requestPath: "/v1/chat/completions",
			finalFormat: types.RelayFormatOpenAI,
			want:        "https://api.mistral.ai/openai/v1/chat/completions",
		},
		{
			name:        "chat on a proxy base falls back to the native endpoint",
			requestPath: "/v1/chat/completions",
			baseURL:     "https://gptload.elycyrene.net/proxy/mistral",
			finalFormat: types.RelayFormatOpenAI,
			want:        "https://gptload.elycyrene.net/proxy/mistral/v1/chat/completions",
		},
		{
			name:        "responses passes through the native endpoint",
			relayMode:   relayconstant.RelayModeResponses,
			requestPath: "/v1/responses",
			finalFormat: types.RelayFormatOpenAIResponses,
			want:        "https://api.mistral.ai/v1/responses",
		},
		{
			name:        "responses input tokens passes through",
			relayMode:   relayconstant.RelayModeResponsesInputTokens,
			requestPath: "/v1/responses/input_tokens",
			finalFormat: types.RelayFormatOpenAIResponses,
			want:        "https://api.mistral.ai/v1/responses/input_tokens",
		},
		{
			name:        "moderations uses the native endpoint",
			relayMode:   relayconstant.RelayModeModerations,
			requestPath: "/v1/moderations",
			finalFormat: types.RelayFormatOpenAI,
			want:        "https://api.mistral.ai/v1/moderations",
		},
		{
			name:        "ocr passes through the native endpoint",
			relayMode:   relayconstant.RelayModeOCR,
			requestPath: "/v1/ocr",
			want:        "https://api.mistral.ai/v1/ocr",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseURL := test.baseURL
			if baseURL == "" {
				baseURL = "https://api.mistral.ai"
			}
			info := &relaycommon.RelayInfo{
				RelayMode:               test.relayMode,
				FinalRequestRelayFormat: test.finalFormat,
				RequestURLPath:          test.requestPath,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: baseURL,
				},
			}

			got, err := adaptor.GetRequestURL(info)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("GetRequestURL = %q, want %q", got, test.want)
			}
		})
	}
}

func TestIsOfficialMistralBase(t *testing.T) {
	t.Parallel()

	require.True(t, isOfficialMistralBase("https://api.mistral.ai"))
	require.True(t, isOfficialMistralBase("https://api.mistral.ai/"))
	require.False(t, isOfficialMistralBase("https://gptload.elycyrene.net/proxy/mistral"))
	require.False(t, isOfficialMistralBase("https://api.mistral.ai.evil.example.com"))
	require.False(t, isOfficialMistralBase("://bad url"))
}

func TestDoResponseResponsesRendersNativeResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_1","object":"response","created_at":1792584000,"model":"mistral-large-4",
			"status":"completed",
			"output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed",
				"content":[{"type":"output_text","text":"answer","annotations":[]}]}],
			"usage":{"input_tokens":12,"output_tokens":10,"total_tokens":22,
				"input_tokens_details":{"cached_tokens":4},
				"output_tokens_details":{"reasoning_tokens":6}}
		}`)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode:               relayconstant.RelayModeResponses,
		RelayFormat:             types.RelayFormatOpenAIResponses,
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "mistral-large-4",
		},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse returned error: %v", apiErr)
	}
	usageDto, ok := usage.(*dto.Usage)
	if !ok {
		t.Fatalf("usage = %#v, want *dto.Usage", usage)
	}
	if usageDto.PromptTokens != 12 || usageDto.CompletionTokens != 10 || usageDto.TotalTokens != 22 {
		t.Fatalf("usage = %#v, want 12/10/22", usageDto)
	}
	if usageDto.PromptTokensDetails.CachedTokens != 4 {
		t.Fatalf("cached tokens = %d, want 4", usageDto.PromptTokensDetails.CachedTokens)
	}
	if usageDto.CompletionTokenDetails.ReasoningTokens != 6 {
		t.Fatalf("reasoning tokens = %d, want 6", usageDto.CompletionTokenDetails.ReasoningTokens)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"output_text"`) || !strings.Contains(body, `"text":"answer"`) {
		t.Fatalf("native response output was not preserved: %s", body)
	}
}

func TestDoResponseResponsesStreamPassesThroughNativeEvents(t *testing.T) {
	oldTimeout := globalconstant.StreamingTimeout
	globalconstant.StreamingTimeout = 30
	t.Cleanup(func() { globalconstant.StreamingTimeout = oldTimeout })

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	reasoningDelta := `{"type":"response.reasoning_text.delta","sequence_number":3,"item_id":"rs_1","output_index":0,"content_index":0,"delta":"think"}`
	completed := `{"type":"response.completed","sequence_number":5,"response":{"id":"resp_1","status":"completed","usage":{"input_tokens":3,"output_tokens":5,"total_tokens":8}}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"event: response.created\n" +
				`data: {"type":"response.created","sequence_number":1,"response":{"id":"resp_1","status":"in_progress"}}` + "\n\n" +
				"event: response.reasoning_text.delta\n" +
				"data: " + reasoningDelta + "\n\n" +
				"event: response.completed\n" +
				"data: " + completed + "\n\n",
		)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode:               relayconstant.RelayModeResponses,
		RelayFormat:             types.RelayFormatOpenAIResponses,
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
		IsStream:                true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "mistral-large-4",
		},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse returned error: %v", apiErr)
	}
	usageDto, ok := usage.(*dto.Usage)
	if !ok || usageDto.PromptTokens != 3 || usageDto.CompletionTokens != 5 || usageDto.TotalTokens != 8 {
		t.Fatalf("usage = %#v, want 3/5/8", usage)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, reasoningDelta) {
		t.Fatalf("stream did not preserve reasoning delta event: %s", body)
	}
	if !strings.Contains(body, "event: response.completed") || !strings.Contains(body, completed) {
		t.Fatalf("stream did not preserve terminal Responses event: %s", body)
	}
	if strings.Contains(body, "[DONE]") {
		t.Fatalf("stream unexpectedly contains [DONE]: %s", body)
	}
}

func TestDoResponseResponsesInputTokens(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/input_tokens", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"object":"response.input_tokens","input_tokens":42}`)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponsesInputTokens,
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse returned error: %v", apiErr)
	}
	usageDto, ok := usage.(*dto.Usage)
	if !ok || usageDto.PromptTokens != 42 || usageDto.TotalTokens != 42 {
		t.Fatalf("usage = %#v, want input tokens 42", usage)
	}
	if !strings.Contains(recorder.Body.String(), `"input_tokens":42`) {
		t.Fatalf("input tokens body was not preserved: %s", recorder.Body.String())
	}
}

func TestDoResponseChatNormalizesThinkingBlocks(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_1","object":"chat.completion","created":123,"model":"mistral-small-latest",
			"choices":[{"index":0,"message":{"role":"assistant","content":[
				{"type":"thinking","thinking":"reason"},{"type":"text","text":"answer"}
			]},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
		}`)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "mistral-small-latest",
		},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse returned error: %v", apiErr)
	}
	if usage == nil {
		t.Fatal("DoResponse returned nil usage")
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"content":"answer"`) || !strings.Contains(body, `"reasoning_content":"reason"`) {
		t.Fatalf("chat response missing normalized thinking blocks: %s", body)
	}
}

func TestMistralMessagesHandlerReturnsClaudeResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl_1","object":"chat.completion","created":123,"model":"mistral-small-latest",
			"choices":[{"index":0,"message":{"role":"assistant","content":[
				{"type":"thinking","thinking":"reason"},{"type":"text","text":"answer"}
			]},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
		}`)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode:               relayconstant.RelayModeChatCompletions,
		RelayFormat:             types.RelayFormatClaude,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
		ClaudeConvertInfo:       &relaycommon.ClaudeConvertInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "mistral-small-latest",
		},
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse returned error: %v", apiErr)
	}
	if usage == nil {
		t.Fatal("DoResponse returned nil usage")
	}
	var claudeResponse dto.ClaudeResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &claudeResponse); err != nil {
		t.Fatalf("response is not Claude format: %v", err)
	}
	if claudeResponse.Type != "message" || len(claudeResponse.Content) == 0 {
		t.Fatalf("Claude response = %#v, want assistant answer", claudeResponse)
	}
	foundText := false
	for _, block := range claudeResponse.Content {
		if block.Type == "text" && block.GetText() == "answer" {
			foundText = true
			break
		}
	}
	if !foundText {
		t.Fatalf("Claude response = %#v, want text block with answer", claudeResponse)
	}
}
