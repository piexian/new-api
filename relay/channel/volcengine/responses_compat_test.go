package volcengine

import (
	"net/http/httptest"
	"testing"

	channelconstant "github.com/QuantumNous/new-api/constant"
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
			ChannelBaseUrl:    "https://ark.cn-beijing.volces.com",
			ChannelType:       channelconstant.ChannelTypeVolcEngine,
			UpstreamModelName: "doubao-seed-1-6-250615",
		},
	}
	request := dto.OpenAIResponsesRequest{
		Model: "doubao-seed-1-6-250615",
		Input: []byte(`"hello"`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	require.Equal(t, request, converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.FinalRequestRelayFormat)

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/responses", url)
}
