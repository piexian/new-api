package opencode

import (
	"net/http"
	"net/http/httptest"
	"testing"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
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
	require.Equal(t, "https://opencode.ai/zen", NormalizeRoot("https://opencode.ai/zen/v1/models/gemini-3.5-flash"))
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

func TestGetRequestURLRoutesKnownModelsByBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		model   string
		want    string
		mode    int
	}{
		{
			name:    "go chat Kimi K3",
			baseURL: channelconstant.OpenCodeGoBaseURLAlias,
			model:   "kimi-k3",
			want:    "https://opencode.ai/zen/go/v1/chat/completions",
			mode:    requestModeOpenAI,
		},
		{
			name:    "go anthropic MiniMax M3",
			baseURL: channelconstant.OpenCodeGoBaseURLAlias,
			model:   "minimax-m3",
			want:    "https://opencode.ai/zen/go/v1/messages",
			mode:    requestModeClaude,
		},
		{
			name:    "zen responses GPT 5.6 Sol",
			baseURL: channelconstant.OpenCodeZenBaseURLAlias,
			model:   "gpt-5.6-sol",
			want:    "https://opencode.ai/zen/v1/responses",
			mode:    requestModeResponses,
		},
		{
			name:    "zen anthropic Claude Sonnet 5",
			baseURL: channelconstant.OpenCodeZenBaseURLAlias,
			model:   "claude-sonnet-5",
			want:    "https://opencode.ai/zen/v1/messages",
			mode:    requestModeClaude,
		},
		{
			name:    "zen Gemini 3.5 Flash",
			baseURL: channelconstant.OpenCodeZenBaseURLAlias,
			model:   "gemini-3.5-flash",
			want:    "https://opencode.ai/zen/v1/models/gemini-3.5-flash:generateContent",
			mode:    requestModeGemini,
		},
		{
			name:    "zen chat Grok 4.5",
			baseURL: channelconstant.OpenCodeZenBaseURLAlias,
			model:   "grok-4.5",
			want:    "https://opencode.ai/zen/v1/chat/completions",
			mode:    requestModeOpenAI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info := &relaycommon.RelayInfo{
				RelayMode:      relayconstant.RelayModeChatCompletions,
				RelayFormat:    types.RelayFormatOpenAI,
				RequestURLPath: "/v1/chat/completions",
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
			require.Equal(t, tt.mode, adaptor.RequestMode)
			require.True(t, adaptor.RouteByModel)
		})
	}
}

func TestOpenCodeModelRoutingConvertsOpenAIChatRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		model    string
		wantType any
		format   types.RelayFormat
	}{
		{
			name:     "chat to responses",
			baseURL:  channelconstant.OpenCodeZenBaseURLAlias,
			model:    "gpt-5.6-sol",
			wantType: &dto.OpenAIResponsesRequest{},
			format:   types.RelayFormatOpenAIResponses,
		},
		{
			name:     "chat to Claude",
			baseURL:  channelconstant.OpenCodeGoBaseURLAlias,
			model:    "minimax-m3",
			wantType: &dto.ClaudeRequest{},
			format:   types.RelayFormatClaude,
		},
		{
			name:     "chat to Gemini",
			baseURL:  channelconstant.OpenCodeZenBaseURLAlias,
			model:    "gemini-3.5-flash",
			wantType: &dto.GeminiChatRequest{},
			format:   types.RelayFormatGemini,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
			info := &relaycommon.RelayInfo{
				RelayMode:      relayconstant.RelayModeChatCompletions,
				RelayFormat:    types.RelayFormatOpenAI,
				RequestURLPath: "/v1/chat/completions",
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl:    tt.baseURL,
					ChannelType:       channelconstant.ChannelTypeOpenCode,
					UpstreamModelName: tt.model,
				},
			}
			adaptor := &Adaptor{}
			adaptor.Init(info)

			converted, err := adaptor.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
				Model:    tt.model,
				Messages: []dto.Message{{Role: "user", Content: "hi"}},
			})

			require.NoError(t, err)
			require.IsType(t, tt.wantType, converted)
			require.Equal(t, tt.format, info.FinalRequestRelayFormat)
		})
	}
}

func TestOpenCodeModelRoutingConvertsResponsesToChat(t *testing.T) {
	t.Parallel()

	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeResponses,
		RelayFormat:    types.RelayFormatOpenAIResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    channelconstant.OpenCodeZenBaseURLAlias,
			ChannelType:       channelconstant.ChannelTypeOpenCode,
			UpstreamModelName: "grok-4.5",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "grok-4.5",
		Input: []byte(`[{"role":"user","content":"hi"}]`),
	})

	require.NoError(t, err)
	require.IsType(t, &dto.GeneralOpenAIRequest{}, converted)
	require.Equal(t, types.RelayFormatOpenAI, info.FinalRequestRelayFormat)
	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://opencode.ai/zen/v1/chat/completions", requestURL)
}

func TestOpenCodePassThroughKeepsClientProtocolRoute(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeChatCompletions,
		RelayFormat:    types.RelayFormatOpenAI,
		RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    channelconstant.OpenCodeZenBaseURLAlias,
			ChannelType:       channelconstant.ChannelTypeOpenCode,
			UpstreamModelName: "claude-sonnet-5",
			ChannelSetting:    dto.ChannelSettings{PassThroughBodyEnabled: true},
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)

	requestURL, err := adaptor.GetRequestURL(info)

	require.NoError(t, err)
	require.Equal(t, requestModeOpenAI, adaptor.RequestMode)
	require.False(t, adaptor.RouteByModel)
	require.Equal(t, "https://opencode.ai/zen/v1/chat/completions", requestURL)
}

func TestOpenCodeModelRoutedClaudeUsesAnthropicHeaders(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeChatCompletions,
		RelayFormat:    types.RelayFormatOpenAI,
		RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "opencode-key",
			ChannelBaseUrl:    channelconstant.OpenCodeGoBaseURLAlias,
			ChannelType:       channelconstant.ChannelTypeOpenCode,
			UpstreamModelName: "minimax-m3",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)
	headers := make(http.Header)

	require.NoError(t, adaptor.SetupRequestHeader(c, &headers, info))
	require.Equal(t, "opencode-key", headers.Get("x-api-key"))
	require.Empty(t, headers.Get("Authorization"))
	require.Equal(t, "2023-06-01", headers.Get("anthropic-version"))
	require.Contains(t, headers.Get("User-Agent"), "claude-cli/2.1.165")
}

func TestOpenCodeModelInventoriesMatchCurrentRoutes(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{
		"grok-4.5",
		"glm-5.2",
		"glm-5.1",
		"kimi-k3",
		"kimi-k2.7-code",
		"kimi-k2.6",
		"deepseek-v4-pro",
		"deepseek-v4-flash",
		"mimo-v2.5",
		"mimo-v2.5-pro",
	}, channelconstant.OpenCodeGoChatModels)
	require.Contains(t, channelconstant.OpenCodeGoClaudeModels, "minimax-m3")
	require.Contains(t, channelconstant.OpenCodeZenResponsesModels, "gpt-5.6-sol")
	require.Contains(t, channelconstant.OpenCodeZenResponsesModels, "gpt-5.6-terra")
	require.Contains(t, channelconstant.OpenCodeZenResponsesModels, "gpt-5.6-luna")
	require.Contains(t, channelconstant.OpenCodeZenClaudeModels, "claude-sonnet-5")
	require.Contains(t, channelconstant.OpenCodeZenChatModels, "glm-5.2")
	require.Contains(t, channelconstant.OpenCodeZenChatModels, "kimi-k2.7-code")
	require.Contains(t, channelconstant.OpenCodeZenChatModels, "grok-4.5")
	require.NotContains(t, channelconstant.OpenCodeGoChatModels, "glm-5")
	require.NotContains(t, channelconstant.OpenCodeZenClaudeModels, "claude-opus-4-1")
}
