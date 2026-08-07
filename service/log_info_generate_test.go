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
