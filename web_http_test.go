package main

import (
	"crypto/sha256"
	"errors"
	"io/fs"
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
	assertNoStoreResponseHeaders(t, headRecorder.Header())
	assertHeaderContainsToken(t, headRecorder.Header(), "Vary", "Cookie")

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
	assertNoStoreResponseHeaders(t, legacyCSSRecorder.Header())

	logoRecorder := httptest.NewRecorder()
	logoRequest := httptest.NewRequest(http.MethodGet, "/logo.png", nil)
	logoRequest.AddCookie(&http.Cookie{Name: common.FrontendThemeCookieName, Value: common.FrontendThemeDefault})
	engine.ServeHTTP(logoRecorder, logoRequest)
	require.Equal(t, http.StatusOK, logoRecorder.Code)
	assert.Contains(t, logoRecorder.Header().Get("Cache-Control"), "max-age=3600")
	assert.NotContains(t, logoRecorder.Header().Get("Cache-Control"), "immutable")

	staticDirectoryRecorder := httptest.NewRecorder()
	staticDirectoryRequest := httptest.NewRequest(http.MethodGet, "/static/", nil)
	staticDirectoryRequest.AddCookie(&http.Cookie{Name: common.FrontendThemeCookieName, Value: common.FrontendThemeDefault})
	engine.ServeHTTP(staticDirectoryRecorder, staticDirectoryRequest)
	require.Equal(t, http.StatusNotFound, staticDirectoryRecorder.Code)
	assertNoStoreResponseHeaders(t, staticDirectoryRecorder.Header())
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
				assertNoStoreResponseHeaders(t, recorder.Header())
				assertHeaderContainsToken(t, recorder.Header(), "Vary", "Cookie")
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
					assert.Contains(t, recorder.Header().Get("Cache-Control"), "max-age=31536000", path)
					assert.Contains(t, recorder.Header().Get("Cache-Control"), "immutable", path)
					assert.Contains(t, recorder.Header().Get("CDN-Cache-Control"), "immutable", path)
					if method == http.MethodHead {
						assert.Empty(t, recorder.Body.String(), path)
						assert.Empty(t, recorder.Header().Get("Content-Encoding"), path)
					}
				}
			}
		})
	}
}

func TestEmbeddedWebStaticAssetsFallbackAcrossThemes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := newEmbeddedWebEngine()

	tests := []struct {
		name         string
		assetPath    string
		requestTheme string
	}{
		{
			name: "default asset with classic cookie",
			assetPath: findThemeOnlyAsyncAsset(
				t,
				buildFS,
				"web/default/dist",
				classicBuildFS,
				"web/classic/dist",
			),
			requestTheme: common.FrontendThemeClassic,
		},
		{
			name: "classic asset with default cookie",
			assetPath: findThemeOnlyAsyncAsset(
				t,
				classicBuildFS,
				"web/classic/dist",
				buildFS,
				"web/default/dist",
			),
			requestTheme: common.FrontendThemeDefault,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, test.assetPath, nil)
			request.AddCookie(&http.Cookie{Name: common.FrontendThemeCookieName, Value: test.requestTheme})
			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code, test.assetPath)
			assert.Contains(t, recorder.Header().Get("Content-Type"), "javascript")
			assert.Contains(t, recorder.Header().Get("Cache-Control"), "immutable")
		})
	}
}

func TestEmbeddedWebStaticAssetsAreSafeForSharedCache(t *testing.T) {
	assertStaticAssetsAreContentAddressed(t, buildFS, "web/default/dist")
	assertStaticAssetsAreContentAddressed(t, classicBuildFS, "web/classic/dist")
	assertSharedStaticAssetsMatch(t)
}

func assertStaticAssetsAreContentAddressed(t *testing.T, themeFS fs.FS, root string) {
	t.Helper()
	hashPattern := regexp.MustCompile(`\.[0-9a-f]{8,}\.`)
	err := fs.WalkDir(themeFS, root+"/static", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		assert.Regexp(t, hashPattern, strings.TrimPrefix(path, root), path)
		return nil
	})
	require.NoError(t, err)
}

func assertSharedStaticAssetsMatch(t *testing.T) {
	t.Helper()
	defaultRoot := "web/default/dist"
	classicRoot := "web/classic/dist"
	err := fs.WalkDir(buildFS, defaultRoot+"/static", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relativePath := strings.TrimPrefix(path, defaultRoot)
		classicPath := classicRoot + relativePath
		classicData, classicErr := fs.ReadFile(classicBuildFS, classicPath)
		if errors.Is(classicErr, fs.ErrNotExist) {
			return nil
		}
		if classicErr != nil {
			return classicErr
		}
		defaultData, defaultErr := fs.ReadFile(buildFS, path)
		if defaultErr != nil {
			return defaultErr
		}
		assert.Equal(t, sha256.Sum256(defaultData), sha256.Sum256(classicData), relativePath)
		return nil
	})
	require.NoError(t, err)
}

func findThemeOnlyAsyncAsset(
	t *testing.T,
	sourceFS fs.FS,
	sourceRoot string,
	otherFS fs.FS,
	otherRoot string,
) string {
	t.Helper()
	searchRoot := sourceRoot + "/static/js/async"
	assetPath := ""
	err := fs.WalkDir(sourceFS, searchRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if assetPath != "" || entry.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}
		relativePath := strings.TrimPrefix(path, sourceRoot)
		if _, statErr := fs.Stat(otherFS, otherRoot+relativePath); statErr != nil {
			if !errors.Is(statErr, fs.ErrNotExist) {
				return statErr
			}
			assetPath = relativePath
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, assetPath)
	return assetPath
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
