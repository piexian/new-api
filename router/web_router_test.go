package router

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeIndexPageUsesMatchingCSPNonce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	page := []byte(`<script nonce="` + middleware.CSPNoncePlaceholder + `"></script>`)

	serveIndexPage(ctx, page)

	require.Equal(t, http.StatusOK, recorder.Code)
	policy := recorder.Header().Get("Content-Security-Policy")
	match := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(policy)
	require.Len(t, match, 2)
	assert.Contains(t, recorder.Body.String(), `nonce="`+match[1]+`"`)
	assert.NotContains(t, recorder.Body.String(), middleware.CSPNoncePlaceholder)
	assertNoStoreResponseHeaders(t, recorder.Header())
	assertHeaderContainsToken(t, recorder.Header(), "Vary", "Cookie")
}

func TestServeIndexPageHandlesHeadWithoutBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodHead, "/dashboard", nil)
	page := []byte("<!doctype html><html></html>")

	serveIndexPage(ctx, page)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Body.String())
	assert.Equal(t, strconv.Itoa(len(page)), recorder.Header().Get("Content-Length"))
	assert.NotEmpty(t, recorder.Header().Get("Content-Security-Policy"))
	assertNoStoreResponseHeaders(t, recorder.Header())
	assertHeaderContainsToken(t, recorder.Header(), "Vary", "Cookie")
}

func TestCrawlerRoutesReturnDedicatedContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerCrawlerRoutes(engine)

	robotsRecorder := httptest.NewRecorder()
	robotsRequest := httptest.NewRequest(http.MethodGet, "https://api.example.com/robots.txt", nil)
	engine.ServeHTTP(robotsRecorder, robotsRequest)
	require.Equal(t, http.StatusOK, robotsRecorder.Code)
	assert.Contains(t, robotsRecorder.Header().Get("Content-Type"), "text/plain")
	assert.Equal(t, "public, max-age=3600", robotsRecorder.Header().Get("CDN-Cache-Control"))
	assert.Contains(t, robotsRecorder.Body.String(), "User-agent: *")
	assert.NotContains(t, strings.ToLower(robotsRecorder.Body.String()), "<!doctype html>")

	sitemapRecorder := httptest.NewRecorder()
	sitemapRequest := httptest.NewRequest(http.MethodGet, "http://api.example.com/sitemap.xml", nil)
	sitemapRequest.Header.Set("X-Forwarded-Proto", "https")
	engine.ServeHTTP(sitemapRecorder, sitemapRequest)
	require.Equal(t, http.StatusOK, sitemapRecorder.Code)
	assert.Contains(t, sitemapRecorder.Header().Get("Content-Type"), "application/xml")
	assert.Contains(t, sitemapRecorder.Body.String(), "https://api.example.com/")
	assert.NotContains(t, strings.ToLower(sitemapRecorder.Body.String()), "<!doctype html>")

	headRecorder := httptest.NewRecorder()
	headRequest := httptest.NewRequest(http.MethodHead, "https://api.example.com/sitemap.xml", nil)
	engine.ServeHTTP(headRecorder, headRequest)
	require.Equal(t, http.StatusOK, headRecorder.Code)
	assert.Empty(t, headRecorder.Body.String())
	assert.NotEmpty(t, headRecorder.Header().Get("Content-Length"))
}

func assertNoStoreResponseHeaders(t *testing.T, header http.Header) {
	t.Helper()
	assert.Contains(t, header.Get("Cache-Control"), "no-store")
	assert.Equal(t, "no-store", header.Get("CDN-Cache-Control"))
	assert.Equal(t, "no-store", header.Get("Cloudflare-CDN-Cache-Control"))
	assert.Equal(t, "no-cache", header.Get("Pragma"))
	assert.Equal(t, "0", header.Get("Expires"))
}

func assertHeaderContainsToken(t *testing.T, header http.Header, name, expected string) {
	t.Helper()
	for _, value := range header.Values(name) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), expected) {
				return
			}
		}
	}
	assert.Fail(t, "header token not found", "%s does not contain %s", name, expected)
}
