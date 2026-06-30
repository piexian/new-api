package moark

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLPreservesMoarkPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		baseURL     string
		relayFormat types.RelayFormat
		relayMode   int
		requestPath string
		want        string
	}{
		{
			name:        "moderations",
			baseURL:     "https://api.moark.com",
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeModerations,
			requestPath: "/v1/moderations",
			want:        "https://api.moark.com/v1/moderations",
		},
		{
			name:        "anthropic messages",
			baseURL:     "https://api.moark.com",
			relayFormat: types.RelayFormatClaude,
			requestPath: "/v1/messages",
			want:        "https://api.moark.com/v1/messages",
		},
		{
			name:        "native async with v1 base",
			baseURL:     "https://api.moark.com/v1",
			relayFormat: types.RelayFormatMoarkNative,
			relayMode:   relayconstant.RelayModeMoarkNative,
			requestPath: "/v1/async/music/generations",
			want:        "https://api.moark.com/v1/async/music/generations",
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
					ChannelBaseUrl: tt.baseURL,
					ChannelType:    constant.ChannelTypeMoark,
				},
			}

			got, err := (&Adaptor{}).GetRequestURL(info)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSetupRequestHeaderUsesBearerForClaudeCompatibleRequests(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	headers := http.Header{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test-key",
		},
	}

	err := (&Adaptor{}).SetupRequestHeader(c, &headers, info)

	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", headers.Get("Authorization"))
	require.Empty(t, headers.Get("x-api-key"))
	require.Equal(t, "2023-06-01", headers.Get("anthropic-version"))
}

func TestConvertClaudeRequestKeepsNativeProtocol(t *testing.T) {
	t.Parallel()

	request := &dto.ClaudeRequest{Model: "claude-sonnet-4-5"}

	got, err := (&Adaptor{}).ConvertClaudeRequest(nil, nil, request)

	require.NoError(t, err)
	require.Same(t, request, got)
}

func TestGetModelListIncludesModerationAndTaskModels(t *testing.T) {
	t.Parallel()

	models := (&Adaptor{}).GetModelList()

	require.Contains(t, models, "moark-text-moderation")
	require.Contains(t, models, constant.MoarkTaskModel)
}
