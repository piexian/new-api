package deepseek

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestConvertOpenAIResponsesRequestUsesDirectChatCompat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-chat",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "deepseek-chat",
		Input: []byte(`"hello"`),
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
	if chatReq.Model != "deepseek-chat" {
		t.Fatalf("model = %q, want deepseek-chat", chatReq.Model)
	}
	if len(chatReq.Messages) != 1 || chatReq.Messages[0].Role != "user" || chatReq.Messages[0].Content != "hello" {
		t.Fatalf("messages = %#v, want one user hello message", chatReq.Messages)
	}
}

func TestConvertOpenAIResponsesRequestAppliesDeepSeekV4ThinkingSuffix(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-v4-chat-max",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "deepseek-v4-chat-max",
		Input: []byte(`"hello"`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if chatReq.Model != "deepseek-v4-chat" {
		t.Fatalf("model = %q, want deepseek-v4-chat", chatReq.Model)
	}
	if string(chatReq.THINKING) != `{"type":"enabled"}` {
		t.Fatalf("THINKING = %s, want enabled thinking", string(chatReq.THINKING))
	}
	if chatReq.ReasoningEffort != "max" || info.ReasoningEffort != "max" {
		t.Fatalf("reasoning effort request/info = %q/%q, want max/max", chatReq.ReasoningEffort, info.ReasoningEffort)
	}
	if info.UpstreamModelName != "deepseek-v4-chat" {
		t.Fatalf("UpstreamModelName = %q, want deepseek-v4-chat", info.UpstreamModelName)
	}
}

func TestConvertOpenAIResponsesRequestAllowsStream(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	stream := true

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, &relaycommon.RelayInfo{}, dto.OpenAIResponsesRequest{
		Model:  "deepseek-chat",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}
	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if chatReq.Stream == nil || !*chatReq.Stream {
		t.Fatalf("Stream = %#v, want true", chatReq.Stream)
	}
}
