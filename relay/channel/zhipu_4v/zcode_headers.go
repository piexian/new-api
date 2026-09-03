package zhipu_4v

import (
	"net/http"

	"github.com/google/uuid"
)

// ZCode 客户端版本取自 zcode.z.ai /api/v1/client/configs 的 minimalVersion。
const zcodeClientVersion = "3.5.3"

// setupZCodeTraceHeaders 清除 Claude 系客户端指纹头并写入 ZCode 请求级
// tracing 头。Coding Plan 渠道的默认（非 ZCode 模式）行为。
func setupZCodeTraceHeaders(req *http.Header) {
	if req == nil {
		return
	}
	for _, name := range zcodeReplacedClaudeHeaders {
		req.Del(name)
	}
	req.Set("x-request-id", uuid.NewString())
	req.Set("x-zcode-session-type", "main")
	req.Set("x-zcode-trace-id", uuid.NewString())
	req.Set("x-query-id", uuid.NewString())
	req.Set("x-session-id", uuid.NewString())
}

// setupZCodeCompatibilityHeaders 在 tracing 头之上补齐 ZCode 桌面端
// production 设备指纹。GLM Coding Plan 网关按 ZCode 指纹发放客户端权益
// （夜间活动 0 扣费、全天 1.5× 加成），仅在渠道开启 ZCode 模式时使用。
func setupZCodeCompatibilityHeaders(req *http.Header) {
	if req == nil {
		return
	}
	setupZCodeTraceHeaders(req)
	req.Set("User-Agent", "ZCode/"+zcodeClientVersion)
	req.Set("HTTP-Referer", "https://zcode.z.ai")
	req.Set("X-Title", "Z Code@electron")
	req.Set("X-ZCode-App-Version", zcodeClientVersion)
	req.Set("X-Platform", "win32-x64")
	req.Set("X-Release-Channel", "production")
	req.Set("X-Client-Language", "zh-CN")
	req.Set("X-Client-Timezone", "Asia/Shanghai")
	req.Set("X-Os-Category", "windows")
}

var zcodeReplacedClaudeHeaders = []string{
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
	"X-Claude-Code-Session-Id",
	"x-client-request-id",
}
