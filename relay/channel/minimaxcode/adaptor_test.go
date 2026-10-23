package minimaxcode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestParseOAuthKey(t *testing.T) {
	if _, err := ParseOAuthKey(`sk-plain`); err == nil {
		t.Fatal("plain key should be rejected")
	}
	if _, err := ParseOAuthKey(`{"refresh_token":"r"}`); err == nil {
		t.Fatal("missing access_token should be rejected")
	}
	key, err := ParseOAuthKey(`{"access_token":"a","refresh_token":"r","region":"io","group_id":"g1"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Region != "io" || key.GroupID != "g1" {
		t.Fatalf("unexpected key: %+v", key)
	}
}

func TestNormalizeRegion(t *testing.T) {
	cases := map[string]string{
		"":         RegionCN,
		"cn":       RegionCN,
		"CN":       RegionCN,
		"io":       RegionGlobal,
		"global":   RegionGlobal,
		"OVERSEAS": RegionGlobal,
	}
	for input, want := range cases {
		if got := NormalizeRegion(input); got != want {
			t.Fatalf("NormalizeRegion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := map[string]string{
		"":                          DefaultBaseURL,
		"https://agent.minimax.cn/": DefaultBaseURL,
		"https://agent.minimax.io":  "https://agent.minimax.io",
		"https://agent.minimax.io/mavis/api/v1/llm/v1/messages": "https://agent.minimax.io",
		"https://agent.minimax.io/mavis/api/v1/llm/v1":          "https://agent.minimax.io",
	}
	for input, want := range cases {
		if got := NormalizeBaseURL(input); got != want {
			t.Fatalf("NormalizeBaseURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGetRequestURL(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: ""},
	}
	url, err := (&Adaptor{}).GetRequestURL(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != DefaultBaseURL+MessagesPath {
		t.Fatalf("unexpected url: %s", url)
	}

	info.RelayMode = relayconstant.RelayModeClaudeCountTokens
	if _, err := (&Adaptor{}).GetRequestURL(info); err == nil {
		t.Fatal("count_tokens should be rejected")
	}
}

func TestSetupRequestHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	reqHeader := &http.Header{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: `{"access_token":"tok","refresh_token":"r"}`},
	}
	if err := (&Adaptor{}).SetupRequestHeader(c, reqHeader, info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := reqHeader.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := reqHeader.Get("User-Agent"); got != UserAgent {
		t.Fatalf("User-Agent = %q", got)
	}
	for _, name := range []string{"X-Mavis-Session-Id", "X-Mavis-Agent-Id", "X-Mavis-Timezone-Offset"} {
		if reqHeader.Get(name) == "" {
			t.Fatalf("%s is empty", name)
		}
	}
	if got := reqHeader.Get("X-Mavis-Agent-Id"); got != AgentID {
		t.Fatalf("X-Mavis-Agent-Id = %q", got)
	}
	_, zoneOffset := time.Now().Zone()
	if got := reqHeader.Get("X-Mavis-Timezone-Offset"); got != strconv.Itoa(zoneOffset) {
		t.Fatalf("X-Mavis-Timezone-Offset = %q, want %d", got, zoneOffset)
	}
	sessionID := reqHeader.Get("X-Mavis-Session-Id")
	if len(strings.Split(sessionID, "-")) != 5 {
		t.Fatalf("session id is not uuid-shaped: %q", sessionID)
	}
	// 相同令牌派生稳定的 session id
	reqHeader2 := &http.Header{}
	if err := (&Adaptor{}).SetupRequestHeader(c, reqHeader2, info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqHeader2.Get("X-Mavis-Session-Id") != sessionID {
		t.Fatal("session id should be stable for the same token")
	}
}

func TestStableUUID(t *testing.T) {
	a := stableUUID("seed-a")
	b := stableUUID("seed-a")
	c := stableUUID("seed-b")
	if a != b || a == c {
		t.Fatalf("stableUUID not stable/unique: %q %q %q", a, b, c)
	}
}

func TestBuildUpstreamRequestIgnoresIdentityPassHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := &relaycommon.RelayInfo{
		UseRuntimeHeadersOverride: true,
		// 等价于亲和性模板 pass_headers 透传的客户端头。
		RuntimeHeadersOverride: map[string]any{
			"user-agent":         "zcode/9.9.9",
			"authorization":      "Bearer client",
			"x-mavis-session-id": "client-session",
			"x-session-id":       "kept",
		},
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: `{"access_token":"a"}`},
	}

	req, err := channel.BuildUpstreamRequest(context.Background(), &Adaptor{}, ctx, info, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("BuildUpstreamRequest: %v", err)
	}
	if got := req.Header.Get("User-Agent"); got != UserAgent {
		t.Fatalf("User-Agent = %q, want %q", got, UserAgent)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer a" {
		t.Fatalf("Authorization = %q, want channel token", got)
	}
	if got := req.Header.Get("X-Mavis-Session-Id"); got != stableUUID("mavis-session|a") {
		t.Fatalf("X-Mavis-Session-Id = %q, want derived id", got)
	}
	if got := req.Header.Get("x-session-id"); got != "kept" {
		t.Fatalf("x-session-id = %q, want passthrough", got)
	}
}
