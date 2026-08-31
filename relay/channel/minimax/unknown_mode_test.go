package minimax

import (
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

// /v1/chat/completions 与 /v1/messages 经 Path2RelayMode 后 RelayMode 为 Unknown(0)，
// GetRequestURL 必须将其按聊天补全处理，不允许报 unsupported relay mode
func TestGetRequestURLUnknownModeFallsBackToChatCompletions(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeUnknown,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.minimaxi.com",
		},
	}

	got, err := GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL(RelayModeUnknown) returned error: %v", err)
	}
	if !strings.HasSuffix(got, "/v1/chat/completions") {
		t.Fatalf("GetRequestURL(RelayModeUnknown) = %q, want suffix /v1/chat/completions", got)
	}
}
