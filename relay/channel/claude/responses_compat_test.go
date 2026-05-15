package claude

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestUsesDirectClaudeCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	maxOutputTokens := uint(64)
	temperature := 0.2
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-5-sonnet",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "claude-3-5-sonnet",
		Instructions:    []byte(`"be concise"`),
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
	})

	require.NoError(t, err)
	claudeReq, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.ClaudeRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.FinalRequestRelayFormat)
	require.Equal(t, "claude-3-5-sonnet", claudeReq.Model)
	require.NotNil(t, claudeReq.MaxTokens)
	require.Equal(t, maxOutputTokens, *claudeReq.MaxTokens)
	require.Equal(t, &temperature, claudeReq.Temperature)

	system, ok := claudeReq.System.([]dto.ClaudeMediaMessage)
	require.True(t, ok, "system = %#v, want []dto.ClaudeMediaMessage", claudeReq.System)
	require.Len(t, system, 1)
	require.Equal(t, "text", system[0].Type)
	require.Equal(t, "be concise", system[0].GetText())
	require.Len(t, claudeReq.Messages, 1)
	require.Equal(t, "user", claudeReq.Messages[0].Role)
	require.Equal(t, "hello", claudeReq.Messages[0].Content)
}

func TestConvertOpenAIResponsesRequestMapsFunctionCallToClaudeToolUse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-5-sonnet",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "claude-3-5-sonnet",
		Input: []byte(`[
			{"type":"function_call","call_id":"call_1","name":"lookup","arguments":{"q":"docs"}},
			{"type":"function_call_output","call_id":"call_1","output":"ok"}
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
	claudeReq, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.ClaudeRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.FinalRequestRelayFormat)
	require.NotNil(t, claudeReq.Tools)
	require.Len(t, claudeReq.Messages, 3)
	require.Equal(t, "user", claudeReq.Messages[0].Role)
	require.Equal(t, "assistant", claudeReq.Messages[1].Role)
	assistantContent, ok := claudeReq.Messages[1].Content.([]dto.ClaudeMediaMessage)
	require.True(t, ok, "assistant content = %#v, want []dto.ClaudeMediaMessage", claudeReq.Messages[1].Content)
	require.NotEmpty(t, assistantContent)
	toolUse := assistantContent[len(assistantContent)-1]
	require.Equal(t, "tool_use", toolUse.Type)
	require.Equal(t, "call_1", toolUse.Id)
	require.Equal(t, "lookup", toolUse.Name)
	require.Equal(t, "user", claudeReq.Messages[2].Role)
	toolResult, ok := claudeReq.Messages[2].Content.([]dto.ClaudeMediaMessage)
	require.True(t, ok, "tool result = %#v, want []dto.ClaudeMediaMessage", claudeReq.Messages[2].Content)
	require.Len(t, toolResult, 1)
	require.Equal(t, "tool_result", toolResult[0].Type)
	require.Equal(t, "call_1", toolResult[0].ToolUseId)
}

func TestClaudeResponsesHandlerWrapsClaudeResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body, _ := common.Marshal(dto.ClaudeResponse{
		Id:         "msg_123",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-5-sonnet",
		StopReason: "end_turn",
		Content: []dto.ClaudeMediaMessage{
			{Type: "text", Text: common.GetPointer("hello")},
		},
		Usage: &dto.ClaudeUsage{
			InputTokens:  2,
			OutputTokens: 3,
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
			UpstreamModelName: "claude-3-5-sonnet",
		},
	}

	usage, apiErr := ClaudeResponsesHandler(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
	require.Equal(t, http.StatusOK, recorder.Code)

	var got dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, "response", got.Object)
	require.Equal(t, "claude-3-5-sonnet", got.Model)
	require.Len(t, got.Output, 1)
	require.Equal(t, "message", got.Output[0].Type)
	require.Len(t, got.Output[0].Content, 1)
	require.Equal(t, "hello", got.Output[0].Content[0].Text)
	require.NotNil(t, got.Usage)
	require.Equal(t, 2, got.Usage.InputTokens)
	require.Equal(t, 3, got.Usage.OutputTokens)
}

func TestConvertOpenAIResponsesRequestRejectsStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	stream := true

	_, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model:  "claude-3-5-sonnet",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	require.Error(t, err)
}
