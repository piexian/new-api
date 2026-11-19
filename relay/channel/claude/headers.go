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
	// 与 @anthropic-ai/claude-code CLI 同步:CLI 与内嵌 @anthropic-ai/claude-agent-sdk
	// 构建号成对发布(2.1.165 ↔ 0.3.165),当前对齐 2.1.328 ↔ 0.3.328。
	claudeCodeVersion     = "2.1.328"
	claudeAgentSDKVersion = "0.3.328"
	// claudeClientVersion 是 Claude Desktop(desktop_app 平台)版本,与 CLI 发布线无关。
	claudeClientVersion = "1.11187.4"
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
	// 组织/工作区归属可选头;非透传模式默认过滤,由渠道开关 allow_user_context_headers 控制
	"anthropic-user-profile-id",
	"anthropic-workspace-id",
}

func CommonClaudeHeadersOperation(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) {
	if relaycommon.IsRequestPassThroughEnabled(info) {
		clearHeaders(req, claudeCodeFingerprintHeaderNames)
		copyIncomingHeaders(c, req, claudeCodeFingerprintHeaderNames)
		return
	}

	applyClaudeCodeFingerprint(req)
	copyIncomingHeaders(c, req, []string{"anthropic-beta", "X-Claude-Code-Session-Id"})
	// 归属可选头默认不透传,渠道开启 allow_user_context_headers 后放行
	if info != nil && info.ChannelMeta != nil && info.ChannelMeta.ChannelOtherSettings.AllowUserContextHeaders {
		copyIncomingHeaders(c, req, []string{"anthropic-user-profile-id", "anthropic-workspace-id"})
	}
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
	// agent-sdk 0.3.328 内嵌客户端仍 pin 2023-06-01(独立 @anthropic-ai/sdk 1.10.0 已发
	// 2026-10-21,但 CLI 不用它),指纹与真实 CLI 保持一致故不升级。
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
