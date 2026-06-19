package zenmux

import (
	"testing"

	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestGetRequestURLSwitchesZenMuxEndpointsByRelayFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		baseURL     string
		relayFormat types.RelayFormat
		relayMode   int
		requestPath string
		model       string
		isStream    bool
		want        string
	}{
		{
			name:        "openai chat",
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeChatCompletions,
			requestPath: "/v1/chat/completions",
			want:        "https://zenmux.ai/api/v1/chat/completions",
		},
		{
			name:        "openai responses",
			relayFormat: types.RelayFormatOpenAIResponses,
			relayMode:   relayconstant.RelayModeResponses,
			requestPath: "/v1/responses",
			want:        "https://zenmux.ai/api/v1/responses",
		},
		{
			name:        "openai embeddings",
			relayFormat: types.RelayFormatEmbedding,
			relayMode:   relayconstant.RelayModeEmbeddings,
			requestPath: "/v1/embeddings",
			want:        "https://zenmux.ai/api/v1/embeddings",
		},
		{
			name:        "anthropic messages",
			baseURL:     "https://zenmux.ai/api/v1",
			relayFormat: types.RelayFormatClaude,
			requestPath: "/v1/messages",
			want:        "https://zenmux.ai/api/anthropic/v1/messages",
		},
		{
			name:        "vertex generate content",
			baseURL:     "https://zenmux.ai/api/anthropic",
			relayFormat: types.RelayFormatGemini,
			relayMode:   relayconstant.RelayModeGemini,
			requestPath: "/v1beta/models/google/gemini-2.5-pro:generateContent",
			model:       "google/gemini-2.5-pro",
			want:        "https://zenmux.ai/api/vertex-ai/v1/publishers/google/models/gemini-2.5-pro:generateContent",
		},
		{
			name:        "vertex stream generate content defaults provider",
			baseURL:     "https://zenmux.ai/api/vertex-ai",
			relayFormat: types.RelayFormatGemini,
			relayMode:   relayconstant.RelayModeGemini,
			requestPath: "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
			model:       "gemini-2.5-flash",
			isStream:    true,
			want:        "https://zenmux.ai/api/vertex-ai/v1/publishers/google/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info := &relaycommon.RelayInfo{
				RelayMode:      tt.relayMode,
				RelayFormat:    tt.relayFormat,
				RequestURLPath: tt.requestPath,
				IsStream:       tt.isStream,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl:    tt.baseURL,
					ChannelType:       channelconstant.ChannelTypeZenMux,
					UpstreamModelName: tt.model,
				},
			}
			adaptor := &Adaptor{}
			adaptor.Init(info)

			got, err := adaptor.GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetRequestURLAppendsClaudeBetaQuery(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayFormat:         types.RelayFormatClaude,
		IsClaudeBetaQuery:   true,
		RequestURLPath:      "/v1/messages",
		ChannelMeta:         &relaycommon.ChannelMeta{ChannelType: channelconstant.ChannelTypeZenMux},
		ClaudeConvertInfo:   &relaycommon.ClaudeConvertInfo{},
		ThinkingContentInfo: relaycommon.ThinkingContentInfo{},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	got, err := adaptor.GetRequestURL(info)

	require.NoError(t, err)
	require.Equal(t, "https://zenmux.ai/api/anthropic/v1/messages?beta=true", got)
}
