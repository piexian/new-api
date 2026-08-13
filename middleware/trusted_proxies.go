package middleware

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var defaultTrustedProxyCIDRs = []string{
	"127.0.0.0/8",
	"::1",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"fc00::/7",
}

// ConfigureTrustedProxies controls which direct peers may supply client IP
// forwarding headers used by Gin's ClientIP method.
func ConfigureTrustedProxies(engine *gin.Engine) error {
	// 优先读 EO-Connecting-IP：EdgeOne 回源经 CF 隧道时，XFF 最右侧是 EO 节点 IP，
	// 真实客户端 IP 只在 EO-Connecting-IP 里；直连 CF 的流量无此头，自动回退 XFF
	engine.RemoteIPHeaders = []string{"EO-Connecting-IP", "X-Forwarded-For", "X-Real-IP"}

	rawTrustedProxies := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if rawTrustedProxies == "" {
		log.Print("WARNING: TRUSTED_PROXIES is unset or blank; trusting loopback, RFC 1918, and IPv6 ULA proxy addresses for compatibility. Set TRUSTED_PROXIES=none to trust no proxies, or configure explicit proxy IPs/CIDRs to replace these defaults.")
		return engine.SetTrustedProxies(defaultTrustedProxyCIDRs)
	}

	if strings.EqualFold(rawTrustedProxies, "none") {
		return engine.SetTrustedProxies(nil)
	}

	parts := strings.Split(rawTrustedProxies, ",")
	trustedProxies := make([]string, 0, len(parts))
	for _, part := range parts {
		trustedProxy := strings.TrimSpace(part)
		if trustedProxy == "" {
			continue
		}
		if strings.EqualFold(trustedProxy, "none") {
			return errors.New("TRUSTED_PROXIES=none must be used alone")
		}
		trustedProxies = append(trustedProxies, trustedProxy)
	}

	if len(trustedProxies) == 0 {
		return errors.New("TRUSTED_PROXIES does not contain an IP address or CIDR")
	}
	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		return fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}
	return nil
}
