package responsescompat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChatCompletionToResponseText(t *testing.T) {
	resp, usage := ChatCompletionToResponse(nil, nil, &dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Model:   "test-model",
		Object:  "chat.completion",
		Created: int64(123),
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

	require.NotNil(t, resp)
	require.Equal(t, "response", resp.Object)
	require.Equal(t, "test-model", resp.Model)
	require.Len(t, resp.Output, 1)
	require.Equal(t, "message", resp.Output[0].Type)
	require.Equal(t, "assistant", resp.Output[0].Role)
	require.Equal(t, "output_text", resp.Output[0].Content[0].Type)
	require.Equal(t, "hello", resp.Output[0].Content[0].Text)
	require.Equal(t, 2, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
}

func TestChatCompletionToResponseToolCall(t *testing.T) {
	message := dto.Message{
		Role:    "assistant",
		Content: "",
	}
	message.SetToolCalls([]dto.ToolCallRequest{
		{
			ID:   "call_1",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "lookup",
				Arguments: `{"q":"docs"}`,
			},
		},
	})

	resp, _ := ChatCompletionToResponse(nil, nil, &dto.OpenAITextResponse{
		Model: "test-model",
		Choices: []dto.OpenAITextResponseChoice{
			{Message: message, FinishReason: "tool_calls"},
		},
	})

	require.NotNil(t, resp)
	require.Len(t, resp.Output, 1)
	require.Equal(t, "function_call", resp.Output[0].Type)
	require.Equal(t, "call_1", resp.Output[0].CallId)
	require.Equal(t, "lookup", resp.Output[0].Name)
	require.JSONEq(t, `{"q":"docs"}`, string(resp.Output[0].Arguments))
}

func TestStreamEmitterTextLifecycle(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	emitter := NewStreamEmitter(c, nil)
	emitter.SetModel("test-model")
	emitter.SetCreatedAt(123)
	emitter.SetUsage(&dto.Usage{
		PromptTokens:     2,
		CompletionTokens: 3,
		TotalTokens:      5,
	})

	require.True(t, emitter.SendTextDelta("hel"))
	require.True(t, emitter.SendTextDelta("lo"))
	require.True(t, emitter.Complete())
	require.Nil(t, emitter.Err())
	require.Equal(t, 2, emitter.Usage().InputTokens)
	require.Equal(t, 3, emitter.Usage().OutputTokens)

	body := recorder.Body.String()
	for _, want := range []string{
		"event: response.created",
		"event: response.output_item.added",
		"event: response.content_part.added",
		"event: response.output_text.delta",
		"event: response.output_text.done",
		"event: response.content_part.done",
		"event: response.output_item.done",
		"event: response.completed",
		`"delta":"hel"`,
		`"delta":"lo"`,
		`"text":"hello"`,
		`"input_tokens":2`,
		`"output_tokens":3`,
		"data: [DONE]",
	} {
		require.Contains(t, body, want)
	}
}

func TestStreamEmitterIgnoresEmptyUsageOverride(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	emitter := NewStreamEmitter(c, nil)
	emitter.SetUsage(&dto.Usage{
		PromptTokens:     2,
		CompletionTokens: 3,
		TotalTokens:      5,
	})
	emitter.SetUsage(&dto.Usage{})

	require.Equal(t, 5, emitter.Usage().TotalTokens)
}

func TestStreamEmitterCompletesEmptyStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	emitter := NewStreamEmitter(c, nil)

	require.True(t, emitter.Complete())
	require.Nil(t, emitter.Err())

	body := recorder.Body.String()
	require.Contains(t, body, "event: response.created")
	require.Contains(t, body, "event: response.completed")
	require.NotContains(t, body, "event: response.output_text.delta")
	var completed dto.ResponsesStreamResponse
	for _, part := range strings.Split(body, "\n\n") {
		if strings.Contains(part, "event: response.completed") {
			data := strings.TrimSpace(strings.TrimPrefix(strings.Split(part, "\n")[1], "data: "))
			require.NoError(t, common.Unmarshal(common.StringToByteSlice(data), &completed))
		}
	}
	require.NotNil(t, completed.Response)
	require.Empty(t, completed.Response.Output)
}
