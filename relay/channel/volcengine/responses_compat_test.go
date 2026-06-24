package volcengine

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

func TestConvertOpenAIResponsesRequestPassesThroughOfficialResponsesAPI(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://ark.cn-beijing.volces.com",
			ChannelType:       channelconstant.ChannelTypeVolcEngine,
			UpstreamModelName: "doubao-seed-1-6-250615",
		},
	}
	request := dto.OpenAIResponsesRequest{
		Model:    "doubao-seed-1-6-250615",
		Input:    []byte(`"hello"`),
		Caching:  []byte(`{"type":"enabled"}`),
		Thinking: []byte(`{"type":"disabled"}`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)

	require.NoError(t, err)
	require.Equal(t, request, converted)
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.FinalRequestRelayFormat)

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/responses", url)
}

func TestConvertClaudeRequestPassesThroughOfficialMessagesAPI(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://ark.cn-beijing.volces.com",
			ChannelType:       channelconstant.ChannelTypeVolcEngine,
			UpstreamModelName: "doubao-seed-2-1-pro-260628",
		},
	}
	request := &dto.ClaudeRequest{
		Model:     "doubao-seed-2-1-pro-260628",
		MaxTokens: func() *uint { v := uint(1024); return &v }(),
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, request)

	require.NoError(t, err)
	require.Same(t, request, converted)

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/compatible/v1/messages", url)
}

func TestGetRequestURLSwitchesDoubaoPlanEndpointsByRelayFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		baseURL     string
		model       string
		relayFormat types.RelayFormat
		relayMode   int
		want        string
	}{
		{
			name:        "coding plan claude",
			baseURL:     "doubao-coding-plan",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/coding/v1/messages",
		},
		{
			name:      "coding plan openai",
			baseURL:   "doubao-coding-plan",
			relayMode: relayconstant.RelayModeChatCompletions,
			want:      "https://ark.cn-beijing.volces.com/api/coding/v3/chat/completions",
		},
		{
			name:      "coding plan responses",
			baseURL:   "doubao-coding-plan",
			relayMode: relayconstant.RelayModeResponses,
			want:      "https://ark.cn-beijing.volces.com/api/coding/v3/responses",
		},
		{
			name:      "coding plan embeddings",
			baseURL:   "doubao-coding-plan",
			relayMode: relayconstant.RelayModeEmbeddings,
			want:      "https://ark.cn-beijing.volces.com/api/coding/v3/embeddings",
		},
		{
			name:        "agent plan claude",
			baseURL:     "doubao-agent-plan",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/plan/v1/messages",
		},
		{
			name:        "official agent plan v3 base claude",
			baseURL:     "https://ark.cn-beijing.volces.com/api/plan/v3",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/plan/v1/messages",
		},
		{
			name:        "official coding plan v3 base claude",
			baseURL:     "https://ark.cn-beijing.volces.com/api/coding/v3",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/coding/v1/messages",
		},
		{
			name:        "ordinary claude",
			baseURL:     "https://ark.cn-beijing.volces.com",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/compatible/v1/messages",
		},
		{
			name:        "official compatible base claude",
			baseURL:     "https://ark.cn-beijing.volces.com/api/compatible",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/compatible/v1/messages",
		},
		{
			name:        "official compatible v1 base claude",
			baseURL:     "https://ark.cn-beijing.volces.com/api/compatible/v1",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/compatible/v1/messages",
		},
		{
			name:        "official v3 base claude switches to compatible messages",
			baseURL:     "https://ark.cn-beijing.volces.com/api/v3",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/compatible/v1/messages",
		},
		{
			name:        "ordinary bot claude keeps chat completions endpoint",
			baseURL:     "https://ark.cn-beijing.volces.com",
			model:       "bot-123",
			relayFormat: types.RelayFormatClaude,
			want:        "https://ark.cn-beijing.volces.com/api/v3/bots/chat/completions",
		},
		{
			name:      "agent plan openai",
			baseURL:   "doubao-agent-plan",
			relayMode: relayconstant.RelayModeChatCompletions,
			want:      "https://ark.cn-beijing.volces.com/api/plan/v3/chat/completions",
		},
		{
			name:      "agent plan bot keeps special plan chat endpoint",
			baseURL:   "doubao-agent-plan",
			model:     "bot-123",
			relayMode: relayconstant.RelayModeChatCompletions,
			want:      "https://ark.cn-beijing.volces.com/api/plan/v3/chat/completions",
		},
		{
			name:      "agent plan responses",
			baseURL:   "doubao-agent-plan",
			relayMode: relayconstant.RelayModeResponses,
			want:      "https://ark.cn-beijing.volces.com/api/plan/v3/responses",
		},
		{
			name:      "agent plan images",
			baseURL:   "doubao-agent-plan",
			relayMode: relayconstant.RelayModeImagesGenerations,
			want:      "https://ark.cn-beijing.volces.com/api/plan/v3/images/generations",
		},
		{
			name:      "official v3 base chat",
			baseURL:   "https://ark.cn-beijing.volces.com/api/v3",
			relayMode: relayconstant.RelayModeChatCompletions,
			want:      "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
		},
		{
			name:      "official v3 base responses",
			baseURL:   "https://ark.cn-beijing.volces.com/api/v3",
			relayMode: relayconstant.RelayModeResponses,
			want:      "https://ark.cn-beijing.volces.com/api/v3/responses",
		},
		{
			name:      "official v3 base images",
			baseURL:   "https://ark.cn-beijing.volces.com/api/v3",
			relayMode: relayconstant.RelayModeImagesGenerations,
			want:      "https://ark.cn-beijing.volces.com/api/v3/images/generations",
		},
		{
			name:      "official agent plan base responses",
			baseURL:   "https://ark.cn-beijing.volces.com/api/plan/v3",
			relayMode: relayconstant.RelayModeResponses,
			want:      "https://ark.cn-beijing.volces.com/api/plan/v3/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := tt.model
			if model == "" {
				model = "ark-code-latest"
			}
			info := &relaycommon.RelayInfo{
				RelayMode:   tt.relayMode,
				RelayFormat: tt.relayFormat,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl:    tt.baseURL,
					ChannelType:       channelconstant.ChannelTypeVolcEngine,
					UpstreamModelName: model,
				},
			}

			url, err := (&Adaptor{}).GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, tt.want, url)
		})
	}
}

func TestSetupRequestHeaderForClaudeMessagesCompatibleAPI(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "application/json")
	c.Request.Header.Set("anthropic-version", "2023-06-01")
	c.Request.Header.Set("anthropic-beta", "tools-2024-05-16")

	headers := make(http.Header)
	err := (&Adaptor{}).SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "ark-key",
			ChannelBaseUrl:    "https://ark.cn-beijing.volces.com",
			UpstreamModelName: "doubao-seed-2-1-pro-260628",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Equal(t, "application/json", headers.Get("Accept"))
	require.Equal(t, "Bearer ark-key", headers.Get("Authorization"))
	require.Equal(t, "ark-key", headers.Get("x-api-key"))
	require.Equal(t, "2023-06-01", headers.Get("anthropic-version"))
	require.Equal(t, "tools-2024-05-16", headers.Get("anthropic-beta"))
}

func TestSetupRequestHeaderPassesArkBetaHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "application/json")
	c.Request.Header.Set("ark-beta-image-process", "true")
	c.Request.Header.Set("ark-beta-knowledge-search", "true")
	c.Request.Header.Set("ark-beta-mcp", "true")

	headers := make(http.Header)
	err := (&Adaptor{}).SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "ark-key",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Equal(t, "application/json", headers.Get("Accept"))
	require.Equal(t, "Bearer ark-key", headers.Get("Authorization"))
	require.Equal(t, "true", headers.Get("ark-beta-image-process"))
	require.Equal(t, "true", headers.Get("ark-beta-knowledge-search"))
	require.Equal(t, "true", headers.Get("ark-beta-mcp"))
}
