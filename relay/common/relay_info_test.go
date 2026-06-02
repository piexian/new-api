package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	commonpkg "github.com/QuantumNous/new-api/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoGetFinalRequestRelayFormatPrefersExplicitFinal(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToConversionChain(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToRelayFormat(t *testing.T) {
	info := &RelayInfo{
		RelayFormat: types.RelayFormatGemini,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatNilReceiver(t *testing.T) {
	var info *RelayInfo
	require.Equal(t, types.RelayFormat(""), info.GetFinalRequestRelayFormat())
}

func TestTaskSubmitReqUnmarshalPreservesNativeMediaFieldsInMetadata(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model": "grok-imagine-video",
		"prompt": "make it move",
		"duration": "6",
		"image": {"url": "https://example.com/still.png"},
		"video": {"url": "https://example.com/source.mp4"},
		"reference_images": [{"url": "https://example.com/ref.png"}],
		"metadata": "{\"seed\":7}"
	}`)

	var req TaskSubmitReq
	require.NoError(t, commonpkg.Unmarshal(body, &req))

	require.Equal(t, "grok-imagine-video", req.Model)
	require.Equal(t, "make it move", req.Prompt)
	require.Equal(t, 6, req.Duration)
	require.Empty(t, req.Image)
	require.Equal(t, float64(7), req.Metadata["seed"])

	image, ok := req.Metadata["image"].(map[string]interface{})
	require.True(t, ok, "image = %T", req.Metadata["image"])
	require.Equal(t, "https://example.com/still.png", image["url"])

	video, ok := req.Metadata["video"].(map[string]interface{})
	require.True(t, ok, "video = %T", req.Metadata["video"])
	require.Equal(t, "https://example.com/source.mp4", video["url"])

	refs, ok := req.Metadata["reference_images"].([]interface{})
	require.True(t, ok, "reference_images = %T", req.Metadata["reference_images"])
	require.Len(t, refs, 1)
}

func TestGenRelayInfoXAIRealtimeUsesNativeModeAndClientWebSocket(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tts?language=en", nil)

	info, err := GenRelayInfo(c, types.RelayFormatXAIRealtime, nil, nil)

	require.NoError(t, err)
	require.Equal(t, types.RelayFormat(types.RelayFormatXAIRealtime), info.RelayFormat)
	require.Equal(t, relayconstant.RelayModeXAINative, info.RelayMode)
	require.True(t, info.IsStream)
	require.Equal(t, "/v1/tts?language=en", info.RequestURLPath)
}
