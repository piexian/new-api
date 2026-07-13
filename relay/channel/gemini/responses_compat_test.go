package gemini

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestUsesDirectGeminiCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	maxOutputTokens := uint(64)
	temperature := 0.2
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeGemini,
			UpstreamModelName: "gemini-2.5-flash",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "gemini-2.5-flash",
		Instructions:    []byte(`"be concise"`),
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
	})

	require.NoError(t, err)
	geminiReq, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.GeminiChatRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.FinalRequestRelayFormat)
	require.NotNil(t, geminiReq.SystemInstructions)
	require.Len(t, geminiReq.SystemInstructions.Parts, 1)
	require.Equal(t, "be concise", geminiReq.SystemInstructions.Parts[0].Text)
	require.Equal(t, &temperature, geminiReq.GenerationConfig.Temperature)
	require.NotNil(t, geminiReq.GenerationConfig.MaxOutputTokens)
	require.Equal(t, maxOutputTokens, *geminiReq.GenerationConfig.MaxOutputTokens)
	require.Len(t, geminiReq.Contents, 1)
	require.Equal(t, "user", geminiReq.Contents[0].Role)
	require.Len(t, geminiReq.Contents[0].Parts, 1)
	require.Equal(t, "hello", geminiReq.Contents[0].Parts[0].Text)
}

func TestConvertOpenAIResponsesRequestMapsCachedContentExtraBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeGemini,
			UpstreamModelName: "gemini-2.5-flash",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:     "gemini-2.5-flash",
		Input:     []byte(`"hello"`),
		ExtraBody: []byte(`{"google":{"cached_content":"cachedContents/cache-123"}}`),
	})

	require.NoError(t, err)
	geminiReq, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.GeminiChatRequest", converted)
	require.Equal(t, "cachedContents/cache-123", geminiReq.CachedContent)
}

func TestConvertOpenAIRequestMapsCachedContentExtraBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeGemini,
			UpstreamModelName: "gemini-2.5-flash",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model: "gemini-2.5-flash",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
		ExtraBody: []byte(`{"google":{"cached_content":"cachedContents/cache-123"}}`),
	})

	require.NoError(t, err)
	geminiReq, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok, "ConvertOpenAIRequest returned %T, want *dto.GeminiChatRequest", converted)
	require.Equal(t, "cachedContents/cache-123", geminiReq.CachedContent)
}

func TestConvertOpenAIResponsesRequestMapsFunctionItemsToGemini(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeGemini,
			UpstreamModelName: "gemini-2.5-flash",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "gemini-2.5-flash",
		Input: []byte(`[
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
	geminiReq, ok := converted.(*dto.GeminiChatRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.GeminiChatRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.FinalRequestRelayFormat)
	require.NotEmpty(t, geminiReq.Tools)
	require.Len(t, geminiReq.Contents, 2)
	require.Equal(t, "model", geminiReq.Contents[0].Role)
	require.NotNil(t, geminiReq.Contents[0].Parts[0].FunctionCall)
	require.Equal(t, "lookup", geminiReq.Contents[0].Parts[0].FunctionCall.FunctionName)
	require.Equal(t, "user", geminiReq.Contents[1].Role)
	require.NotNil(t, geminiReq.Contents[1].Parts[0].FunctionResponse)
	require.Equal(t, "lookup", geminiReq.Contents[1].Parts[0].FunctionResponse.Name)
}

func TestGeminiResponsesHandlerWrapsGeminiResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	stop := "STOP"
	body, _ := common.Marshal(dto.GeminiChatResponse{
		Candidates: []dto.GeminiChatCandidate{
			{
				Index:        0,
				FinishReason: &stop,
				Content: dto.GeminiChatContent{
					Role: "model",
					Parts: []dto.GeminiPart{
						{Text: "hello"},
					},
				},
			},
		},
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     2,
			CandidatesTokenCount: 3,
			TotalTokenCount:      5,
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
			UpstreamModelName: "gemini-2.5-flash",
		},
	}

	usage, apiErr := GeminiResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
	require.Equal(t, http.StatusOK, recorder.Code)

	var got dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, "response", got.Object)
	require.Equal(t, "gemini-2.5-flash", got.Model)
	require.Len(t, got.Output, 1)
	require.Equal(t, "message", got.Output[0].Type)
	require.Len(t, got.Output[0].Content, 1)
	require.Equal(t, "hello", got.Output[0].Content[0].Text)
	require.NotNil(t, got.Usage)
	require.Equal(t, 2, got.Usage.InputTokens)
	require.Equal(t, 3, got.Usage.OutputTokens)
}

func TestConvertOpenAIResponsesRequestSupportsStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	stream := true

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model:  "gemini-2.5-flash",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	require.NoError(t, err)
	require.IsType(t, &dto.GeminiChatRequest{}, converted)
}
