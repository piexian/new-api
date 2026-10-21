package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func boolPtr(v bool) *bool { return &v }

func TestResolveChannelProtocolSupportDefaults(t *testing.T) {
	// OpenAI defaults: Chat + Responses ON, Messages + Gemini OFF.
	openaiDefault := relaycommon.ResolveChannelProtocolSupport(constant.ChannelTypeOpenAI, "", dto.ChannelSettings{})
	assert.Equal(t, relaycommon.ResolvedProtocolSupport{
		Chat:      true,
		Responses: true,
		Messages:  false,
		Gemini:    false,
	}, openaiDefault)

	// OpenAI-compatible upstreams relay /v1/responses natively through the openai adaptor.
	for _, channelType := range []int{
		constant.ChannelTypeCustom,
		constant.ChannelTypeOpenRouter,
		constant.ChannelTypeXinference,
		constant.ChannelTypeKilo,
		constant.ChannelTypeGroq,
		constant.ChannelTypeVLLM,
		constant.ChannelTypeSGLang,
		constant.ChannelCloudflare,
	} {
		support := relaycommon.ResolveChannelProtocolSupport(channelType, "", dto.ChannelSettings{})
		assert.True(t, support.Chat, "channel type %d", channelType)
		assert.True(t, support.Responses, "channel type %d", channelType)
	}

	// Own adaptors without a native /v1/responses endpoint.
	for _, channelType := range []int{
		constant.ChannelTypeSiliconFlow,
		constant.ChannelTypeMistral,
		constant.ChannelTypeCerebras,
		constant.ChannelTypeBaidu,
	} {
		support := relaycommon.ResolveChannelProtocolSupport(channelType, "", dto.ChannelSettings{})
		assert.True(t, support.Chat, "channel type %d", channelType)
		assert.False(t, support.Responses, "channel type %d", channelType)
	}

	// Legacy conversion flags no longer change protocol support.
	legacy := relaycommon.ResolveChannelProtocolSupport(constant.ChannelTypeOpenAI, "", dto.ChannelSettings{
		ResponsesViaChatFallbackEnabled:   boolPtr(true),
		ChatCompletionsToResponsesEnabled: boolPtr(true),
	})
	assert.Equal(t, openaiDefault, legacy)

	// Explicit protocol_support overrides defaults.
	explicit := relaycommon.ResolveChannelProtocolSupport(constant.ChannelTypeOpenAI, "", dto.ChannelSettings{
		ProtocolSupport: &dto.ChannelProtocolSupport{
			Chat:      boolPtr(false),
			Responses: boolPtr(true),
			Messages:  boolPtr(true),
			Gemini:    boolPtr(false),
		},
	})
	assert.Equal(t, relaycommon.ResolvedProtocolSupport{
		Chat:      false,
		Responses: true,
		Messages:  true,
		Gemini:    false,
	}, explicit)
}

func TestPreferredTargetProtocols(t *testing.T) {
	all := relaycommon.ResolvedProtocolSupport{Chat: true, Responses: true, Messages: true, Gemini: true}

	// Native protocol wins over the Chat-first order.
	assert.Equal(t, []relaycommon.ChannelProtocol{
		relaycommon.ProtocolGemini, relaycommon.ProtocolChat, relaycommon.ProtocolResponses,
	}, all.PreferredTargetProtocols(relaycommon.ProtocolMessages, relaycommon.ProtocolGemini, false))
	assert.Equal(t, []relaycommon.ChannelProtocol{
		relaycommon.ProtocolChat, relaycommon.ProtocolResponses, relaycommon.ProtocolGemini,
	}, all.PreferredTargetProtocols(relaycommon.ProtocolMessages, "", false))

	messagesOnly := relaycommon.ResolvedProtocolSupport{Messages: true}
	// Claude responses cannot be rendered as Gemini.
	assert.Empty(t, messagesOnly.PreferredTargetProtocols(relaycommon.ProtocolGemini, "", false))
	// Responses via Messages is non-stream only.
	assert.Empty(t, messagesOnly.PreferredTargetProtocols(relaycommon.ProtocolResponses, "", true))
	assert.Equal(t, []relaycommon.ChannelProtocol{relaycommon.ProtocolMessages},
		messagesOnly.PreferredTargetProtocols(relaycommon.ProtocolResponses, "", false))
}

func TestShouldInterceptInboundProtocol(t *testing.T) {
	// count_tokens cannot be converted: rejected whenever Messages is off.
	assert.True(t, relaycommon.ShouldInterceptInboundProtocol(constant.ChannelTypeOpenAI, "", dto.ChannelSettings{},
		false, types.RelayFormatClaude, relayconstant.RelayModeClaudeCountTokens))
	// Disabled protocol with body pass-through is rejected; without it the adaptor converts.
	assert.True(t, relaycommon.ShouldInterceptInboundProtocol(constant.ChannelTypeOpenAI, "", dto.ChannelSettings{},
		true, types.RelayFormatClaude, relayconstant.RelayModeUnknown))
	assert.False(t, relaycommon.ShouldInterceptInboundProtocol(constant.ChannelTypeOpenAI, "", dto.ChannelSettings{},
		false, types.RelayFormatClaude, relayconstant.RelayModeUnknown))

	// Cohere never passes bodies through, so its internal Responses conversion is kept.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses,
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCohere,
			ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: true},
		},
	}
	assert.Nil(t, checkInboundProtocolInterception(c, info))
}

func TestPlanInboundRoute(t *testing.T) {
	newInfo := func(channelType int, setting dto.ChannelSettings) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    channelType,
			ChannelSetting: setting,
		}}
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	// Off only by the channel-type default: keep the adaptor's built-in conversion.
	route, apiErr := planInboundRoute(c, newInfo(constant.ChannelTypeOpenAI, dto.ChannelSettings{}), relaycommon.ProtocolMessages)
	require.Nil(t, apiErr)
	assert.Equal(t, inboundRoute{}, route)

	// Explicitly disabled: convert to the enabled protocols in preference order.
	route, apiErr = planInboundRoute(c, newInfo(constant.ChannelTypeNewAPI, dto.ChannelSettings{
		ProtocolSupport: &dto.ChannelProtocolSupport{Messages: boolPtr(false)},
	}), relaycommon.ProtocolMessages)
	require.Nil(t, apiErr)
	assert.True(t, route.dispatch)
	assert.Equal(t, []relaycommon.ChannelProtocol{
		relaycommon.ProtocolChat, relaycommon.ProtocolResponses, relaycommon.ProtocolGemini,
	}, route.targets)

	// Explicitly enabled on a channel that does not support it by default: generic adaptor.
	route, apiErr = planInboundRoute(c, newInfo(constant.ChannelTypeOpenAI, dto.ChannelSettings{
		ProtocolSupport: &dto.ChannelProtocolSupport{Messages: boolPtr(true)},
	}), relaycommon.ProtocolMessages)
	require.Nil(t, apiErr)
	assert.False(t, route.dispatch)
	assert.IsType(t, newGenericProtocolAdaptor(relaycommon.ProtocolMessages), route.adaptor)

	// Gemini channel with OpenAI compat: Messages disabled keeps the native Gemini conversion.
	route, apiErr = planInboundRoute(c, newInfo(constant.ChannelTypeGemini, dto.ChannelSettings{
		UpstreamOpenAICompatEnabled: true,
		ProtocolSupport:             &dto.ChannelProtocolSupport{Messages: boolPtr(false)},
	}), relaycommon.ProtocolMessages)
	require.Nil(t, apiErr)
	assert.Equal(t, inboundRoute{}, route)

	// Gemini inbound with only Messages left enabled has no renderable target.
	_, apiErr = planInboundRoute(c, newInfo(constant.ChannelTypeNewAPI, dto.ChannelSettings{
		ProtocolSupport: &dto.ChannelProtocolSupport{
			Chat:      boolPtr(false),
			Responses: boolPtr(false),
			Gemini:    boolPtr(false),
		},
	}), relaycommon.ProtocolGemini)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestRequiresGenericAdaptor(t *testing.T) {
	newInfo := func(channelType int, ps *dto.ChannelProtocolSupport) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    channelType,
			ChannelSetting: dto.ChannelSettings{ProtocolSupport: ps},
		}}
	}
	// Explicitly enabling a protocol the OpenAI channel lacks by default uses the generic adaptor.
	assert.True(t, requiresGenericAdaptor(newInfo(constant.ChannelTypeOpenAI, &dto.ChannelProtocolSupport{Messages: boolPtr(true)}), relaycommon.ProtocolMessages))
	// Merely off by default (e.g. the Responses->Chat fallback on Anthropic) keeps the channel adaptor.
	assert.False(t, requiresGenericAdaptor(newInfo(constant.ChannelTypeAnthropic, nil), relaycommon.ProtocolChat))
	// Native-protocol channels keep their own auth even when a protocol is explicitly enabled.
	for _, channelType := range []int{constant.ChannelTypeVertexAi, constant.ChannelTypeGemini, constant.ChannelTypeAws, constant.ChannelTypeCodex} {
		assert.False(t, requiresGenericAdaptor(newInfo(channelType, &dto.ChannelProtocolSupport{
			Chat: boolPtr(true), Messages: boolPtr(true),
		}), relaycommon.ProtocolChat), "channel type %d", channelType)
	}
}

func TestProtocolSupportDefaultResponsesPassThroughOnOpenAICompatibleChannel(t *testing.T) {
	service.InitHttpClient()
	sent := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":{"message":"probe","type":"upstream_error"}}`)
	}))
	defer upstream.Close()

	raw := `{"model":"gpt-4o","input":"hello"}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenRouter)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal([]byte(raw), &req))
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-4o",
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		Request:         &req,
	}
	info.InitRequestConversionChain()

	apiErr := ResponsesHelper(c, info)
	require.NotNil(t, apiErr)
	assert.Equal(t, "/v1/responses", <-sent)
}

func TestProtocolSupportChatTargetUsesOpenAIWireProtocol(t *testing.T) {
	service.InitHttpClient()
	newClaudeRequest := func(t *testing.T, channelType int, upstreamURL string, ps *dto.ChannelProtocolSupport) (*gin.Context, *relaycommon.RelayInfo) {
		raw := `{"model":"llama3","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		common.SetContextKey(c, constant.ContextKeyChannelType, channelType)
		common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstreamURL)
		common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
		common.SetContextKey(c, constant.ContextKeyOriginalModel, "llama3")
		common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{ProtocolSupport: ps})
		var req dto.ClaudeRequest
		require.NoError(t, common.Unmarshal([]byte(raw), &req))
		info := &relaycommon.RelayInfo{
			OriginModelName: "llama3",
			RequestURLPath:  "/v1/messages",
			RelayFormat:     types.RelayFormatClaude,
			RelayMode:       relayconstant.RelayModeUnknown,
			Request:         &req,
		}
		info.InitRequestConversionChain()
		return c, info
	}

	// Ollama's native /api/chat cannot be rendered as Claude, so with Messages
	// disabled the Chat target goes to its OpenAI-compatible endpoint.
	sent := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":{"message":"probe","type":"upstream_error"}}`)
	}))
	defer upstream.Close()
	c, info := newClaudeRequest(t, constant.ChannelTypeOllama, upstream.URL+"/v1", &dto.ChannelProtocolSupport{Messages: boolPtr(false)})
	require.NotNil(t, ClaudeHelper(c, info))
	assert.Equal(t, "/v1/chat/completions", <-sent)

	// Channels with a private Chat protocol and no other target fail before reaching upstream.
	var upstreamHits atomic.Int32
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer blocked.Close()
	c, info = newClaudeRequest(t, constant.ChannelTypeCohere, blocked.URL, &dto.ChannelProtocolSupport{Messages: boolPtr(false)})
	apiErr := ClaudeHelper(c, info)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Zero(t, upstreamHits.Load())
}

func TestProtocolSupportLocalInterceptionWhenPassThroughEnabled(t *testing.T) {
	service.InitHttpClient()
	var upstreamHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	setting := dto.ChannelSettings{
		PassThroughBodyEnabled: true,
		ProtocolSupport: &dto.ChannelProtocolSupport{
			Chat:      boolPtr(true),
			Responses: boolPtr(false),
			Messages:  boolPtr(false),
			Gemini:    boolPtr(false),
		},
	}

	raw := `{"model":"gpt-4o","input":"hello"}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, setting)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal([]byte(raw), &req))
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-4o",
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		Request:         &req,
	}
	info.InitRequestConversionChain()

	apiErr := ResponsesHelper(c, info)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.True(t, types.IsSkipRetryError(apiErr))
	assert.Zero(t, upstreamHits.Load())
}

func TestProtocolSupportDisabledResponsesConvertsToChatOnOpenAIChannel(t *testing.T) {
	service.InitHttpClient()
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

	setting := dto.ChannelSettings{
		ProtocolSupport: &dto.ChannelProtocolSupport{
			Chat:      boolPtr(true),
			Responses: boolPtr(false),
		},
	}
	raw := `{"model":"gpt-4o","input":[{"role":"user","content":"hi"}]}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, setting)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal([]byte(raw), &req))
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-4o",
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		Request:         &req,
	}
	info.InitRequestConversionChain()

	apiErr := ResponsesHelper(c, info)
	require.NotNil(t, apiErr)
	obs := <-sent
	assert.Equal(t, "/v1/chat/completions", obs.path)
	assert.True(t, gjson.Get(obs.body, "messages").Exists())
	assert.False(t, gjson.Get(obs.body, "input").Exists())
}

func TestProtocolSupportDisabledChatConvertsToResponses(t *testing.T) {
	service.InitHttpClient()
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

	setting := dto.ChannelSettings{
		ProtocolSupport: &dto.ChannelProtocolSupport{
			Chat:      boolPtr(false),
			Responses: boolPtr(true),
		},
	}
	raw := `{"model":"gpt-5","messages":[{"role":"user","content":"hi"}]}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-5")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, setting)

	var req dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal([]byte(raw), &req))
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		RequestURLPath:  "/v1/chat/completions",
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		Request:         &req,
	}
	info.InitRequestConversionChain()

	apiErr := TextHelper(c, info)
	require.NotNil(t, apiErr)
	obs := <-sent
	assert.Equal(t, "/v1/responses", obs.path)
	assert.True(t, gjson.Get(obs.body, "input").Exists())
	assert.False(t, gjson.Get(obs.body, "messages").Exists())
}

func TestProtocolSupportDisabledMessagesConvertsToChatOnMultiRouteChannel(t *testing.T) {
	service.InitHttpClient()
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

	// NewAPI (type=78) supports Messages by default; disabling Messages must
	// route /v1/messages to /v1/chat/completions instead of /v1/messages.
	setting := dto.ChannelSettings{
		ProtocolSupport: &dto.ChannelProtocolSupport{
			Chat:     boolPtr(true),
			Messages: boolPtr(false),
		},
	}
	raw := `{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeNewAPI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-sonnet-4-5")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, setting)

	var req dto.ClaudeRequest
	require.NoError(t, common.Unmarshal([]byte(raw), &req))
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-sonnet-4-5",
		RequestURLPath:  "/v1/messages",
		RelayFormat:     types.RelayFormatClaude,
		RelayMode:       relayconstant.RelayModeUnknown,
		Request:         &req,
	}
	info.InitRequestConversionChain()

	apiErr := ClaudeHelper(c, info)
	require.NotNil(t, apiErr)
	obs := <-sent
	assert.Equal(t, "/v1/chat/completions", obs.path)
	assert.True(t, gjson.Get(obs.body, "messages").Exists())
}

func TestChatCompletionsViaResponsesIgnoresStaleFinalRelayFormat(t *testing.T) {
	service.InitHttpClient()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"resp_1","object":"response","created_at":1,"model":"gpt-5","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello","annotations":[]}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`)
	}))
	defer upstream.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-5")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})

	var req dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hi"}]}`), &req))
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		RequestURLPath:  "/v1/chat/completions",
		RelayFormat:     types.RelayFormatOpenAI,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		Request:         &req,
		// Left over from a previous attempt that converted to Chat on another channel.
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
	}
	info.InitChannelMeta(c)
	info.InitRequestConversionChain()

	usage, apiErr := chatCompletionsViaResponses(c, info, newGenericProtocolAdaptor(relaycommon.ProtocolChat), &req)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 5, usage.TotalTokens)
	assert.Equal(t, "hello", gjson.Get(recorder.Body.String(), "choices.0.message.content").String())
}
