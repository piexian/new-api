package ollama

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "cloud root", in: "https://ollama.com", want: "https://ollama.com"},
		{name: "cloud root trailing slash", in: "https://ollama.com/", want: "https://ollama.com"},
		{name: "cloud openai base", in: "https://ollama.com/v1", want: "https://ollama.com"},
		{name: "cloud openai base trailing slash", in: "https://ollama.com/v1/", want: "https://ollama.com"},
		{name: "cloud native base", in: "https://ollama.com/api", want: "https://ollama.com"},
		{name: "cloud native base trailing slash", in: "https://ollama.com/api/", want: "https://ollama.com"},
		{name: "local server", in: "http://localhost:11434", want: "http://localhost:11434"},
		{name: "local server slash", in: "http://localhost:11434/", want: "http://localhost:11434"},
		{name: "whitespace", in: "  https://ollama.com/v1  ", want: "https://ollama.com"},
		{name: "empty", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeBaseURL(tt.in))
		})
	}
}

func TestGetRequestURLRoutes(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		relayFormat types.RelayFormat
		relayMode   int
		requestPath string
		want        string
	}{
		{
			name:        "chat completions native route",
			baseURL:     "https://ollama.com/v1",
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeChatCompletions,
			requestPath: "/v1/chat/completions",
			want:        "https://ollama.com/api/chat",
		},
		{
			name:        "legacy completions native route",
			baseURL:     "https://ollama.com",
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeCompletions,
			requestPath: "/v1/completions",
			want:        "https://ollama.com/api/generate",
		},
		{
			name:        "embeddings native route",
			baseURL:     "https://ollama.com",
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeEmbeddings,
			requestPath: "/v1/embeddings",
			want:        "https://ollama.com/api/embed",
		},
		{
			name:        "responses passthrough",
			baseURL:     "https://ollama.com/v1",
			relayFormat: types.RelayFormatOpenAIResponses,
			relayMode:   relayconstant.RelayModeResponses,
			requestPath: "/v1/responses",
			want:        "https://ollama.com/v1/responses",
		},
		{
			name:        "claude messages passthrough",
			baseURL:     "https://ollama.com",
			relayFormat: types.RelayFormatClaude,
			relayMode:   relayconstant.RelayModeUnknown,
			requestPath: "/v1/messages",
			want:        "https://ollama.com/v1/messages",
		},
		{
			name:        "claude count tokens stays on anthropic route",
			baseURL:     "https://ollama.com/v1",
			relayFormat: types.RelayFormatClaude,
			relayMode:   relayconstant.RelayModeClaudeCountTokens,
			requestPath: "/v1/messages/count_tokens",
			want:        "https://ollama.com/v1/messages/count_tokens",
		},
		{
			name:        "local server chat",
			baseURL:     "http://localhost:11434",
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeUnknown,
			requestPath: "/v1/chat/completions",
			want:        "http://localhost:11434/api/chat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayFormat:    tt.relayFormat,
				RelayMode:      tt.relayMode,
				RequestURLPath: tt.requestPath,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: tt.baseURL,
				},
			}
			got, err := (&Adaptor{}).GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertClaudeRequestPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	request := &dto.ClaudeRequest{Model: "gemma4:31b"}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, &relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude}, request)
	require.NoError(t, err)
	assert.Same(t, request, converted)
}

func TestSetupRequestHeaderClaudeUsesBearerAndAnthropicVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	headers := http.Header{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         "cloud-key",
			ChannelBaseUrl: "https://ollama.com",
		},
	}
	require.NoError(t, (&Adaptor{}).SetupRequestHeader(c, &headers, info))

	assert.Equal(t, "Bearer cloud-key", headers.Get("Authorization"))
	assert.Equal(t, ollamaAnthropicVersion, headers.Get("anthropic-version"))
	assert.Empty(t, headers.Get("x-api-key"))
}
