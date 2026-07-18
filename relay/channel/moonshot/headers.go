package moonshot

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Keep this aligned with the Kimi Code release used when building the service.
// Source: MoonshotAI/kimi-code tag @moonshot-ai/kimi-code@0.27.0.
const kimiCodeCLICompatibilityVersion = "0.27.0"

var (
	kimiCLIHeadersOnce sync.Once
	kimiCLIHeaders     map[string]string
)

var kimiCLIHeaderNames = []string{
	"User-Agent",
	"X-Msh-Platform",
	"X-Msh-Version",
	"X-Msh-Device-Name",
	"X-Msh-Device-Model",
	"X-Msh-Os-Version",
	"X-Msh-Device-Id",
}

var kimiCodingAnthropicHeaderNames = []string{
	"anthropic-version",
	"anthropic-beta",
	"anthropic-dangerous-direct-browser-access",
	"x-app",
}

func setupKimiCodingHeaders(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) {
	if relaycommon.IsRequestPassThroughEnabled(info) {
		clearMoonshotHeaders(req, kimiCLIHeaderNames)
		clearMoonshotHeaders(req, kimiCodingAnthropicHeaderNames)
		copyIncomingMoonshotHeaders(c, req, kimiCLIHeaderNames)
		if info != nil && info.RelayFormat == types.RelayFormatClaude {
			claude.CommonClaudeHeadersOperation(c, req, info)
		}
		return
	}

	for name, value := range getKimiCLIHeaders() {
		req.Set(name, value)
	}
	if shouldUseKimiCodingClaudeEndpoint(info) {
		// The official CLI also adds transport headers here; net/http owns those in this relay.
		req.Set("anthropic-version", "2023-06-01")
		req.Set("anthropic-dangerous-direct-browser-access", "true")
		req.Set("x-app", "cli")
		copyIncomingMoonshotHeaders(c, req, []string{"anthropic-beta"})
	}
}

func getKimiCLIHeaders() map[string]string {
	kimiCLIHeadersOnce.Do(func() {
		hostname, _ := os.Hostname()
		hostname = sanitizeKimiHeaderValue(hostname, "unknown")
		osRelease := kimiOSRelease()
		deviceModel := fmt.Sprintf("%s %s %s", kimiOSLabel(), osRelease, kimiNodeArch(runtime.GOARCH))
		deviceModel = sanitizeKimiHeaderValue(deviceModel, "unknown")

		kimiCLIHeaders = map[string]string{
			"User-Agent":         "kimi-code-cli/" + kimiCodeCLICompatibilityVersion,
			"X-Msh-Platform":     "kimi_code_cli",
			"X-Msh-Version":      kimiCodeCLICompatibilityVersion,
			"X-Msh-Device-Name":  hostname,
			"X-Msh-Device-Model": deviceModel,
			"X-Msh-Os-Version":   osRelease,
			"X-Msh-Device-Id":    localKimiDeviceID(hostname, deviceModel),
		}
	})
	return kimiCLIHeaders
}

func kimiOSRelease() string {
	if runtime.GOOS == "linux" {
		if value, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
			return sanitizeKimiHeaderValue(string(value), "unknown")
		}
	}
	return sanitizeKimiHeaderValue(runtime.GOOS, "unknown")
}

func localKimiDeviceID(hostname string, deviceModel string) string {
	homeDir := strings.TrimSpace(os.Getenv("KIMI_CODE_HOME"))
	if homeDir == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			homeDir = filepath.Join(userHome, ".kimi-code")
		}
	}
	if homeDir != "" {
		if value, err := os.ReadFile(filepath.Join(homeDir, "device_id")); err == nil {
			if deviceID := sanitizeKimiHeaderValue(string(value), ""); deviceID != "" {
				return deviceID
			}
		}
	}

	deviceHash := sha256.Sum256([]byte(hostname + "\x00" + deviceModel))
	deviceBytes := append([]byte(nil), deviceHash[:16]...)
	deviceBytes[6] = (deviceBytes[6] & 0x0f) | 0x40
	deviceBytes[8] = (deviceBytes[8] & 0x3f) | 0x80
	deviceID, err := uuid.FromBytes(deviceBytes)
	if err != nil {
		return fmt.Sprintf("%x", deviceHash[:16])
	}
	return deviceID.String()
}

func kimiOSLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}

func kimiNodeArch(value string) string {
	switch value {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return value
	}
}

func sanitizeKimiHeaderValue(value string, fallback string) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func copyIncomingMoonshotHeaders(c *gin.Context, req *http.Header, names []string) {
	if c == nil || c.Request == nil {
		return
	}
	for _, name := range names {
		values := c.Request.Header.Values(name)
		if len(values) == 0 {
			continue
		}
		req.Del(name)
		for _, value := range values {
			req.Add(name, value)
		}
	}
}

func clearMoonshotHeaders(req *http.Header, names []string) {
	for _, name := range names {
		req.Del(name)
	}
}
