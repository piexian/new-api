package main

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/router"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedWebRoutesAndAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := newEmbeddedWebEngine()

	headRecorder := httptest.NewRecorder()
	headRequest := httptest.NewRequest(http.MethodHead, "/dashboard", nil)
	headRequest.Header.Set("Accept-Encoding", "gzip")
	headRequest.AddCookie(&http.Cookie{Name: common.FrontendThemeCookieName, Value: common.FrontendThemeDefault})
	engine.ServeHTTP(headRecorder, headRequest)
	require.Equal(t, http.StatusOK, headRecorder.Code)
	assert.Empty(t, headRecorder.Body.String())
	assert.Empty(t, headRecorder.Header().Get("Content-Encoding"))
	assert.Contains(t, headRecorder.Header().Get("Content-Security-Policy"), "'nonce-")

	robotsRecorder := httptest.NewRecorder()
	engine.ServeHTTP(robotsRecorder, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	require.Equal(t, http.StatusOK, robotsRecorder.Code)
	assert.Contains(t, robotsRecorder.Header().Get("Content-Type"), "text/plain")
	assert.NotContains(t, strings.ToLower(robotsRecorder.Body.String()), "<!doctype html>")

	sitemapRecorder := httptest.NewRecorder()
	sitemapRequest := httptest.NewRequest(http.MethodGet, "http://api.example.com/sitemap.xml", nil)
	sitemapRequest.Header.Set("X-Forwarded-Proto", "https")
	engine.ServeHTTP(sitemapRecorder, sitemapRequest)
	require.Equal(t, http.StatusOK, sitemapRecorder.Code)
	assert.Contains(t, sitemapRecorder.Header().Get("Content-Type"), "application/xml")
	assert.Contains(t, sitemapRecorder.Body.String(), "https://api.example.com/")

	cssPattern := regexp.MustCompile(`href="(/static/css/[^"]+\.css)"`)
	cssMatches := cssPattern.FindAllSubmatch(indexPage, -1)
	require.NotEmpty(t, cssMatches)
	for _, match := range cssMatches {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, string(match[1]), nil)
		request.AddCookie(&http.Cookie{Name: common.FrontendThemeCookieName, Value: common.FrontendThemeDefault})
		engine.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code, string(match[1]))
	}

	legacyCSSRecorder := httptest.NewRecorder()
	legacyCSSRequest := httptest.NewRequest(http.MethodGet, "/static/css/main.css", nil)
	legacyCSSRequest.AddCookie(&http.Cookie{Name: common.FrontendThemeCookieName, Value: common.FrontendThemeDefault})
	engine.ServeHTTP(legacyCSSRecorder, legacyCSSRequest)
	require.Equal(t, http.StatusNotFound, legacyCSSRecorder.Code)
	assert.Contains(t, legacyCSSRecorder.Body.String(), "The requested resource was not found")
	assert.NotContains(t, legacyCSSRecorder.Body.String(), "/static/css/main.css")
}

func TestEmbeddedWebHeadCoversFrontendRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := newEmbeddedWebEngine()

	tests := []struct {
		name   string
		theme  string
		routes []string
	}{
		{
			name:  "default",
			theme: common.FrontendThemeDefault,
			routes: []string{
				"/", "/setup/", "/rankings/", "/pricing/", "/pricing/test-model/", "/about/",
				"/user-agreement", "/privacy-policy", "/oauth/test-provider", "/console/topup", "/console/log",
				"/chat2link", "/401", "/403", "/404", "/500", "/503", "/sign-up", "/sign-in",
				"/reset", "/register", "/otp", "/oauth", "/forgot-password", "/user/reset",
				"/wallet/", "/users/", "/usage-logs/", "/usage-logs/all", "/system-info/", "/subscriptions/",
				"/redemption-codes/", "/profile/", "/playground/", "/models/", "/models/all", "/keys/",
				"/ip-bans/", "/invite-rewards/", "/dashboard/", "/dashboard/usage", "/channels/",
				"/errors/example", "/chat/test-chat", "/system-settings", "/system-settings/",
				"/system-settings/site/", "/system-settings/site/general", "/system-settings/security/",
				"/system-settings/security/general", "/system-settings/operations/",
				"/system-settings/operations/general", "/system-settings/models/",
				"/system-settings/models/general", "/system-settings/content/",
				"/system-settings/content/general", "/system-settings/billing/",
				"/system-settings/billing/general", "/system-settings/auth/", "/system-settings/auth/general",
			},
		},
		{
			name:  "classic",
			theme: common.FrontendThemeClassic,
			routes: []string{
				"/", "/setup", "/forbidden", "/console/models", "/console/deployment", "/console/subscription",
				"/console/channel", "/console/token", "/console/playground", "/console/redemption", "/console/user",
				"/console/ip_ban", "/user/reset", "/login", "/register", "/reset", "/oauth/github",
				"/oauth/discord", "/oauth/oidc", "/oauth/linuxdo", "/oauth/test-provider", "/console/setting",
				"/console/personal", "/console/topup", "/console/invite", "/console/log", "/console/email-log",
				"/console", "/console/midjourney", "/console/task", "/pricing", "/rankings", "/about",
				"/user-agreement", "/privacy-policy", "/console/chat", "/console/chat/test-chat", "/chat2link",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, path := range test.routes {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodHead, path, nil)
				request.Header.Set("Accept-Encoding", "gzip")
				request.AddCookie(&http.Cookie{Name: common.FrontendThemeCookieName, Value: test.theme})
				engine.ServeHTTP(recorder, request)

				assert.Equal(t, http.StatusOK, recorder.Code, path)
				assert.Empty(t, recorder.Body.String(), path)
				assert.Empty(t, recorder.Header().Get("Content-Encoding"), path)
				assert.Contains(t, recorder.Header().Get("Content-Type"), "text/html", path)
				assert.Contains(t, recorder.Header().Get("Content-Security-Policy"), "'nonce-", path)
			}
		})
	}
}

func TestEmbeddedWebReferencedStaticAssetsSupportGetAndHead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := newEmbeddedWebEngine()
	assetPattern := regexp.MustCompile(`(?:href|src)="(/static/[^"]+)"`)

	tests := []struct {
		name  string
		theme string
		index []byte
	}{
		{name: "default", theme: common.FrontendThemeDefault, index: indexPage},
		{name: "classic", theme: common.FrontendThemeClassic, index: classicIndexPage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matches := assetPattern.FindAllSubmatch(test.index, -1)
			require.NotEmpty(t, matches)
			for _, match := range matches {
				path := string(match[1])
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					recorder := httptest.NewRecorder()
					request := httptest.NewRequest(method, path, nil)
					request.Header.Set("Accept-Encoding", "gzip")
					request.AddCookie(&http.Cookie{Name: common.FrontendThemeCookieName, Value: test.theme})
					engine.ServeHTTP(recorder, request)

					assert.Equal(t, http.StatusOK, recorder.Code, "%s %s", method, path)
					if method == http.MethodHead {
						assert.Empty(t, recorder.Body.String(), path)
						assert.Empty(t, recorder.Header().Get("Content-Encoding"), path)
					}
				}
			}
		})
	}
}

func newEmbeddedWebEngine() *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.SecurityHeaders())
	router.SetWebRouter(engine, router.ThemeAssets{
		DefaultBuildFS:   buildFS,
		DefaultIndexPage: indexPage,
		ClassicBuildFS:   classicBuildFS,
		ClassicIndexPage: classicIndexPage,
	})
	return engine
}
