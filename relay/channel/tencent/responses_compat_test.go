package tencent

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestConvertOpenAIResponsesRequestUsesDirectTencentCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	common.SetContextKey(c, constant.ContextKeyChannelKey, "Bearer 12345|secret-id|secret-key")
	maxOutputTokens := uint(64)
	temperature := 0.2
	topP := 0.8
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "hunyuan-turbos-latest",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "hunyuan-turbos-latest",
		Instructions:    []byte(`"be concise"`),
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
		TopP:            &topP,
	})

	require.NoError(t, err)
	tencentReq, ok := converted.(*TencentChatRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *TencentChatRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAI), info.FinalRequestRelayFormat)
	require.Equal(t, int64(12345), adaptor.AppID)
	require.NotEmpty(t, adaptor.Sign)
	require.NotNil(t, tencentReq.Model)
	require.Equal(t, "hunyuan-turbos-latest", *tencentReq.Model)
	require.Equal(t, &temperature, tencentReq.Temperature)
	require.Equal(t, &topP, tencentReq.TopP)
	require.Len(t, tencentReq.Messages, 2)
	require.Equal(t, "system", tencentReq.Messages[0].Role)
	require.Equal(t, "be concise", tencentReq.Messages[0].Content)
	require.Equal(t, "user", tencentReq.Messages[1].Role)
	require.Equal(t, "hello", tencentReq.Messages[1].Content)
}

func TestTencentResponsesHandlerWrapsTencentResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body, _ := common.Marshal(TencentChatResponseSB{
		Response: TencentChatResponse{
			Id: "chat_123",
			Choices: []TencentResponseChoices{
				{
					FinishReason: "stop",
					Messages: TencentMessage{
						Role:    "assistant",
						Content: "hello",
					},
				},
			},
			Usage: TencentUsage{
				PromptTokens:     2,
				CompletionTokens: 3,
				TotalTokens:      5,
			},
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
			UpstreamModelName: "hunyuan-turbos-latest",
		},
	}

	usage, apiErr := tencentResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
	require.Equal(t, 5, usage.TotalTokens)
	require.Equal(t, http.StatusAccepted, recorder.Code)

	var got dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, "response", got.Object)
	require.Equal(t, "hunyuan-turbos-latest", got.Model)
	require.Len(t, got.Output, 1)
	require.Equal(t, "message", got.Output[0].Type)
	require.Len(t, got.Output[0].Content, 1)
	require.Equal(t, "hello", got.Output[0].Content[0].Text)
	require.NotNil(t, got.Usage)
	require.Equal(t, 2, got.Usage.InputTokens)
	require.Equal(t, 3, got.Usage.OutputTokens)
	require.Equal(t, 5, got.Usage.TotalTokens)
}

func TestConvertOpenAIResponsesRequestAllowsStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	common.SetContextKey(c, constant.ContextKeyChannelKey, "Bearer 12345|secret-id|secret-key")
	stream := true
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "hunyuan-turbos-latest",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:  "hunyuan-turbos-latest",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	require.NoError(t, err)
	tencentReq, ok := converted.(*TencentChatRequest)
	require.True(t, ok)
	require.NotNil(t, tencentReq.Stream)
	require.True(t, *tencentReq.Stream)
}

func TestTencentResponsesStreamHandlerWrapsNativeStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := strings.Join([]string{
		tencentStreamLine(t, TencentChatResponse{
			Id:      "chat_123",
			Created: 123,
			Choices: []TencentResponseChoices{
				{Delta: TencentMessage{Role: "assistant", Content: "hel"}},
			},
		}),
		tencentStreamLine(t, TencentChatResponse{
			Id:      "chat_123",
			Created: 123,
			Choices: []TencentResponseChoices{
				{Delta: TencentMessage{Role: "assistant", Content: "lo"}},
			},
		}),
		tencentStreamLine(t, TencentChatResponse{
			Id:      "chat_123",
			Created: 123,
			Choices: []TencentResponseChoices{
				{FinishReason: "stop"},
			},
			Usage: TencentUsage{
				PromptTokens:     2,
				CompletionTokens: 3,
				TotalTokens:      5,
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
			UpstreamModelName: "hunyuan-turbos-latest",
		},
	}

	usage, apiErr := tencentResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
	require.Equal(t, 5, usage.TotalTokens)

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

func tencentStreamLine(t *testing.T, event TencentChatResponse) string {
	t.Helper()
	body, err := common.Marshal(event)
	require.NoError(t, err)
	return "data: " + string(body) + "\n\n"
}
