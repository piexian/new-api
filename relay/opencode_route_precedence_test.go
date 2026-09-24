package relay

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenCodeKnownModelsOverrideChannelPassThrough(t *testing.T) {
	for _, tc := range []struct {
		name, baseURL, model string
		apiType, channelType int
		format               types.RelayFormat
		mode                 int
		wantPassThrough      bool
	}{
		{"zen responses to chat", constant.OpenCodeZenBaseURLAlias, "space-bunny-free", constant.APITypeOpenCode, constant.ChannelTypeOpenCode, types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses, false},
		{"zen chat to responses", constant.OpenCodeZenBaseURLAlias, "gpt-6-sol", constant.APITypeOpenCode, constant.ChannelTypeOpenCode, types.RelayFormatOpenAI, relayconstant.RelayModeChatCompletions, false},
		{"go chat to claude", constant.OpenCodeGoBaseURLAlias, "qwen3.8-flash", constant.APITypeOpenCode, constant.ChannelTypeOpenCode, types.RelayFormatOpenAI, relayconstant.RelayModeChatCompletions, false},
		{"unknown zen model", constant.OpenCodeZenBaseURLAlias, "unlisted-open-code-model", constant.APITypeOpenCode, constant.ChannelTypeOpenCode, types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses, true},
		{"go Chat-only model", constant.OpenCodeGoBaseURLAlias, "space-bunny-free", constant.APITypeOpenCode, constant.ChannelTypeOpenCode, types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses, false},
		{"other channel", constant.OpenCodeZenBaseURLAlias, "space-bunny-free", constant.APITypeOpenAI, constant.ChannelTypeOpenAI, types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayFormat: tc.format,
				RelayMode:   tc.mode,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiType:           tc.apiType,
					ChannelType:       tc.channelType,
					ChannelBaseUrl:    tc.baseURL,
					UpstreamModelName: tc.model,
					ChannelSetting:    dto.ChannelSettings{PassThroughBodyEnabled: true},
				},
			}
			require.Equal(t, tc.wantPassThrough, shouldPassThroughModelRequest(info))
		})
	}
}

func TestOpenCodePassThroughUsesModelRouteInRelayHelpers(t *testing.T) {
	service.InitHttpClient()
	for _, tc := range []struct {
		name, basePath, model          string
		incomingPath, upstreamPath     string
		wantedField, missingField      string
		responses, unknown, globalOnly bool
	}{
		{"zen responses to chat", "/zen", "space-bunny-free", "/v1/responses", "/v1/chat/completions", "messages", "input", true, false, false},
		{"zen responses global passthrough", "/zen", "space-bunny-free", "/v1/responses", "/v1/chat/completions", "messages", "input", true, false, true},
		{"go responses to Chat-only model", "/zen/go", "space-bunny-free", "/v1/responses", "/v1/chat/completions", "messages", "input", true, false, false},
		{"zen chat to responses", "/zen", "gpt-6-sol", "/v1/chat/completions", "/v1/responses", "input", "messages", false, false, false},
		{"go chat to claude", "/zen/go", "qwen3.8-flash", "/v1/chat/completions", "/v1/messages", "messages", "input", false, false, false},
		{"unknown zen responses passthrough", "/zen", "unlisted-open-code-model", "/v1/responses", "/v1/responses", "input", "messages", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.globalOnly {
				original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
				model_setting.GetGlobalSettings().PassThroughRequestEnabled = true
				t.Cleanup(func() { model_setting.GetGlobalSettings().PassThroughRequestEnabled = original })
			}
			type observed struct{ path, body string }
			sent := make(chan observed, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				sent <- observed{r.URL.Path, string(body)}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, `{"error":{"message":"probe","type":"upstream_error"}}`)
			}))
			defer upstream.Close()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenCode)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL+tc.basePath)
			common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, tc.model)
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: !tc.globalOnly})
			info := &relaycommon.RelayInfo{OriginModelName: tc.model, RequestURLPath: tc.incomingPath}
			var raw string
			if tc.responses {
				info.RelayFormat = types.RelayFormatOpenAIResponses
				info.RelayMode = relayconstant.RelayModeResponses
				raw = `{"model":"` + tc.model + `","input":[{"role":"user","content":"hi"}],"service_tier":"flex","extra_marker":true}`
				var request dto.OpenAIResponsesRequest
				require.NoError(t, common.Unmarshal([]byte(raw), &request))
				info.Request = &request
			} else {
				info.RelayFormat = types.RelayFormatOpenAI
				info.RelayMode = relayconstant.RelayModeChatCompletions
				raw = `{"model":"` + tc.model + `","messages":[{"role":"user","content":"hi"}],"service_tier":"flex","extra_marker":true}`
				var request dto.GeneralOpenAIRequest
				require.NoError(t, common.Unmarshal([]byte(raw), &request))
				info.Request = &request
			}
			c.Request = httptest.NewRequest(http.MethodPost, tc.incomingPath, strings.NewReader(raw))
			c.Request.Header.Set("Content-Type", "application/json")
			info.InitRequestConversionChain()
			var apiErr *types.NewAPIError
			if tc.responses {
				apiErr = ResponsesHelper(c, info)
			} else {
				apiErr = TextHelper(c, info)
			}
			require.NotNil(t, apiErr)
			request := <-sent
			require.Equal(t, tc.basePath+tc.upstreamPath, request.path)
			require.True(t, gjson.Get(request.body, tc.wantedField).Exists())
			require.False(t, gjson.Get(request.body, tc.missingField).Exists())
			if tc.unknown {
				require.True(t, gjson.Get(request.body, "extra_marker").Bool())
			} else {
				require.False(t, gjson.Get(request.body, "extra_marker").Exists())
				require.False(t, gjson.Get(request.body, "service_tier").Exists())
				require.Greater(t, len(info.RequestConversionChain), 1)
			}
		})
	}
}

func TestOpenCodeClaudeHelperRoutesGoChatOnlyModel(t *testing.T) {
	service.InitHttpClient()
	for _, globalOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("global-passthrough=%t", globalOnly), func(t *testing.T) {
			if globalOnly {
				original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
				model_setting.GetGlobalSettings().PassThroughRequestEnabled = true
				t.Cleanup(func() { model_setting.GetGlobalSettings().PassThroughRequestEnabled = original })
			}
			sent := make(chan struct{ path, body, auth string }, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				sent <- struct{ path, body, auth string }{r.URL.Path, string(body), r.Header.Get("Authorization")}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, `{"error":{"message":"probe","type":"server_error"}}`)
			}))
			defer upstream.Close()
			raw := `{"model":"space-bunny-free","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"service_tier":"flex"}`
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(raw))
			c.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenCode)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL+"/zen/go")
			common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "space-bunny-free")
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: !globalOnly})
			info := &relaycommon.RelayInfo{OriginModelName: "space-bunny-free", RequestURLPath: "/v1/messages", RelayFormat: types.RelayFormatClaude, RelayMode: relayconstant.RelayModeUnknown}
			var req dto.ClaudeRequest
			require.NoError(t, common.Unmarshal([]byte(raw), &req))
			info.Request = &req
			info.InitRequestConversionChain()
			require.NotNil(t, ClaudeHelper(c, info))
			request := <-sent
			require.Equal(t, "/zen/go/v1/chat/completions", request.path)
			require.Equal(t, "Bearer test-key", request.auth)
			require.True(t, gjson.Get(request.body, "messages").Exists())
			require.False(t, gjson.Get(request.body, "service_tier").Exists())
			require.Equal(t, []types.RelayFormat{types.RelayFormatClaude, types.RelayFormatOpenAI}, info.RequestConversionChain)
		})
	}
}

func TestOpenCodeGeminiHelperRoutesGoChatOnlyModel(t *testing.T) {
	service.InitHttpClient()
	sent := make(chan struct{ path, body string }, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sent <- struct{ path, body string }{r.URL.Path, string(body)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":{"message":"probe","type":"server_error"}}`)
	}))
	defer upstream.Close()
	path := "/v1beta/models/space-bunny-free:generateContent"
	raw := `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenCode)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL+"/zen/go")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "space-bunny-free")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
	info := &relaycommon.RelayInfo{OriginModelName: "space-bunny-free", RequestURLPath: path, RelayFormat: types.RelayFormatGemini, RelayMode: relayconstant.RelayModeGemini}
	var req dto.GeminiChatRequest
	require.NoError(t, common.Unmarshal([]byte(raw), &req))
	info.Request = &req
	info.InitRequestConversionChain()
	require.NotNil(t, GeminiHelper(c, info))
	request := <-sent
	require.Equal(t, "/zen/go/v1/chat/completions", request.path)
	require.True(t, gjson.Get(request.body, "messages").Exists())
	require.False(t, gjson.Get(request.body, "contents").Exists())
	require.Equal(t, []types.RelayFormat{types.RelayFormatGemini, types.RelayFormatOpenAI}, info.RequestConversionChain)
}
