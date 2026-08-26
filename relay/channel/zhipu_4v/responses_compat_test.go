package zhipu_4v

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
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
	topP := 1.0
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "glm-4v-plus",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "glm-4v-plus",
		TopP:  &topP,
		Input: []byte(`[
			{
				"type":"message",
				"role":"user",
				"content":[
					{"type":"input_text","text":"look"},
					{"type":"input_image","image_url":"data:image/png;base64,abc","detail":"low"}
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
	if chatReq.TopP == nil || *chatReq.TopP != 0.99 {
		t.Fatalf("TopP = %#v, want capped 0.99", chatReq.TopP)
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
	image := content[1].GetImageMedia()
	if content[1].Type != dto.ContentTypeImageURL || image == nil || image.Url != "abc" || image.Detail != "low" {
		t.Fatalf("image content = %#v, want stripped base64 image with detail low", content[1])
	}
}

func TestZhipuClaudeResponsesStreamHandlerConvertsToResponsesSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 300
	t.Cleanup(func() { constant.StreamingTimeout = oldStreamingTimeout })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	stream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"glm-4.6","usage":{"input_tokens":2,"output_tokens":0}}}` + "\n",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}` + "\n",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}` + "\n",
		"data: [DONE]\n",
	}, "")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(stream)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "glm-4.6",
		},
	}

	usage, apiErr := zhipuClaudeResponsesStreamHandler(c, info, resp)
	if apiErr != nil {
		t.Fatalf("zhipuClaudeResponsesStreamHandler returned error: %v", apiErr)
	}
	if usage == nil || usage.PromptTokens != 2 || usage.CompletionTokens != 3 {
		t.Fatalf("usage = %#v, want input=2 output=3", usage)
	}
	body := recorder.Body.String()
	for _, fragment := range []string{"response.created", "response.output_text.delta", "hello", "response.completed"} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("stream body missing %q: %s", fragment, body)
		}
	}
}
