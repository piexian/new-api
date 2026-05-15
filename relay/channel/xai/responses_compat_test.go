package xai

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestPassesThroughOfficialResponsesAPI(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeResponses,
		RelayFormat:    types.RelayFormatOpenAIResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.x.ai",
			UpstreamModelName: "grok-4",
		},
	}
	request := dto.OpenAIResponsesRequest{
		Input: []byte(`"hello"`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	got, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok, "ConvertOpenAIResponsesRequest returned %T, want dto.OpenAIResponsesRequest", converted)
	require.Equal(t, "grok-4", got.Model)
	require.Equal(t, request.Input, got.Input)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.FinalRequestRelayFormat)

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.x.ai/v1/responses", url)
}
