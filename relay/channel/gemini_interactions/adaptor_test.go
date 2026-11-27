package gemini_interactions

import (
	"testing"

	rootconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
)

func TestGetRequestURLConvertedMode(t *testing.T) {
	info := &common.RelayInfo{RelayMode: constant.RelayModeChatCompletions, IsStream: false}
	info.ChannelMeta = &common.ChannelMeta{
		ChannelType:    rootconstant.ChannelTypeGeminiInteractions,
		ChannelBaseUrl: "https://generativelanguage.googleapis.com",
	}
	info.UpstreamModelName = "gemini-3.1-flash-lite"
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://generativelanguage.googleapis.com/v1beta/interactions" {
		t.Fatalf("converted url = %s", url)
	}
}

func TestGetRequestURLInboundMirror(t *testing.T) {
	info := &common.RelayInfo{RelayMode: constant.RelayModeGeminiInteractions}
	info.ChannelMeta = &common.ChannelMeta{ChannelBaseUrl: "https://x.com"}
	info.RequestURLPath = "/v1beta/interactions/v1_abc/cancel"
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://x.com/v1beta/interactions/v1_abc/cancel" {
		t.Fatalf("mirror url = %s", url)
	}
}

func TestGetRequestURLApiVersionOverride(t *testing.T) {
	info := &common.RelayInfo{RelayMode: constant.RelayModeChatCompletions}
	info.ChannelMeta = &common.ChannelMeta{
		ChannelType:    rootconstant.ChannelTypeGeminiInteractions,
		ChannelBaseUrl: "https://x.com",
		ApiVersion:     "v1",
	}
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://x.com/v1/interactions" {
		t.Fatalf("versioned url = %s", url)
	}
}

func TestGetRequestURLDecisionsModes(t *testing.T) {
	adaptor := &Adaptor{}

	// 原生 decisions 入站指向原生端点
	info := &common.RelayInfo{RelayMode: constant.RelayModeDecisions}
	info.ChannelMeta = &common.ChannelMeta{
		ChannelType:    rootconstant.ChannelTypeGeminiInteractions,
		ChannelBaseUrl: "https://gptload1.pie-xian.com/proxy/211",
	}
	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://gptload1.pie-xian.com/proxy/211/v1beta/decisions" {
		t.Fatalf("decisions url = %s", url)
	}

	// systemone 转换链路上的 decisions 模型同样指向原生端点
	info.RelayMode = constant.RelayModeTypeSafeNative
	info.UpstreamModelName = "gemini-decision-1"
	url, err = adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://gptload1.pie-xian.com/proxy/211/v1beta/decisions" {
		t.Fatalf("systemone decisions url = %s", url)
	}
}

func TestGetRequestURLDecisionsDefaultsBaseURL(t *testing.T) {
	// 空 base 渠道回填默认 Google 主机
	info := &common.RelayInfo{RelayMode: constant.RelayModeDecisions}
	info.ChannelMeta = &common.ChannelMeta{
		ChannelType: rootconstant.ChannelTypeGeminiInteractions,
	}
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://generativelanguage.googleapis.com/v1beta/decisions" {
		t.Fatalf("default base decisions url = %s", url)
	}
}
