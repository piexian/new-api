package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConfigureTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		config        string
		remoteAddr    string
		forwardedFor  string
		wantClientIP  string
		wantConfigErr bool
	}{
		{
			name:         "default trusts loopback proxy",
			remoteAddr:   "127.0.0.1:12345",
			forwardedFor: "198.51.100.10",
			wantClientIP: "198.51.100.10",
		},
		{
			name:         "default trusts private proxy",
			remoteAddr:   "172.20.0.2:12345",
			forwardedFor: "198.51.100.11",
			wantClientIP: "198.51.100.11",
		},
		{
			name:         "public peer cannot spoof forwarded header",
			remoteAddr:   "203.0.113.9:12345",
			forwardedFor: "198.51.100.12",
			wantClientIP: "203.0.113.9",
		},
		{
			name:         "forwarded chain stops at first untrusted hop",
			remoteAddr:   "127.0.0.1:12345",
			forwardedFor: "192.0.2.20, 203.0.113.20",
			wantClientIP: "203.0.113.20",
		},
		{
			name:         "none ignores forwarded headers",
			config:       "none",
			remoteAddr:   "127.0.0.1:12345",
			forwardedFor: "198.51.100.13",
			wantClientIP: "127.0.0.1",
		},
		{
			name:         "explicit list replaces defaults",
			config:       "203.0.113.0/24",
			remoteAddr:   "127.0.0.1:12345",
			forwardedFor: "198.51.100.14",
			wantClientIP: "127.0.0.1",
		},
		{
			name:         "cloudflare tunnel through nginx resolves visitor",
			config:       "127.0.0.0/8, 172.20.0.0/16",
			remoteAddr:   "127.0.0.1:12345",
			forwardedFor: "198.51.100.15, 172.20.0.5",
			wantClientIP: "198.51.100.15",
		},
		{
			name:          "none cannot be combined with CIDRs",
			config:        "none,127.0.0.1",
			wantConfigErr: true,
		},
		{
			name:          "empty comma list is rejected",
			config:        " , ",
			wantConfigErr: true,
		},
		{
			name:          "invalid CIDR is rejected",
			config:        "not-an-ip",
			wantConfigErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TRUSTED_PROXIES", tt.config)
			engine := gin.New()
			if err := ConfigureTrustedProxies(engine); (err != nil) != tt.wantConfigErr {
				t.Fatalf("ConfigureTrustedProxies() error = %v, wantConfigErr %v", err, tt.wantConfigErr)
			}
			if tt.wantConfigErr {
				return
			}

			var clientIP string
			engine.GET("/", func(c *gin.Context) {
				clientIP = c.ClientIP()
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}
			engine.ServeHTTP(httptest.NewRecorder(), req)

			if clientIP != tt.wantClientIP {
				t.Fatalf("ClientIP() = %q, want %q", clientIP, tt.wantClientIP)
			}
		})
	}
}
