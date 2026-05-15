package cohere

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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestUsesDirectCohereCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	maxOutputTokens := uint(64)
	temperature := 0.2
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "command-a-03-2025",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "command-a-03-2025",
		Instructions:    []byte(`"system prompt"`),
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
	})

	require.NoError(t, err)
	cohereReq, ok := converted.(*CohereChatRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *CohereChatRequest", converted)
	require.Equal(t, "command-a-03-2025", cohereReq.Model)
	require.NotNil(t, cohereReq.MaxTokens)
	require.Equal(t, maxOutputTokens, *cohereReq.MaxTokens)
	require.Equal(t, &temperature, cohereReq.Temperature)
	require.Len(t, cohereReq.Messages, 2)
	require.Equal(t, "system", cohereReq.Messages[0].Role)
	require.Equal(t, "system prompt", cohereReq.Messages[0].Content)
	require.Equal(t, "user", cohereReq.Messages[1].Role)
	require.Equal(t, "hello", cohereReq.Messages[1].Content)
}

func TestConvertOpenAIResponsesRequestMapsMultimodalAndToolCalls(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "command-a-03-2025",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "command-a-03-2025",
		Input: []byte(`[
			{
				"type":"message",
				"role":"user",
				"content":[
					{"type":"input_text","text":"describe"},
					{"type":"input_image","image_url":"https://example.com/a.png","detail":"high"}
				]
			},
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":{"q":"docs"}},
			{"type":"function_call_output","call_id":"call_1","output":{"ok":true}}
		]`),
		Tools: []byte(`[
			{
				"type":"function",
				"name":"lookup",
				"description":"lookup docs",
				"parameters":{"type":"object","properties":{"q":{"type":"string"}}}
			}
		]`),
	})

	require.NoError(t, err)
	cohereReq, ok := converted.(*CohereChatRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *CohereChatRequest", converted)
	require.Len(t, cohereReq.Messages, 3)
	userBlocks, ok := cohereReq.Messages[0].Content.([]CohereContentBlock)
	require.True(t, ok)
	require.Equal(t, "text", userBlocks[0].Type)
	require.Equal(t, "image_url", userBlocks[1].Type)
	require.Equal(t, "https://example.com/a.png", userBlocks[1].ImageURL.URL)
	require.Equal(t, "assistant", cohereReq.Messages[1].Role)
	require.Len(t, cohereReq.Messages[1].ToolCalls, 1)
	require.Equal(t, "lookup", cohereReq.Messages[1].ToolCalls[0].Function.Name)
	require.Equal(t, "tool", cohereReq.Messages[2].Role)
	require.Equal(t, "call_1", cohereReq.Messages[2].ToolCallId)
	require.Len(t, cohereReq.Tools, 1)
	require.Equal(t, "lookup", cohereReq.Tools[0].Function.Name)
}

func TestCohereResponsesHandlerWrapsCohereResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := &closeNotifyRecorder{ResponseRecorder: httptest.NewRecorder(), closeNotify: make(chan bool, 1)}
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body, _ := common.Marshal(CohereChatResponse{
		ID:           "chat_123",
		FinishReason: "COMPLETE",
		Message: CohereResponseMessage{
			Role: "assistant",
			Content: []CohereContentBlock{
				{Type: "text", Text: "hello"},
			},
		},
		Usage: CohereUsage{
			Tokens: CohereTokens{
				InputTokens:  2,
				OutputTokens: 3,
			},
		},
	})
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "command-a-03-2025",
		},
	}

	usage, apiErr := cohereResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
	require.Equal(t, http.StatusOK, recorder.Code)

	var got dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, "response", got.Object)
	require.Equal(t, "command-a-03-2025", got.Model)
	require.Len(t, got.Output, 1)
	require.Equal(t, "message", got.Output[0].Type)
	require.Len(t, got.Output[0].Content, 1)
	require.Equal(t, "hello", got.Output[0].Content[0].Text)
	require.NotNil(t, got.Usage)
	require.Equal(t, 2, got.Usage.InputTokens)
	require.Equal(t, 3, got.Usage.OutputTokens)
}

func TestConvertOpenAIResponsesRequestAllowsStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	stream := true

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model:  "command-a-03-2025",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	require.NoError(t, err)
	cohereReq, ok := converted.(*CohereChatRequest)
	require.True(t, ok)
	require.True(t, cohereReq.Stream)
}

func TestCohereResponsesStreamHandlerWrapsNativeStream(t *testing.T) {
	oldTimeout := globalconstant.StreamingTimeout
	globalconstant.StreamingTimeout = 30
	t.Cleanup(func() { globalconstant.StreamingTimeout = oldTimeout })
	gin.SetMode(gin.TestMode)
	recorder := &closeNotifyRecorder{ResponseRecorder: httptest.NewRecorder(), closeNotify: make(chan bool, 1)}
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := strings.Join([]string{
		cohereStreamLine(t, map[string]any{"type": "message-start", "id": "chat_123"}),
		cohereStreamLine(t, map[string]any{
			"type": "content-delta",
			"id":   "chat_123",
			"delta": map[string]any{
				"message": map[string]any{
					"content": map[string]any{"type": "text", "text": "hel"},
				},
			},
		}),
		cohereStreamLine(t, map[string]any{
			"type": "content-delta",
			"id":   "chat_123",
			"delta": map[string]any{
				"message": map[string]any{
					"content": map[string]any{"type": "text", "text": "lo"},
				},
			},
		}),
		cohereStreamLine(t, map[string]any{
			"type": "message-end",
			"id":   "chat_123",
			"delta": map[string]any{
				"finish_reason": "COMPLETE",
				"usage": map[string]any{
					"tokens": map[string]any{"input_tokens": 2, "output_tokens": 3},
				},
			},
		}),
		"data: [DONE]\n\n",
	}, "")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		IsStream:  true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "command-a-03-2025",
		},
	}

	usage, apiErr := cohereResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)

	bodyText := recorder.Body.String()
	for _, want := range []string{
		"event: response.created",
		"event: response.output_text.delta",
		`"delta":"hel"`,
		`"delta":"lo"`,
		"event: response.completed",
		`"text":"hello"`,
		`"input_tokens":2`,
		`"output_tokens":3`,
		"data: [DONE]",
	} {
		require.Contains(t, bodyText, want)
	}
}

func cohereStreamLine(t *testing.T, event any) string {
	t.Helper()
	body, err := common.Marshal(event)
	require.NoError(t, err)
	return "data: " + string(body) + "\n\n"
}
