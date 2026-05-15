package mistral

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
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "mistral-small-latest",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "mistral-small-latest",
		Input: []byte(`[
			{
				"type":"message",
				"role":"user",
				"content":[
					{"type":"input_text","text":"look"},
					{"type":"input_image","image_url":"https://example.com/a.png","detail":"low"}
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
	if content[1].Type != dto.ContentTypeImageURL || content[1].ImageUrl != "https://example.com/a.png" {
		t.Fatalf("image content = %#v, want Mistral string image URL", content[1])
	}
}

func TestConvertOpenAIResponsesRequestRewritesToolCallIDs(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "mistral-small-latest",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "mistral-small-latest",
		Input: []byte(`[
			{"type":"function_call","call_id":"call_long_id","name":"lookup","arguments":{"q":"docs"}},
			{"type":"function_call_output","call_id":"call_long_id","output":"ok"}
		]`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if len(chatReq.Messages) != 2 {
		t.Fatalf("messages = %#v, want assistant tool call and tool output", chatReq.Messages)
	}
	toolCalls := chatReq.Messages[0].ParseToolCalls()
	if len(toolCalls) != 1 {
		t.Fatalf("tool calls = %#v, want one tool call", toolCalls)
	}
	if !mistralToolCallIdRegexp.MatchString(toolCalls[0].ID) {
		t.Fatalf("tool call id = %q, want Mistral-compatible 9 character id", toolCalls[0].ID)
	}
	if chatReq.Messages[1].ToolCallId != toolCalls[0].ID {
		t.Fatalf("tool output id = %q, want rewritten id %q", chatReq.Messages[1].ToolCallId, toolCalls[0].ID)
	}
}
