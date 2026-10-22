package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func TestChannelConfiguredForGroupModel(t *testing.T) {
	channel := &model.Channel{Group: "default, vip", Models: "gpt-5, gpt-5-mini"}

	require.True(t, channelConfiguredForGroupModel(channel, []string{"default"}, "gpt-5"))
	require.True(t, channelConfiguredForGroupModel(channel, []string{"svip", "vip"}, "gpt-5-mini"))
	require.False(t, channelConfiguredForGroupModel(channel, []string{"default"}, "gpt-4o"))
	require.False(t, channelConfiguredForGroupModel(channel, []string{"svip"}, "gpt-5"))
	require.False(t, channelConfiguredForGroupModel(nil, []string{"default"}, "gpt-5"))
}
