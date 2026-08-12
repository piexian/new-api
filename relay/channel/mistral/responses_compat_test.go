package mistral

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	globalconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestConvertOpenAIResponsesRequestPreservesMultimodalContent(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "mistral-small-latest",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "mistral-small-latest",
		Input: []byte(`[
			{
				"type":"message",
				"role":"user",
				"content":[
					{"type":"input_text","text":"look"},
					{"type":"input_image","image_url":"https://example.com/a.png","detail":"low"}
				]
			}
		]`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatOpenAI {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatOpenAI)
	}
	if len(chatReq.Messages) != 1 {
		t.Fatalf("messages = %#v, want one message", chatReq.Messages)
	}
	content := chatReq.Messages[0].ParseContent()
	if len(content) != 2 {
		t.Fatalf("content = %#v, want text and image parts", content)
	}
	if content[0].Type != dto.ContentTypeText || content[0].Text != "look" {
		t.Fatalf("text content = %#v, want look", content[0])
	}
	if content[1].Type != dto.ContentTypeImageURL || content[1].ImageUrl != "https://example.com/a.png" {
		t.Fatalf("image content = %#v, want Mistral string image URL", content[1])
	}
}

func TestConvertOpenAIResponsesRequestRewritesToolCallIDs(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "mistral-small-latest",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "mistral-small-latest",
		Input: []byte(`[
			{"type":"function_call","call_id":"call_long_id","name":"lookup","arguments":{"q":"docs"}},
			{"type":"function_call_output","call_id":"call_long_id","output":"ok"}
		]`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if len(chatReq.Messages) != 2 {
		t.Fatalf("messages = %#v, want assistant tool call and tool output", chatReq.Messages)
	}
	toolCalls := chatReq.Messages[0].ParseToolCalls()
	if len(toolCalls) != 1 {
		t.Fatalf("tool calls = %#v, want one tool call", toolCalls)
	}
	if !mistralToolCallIdRegexp.MatchString(toolCalls[0].ID) {
		t.Fatalf("tool call id = %q, want Mistral-compatible 9 character id", toolCalls[0].ID)
	}
	if chatReq.Messages[1].ToolCallId != toolCalls[0].ID {
		t.Fatalf("tool output id = %q, want rewritten id %q", chatReq.Messages[1].ToolCallId, toolCalls[0].ID)
	}
}

func TestConvertOpenAIResponsesRequestPreservesEmptyToolOutput(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "mistral-small-latest",
		Input: []byte(`[
			{"type":"function_call","call_id":"123456789","name":"lookup","arguments":{}},
			{"type":"function_call_output","call_id":"123456789","output":""}
		]`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	chatReq := converted.(*dto.GeneralOpenAIRequest)
	if got := chatReq.Messages[1].Content; got != "" {
		t.Fatalf("tool output content = %#v, want empty string", got)
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

func TestGetRequestURLUsesChatCompletionsForConvertedInterfaces(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	for _, test := range []struct {
		name        string
		requestPath string
		relayFormat types.RelayFormat
		finalFormat types.RelayFormat
	}{
		{name: "chat", requestPath: "/v1/chat/completions", relayFormat: types.RelayFormatOpenAI, finalFormat: types.RelayFormatOpenAI},
		{name: "responses", requestPath: "/v1/responses", relayFormat: types.RelayFormatOpenAIResponses, finalFormat: types.RelayFormatOpenAI},
		{name: "messages", requestPath: "/v1/messages", relayFormat: types.RelayFormatClaude, finalFormat: types.RelayFormatOpenAI},
	} {
		t.Run(test.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayFormat:             test.relayFormat,
				FinalRequestRelayFormat: test.finalFormat,
				RequestURLPath:          test.requestPath,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: "https://api.mistral.ai",
				},
			}

			got, err := adaptor.GetRequestURL(info)
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != "https://api.mistral.ai/v1/chat/completions" {
				t.Fatalf("GetRequestURL = %q, want Mistral chat completions URL", got)
			}
		})
	}
}

func TestMistralResponsesHandlerNormalizesThinkingBlocks(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
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
		RelayMode:               relayconstant.RelayModeResponses,
		RelayFormat:             types.RelayFormatOpenAIResponses,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
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
	if !strings.Contains(recorder.Body.String(), `"text":"answer"`) {
		t.Fatalf("response body missing normalized answer: %s", recorder.Body.String())
	}
}

func TestMistralResponsesStreamHandlerNormalizesThinkingBlocks(t *testing.T) {
	oldTimeout := globalconstant.StreamingTimeout
	globalconstant.StreamingTimeout = 30
	t.Cleanup(func() { globalconstant.StreamingTimeout = oldTimeout })
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"mistral-small-latest\",\"choices\":[{\"index\":0,\"delta\":{\"content\":[{\"type\":\"thinking\",\"thinking\":\"reason\"}]},\"finish_reason\":null}]}\n\n" +
				"data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"mistral-small-latest\",\"choices\":[{\"index\":0,\"delta\":{\"content\":[{\"type\":\"text\",\"text\":\"answer\"}]},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n" +
				"data: [DONE]\n\n")),
	}
	info := &relaycommon.RelayInfo{
		RelayMode:               relayconstant.RelayModeResponses,
		RelayFormat:             types.RelayFormatOpenAIResponses,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
		IsStream:                true,
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
	for _, want := range []string{`"delta":"reason"`, `"delta":"answer"`, "event: response.completed"} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("stream response missing %q: %s", want, recorder.Body.String())
		}
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
	if claudeResponse.Type != "message" || len(claudeResponse.Content) == 0 || claudeResponse.Content[0].GetText() != "answer" {
		t.Fatalf("Claude response = %#v, want assistant answer", claudeResponse)
	}
}
