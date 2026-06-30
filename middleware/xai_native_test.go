package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetModelRequestUsesDefaultModelsForXAINativeRoutes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		path      string
		wantModel string
	}{
		{name: "client secrets", path: "/v1/realtime/client_secrets", wantModel: "grok-voice-latest"},
		{name: "tts voices", path: "/v1/tts/voices", wantModel: "grok-voice-latest"},
		{name: "stt", path: "/v1/stt", wantModel: "grok-voice-latest"},
		{name: "custom voices", path: "/v1/custom-voices/abc123/audio", wantModel: "grok-voice-latest"},
		{name: "responses websocket", path: "/v1/responses", wantModel: "grok-4.3"},
		{name: "responses retrieve", path: "/v1/responses/resp_123", wantModel: "grok-4.3"},
		{name: "deferred chat", path: "/v1/chat/deferred-completion/req_123", wantModel: "grok-4.3"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, test.path, nil)

			req, shouldSelect, err := getModelRequest(c)

			require.NoError(t, err)
			require.True(t, shouldSelect)
			require.Equal(t, test.wantModel, req.Model)
			require.Equal(t, relayconstant.RelayModeXAINative, c.GetInt("relay_mode"))
		})
	}
}

func TestGetModelRequestAllowsQueryModelOverrideForXAINativeRoutes(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tts/voices?model=custom-voice-router", nil)

	req, shouldSelect, err := getModelRequest(c)

	require.NoError(t, err)
	require.True(t, shouldSelect)
	require.Equal(t, "custom-voice-router", req.Model)
}

func TestXAINativeRouteOnlyTreatsResponsesRootAsWebSocketForGET(t *testing.T) {
	t.Parallel()

	require.True(t, isXAINativeRoute(http.MethodGet, "/v1/responses"))
	require.False(t, isXAINativeRoute(http.MethodPost, "/v1/responses"))
	require.False(t, isXAINativeRoute(http.MethodPost, "/v1/responses/compact"))
	require.False(t, isXAINativeRoute(http.MethodGet, "/v1/responses/compact"))
}

func TestGetModelRequestUsesMoarkAsyncBodyModel(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/async/music/generations", strings.NewReader(`{"model":"MusicModel","prompt":"test"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	req, shouldSelect, err := getModelRequest(c)

	require.NoError(t, err)
	require.True(t, shouldSelect)
	require.Equal(t, "MusicModel", req.Model)
	require.Equal(t, relayconstant.RelayModeMoarkNative, c.GetInt("relay_mode"))
}

func TestGetModelRequestUsesMoarkTaskDefaultModel(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	req, shouldSelect, err := getModelRequest(c)

	require.NoError(t, err)
	require.True(t, shouldSelect)
	require.Equal(t, constant.MoarkTaskModel, req.Model)
	require.Equal(t, relayconstant.RelayModeMoarkNative, c.GetInt("relay_mode"))
}

func TestGetModelRequestAllowsQueryModelOverrideForMoarkNativeRoutes(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/task/task_123?model=Wan2.7", nil)

	req, shouldSelect, err := getModelRequest(c)

	require.NoError(t, err)
	require.True(t, shouldSelect)
	require.Equal(t, "Wan2.7", req.Model)
	require.Equal(t, relayconstant.RelayModeMoarkNative, c.GetInt("relay_mode"))
}
