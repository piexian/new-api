package qwentokenplan

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLSwitchesQwenTokenPlanEndpoints(t *testing.T) {
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
			name:        "openai chat",
			relayFormat: types.RelayFormatOpenAI,
			relayMode:   relayconstant.RelayModeChatCompletions,
			requestPath: "/v1/chat/completions",
			want:        channelconstant.QwenTokenPlanOpenAIBaseURL + "/chat/completions",
		},
		{
			name:        "openai responses from anthropic base",
			baseURL:     channelconstant.QwenTokenPlanAnthropicBaseURL,
			relayFormat: types.RelayFormatOpenAIResponses,
			relayMode:   relayconstant.RelayModeResponses,
			requestPath: "/v1/responses",
			want:        channelconstant.QwenTokenPlanOpenAIBaseURL + "/responses",
		},
		{
			name:        "anthropic messages from openai base",
			baseURL:     channelconstant.QwenTokenPlanOpenAIBaseURL,
			relayFormat: types.RelayFormatClaude,
			requestPath: "/v1/messages",
			want:        channelconstant.QwenTokenPlanAnthropicBaseURL + "/v1/messages",
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
					ChannelType:    channelconstant.ChannelTypeQwenTokenPlan,
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

func TestSetupRequestHeaderDoesNotForwardUsageToken(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: `{"type":"qwen_token_plan","api_key":"sk-sp-relay","access_token":"cli-access-token","expires_at":"2099-01-01T00:00:00Z","user":{}}`},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)
	header := http.Header{}

	require.NoError(t, adaptor.SetupRequestHeader(context, &header, info))
	require.Equal(t, "Bearer sk-sp-relay", header.Get("Authorization"))
	require.NotContains(t, header.Get("Authorization"), "cli-access-token")
}

func TestBoundCredentialParsing(t *testing.T) {
	t.Parallel()
	credential, err := ParseCredential(`{"type":"qwen_token_plan","api_key":"sk-sp-relay","access_token":"cli-access-token","expires_at":"2099-01-01T00:00:00Z","user":{"aliyun_id":"123"}}`)
	require.NoError(t, err)
	require.Equal(t, "sk-sp-relay", credential.APIKey)
	require.Equal(t, "cli-access-token", credential.AccessToken)
	require.Equal(t, "123", credential.User.AliyunID)
	_, err = ParseCredential("sk-sp-relay")
	require.Error(t, err)
}

func TestOAuthExpiredAcceptsQwenTimestampFormat(t *testing.T) {
	t.Parallel()
	credential := &Credential{ExpiresAt: "2099-01-01 00:00:00"}
	require.False(t, credential.OAuthExpired(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
}
