package xunfei

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
	closeNotify chan bool
}

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	return r.closeNotify
}

func TestConvertOpenAIResponsesRequestUsesDirectChatCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	maxOutputTokens := uint(64)
	temperature := 0.2
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "SparkDesk-v3.5",
		},
	}
	adaptor := &Adaptor{}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "SparkDesk-v3.5",
		Instructions:    []byte(`"be concise"`),
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
		Temperature:     &temperature,
	})

	require.NoError(t, err)
	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	require.Same(t, chatReq, adaptor.request)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAI), info.FinalRequestRelayFormat)
	require.Equal(t, "SparkDesk-v3.5", chatReq.Model)
	require.NotNil(t, chatReq.MaxCompletionTokens)
	require.Equal(t, maxOutputTokens, *chatReq.MaxCompletionTokens)
	require.Equal(t, &temperature, chatReq.Temperature)
	require.Len(t, chatReq.Messages, 2)
	require.Equal(t, "system", chatReq.Messages[0].Role)
	require.Equal(t, "be concise", chatReq.Messages[0].Content)
	require.Equal(t, "user", chatReq.Messages[1].Role)
	require.Equal(t, "hello", chatReq.Messages[1].Content)

	xunfeiReq := requestOpenAI2Xunfei(*chatReq, "app-id", "generalv3.5")
	require.Equal(t, "app-id", xunfeiReq.Header.AppId)
	require.Equal(t, "generalv3.5", xunfeiReq.Parameter.Chat.Domain)
	require.Equal(t, uint(64), xunfeiReq.Parameter.Chat.MaxTokens)
	require.Len(t, xunfeiReq.Payload.Message.Text, 2)
	require.Equal(t, "system", xunfeiReq.Payload.Message.Text[0].Role)
	require.Equal(t, "be concise", xunfeiReq.Payload.Message.Text[0].Content)
	require.Equal(t, "user", xunfeiReq.Payload.Message.Text[1].Role)
	require.Equal(t, "hello", xunfeiReq.Payload.Message.Text[1].Content)
}

func TestResponseXunfei2OpenAIIsResponsesCompatible(t *testing.T) {
	t.Parallel()

	response := responseXunfei2OpenAI(&XunfeiChatResponse{
		Payload: struct {
			Choices struct {
				Status int                          `json:"status"`
				Seq    int                          `json:"seq"`
				Text   []XunfeiChatResponseTextItem `json:"text"`
			} `json:"choices"`
			Usage struct {
				Text dto.Usage `json:"text"`
			} `json:"usage"`
		}{
			Choices: struct {
				Status int                          `json:"status"`
				Seq    int                          `json:"seq"`
				Text   []XunfeiChatResponseTextItem `json:"text"`
			}{
				Text: []XunfeiChatResponseTextItem{{Content: "hello"}},
			},
			Usage: struct {
				Text dto.Usage `json:"text"`
			}{
				Text: dto.Usage{
					PromptTokens:     2,
					CompletionTokens: 3,
					TotalTokens:      5,
				},
			},
		},
	})

	require.Equal(t, "chat.completion", response.Object)
	require.Len(t, response.Choices, 1)
	require.Equal(t, "hello", response.Choices[0].Message.Content)
	require.Equal(t, 2, response.Usage.PromptTokens)
	require.Equal(t, 3, response.Usage.CompletionTokens)
	require.Equal(t, 5, response.Usage.TotalTokens)
}

func TestConvertOpenAIResponsesRequestAllowsStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	stream := true

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model:  "SparkDesk-v3.5",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	require.NoError(t, err)
	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.NotNil(t, chatReq.Stream)
	require.True(t, *chatReq.Stream)
}

func TestXunfeiResponsesStreamHandlerWrapsNativeWebSocketStream(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var request XunfeiChatRequest
		require.NoError(t, conn.ReadJSON(&request))
		require.Len(t, request.Payload.Message.Text, 1)
		require.Equal(t, "hello", request.Payload.Message.Text[0].Content)

		require.NoError(t, conn.WriteJSON(xunfeiTestStreamResponse("hel", 1, dto.Usage{})))
		require.NoError(t, conn.WriteJSON(xunfeiTestStreamResponse("lo", 2, dto.Usage{
			PromptTokens:     2,
			CompletionTokens: 3,
			TotalTokens:      5,
		})))
	}))
	defer server.Close()

	oldBuilder := xunfeiAuthURLBuilder
	xunfeiAuthURLBuilder = func(hostUrl string, apiKey string, apiSecret string) string {
		return "ws" + strings.TrimPrefix(server.URL, "http")
	}
	t.Cleanup(func() { xunfeiAuthURLBuilder = oldBuilder })

	gin.SetMode(gin.TestMode)
	recorder := &closeNotifyRecorder{ResponseRecorder: httptest.NewRecorder(), closeNotify: make(chan bool, 1)}
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?api-version=v3.5", nil)
	stream := true
	textRequest := dto.GeneralOpenAIRequest{
		Model:  "SparkDesk-v3.5",
		Stream: &stream,
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		IsStream:  true,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "SparkDesk-v3.5",
		},
	}

	usage, apiErr := xunfeiResponsesStreamHandler(c, info, textRequest, "app-id", "api-secret", "api-key")
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

func xunfeiTestStreamResponse(content string, status int, usage dto.Usage) XunfeiChatResponse {
	var response XunfeiChatResponse
	response.Payload.Choices.Status = status
	response.Payload.Choices.Text = []XunfeiChatResponseTextItem{{Content: content, Role: "assistant"}}
	response.Payload.Usage.Text = usage
	return response
}
