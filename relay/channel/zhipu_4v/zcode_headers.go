package zhipu_4v

import (
	"net/http"

	"github.com/google/uuid"
)

// ZCode exposes request-scoped tracing headers, but its client/device
// fingerprint is not required for the Coding Plan API and must not be forged.
func setupZCodeCompatibilityHeaders(req *http.Header) {
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
