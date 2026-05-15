package cloudflare

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
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.cloudflare.com",
			ApiVersion:        "account_123",
			UpstreamModelName: "@cf/openai/gpt-oss-20b",
		},
	}
	request := dto.OpenAIResponsesRequest{
		Model: "@cf/openai/gpt-oss-20b",
		Input: []byte(`"hello"`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	require.Equal(t, request, converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.FinalRequestRelayFormat)

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.cloudflare.com/client/v4/accounts/account_123/ai/v1/responses", url)
}
