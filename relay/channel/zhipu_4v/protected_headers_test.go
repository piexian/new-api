package zhipu_4v

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relay/channel"

	"github.com/gin-gonic/gin"
)

func TestBuildUpstreamRequest_ZCodeModeIgnoresPassHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := zcodeTestInfo(1)
	off := false
	info.ChannelSetting.ZcodeClientSigningEnabled = &off
	info.UseRuntimeHeadersOverride = true
	// 等价于亲和性模板 pass_headers 透传的客户端头。
	info.RuntimeHeadersOverride = map[string]any{
		"user-agent":       "zcode/9.9.9",
		"x-session-id":     "client-session",
		"x-stainless-lang": "js",
		"x-custom":         "kept",
	}

	req, err := channel.BuildUpstreamRequest(context.Background(), &Adaptor{}, ctx, info, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("BuildUpstreamRequest: %v", err)
	}
	if ua := req.Header.Get("User-Agent"); !strings.HasPrefix(ua, "ZCode/") {
		t.Fatalf("User-Agent = %q, want ZCode fingerprint", ua)
	}
	if got := req.Header.Get("x-session-id"); got != "" {
		t.Fatalf("x-session-id = %q, want empty", got)
	}
	if got := req.Header.Get("X-Stainless-Lang"); got != "" {
		t.Fatalf("X-Stainless-Lang = %q, want removed", got)
	}
	if got := req.Header.Get("x-custom"); got != "kept" {
		t.Fatalf("x-custom = %q, want kept", got)
	}
}

func TestProtectedRequestHeaders_OnlyInZCodeMode(t *testing.T) {
	info := zcodeTestInfo(1)
	if len((&Adaptor{}).ProtectedRequestHeaders(info)) == 0 {
		t.Fatal("ZCode mode should protect fingerprint headers")
	}
	info.ChannelSetting.ZcodeModeEnabled = false
	if got := (&Adaptor{}).ProtectedRequestHeaders(info); got != nil {
		t.Fatalf("non-ZCode channel should not protect headers, got %v", got)
	}
}
