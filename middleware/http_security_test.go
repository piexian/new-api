package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzipSkipsHeadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Gzip())
	engine.GET("/dashboard", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("a", 128))
	})
	engine.HEAD("/dashboard", func(c *gin.Context) {
		c.Header("Content-Length", "128")
		c.Status(http.StatusOK)
	})

	headRecorder := httptest.NewRecorder()
	headRequest := httptest.NewRequest(http.MethodHead, "/dashboard", nil)
	headRequest.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(headRecorder, headRequest)
	require.Equal(t, http.StatusOK, headRecorder.Code)
	assert.Empty(t, headRecorder.Header().Get("Content-Encoding"))
	assert.Empty(t, headRecorder.Body.String())

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	getRequest.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(getRecorder, getRequest)
	require.Equal(t, http.StatusOK, getRecorder.Code)
	assert.Equal(t, "gzip", getRecorder.Header().Get("Content-Encoding"))
}

func TestCORSDoesNotAllowCredentialedWildcardRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORS())
	engine.POST("/v1/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/v1/test", nil)
	request.Header.Set("Origin", "https://example.net")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "authorization")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodPost)
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(SecurityHeaders())
	engine.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Security-Policy"), "default-src 'none'")
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "SAMEORIGIN", recorder.Header().Get("X-Frame-Options"))
	assert.Equal(t, "0", recorder.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", recorder.Header().Get("Referrer-Policy"))
	assert.NotEmpty(t, recorder.Header().Get("Permissions-Policy"))
}

func TestContentSecurityPolicyUsesNonceAndConfiguredAnalyticsOrigin(t *testing.T) {
	t.Setenv("UMAMI_SCRIPT_URL", "https://stats.example.com/custom/script.js")
	policy := contentSecurityPolicy("test-nonce")

	assert.Contains(t, policy, "'nonce-test-nonce'")
	assert.Contains(t, policy, "https://stats.example.com")
	assert.Contains(t, policy, "https://challenges.cloudflare.com")
	assert.Contains(t, policy, "object-src 'none'")
	for _, directive := range strings.Split(policy, ";") {
		directive = strings.TrimSpace(directive)
		if strings.HasPrefix(directive, "script-src ") {
			assert.NotContains(t, directive, "'unsafe-inline'")
		}
	}
}
