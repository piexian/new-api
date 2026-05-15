package openai

import (
	"bytes"
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

func TestChatCompletionResponsesHandlerWrapsChatCompletion(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body, _ := common.Marshal(dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Model:   "deepseek-chat",
		Object:  "chat.completion",
		Created: float64(123),
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:    "assistant",
					Content: "hello",
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     2,
			CompletionTokens: 3,
			TotalTokens:      5,
		},
	})
	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-chat",
		},
	}

	usage, apiErr := ChatCompletionResponsesHandler(c, info, resp)
	if apiErr != nil {
		t.Fatalf("ChatCompletionResponsesHandler returned error: %v", apiErr)
	}
	if usage == nil || usage.InputTokens != 2 || usage.OutputTokens != 3 || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v, want input=2 output=3 total=5", usage)
	}
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}

	var got dto.OpenAIResponsesResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not OpenAIResponsesResponse: %v", err)
	}
	if got.Object != "response" || got.Model != "deepseek-chat" {
		t.Fatalf("response object/model = %q/%q, want response/deepseek-chat", got.Object, got.Model)
	}
	if len(got.Output) != 1 || got.Output[0].Type != "message" || got.Output[0].Role != "assistant" {
		t.Fatalf("output = %#v, want one assistant message", got.Output)
	}
	if len(got.Output[0].Content) != 1 || got.Output[0].Content[0].Text != "hello" {
		t.Fatalf("output content = %#v, want hello output_text", got.Output[0].Content)
	}
	if got.Usage == nil || got.Usage.InputTokens != 2 || got.Usage.OutputTokens != 3 {
		t.Fatalf("response usage = %#v, want input=2 output=3", got.Usage)
	}
}

func TestChatCompletionResponsesHandlerReturnsUpstreamError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"error":{"message":"bad request","type":"invalid_request_error","code":"bad_request"}}`)
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	usage, apiErr := ChatCompletionResponsesHandler(c, &relaycommon.RelayInfo{}, resp)
	if apiErr == nil {
		t.Fatal("ChatCompletionResponsesHandler returned nil error, want upstream OpenAI error")
	}
	if usage != nil {
		t.Fatalf("usage = %#v, want nil", usage)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
	openAIError := apiErr.ToOpenAIError()
	if openAIError.Type != "invalid_request_error" {
		t.Fatalf("error type = %q, want invalid_request_error", openAIError.Type)
	}
}

func TestChatCompletionResponsesHandlerRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte(`not-json`))),
	}

	usage, apiErr := ChatCompletionResponsesHandler(c, &relaycommon.RelayInfo{}, resp)
	if apiErr == nil {
		t.Fatal("ChatCompletionResponsesHandler returned nil error, want invalid body error")
	}
	if usage != nil {
		t.Fatalf("usage = %#v, want nil", usage)
	}
	if apiErr.GetErrorCode() != types.ErrorCodeBadResponseBody {
		t.Fatalf("error code = %q, want %q", apiErr.GetErrorCode(), types.ErrorCodeBadResponseBody)
	}
}

func TestChatCompletionResponsesStreamHandlerWrapsChatStream(t *testing.T) {
	oldTimeout := globalconstant.StreamingTimeout
	globalconstant.StreamingTimeout = 30
	t.Cleanup(func() { globalconstant.StreamingTimeout = oldTimeout })
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	chunk1, _ := common.Marshal(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Object:  "chat.completion.chunk",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role: "assistant",
				},
			},
		},
	})
	text := "hello"
	chunk2, _ := common.Marshal(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Object:  "chat.completion.chunk",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: &text,
				},
			},
		},
	})
	finishReason := "stop"
	chunk3, _ := common.Marshal(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Object:  "chat.completion.chunk",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: &finishReason,
			},
		},
		Usage: &dto.Usage{
			PromptTokens:     2,
			CompletionTokens: 3,
			TotalTokens:      5,
		},
	})
	body := []byte("data: " + string(chunk1) + "\n\n" +
		"data: " + string(chunk2) + "\n\n" +
		"data: " + string(chunk3) + "\n\n" +
		"data: [DONE]\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-chat",
		},
	}

	usage, apiErr := ChatCompletionResponsesStreamHandler(c, info, resp)
	if apiErr != nil {
		t.Fatalf("ChatCompletionResponsesStreamHandler returned error: %v", apiErr)
	}
	if usage == nil || usage.InputTokens != 2 || usage.OutputTokens != 3 || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v, want input=2 output=3 total=5", usage)
	}
	bodyText := recorder.Body.String()
	for _, want := range []string{
		"event: response.created",
		"event: response.output_text.delta",
		`"delta":"hello"`,
		"event: response.completed",
		`"input_tokens":2`,
		`"output_tokens":3`,
	} {
		if !strings.Contains(bodyText, want) {
			t.Fatalf("stream body missing %q:\n%s", want, bodyText)
		}
	}
}

func TestChatCompletionResponsesStreamHandlerKeepsUsageAfterFinishChunk(t *testing.T) {
	oldTimeout := globalconstant.StreamingTimeout
	globalconstant.StreamingTimeout = 30
	t.Cleanup(func() { globalconstant.StreamingTimeout = oldTimeout })
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	text := "hello"
	textChunk, _ := common.Marshal(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Object:  "chat.completion.chunk",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: &text,
				},
			},
		},
	})
	finishReason := "stop"
	finishChunk, _ := common.Marshal(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Object:  "chat.completion.chunk",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				FinishReason: &finishReason,
			},
		},
	})
	usageChunk, _ := common.Marshal(dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Object:  "chat.completion.chunk",
		Created: 123,
		Model:   "deepseek-chat",
		Choices: []dto.ChatCompletionsStreamResponseChoice{},
		Usage: &dto.Usage{
			PromptTokens:     7,
			CompletionTokens: 11,
			TotalTokens:      18,
		},
	})
	body := []byte("data: " + string(textChunk) + "\n\n" +
		"data: " + string(finishChunk) + "\n\n" +
		"data: " + string(usageChunk) + "\n\n" +
		"data: [DONE]\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-chat",
		},
	}

	usage, apiErr := ChatCompletionResponsesStreamHandler(c, info, resp)
	if apiErr != nil {
		t.Fatalf("ChatCompletionResponsesStreamHandler returned error: %v", apiErr)
	}
	if usage == nil || usage.InputTokens != 7 || usage.OutputTokens != 11 || usage.TotalTokens != 18 {
		t.Fatalf("usage = %#v, want input=7 output=11 total=18", usage)
	}
	bodyText := recorder.Body.String()
	if !strings.Contains(bodyText, `"input_tokens":7`) || !strings.Contains(bodyText, `"output_tokens":11`) {
		t.Fatalf("stream body missing usage-only chunk values:\n%s", bodyText)
	}
}
