package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	cacheVersion                = "b688f2fb5be447c25e5aa3bd063087a83db32a288bf6a4f35f2d8db310e40b14"
	noStoreCacheControl         = "private, no-store, no-cache, max-age=0, must-revalidate"
	immutableStaticCacheControl = "public, max-age=31536000, immutable"
	shortStaticCacheControl     = "public, max-age=3600"
)

// Cache attaches build metadata; response handlers set their own cache directives.
func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Header("Cache-Version", cacheVersion)
		c.Next()
	}
}

// SetCacheControlHeaders keeps browser and shared-CDN cache directives aligned.
func SetCacheControlHeaders(c *gin.Context, value string) {
	c.Header("Cache-Control", value)
	c.Header("CDN-Cache-Control", value)
	c.Header("Cloudflare-CDN-Cache-Control", value)
}

// SetNoStoreHeaders prevents personalized or nonce-bearing responses from being reused.
func SetNoStoreHeaders(c *gin.Context) {
	c.Header("Cache-Control", noStoreCacheControl)
	c.Header("CDN-Cache-Control", "no-store")
	c.Header("Cloudflare-CDN-Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

// SetStaticCacheHeaders caches hashed build assets permanently and root assets briefly.
func SetStaticCacheHeaders(c *gin.Context, requestPath string) {
	value := shortStaticCacheControl
	if strings.HasPrefix(requestPath, "/static/") {
		value = immutableStaticCacheControl
	}
	SetCacheControlHeaders(c, value)
}

// AppendVaryHeader adds a Vary token without duplicating an existing value.
func AppendVaryHeader(c *gin.Context, value string) {
	for _, headerValue := range c.Writer.Header().Values("Vary") {
		for _, existing := range strings.Split(headerValue, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), value) {
				return
			}
		}
	}
	c.Writer.Header().Add("Vary", value)
}
