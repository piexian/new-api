package mistral

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMistralOCRHandlerPassThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/ocr", nil)

	body := `{"pages":[{"index":0,"markdown":"hello"}],"model":"mistral-ocr-latest","usage_info":{"pages_processed":3,"doc_size_bytes":12345}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	usage, apiErr := MistralOCRHandler(c, resp, &relaycommon.RelayInfo{})
	require.Nil(t, apiErr)
	require.JSONEq(t, body, recorder.Body.String())
	// 约定 1 页 = 1000 tokens
	require.Equal(t, 3000, usage.PromptTokens)
	require.Equal(t, 3000, usage.TotalTokens)
}

func TestMistralOCRHandlerWithoutUsageInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/ocr", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"pages":[],"model":"mistral-ocr-latest"}`)),
	}

	usage, apiErr := MistralOCRHandler(c, resp, &relaycommon.RelayInfo{})
	require.Nil(t, apiErr)
	require.Equal(t, 0, usage.PromptTokens)
	require.Equal(t, 0, usage.TotalTokens)
}

func TestGetRequestURLOCR(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeOCR,
		RelayFormat:    types.RelayFormatOCR,
		RequestURLPath: "/v1/ocr",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeMistral,
			ChannelBaseUrl: "https://api.mistral.ai",
		},
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.mistral.ai/v1/ocr", url)
}

func TestModelListIncludesDocumentAndAudioModels(t *testing.T) {
	for _, model := range []string{
		"mistral-ocr-latest",
		"voxtral-mini-latest",
		"voxtral-small-latest",
		"voxtral-mini-tts-2603",
		"mistral-embed",
		"mistral-moderation-latest",
		"mistral-large-latest",
	} {
		require.Contains(t, ModelList, model)
	}
}
