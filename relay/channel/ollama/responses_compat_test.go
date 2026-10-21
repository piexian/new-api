package ollama

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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

func TestOllamaResponsesInputTokensResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/input_tokens", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"object":"response.input_tokens","input_tokens":42}`)),
	}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponsesInputTokens, ChannelMeta: &relaycommon.ChannelMeta{}}

	usage, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, apiErr)
	require.Equal(t, 42, usage.(*dto.Usage).InputTokens)
	require.JSONEq(t, `{"object":"response.input_tokens","input_tokens":42}`, recorder.Body.String())
}
