package gemini

import (
	"testing"

	rootconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
)

func TestGeminiInteractionsRequestURL(t *testing.T) {
	cases := []struct {
		path    string
		base    string
		want    string
		wantErr bool
	}{
		{path: "/v1beta/interactions", base: "https://generativelanguage.googleapis.com", want: "https://generativelanguage.googleapis.com/v1beta/interactions"},
		{path: "/v1beta2/interactions", base: "https://x.com/", want: "https://x.com/v1beta2/interactions"},
		{path: "/v1/interactions", base: "https://x.com", want: "https://x.com/v1/interactions"},
		{path: "/v1beta/interactions/v1_abc123", base: "https://x.com", want: "https://x.com/v1beta/interactions/v1_abc123"},
		{path: "/v1beta/interactions/v1_abc123/cancel", base: "https://x.com", want: "https://x.com/v1beta/interactions/v1_abc123/cancel"},
		{path: "/v1beta/interactions/v1_abc123?stream=true&last_event_id=e5", base: "https://x.com", want: "https://x.com/v1beta/interactions/v1_abc123?stream=true&last_event_id=e5"},
		{path: "/v2/interactions", base: "https://x.com", wantErr: true},
		{path: "/v1beta/models/gemini:generateContent", base: "https://x.com", wantErr: true},
	}
	for _, tc := range cases {
		info := &common.RelayInfo{RequestURLPath: tc.path, RelayMode: constant.RelayModeGeminiInteractions}
		info.ChannelMeta = &common.ChannelMeta{ChannelBaseUrl: tc.base}
		got, err := geminiInteractionsRequestURL(info)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("path %s: expected error, got %s", tc.path, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("path %s: %v", tc.path, err)
		}
		if got != tc.want {
			t.Fatalf("path %s: got %s want %s", tc.path, got, tc.want)
		}
	}
}

func TestShouldUseInteractionsUpstream(t *testing.T) {
	// 命中默认前缀
	info := &common.RelayInfo{}
	info.ChannelMeta = &common.ChannelMeta{
		ChannelType: rootconstant.ChannelTypeGemini,
	}
	info.UpstreamModelName = "antigravity-preview-05-2026"
	if !shouldUseInteractionsUpstream(info) {
		t.Fatal("antigravity model should use interactions upstream")
	}
	info.UpstreamModelName = "deep-research-max-preview-04-2026"
	if !shouldUseInteractionsUpstream(info) {
		t.Fatal("deep-research model should use interactions upstream")
	}
	// 普通 gemini 模型不转换
	info.UpstreamModelName = "gemini-3.6-flash"
	if shouldUseInteractionsUpstream(info) {
		t.Fatal("normal gemini model should not convert")
	}
	// 入站 interactions 不做二次转换
	info.UpstreamModelName = "antigravity-preview-05-2026"
	info.RelayMode = constant.RelayModeGeminiInteractions
	if shouldUseInteractionsUpstream(info) {
		t.Fatal("inbound interactions passthrough should not re-convert")
	}
	// 非 gemini 渠道(vertex 复用 handler)不转换
	info.RelayMode = 0
	info.ChannelType = rootconstant.ChannelTypeVertexAi
	if shouldUseInteractionsUpstream(info) {
		t.Fatal("vertex channel must not convert")
	}
}

func TestGetRequestURLInteractionsConvertedMode(t *testing.T) {
	info := &common.RelayInfo{RelayMode: constant.RelayModeChatCompletions}
	info.ChannelMeta = &common.ChannelMeta{
		ChannelType:    rootconstant.ChannelTypeGemini,
		ChannelBaseUrl: "https://generativelanguage.googleapis.com",
	}
	info.UpstreamModelName = "antigravity-preview-05-2026"
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://generativelanguage.googleapis.com/v1beta/interactions" {
		t.Fatalf("converted url = %s", url)
	}
}
