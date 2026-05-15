package ollama

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestConvertOpenAIResponsesRequestPassesThroughOfficialResponses(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "http://localhost:11434",
			UpstreamModelName: "llama3.2",
		},
	}
	request := dto.OpenAIResponsesRequest{
		Model: "llama3.2",
		Input: []byte(`"hello"`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}
	if _, ok := converted.(dto.OpenAIResponsesRequest); !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want dto.OpenAIResponsesRequest", converted)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatOpenAIResponses {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatOpenAIResponses)
	}

	url, err := (&Adaptor{}).GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	if url != "http://localhost:11434/v1/responses" {
		t.Fatalf("GetRequestURL = %q, want official Responses endpoint", url)
	}
}
