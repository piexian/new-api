package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestAppendRequestConversionChainIncludesModelRouting(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI},
		RequestModelRoutingChain: []string{
			"K3 Auto Route (k3 -> k3-256k)",
		},
	}
	other := map[string]interface{}{}

	appendRequestConversionChain(relayInfo, other)

	require.Equal(t, []string{
		"OpenAI Compatible",
		"K3 Auto Route (k3 -> k3-256k)",
	}, other["request_conversion"])
}

func TestAppendRequestFormatInfoForUpstreamError(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatOpenAIResponses,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAIResponses, types.RelayFormatOpenAI},
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
	}
	other := map[string]interface{}{"status_code": 503}

	AppendRequestFormatInfo(relayInfo, other)

	require.Equal(t, []string{"OpenAI Responses", "OpenAI Compatible"}, other["request_conversion"])
	require.Equal(t, 503, other["status_code"])
	require.NotContains(t, other, "claude")
}
