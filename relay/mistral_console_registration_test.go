package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/mistral_console"
	"github.com/stretchr/testify/require"
)

func TestMistralConsoleAdaptorRegistration(t *testing.T) {
	adaptor := GetAdaptor(constant.APITypeMistralConsole)
	require.NotNil(t, adaptor)
	require.IsType(t, &mistralconsole.Adaptor{}, adaptor)
	require.Equal(t, "mistral-console", adaptor.GetChannelName())
	require.Equal(t, mistralconsole.ModelList, adaptor.GetModelList())
}
