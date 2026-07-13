package middleware

import (
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const CSPNoncePlaceholder = "__NEW_API_CSP_NONCE__"

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", "default-src 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'none'")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("X-XSS-Protection", "0")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "accelerometer=(), browsing-topics=(), geolocation=(), gyroscope=(), magnetometer=(), usb=()")
		c.Next()
	}
}

func SetContentSecurityPolicy(c *gin.Context, nonce string) {
	c.Header("Content-Security-Policy", contentSecurityPolicy(nonce))
}

func contentSecurityPolicy(nonce string) string {
	scriptSources := []string{
		"'self'",
		"'nonce-" + nonce + "'",
		"'unsafe-eval'",
		"https://analytics.umami.is",
		"https://challenges.cloudflare.com",
		"https://static.cloudflareinsights.com",
		"https://www.googletagmanager.com",
	}
	connectSources := []string{"'self'", "https:", "wss:"}
	if origin := configuredUmamiOrigin(); origin != "" {
		scriptSources = appendUnique(scriptSources, origin)
		connectSources = appendUnique(connectSources, origin)
	}

	return strings.Join([]string{
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'self'",
		"form-action 'self' https:",
		"script-src " + strings.Join(scriptSources, " "),
		"script-src-attr 'none'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob: http: https:",
		"font-src 'self' data: https:",
		"connect-src " + strings.Join(connectSources, " "),
		"frame-src 'self' http: https:",
		"media-src 'self' data: blob: http: https:",
		"worker-src 'self' blob:",
		"manifest-src 'self'",
	}, "; ")
}

func configuredUmamiOrigin() string {
	rawURL := strings.TrimSpace(os.Getenv("UMAMI_SCRIPT_URL"))
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
