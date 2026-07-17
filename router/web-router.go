package router

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// ThemeAssets holds the embedded frontend assets for both themes.
type ThemeAssets struct {
	DefaultBuildFS   embed.FS
	DefaultIndexPage []byte
	ClassicBuildFS   embed.FS
	ClassicIndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")
	classicFS := common.EmbedFolder(assets.ClassicBuildFS, "web/classic/dist")

	router.Use(middleware.Gzip())
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(serveThemeStatic(defaultFS, classicFS))
	registerCrawlerRoutes(router)
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if shouldReturnRelayNotFound(c.Request.URL.Path) {
			middleware.SetNoStoreHeaders(c)
			controller.RelayNotFound(c)
			return
		}
		if getRequestFrontendTheme(c) == common.FrontendThemeClassic {
			serveIndexPage(c, assets.ClassicIndexPage)
		} else {
			serveIndexPage(c, assets.DefaultIndexPage)
		}
	})
}

func serveThemeStatic(defaultFS, classicFS static.ServeFileSystem) gin.HandlerFunc {
	defaultServer := http.StripPrefix("/", http.FileServer(defaultFS))
	classicServer := http.StripPrefix("/", http.FileServer(classicFS))

	return func(c *gin.Context) {
		switch c.Request.URL.Path {
		case "/", "/index.html", "/robots.txt", "/sitemap.xml":
			return
		}

		if getRequestFrontendTheme(c) == common.FrontendThemeClassic {
			if !tryServeStatic(c, classicFS, classicServer) {
				tryServeStatic(c, defaultFS, defaultServer)
			}
			return
		}
		if !tryServeStatic(c, defaultFS, defaultServer) {
			tryServeStatic(c, classicFS, classicServer)
		}
	}
}

func tryServeStatic(c *gin.Context, fs static.ServeFileSystem, server http.Handler) bool {
	file, err := fs.Open(c.Request.URL.Path)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	middleware.SetStaticCacheHeaders(c, c.Request.URL.Path)
	server.ServeHTTP(c.Writer, c.Request)
	c.Abort()
	return true
}

func serveIndexPage(c *gin.Context, indexPage []byte) {
	middleware.SetNoStoreHeaders(c)
	middleware.AppendVaryHeader(c, "Cookie")
	nonce, err := generateCSPNonce()
	if err != nil {
		common.SysError("failed to generate CSP nonce: " + err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	body := bytes.ReplaceAll(indexPage, []byte(middleware.CSPNoncePlaceholder), []byte(nonce))
	c.Header("Content-Type", "text/html; charset=utf-8")
	middleware.SetContentSecurityPolicy(c, nonce)
	if c.Request.Method == http.MethodHead {
		c.Header("Content-Length", strconv.Itoa(len(body)))
		c.Status(http.StatusOK)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", body)
}

func generateCSPNonce() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(value), nil
}

func registerCrawlerRoutes(router *gin.Engine) {
	router.Match([]string{http.MethodGet, http.MethodHead}, "/robots.txt", func(c *gin.Context) {
		body := []byte("User-agent: *\nDisallow: /api/\nDisallow: /v1/\nDisallow: /console\nDisallow: /dashboard\n")
		serveCrawlerContent(c, "text/plain; charset=utf-8", body)
	})
	router.Match([]string{http.MethodGet, http.MethodHead}, "/sitemap.xml", func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		location := html.EscapeString(scheme + "://" + c.Request.Host + "/")
		body := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
			"<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">" +
			"<url><loc>" + location + "</loc></url></urlset>\n")
		serveCrawlerContent(c, "application/xml; charset=utf-8", body)
	})
}

func serveCrawlerContent(c *gin.Context, contentType string, body []byte) {
	middleware.SetCacheControlHeaders(c, "public, max-age=3600")
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.Itoa(len(body)))
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.Data(http.StatusOK, contentType, body)
}

func getRequestFrontendTheme(c *gin.Context) string {
	theme, err := c.Cookie(common.FrontendThemeCookieName)
	if err == nil && (theme == common.FrontendThemeDefault || theme == common.FrontendThemeClassic) {
		return theme
	}
	return common.NormalizeFrontendTheme(common.GetTheme())
}

func shouldReturnRelayNotFound(requestURI string) bool {
	return strings.HasPrefix(requestURI, "/v1") ||
		strings.HasPrefix(requestURI, "/api") ||
		strings.HasPrefix(requestURI, "/assets") ||
		strings.HasPrefix(requestURI, "/static")
}
