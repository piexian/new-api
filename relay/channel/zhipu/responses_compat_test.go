package zhipu

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
	closeNotify chan bool
}

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	return r.closeNotify
}

func TestConvertOpenAIResponsesRequestUsesDirectZhipuCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	topP := 1.0
	temperature := 0.2
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "chatglm_std",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:        "chatglm_std",
		Instructions: []byte(`"be concise"`),
		Input:        []byte(`"hello"`),
		Temperature:  &temperature,
		TopP:         &topP,
	})

	require.NoError(t, err)
	zhipuReq, ok := converted.(*ZhipuRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want *ZhipuRequest", converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAI), info.FinalRequestRelayFormat)
	require.Equal(t, &temperature, zhipuReq.Temperature)
	require.Equal(t, 0.99, zhipuReq.TopP)
	require.Len(t, zhipuReq.Prompt, 3)
	require.Equal(t, "system", zhipuReq.Prompt[0].Role)
	require.Equal(t, "be concise", zhipuReq.Prompt[0].Content)
	require.Equal(t, "user", zhipuReq.Prompt[1].Role)
	require.Equal(t, "Okay", zhipuReq.Prompt[1].Content)
	require.Equal(t, "user", zhipuReq.Prompt[2].Role)
	require.Equal(t, "hello", zhipuReq.Prompt[2].Content)
}

func TestZhipuResponsesHandlerWrapsZhipuResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body, _ := common.Marshal(ZhipuResponse{
		Success: true,
		Data: ZhipuResponseData{
			TaskId: "task_123",
			Choices: []ZhipuMessage{
				{Role: "assistant", Content: "hello"},
			},
			Usage: dto.Usage{
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
			UpstreamModelName: "chatglm_std",
		},
	}

	usage, apiErr := zhipuResponsesHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 2, usage.InputTokens)
	require.Equal(t, 3, usage.OutputTokens)
	require.Equal(t, 5, usage.TotalTokens)
	require.Equal(t, http.StatusAccepted, recorder.Code)

	var got dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &got))
	require.Equal(t, "response", got.Object)
	require.Equal(t, "chatglm_std", got.Model)
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
	stream := true

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model:  "chatglm_std",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	require.NoError(t, err)
	_, ok := converted.(*ZhipuRequest)
	require.True(t, ok)
}

func TestZhipuResponsesStreamHandlerWrapsNativeStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := &closeNotifyRecorder{ResponseRecorder: httptest.NewRecorder(), closeNotify: make(chan bool, 1)}
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	meta, err := common.Marshal(ZhipuStreamMetaResponse{
		RequestId: "task_123",
		Usage: dto.Usage{
			PromptTokens:     2,
			CompletionTokens: 3,
			TotalTokens:      5,
		},
	})
	require.NoError(t, err)
	body := strings.Join([]string{
		"data:hel\n",
		"data:lo\n",
		"meta:" + string(meta) + "\n",
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
			UpstreamModelName: "chatglm_std",
		},
	}

	usage, apiErr := zhipuResponsesStreamHandler(c, info, resp)
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
