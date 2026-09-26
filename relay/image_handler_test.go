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

// GMI 图片必须走 ConvertImageRequest，否则透传的 OpenAI 请求体会被 requestqueue 拒绝。
func TestShouldForceConvertImageRequestForGMICloud(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(""))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeGMICloud},
	}

	require.True(t, shouldForceConvertImageRequest(c, info), "JSON 入站也必须强制转换")
	info.RelayMode = relayconstant.RelayModeImagesEdits
	require.True(t, shouldForceConvertImageRequest(c, info))
	info.RelayMode = relayconstant.RelayModeChatCompletions
	require.False(t, shouldForceConvertImageRequest(c, info))
	require.False(t, shouldForceConvertImageRequest(c, nil))
}

func TestAdaptorImageLogDetails(t *testing.T) {
	t.Parallel()

	fallback := map[string]interface{}{"size": "4k+16:9"}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.Equal(t, fallback, adaptorImageLogDetails(c, fallback), "适配器未写入时回落到请求 DTO 明细")

	c.Set("image_request_detail", map[string]interface{}{"size": "3840x2160"})
	require.Equal(t, map[string]interface{}{"size": "3840x2160"}, adaptorImageLogDetails(c, fallback))

	c.Set("image_request_detail", map[string]interface{}{})
	require.Equal(t, fallback, adaptorImageLogDetails(c, fallback), "空明细不覆盖请求 DTO 明细")
}
