package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetRelayRouterRegistersXAINativeRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	require.NotPanics(t, func() {
		SetRelayRouter(engine)
	})

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, route := range []string{
		"POST /v1/realtime/client_secrets",
		"POST /v1/tts",
		"GET /v1/tts",
		"GET /v1/tts/voices",
		"GET /v1/tts/voices/:voice_id",
		"POST /v1/stt",
		"GET /v1/stt",
		"GET /v1/responses",
		"GET /v1/responses/:response_id",
		"DELETE /v1/responses/:response_id",
		"GET /v1/chat/deferred-completion/:request_id",
		"GET /v1/custom-voices/:voice_id/audio",
	} {
		require.True(t, routes[route], "missing route %s", route)
	}
}

func TestSetRelayRouterRegistersMoarkNativeRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	require.NotPanics(t, func() {
		SetRelayRouter(engine)
	})

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, route := range []string{
		"POST /v1/async/*path",
		"GET /v1/tasks",
		"GET /v1/tasks/available-quota",
		"GET /v1/task/:task_id",
		"GET /v1/task/:task_id/get",
		"POST /v1/task/:task_id/cancel",
		"GET /v1/task/:task_id/status",
	} {
		require.True(t, routes[route], "missing route %s", route)
	}
}
