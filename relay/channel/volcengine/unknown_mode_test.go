package volcengine

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

	a := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeUnknown,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://ark.cn-beijing.volces.com/api/v3",
		},
	}

	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL(RelayModeUnknown) returned error: %v", err)
	}
	if !strings.HasSuffix(got, "/chat/completions") {
		t.Fatalf("GetRequestURL(RelayModeUnknown) = %q, want suffix /chat/completions", got)
	}
}
