package minimax

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
	maxOutputTokens := uint(64)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "MiniMax-M2",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:           "MiniMax-M2",
		Input:           []byte(`"hello"`),
		MaxOutputTokens: &maxOutputTokens,
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
	if chatReq.MaxCompletionTokens == nil || *chatReq.MaxCompletionTokens != maxOutputTokens {
		t.Fatalf("MaxCompletionTokens = %#v, want %d", chatReq.MaxCompletionTokens, maxOutputTokens)
	}
	if chatReq.MaxTokens != nil {
		t.Fatalf("MaxTokens = %#v, want nil", chatReq.MaxTokens)
	}
	if len(chatReq.Messages) != 1 || chatReq.Messages[0].Role != "user" || chatReq.Messages[0].Content != "hello" {
		t.Fatalf("messages = %#v, want one user hello message", chatReq.Messages)
	}
}
