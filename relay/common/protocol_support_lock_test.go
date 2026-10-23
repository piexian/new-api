package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func protocolBool(value bool) *bool {
	return &value
}

func TestLockedChannelsIgnoreProtocolOverrides(t *testing.T) {
	setting := dto.ChannelSettings{ProtocolSupport: &dto.ChannelProtocolSupport{Chat: protocolBool(true)}}

	for _, channelType := range []int{
		constant.ChannelTypeAnthropic,
		constant.ChannelTypeAws,
		constant.ChannelTypeVertexAi,
		constant.ChannelTypeCodex,
		constant.ChannelTypeMiniMaxCode,
		constant.ChannelTypeCohere,
	} {
		require.True(t, ProtocolSupportLocked(channelType, "", setting), "type %d", channelType)
		resolved := ResolveChannelProtocolSupport(channelType, "", setting)
		require.Equal(t, DefaultChannelProtocolSupport(channelType, "", setting), resolved, "type %d", channelType)
	}

	require.False(t, ProtocolSupportLocked(constant.ChannelTypeOpenAI, "", setting))
	require.False(t, ProtocolSupportLocked(constant.ChannelTypeZhipu_v4, "glm-coding-plan", dto.ChannelSettings{}))
	require.True(t, ProtocolSupportLocked(constant.ChannelTypeZhipu_v4, "glm-coding-plan", dto.ChannelSettings{ZcodeModeEnabled: true}))
}

func TestNormalizeProtocolSupportOverridesKeepsOnlyDifferences(t *testing.T) {
	full := dto.ChannelSettings{ProtocolSupport: &dto.ChannelProtocolSupport{
		Chat:      protocolBool(true),
		Responses: protocolBool(false),
		Messages:  protocolBool(false),
		Gemini:    protocolBool(false),
	}}

	normalized := NormalizeProtocolSupportOverrides(constant.ChannelTypeOpenAI, "", full)

	require.NotNil(t, normalized.ProtocolSupport)
	require.Nil(t, normalized.ProtocolSupport.Chat)
	require.Equal(t, false, *normalized.ProtocolSupport.Responses)
	require.Nil(t, normalized.ProtocolSupport.Messages)

	defaultsOnly := dto.ChannelSettings{ProtocolSupport: &dto.ChannelProtocolSupport{Chat: protocolBool(true), Responses: protocolBool(true)}}
	require.Nil(t, NormalizeProtocolSupportOverrides(constant.ChannelTypeOpenAI, "", defaultsOnly).ProtocolSupport)

	require.Nil(t, NormalizeProtocolSupportOverrides(constant.ChannelTypeMiniMaxCode, "", full).ProtocolSupport)
}

func TestNormalizeProtocolSupportMovesGeminiChatToCompatFlag(t *testing.T) {
	setting := dto.ChannelSettings{ProtocolSupport: &dto.ChannelProtocolSupport{Chat: protocolBool(true), Gemini: protocolBool(true)}}

	normalized := NormalizeProtocolSupportOverrides(constant.ChannelTypeGemini, "", setting)

	require.True(t, normalized.UpstreamOpenAICompatEnabled)
	require.Nil(t, normalized.ProtocolSupport)
}
