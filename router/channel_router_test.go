package router

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelStatusRoutesUseOperatePermission(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodPost, "/:id/status", authz.ChannelOperate, controller.UpdateChannelStatus)
	assertChannelRoutePermission(t, http.MethodPost, "/status/batch", authz.ChannelOperate, controller.BatchUpdateChannelStatus)
	assertChannelRoutePermission(t, http.MethodPut, "/", authz.ChannelWrite, controller.UpdateChannel)
}

func TestChannelDeleteRoutesUseSensitiveWritePermission(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodDelete, "/:id", authz.ChannelSensitiveWrite, controller.DeleteChannel)
	assertChannelRoutePermission(t, http.MethodPost, "/batch", authz.ChannelSensitiveWrite, controller.DeleteChannelBatch)
	assertChannelRoutePermission(t, http.MethodDelete, "/disabled", authz.ChannelSensitiveWrite, controller.DeleteDisabledChannel)
	assertChannelRoutePermission(t, http.MethodPut, "/", authz.ChannelWrite, controller.UpdateChannel)
	assertChannelRoutePermission(t, http.MethodPut, "/tag", authz.ChannelWrite, controller.EditTagChannels)
	assertChannelRoutePermission(t, http.MethodPost, "/batch/tag", authz.ChannelWrite, controller.BatchSetChannelTag)
}

func TestChannelStatusRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api")

	require.NotPanics(t, func() {
		registerChannelRoutes(api)
	})
}

func TestSetApiRouterRegistersPermissionRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	require.NotPanics(t, func() {
		SetApiRouter(engine)
	})

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range channelPermissionRoutes {
		assert.Contains(t, routes, route.method+" /api/channel"+route.path)
	}

	requiredRoutes := []string{
		http.MethodGet + " /api/authz/catalog",
		http.MethodPost + " /api/option/test_email",
		http.MethodGet + " /api/option/email_templates",
		http.MethodGet + " /api/option/email_templates/:event/:locale",
		http.MethodPut + " /api/option/email_templates/:event/:locale",
		http.MethodDelete + " /api/option/email_templates/:event/:locale",
		http.MethodPost + " /api/option/email_templates/preview",
		http.MethodPost + " /api/channel/:id/key",
		http.MethodGet + " /api/channel/ops",
		http.MethodPost + " /api/channel/:id/status",
		http.MethodPost + " /api/channel/status/batch",
		http.MethodPost + " /api/channel/codex/oauth/start",
		http.MethodPost + " /api/channel/codex/oauth/complete",
		http.MethodPost + " /api/channel/:id/codex/oauth/start",
		http.MethodPost + " /api/channel/:id/codex/oauth/complete",
		http.MethodPost + " /api/channel/qwen/oauth/start",
		http.MethodPost + " /api/channel/qwen/oauth/complete",
		http.MethodPost + " /api/channel/:id/qwen/oauth/start",
		http.MethodPost + " /api/channel/:id/qwen/oauth/complete",
		http.MethodPost + " /api/channel/:id/codex/refresh",
		http.MethodGet + " /api/channel/:id/codex/usage",
		http.MethodGet + " /api/channel/:id/codex/usage/reset-credits",
		http.MethodPost + " /api/channel/:id/codex/usage/reset",
		http.MethodGet + " /api/channel/:id/minimax/usage",
		http.MethodGet + " /api/channel/:id/zhipu/coding_plan/usage",
		http.MethodGet + " /api/channel/:id/kimi/coding_plan/usage",
		http.MethodGet + " /api/channel/:id/qwen/token_plan/usage",
	}
	for _, route := range requiredRoutes {
		assert.Contains(t, routes, route)
	}
}

func assertChannelRoutePermission(t *testing.T, method string, path string, permission authz.Permission, handler any) {
	t.Helper()
	for _, route := range channelPermissionRoutes {
		if route.method == method && route.path == path {
			assert.Equal(t, permission, route.permission)
			assert.Equal(t, reflect.ValueOf(handler).Pointer(), reflect.ValueOf(route.handler).Pointer())
			return
		}
	}
	t.Fatalf("route %s %s not found", method, path)
}
