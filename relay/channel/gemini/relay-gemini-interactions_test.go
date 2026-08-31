package gemini

import (
	"strings"
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

// 转换层已从 Gemini 渠道移除:chat 入站固定走 generateContent URL
func TestGetRequestURLGeminiNoLongerConverts(t *testing.T) {
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
	if !strings.HasSuffix(url, ":generateContent") {
		t.Fatalf("gemini channel chat inbound should stay on generateContent, got %s", url)
	}
}
