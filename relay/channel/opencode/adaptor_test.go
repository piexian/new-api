package opencode

import (
	"testing"

	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func TestGetRequestURLSwitchesOpenCodeEndpointsByRelayFormat(t *testing.T) {
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
			name:        "zen openai chat",
			baseURL:     channelconstant.OpenCodeZenBaseURLAlias,
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeChatCompletions,
			requestPath: "/v1/chat/completions",
			want:        "https://opencode.ai/zen/v1/chat/completions",
		},
		{
			name:        "zen responses",
			baseURL:     channelconstant.OpenCodeZenBaseURLAlias,
			relayFormat: types.RelayFormatOpenAIResponses,
			relayMode:   relayconstant.RelayModeResponses,
			requestPath: "/v1/responses",
			want:        "https://opencode.ai/zen/v1/responses",
		},
		{
			name:        "zen anthropic messages",
			baseURL:     "https://opencode.ai/zen/v1/chat/completions",
			relayFormat: types.RelayFormatClaude,
			requestPath: "/v1/messages",
			want:        "https://opencode.ai/zen/v1/messages",
		},
		{
			name:        "zen gemini stream",
			baseURL:     channelconstant.OpenCodeZenBaseURLAlias,
			relayFormat: types.RelayFormatGemini,
			relayMode:   relayconstant.RelayModeGemini,
			requestPath: "/v1/models/gemini-3-flash:streamGenerateContent",
			model:       "gemini-3-flash",
			isStream:    true,
			want:        "https://opencode.ai/zen/v1/models/gemini-3-flash:streamGenerateContent?alt=sse",
		},
		{
			name:        "go openai chat",
			baseURL:     channelconstant.OpenCodeGoBaseURLAlias,
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeChatCompletions,
			requestPath: "/v1/chat/completions",
			want:        "https://opencode.ai/zen/go/v1/chat/completions",
		},
		{
			name:        "go anthropic messages",
			baseURL:     channelconstant.OpenCodeGoBaseURLAlias,
			relayFormat: types.RelayFormatClaude,
			requestPath: "/v1/messages",
			want:        "https://opencode.ai/zen/go/v1/messages",
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
					ChannelType:       channelconstant.ChannelTypeOpenCode,
					UpstreamModelName: tt.model,
				},
			}
			adaptor := &Adaptor{}
			adaptor.Init(info)

			got, err := adaptor.GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			if tt.name == "zen gemini stream" {
				require.True(t, info.DisablePing)
			}
		})
	}
}

func TestGetRequestURLRejectsUnsupportedGoProtocols(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		relayFormat types.RelayFormat
		relayMode   int
		requestPath string
	}{
		{
			name:        "responses",
			relayFormat: types.RelayFormatOpenAIResponses,
			relayMode:   relayconstant.RelayModeResponses,
			requestPath: "/v1/responses",
		},
		{
			name:        "gemini",
			relayFormat: types.RelayFormatGemini,
			relayMode:   relayconstant.RelayModeGemini,
			requestPath: "/v1/models/gemini-3-flash:generateContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info := &relaycommon.RelayInfo{
				RelayMode:      tt.relayMode,
				RelayFormat:    tt.relayFormat,
				RequestURLPath: tt.requestPath,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: channelconstant.OpenCodeGoBaseURLAlias,
					ChannelType:    channelconstant.ChannelTypeOpenCode,
				},
			}
			adaptor := &Adaptor{}
			adaptor.Init(info)

			_, err := adaptor.GetRequestURL(info)

			require.Error(t, err)
		})
	}
}

func TestNormalizeRootAndModelSources(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://opencode.ai/zen", NormalizeRoot(channelconstant.OpenCodeZenBaseURLAlias))
	require.Equal(t, "https://opencode.ai/zen/go", NormalizeRoot(channelconstant.OpenCodeGoBaseURLAlias))
	require.Equal(t, "https://opencode.ai/zen", NormalizeRoot("https://opencode.ai/zen/v1/responses"))
	require.Equal(t, "https://opencode.ai/zen/go", NormalizeRoot("https://opencode.ai/zen/go/v1/messages"))

	require.True(t, IsGoBase(channelconstant.OpenCodeGoBaseURLAlias))
	require.False(t, IsGoBase(channelconstant.OpenCodeZenBaseURLAlias))

	modelsURL, ok := ModelsURL(channelconstant.OpenCodeZenBaseURLAlias)
	require.True(t, ok)
	require.Equal(t, "https://opencode.ai/zen/v1/models", modelsURL)
	require.Empty(t, StaticModelListForBase(channelconstant.OpenCodeZenBaseURLAlias))
	require.NotEmpty(t, StaticModelListForBase(channelconstant.OpenCodeGoBaseURLAlias))

	_, ok = ModelsURL(channelconstant.OpenCodeGoBaseURLAlias)
	require.False(t, ok)
}

func TestParseModelsResponse(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"data": [{"id": "gpt-5.5"}, {"id": "models/gemini-3-flash"}],
		"models": [{"name": "claude-sonnet-4-6"}, {"model": "glm-5.1"}]
	}`)

	models, err := ParseModelsResponse(body)

	require.NoError(t, err)
	require.Equal(t, []string{
		"gpt-5.5",
		"gemini-3-flash",
		"claude-sonnet-4-6",
		"glm-5.1",
	}, models)
}
