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

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Keep this aligned with the Kimi Code release used when building the service.
// Source: @moonshot-ai/kimi-code v2.6.2 (npm latest). The X-Msh-* identity header
// scheme is unchanged since 2.0.0; 2.6.x additionally sends X-Msh-Client-Metadata.
const kimiCodeCLICompatibilityVersion = "2.6.2"

// kimiClientMetadataValue 镜像 Kimi Code CLI 2.6.x 普通回合的请求元数据:
// agent_role=main、turn_trigger=turn、step_reason=turn_start。CLI 对缺失的
// user_input_id/parent_request_id 会因 undefined 被 JSON.stringify 省略,
// 网关同样省略这两个字段。
const kimiClientMetadataValue = `{"agent_role":"main","turn_trigger":"turn","step_reason":"turn_start"}`

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
	"X-Msh-Client-Metadata",
}

// kimiCacheAffinityHeaderNames 是唯一允许从客户端透传的缓存亲和请求头；
// 其余客户端头一律过滤，仅渠道 header_override 配置可在框架层覆盖默认值。
var kimiCacheAffinityHeaderNames = []string{
	"X-Claude-Code-Session-Id",
}

func setupKimiCodingHeaders(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) {
	clear(*req)
	req.Set("Content-Type", gin.MIMEJSON)
	req.Set("Accept", gin.MIMEJSON)
	if info != nil {
		req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))
		if info.IsStream {
			req.Set("Accept", "text/event-stream")
		}
	}
	for name, value := range getKimiCLIHeaders() {
		req.Set(name, value)
	}
	copyIncomingMoonshotHeaders(c, req, kimiCacheAffinityHeaderNames)
}

func getKimiCLIHeaders() map[string]string {
	kimiCLIHeadersOnce.Do(func() {
		hostname, _ := os.Hostname()
		hostname = sanitizeKimiHeaderValue(hostname, "unknown")
		osRelease := kimiOSRelease()
		deviceModel := fmt.Sprintf("%s %s %s", kimiOSLabel(), osRelease, kimiNodeArch(runtime.GOARCH))
		deviceModel = sanitizeKimiHeaderValue(deviceModel, "unknown")

		kimiCLIHeaders = map[string]string{
			"User-Agent":            "kimi-code-cli/" + kimiCodeCLICompatibilityVersion,
			"X-Msh-Platform":        "kimi_code_cli",
			"X-Msh-Version":         kimiCodeCLICompatibilityVersion,
			"X-Msh-Device-Name":     hostname,
			"X-Msh-Device-Model":    deviceModel,
			"X-Msh-Os-Version":      osRelease,
			"X-Msh-Device-Id":       localKimiDeviceID(hostname, deviceModel),
			"X-Msh-Client-Metadata": kimiClientMetadataValue,
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
