package claude

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestSetupRequestHeaderAppliesOMPClaudeCodeFingerprint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("anthropic-beta", "client-beta")
	c.Request.Header.Set("X-Claude-Code-Session-Id", "session-123")
	c.Request.Header.Set("User-Agent", "client-agent")

	headers := make(http.Header)
	err := (&Adaptor{}).SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatClaude,
		OriginModelName: "claude-opus-4-6",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "claude-key",
		},
	})
	if err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	wantHeaders := map[string]string{
		"x-api-key":                                 "claude-key",
		"X-Stainless-Retry-Count":                   "0",
		"X-Stainless-Runtime-Version":               "v24.3.0",
		"X-Stainless-Package-Version":               "0.94.0",
		"X-Stainless-Runtime":                       "node",
		"X-Stainless-Lang":                          "js",
		"X-Stainless-Timeout":                       "900",
		"anthropic-client-platform":                 "desktop_app",
		"anthropic-client-version":                  "1.11187.4",
		"anthropic-dangerous-direct-browser-access": "true",
		"anthropic-version":                         "2023-06-01",
		"anthropic-beta":                            "client-beta",
		"X-Claude-Code-Session-Id":                  "session-123",
		"x-app":                                     "cli",
	}
	for name, want := range wantHeaders {
		if got := headers.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if got := headers.Get("User-Agent"); !strings.HasPrefix(got, "claude-cli/2.1.328 (external, local-agent, agent-sdk/0.3.328)") {
		t.Errorf("User-Agent = %q, want OMP Claude Code fingerprint", got)
	}
	for _, name := range []string{"X-Stainless-Arch", "X-Stainless-OS", "x-client-request-id"} {
		if headers.Get(name) == "" {
			t.Errorf("%s should not be empty", name)
		}
	}
}

func TestSetupRequestHeaderUserContextHeadersFollowChannelSwitch(t *testing.T) {
	newContext := func() *gin.Context {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		c.Request.Header.Set("anthropic-user-profile-id", "uprof-123")
		c.Request.Header.Set("anthropic-workspace-id", "wrkspc-456")
		return c
	}

	build := func(otherSettings dto.ChannelOtherSettings) http.Header {
		headers := make(http.Header)
		err := (&Adaptor{}).SetupRequestHeader(newContext(), &headers, &relaycommon.RelayInfo{
			RelayMode:       relayconstant.RelayModeChatCompletions,
			RelayFormat:     types.RelayFormatClaude,
			OriginModelName: "claude-opus-5-5",
			ChannelMeta: &relaycommon.ChannelMeta{
				ApiKey:               "claude-key",
				ChannelOtherSettings: otherSettings,
			},
		})
		if err != nil {
			t.Fatalf("SetupRequestHeader returned error: %v", err)
		}
		return headers
	}

	if got := build(dto.ChannelOtherSettings{}).Get("anthropic-user-profile-id"); got != "" {
		t.Errorf("anthropic-user-profile-id = %q, want filtered by default", got)
	}
	if got := build(dto.ChannelOtherSettings{}).Get("anthropic-workspace-id"); got != "" {
		t.Errorf("anthropic-workspace-id = %q, want filtered by default", got)
	}

	allowed := build(dto.ChannelOtherSettings{AllowUserContextHeaders: true})
	if got := allowed.Get("anthropic-user-profile-id"); got != "uprof-123" {
		t.Errorf("anthropic-user-profile-id = %q, want forwarded", got)
	}
	if got := allowed.Get("anthropic-workspace-id"); got != "wrkspc-456" {
		t.Errorf("anthropic-workspace-id = %q, want forwarded", got)
	}
}

func TestSetupRequestHeaderPassThroughCopiesClientClaudeHeadersWithoutFingerprint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "custom-claude-client")
	c.Request.Header.Set("X-Stainless-Arch", "custom-arch")
	c.Request.Header.Set("anthropic-version", "2099-01-01")
	c.Request.Header.Set("x-client-request-id", "request-123")

	headers := make(http.Header)
	err := (&Adaptor{}).SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatClaude,
		OriginModelName: "claude-opus-4-6",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         "claude-key",
			ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: true},
		},
	})
	if err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	wantHeaders := map[string]string{
		"x-api-key":           "claude-key",
		"User-Agent":          "custom-claude-client",
		"X-Stainless-Arch":    "custom-arch",
		"anthropic-version":   "2099-01-01",
		"x-client-request-id": "request-123",
	}
	for name, want := range wantHeaders {
		if got := headers.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"X-Stainless-Runtime", "anthropic-client-platform", "x-app"} {
		if got := headers.Get(name); got != "" {
			t.Errorf("%s = %q, want no fixed pass-through value", name, got)
		}
	}
}
