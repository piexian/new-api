package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldForceConvertImageRequestForXAIMultipartEdits(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", strings.NewReader(""))
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeXai,
		},
	}

	require.True(t, shouldForceConvertImageRequest(c, info))

	info.RelayMode = relayconstant.RelayModeImagesGenerations
	require.False(t, shouldForceConvertImageRequest(c, info))

	info.RelayMode = relayconstant.RelayModeImagesEdits
	info.ChannelType = constant.ChannelTypeOpenAI
	require.False(t, shouldForceConvertImageRequest(c, info))
}
