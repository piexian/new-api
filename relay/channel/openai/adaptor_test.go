package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func TestGetRequestURLForKiloTrimsV1Prefix(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeKilo,
			ChannelBaseUrl: "https://api.kilo.ai/api/gateway",
		},
		RequestURLPath: "/v1/chat/completions",
		RelayFormat:    types.RelayFormatOpenAI,
	}

	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	if url != "https://api.kilo.ai/api/gateway/chat/completions" {
		t.Fatalf("unexpected request url: %s", url)
	}
}

func TestGetRequestURLForKiloClaudeRelayUsesChatCompletions(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeKilo,
			ChannelBaseUrl: "https://api.kilo.ai/api/gateway",
		},
		RequestURLPath: "/v1/messages",
		RelayFormat:    types.RelayFormatClaude,
	}

	url, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	if url != "https://api.kilo.ai/api/gateway/chat/completions" {
		t.Fatalf("unexpected request url: %s", url)
	}
}
