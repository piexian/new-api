package router

import (
	"embed"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
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

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(serveThemeStatic(defaultFS, classicFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if shouldReturnRelayNotFound(c.Request.RequestURI) {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		if getRequestFrontendTheme(c) == common.FrontendThemeClassic {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.ClassicIndexPage)
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
		}
	})
}

func serveThemeStatic(defaultFS, classicFS static.ServeFileSystem) gin.HandlerFunc {
	defaultServer := http.StripPrefix("/", http.FileServer(defaultFS))
	classicServer := http.StripPrefix("/", http.FileServer(classicFS))

	return func(c *gin.Context) {
		if c.Request.URL.Path == "/" {
			return
		}

		fs, server := selectThemeStatic(c, defaultFS, classicFS, defaultServer, classicServer)
		if !fs.Exists("/", c.Request.URL.Path) {
			return
		}

		server.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func selectThemeStatic(
	c *gin.Context,
	defaultFS static.ServeFileSystem,
	classicFS static.ServeFileSystem,
	defaultServer http.Handler,
	classicServer http.Handler,
) (static.ServeFileSystem, http.Handler) {
	if getRequestFrontendTheme(c) == common.FrontendThemeClassic {
		return classicFS, classicServer
	}
	return defaultFS, defaultServer
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
