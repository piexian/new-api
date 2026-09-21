package typesafe

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLUsesSystemOnePath(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:    76,
		ChannelBaseUrl: "https://api.typesafe.ai",
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.typesafe.ai/v1/systemone", url)
}

func TestSetupRequestHeaderSetsBearerAndJSON(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{ApiKey: "tsk-test"}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", nil)

	header := http.Header{}
	err := adaptor.SetupRequestHeader(c, &header, info)
	require.NoError(t, err)
	require.Equal(t, "Bearer tsk-test", header.Get("Authorization"))
	require.Equal(t, "application/json", header.Get("Content-Type"))
}

func TestExtractUsage(t *testing.T) {
	t.Parallel()

	t.Run("parses usage", func(t *testing.T) {
		body := `{"model":"jev-1.13.0","answers":{"is_urgent":{"type":"noul","noul":0.95}},"usage":{"input_tokens":296,"output_tokens":20}}`
		usage := extractUsage([]byte(body))
		require.NotNil(t, usage)
		require.Equal(t, 296, usage.PromptTokens)
		require.Equal(t, 20, usage.CompletionTokens)
		require.Equal(t, 316, usage.TotalTokens)
	})

	t.Run("missing usage returns nil", func(t *testing.T) {
		usage := extractUsage([]byte(`{"model":"jev-1.13.0","answers":{}}`))
		require.Nil(t, usage)
	})

	t.Run("invalid json returns nil", func(t *testing.T) {
		usage := extractUsage([]byte(`not-json`))
		require.Nil(t, usage)
	})

	t.Run("empty body returns nil", func(t *testing.T) {
		usage := extractUsage(nil)
		require.Nil(t, usage)
	})
}

func TestDoResponsePassthroughAndUsage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	body := `{"model":"jev-1.13.0","answers":{"is_urgent":{"type":"noul","noul":0.95}},"usage":{"input_tokens":296,"output_tokens":20}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	adaptor := &Adaptor{}
	usageAny, apiErr := adaptor.DoResponse(c, resp, &relaycommon.RelayInfo{})
	require.Nil(t, apiErr)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 296, usage.PromptTokens)
	require.Equal(t, 20, usage.CompletionTokens)
	require.Equal(t, body, recorder.Body.String())
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestDoRequestForwardsToUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	var gotPath, gotAuth, gotContentType, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":5,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	payload := `{"model":"jev-latest","state":"test","questions":{"q":{"type":"noul","instructions":"ok?"}}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:    76,
		ChannelBaseUrl: upstream.URL,
		ApiKey:         "tsk-test",
	}

	adaptor := &Adaptor{}
	resp, err := adaptor.DoRequest(c, info, strings.NewReader(payload))
	require.NoError(t, err)
	httpResp, ok := resp.(*http.Response)
	require.True(t, ok)
	defer httpResp.Body.Close()

	require.Equal(t, "/v1/systemone", gotPath)
	require.Equal(t, "Bearer tsk-test", gotAuth)
	require.Equal(t, "application/json", gotContentType)
	require.JSONEq(t, payload, gotBody)

	respBody, err := io.ReadAll(httpResp.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"usage":{"input_tokens":5,"output_tokens":1}}`, string(respBody))
}
