package zhipu_4v

import (
	"net/http/httptest"
	"testing"

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
