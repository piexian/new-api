package claude

import (
	"net/http"
	"runtime"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	claudeCodeVersion     = "2.1.165"
	claudeAgentSDKVersion = "0.3.165"
	claudeClientVersion   = "1.11187.4"
)

var claudeCodeFingerprintHeaderNames = []string{
	"User-Agent",
	"X-Stainless-Retry-Count",
	"X-Stainless-Runtime-Version",
	"X-Stainless-Package-Version",
	"X-Stainless-Runtime",
	"X-Stainless-Lang",
	"X-Stainless-Arch",
	"X-Stainless-OS",
	"X-Stainless-Timeout",
	"anthropic-client-platform",
	"anthropic-client-version",
	"anthropic-dangerous-direct-browser-access",
	"x-app",
	"anthropic-version",
	"anthropic-beta",
	"X-Claude-Code-Session-Id",
	"x-client-request-id",
}

func CommonClaudeHeadersOperation(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) {
	if relaycommon.IsRequestPassThroughEnabled(info) {
		clearHeaders(req, claudeCodeFingerprintHeaderNames)
		copyIncomingHeaders(c, req, claudeCodeFingerprintHeaderNames)
		return
	}

	applyClaudeCodeFingerprint(req)
	copyIncomingHeaders(c, req, []string{"anthropic-beta", "X-Claude-Code-Session-Id"})
	originModel := ""
	if info != nil {
		originModel = info.OriginModelName
	}
	model_setting.GetClaudeSettings().WriteHeaders(originModel, req)
}

func applyClaudeCodeFingerprint(req *http.Header) {
	// Keep transport-managed Accept-Encoding/Connection out of the fingerprint;
	// Go's default transport cannot decode every encoding advertised by OMP.
	req.Set("User-Agent", "claude-cli/"+claudeCodeVersion+" (external, local-agent, agent-sdk/"+claudeAgentSDKVersion+")")
	req.Set("X-Stainless-Retry-Count", "0")
	req.Set("X-Stainless-Runtime-Version", "v24.3.0")
	req.Set("X-Stainless-Package-Version", "0.94.0")
	req.Set("X-Stainless-Runtime", "node")
	req.Set("X-Stainless-Lang", "js")
	req.Set("X-Stainless-Arch", mapStainlessArch(runtime.GOARCH))
	req.Set("X-Stainless-OS", mapStainlessOS(runtime.GOOS))
	req.Set("X-Stainless-Timeout", "900")
	req.Set("anthropic-client-platform", "desktop_app")
	req.Set("anthropic-client-version", claudeClientVersion)
	req.Set("anthropic-dangerous-direct-browser-access", "true")
	req.Set("x-app", "cli")
	req.Set("anthropic-version", "2023-06-01")
	req.Set("x-client-request-id", uuid.NewString())
}

func copyIncomingHeaders(c *gin.Context, req *http.Header, names []string) {
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

func clearHeaders(req *http.Header, names []string) {
	for _, name := range names {
		req.Del(name)
	}
}

func mapStainlessOS(value string) string {
	switch strings.ToLower(value) {
	case "darwin":
		return "MacOS"
	case "windows", "win32":
		return "Windows"
	case "linux":
		return "Linux"
	case "freebsd":
		return "FreeBSD"
	default:
		return "Other::" + strings.ToLower(value)
	}
}

func mapStainlessArch(value string) string {
	switch strings.ToLower(value) {
	case "amd64", "x64":
		return "x64"
	case "arm64", "aarch64":
		return "arm64"
	case "386", "x86", "ia32":
		return "x86"
	default:
		return "other::" + strings.ToLower(value)
	}
}
