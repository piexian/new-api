package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBindOptionalQwenOAuthRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("empty body is allowed", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/", nil)

		request, err := bindOptionalQwenOAuthRequest(context)
		require.NoError(t, err)
		require.Empty(t, request.APIKey)
	})

	t.Run("json body is decoded", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"api_key":"sk-sp-request"}`))
		context.Request.Header.Set("Content-Type", "application/json")

		request, err := bindOptionalQwenOAuthRequest(context)
		require.NoError(t, err)
		require.Equal(t, "sk-sp-request", request.APIKey)
	})

	t.Run("invalid json is rejected", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
		context.Request.Header.Set("Content-Type", "application/json")

		_, err := bindOptionalQwenOAuthRequest(context)
		require.Error(t, err)
	})
}

func TestResolveQwenOAuthAPIKey(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		Key: `{"type":"qwen_token_plan","api_key":"sk-sp-stored","access_token":"oauth-token","expires_at":"2099-01-01T00:00:00Z"}`,
	}

	t.Run("blank request reuses stored key", func(t *testing.T) {
		apiKey, err := resolveQwenOAuthAPIKey(channel, "")
		require.NoError(t, err)
		require.Equal(t, "sk-sp-stored", apiKey)
	})

	t.Run("explicit request replaces stored key", func(t *testing.T) {
		apiKey, err := resolveQwenOAuthAPIKey(channel, " sk-sp-replacement ")
		require.NoError(t, err)
		require.Equal(t, "sk-sp-replacement", apiKey)
	})

	t.Run("new flow requires a key", func(t *testing.T) {
		_, err := resolveQwenOAuthAPIKey(nil, "")
		require.Error(t, err)
	})

	t.Run("invalid replacement does not fall back", func(t *testing.T) {
		_, err := resolveQwenOAuthAPIKey(channel, "sk-invalid")
		require.Error(t, err)
	})
}
