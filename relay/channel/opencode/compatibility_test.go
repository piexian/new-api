package opencode

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
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFreeAndAlphaModelsUseTheirNativeRoutes(t *testing.T) {
	for _, tt := range []struct {
		model string
		path  string
		mode  int
	}{
		{"mimo-v2.5-free", "/v1/chat/completions", requestModeOpenAI},
		{"mimo-v2.6-flash-free", "/v1/chat/completions", requestModeOpenAI},
		{"ling-3.0-flash-fin-free", "/v1/chat/completions", requestModeOpenAI},
		{"muse-spark-1.3-contributor-free", "/v1/responses", requestModeResponses},
		{"omen-alpha", "/v1/chat/completions", requestModeOpenAI},
		{"custom-alpha", "/v1/chat/completions", requestModeOpenAI},
	} {
		t.Run(tt.model, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			request := &dto.GeneralOpenAIRequest{Model: tt.model, Messages: []dto.Message{{Role: "user", Content: "hi"}}}
			info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, request, nil)
			require.NoError(t, err)
			require.Equal(t, relayconstant.RelayModeUnknown, info.RelayMode)
			info.ChannelMeta = &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCode, ChannelBaseUrl: constant.OpenCodeZenBaseURLAlias, UpstreamModelName: tt.model}
			a := &Adaptor{}
			a.Init(info)
			require.Equal(t, tt.mode, a.RequestMode)
			converted, err := a.ConvertOpenAIRequest(c, info, request)
			require.NoError(t, err)
			if tt.mode == requestModeResponses {
				require.IsType(t, &dto.OpenAIResponsesRequest{}, converted)
			} else {
				require.IsType(t, &dto.GeneralOpenAIRequest{}, converted)
			}
			url, err := a.GetRequestURL(info)
			require.NoError(t, err)
			require.Equal(t, constant.OpenCodeZenBaseURL+tt.path, url)
		})
	}
}

func TestChatViaResponsesUnknownModeConvertsResponseBack(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeUnknown,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: constant.OpenCodeZenBaseURLAlias, UpstreamModelName: "muse-spark-1.3-contributor-free"},
	}
	a := &Adaptor{}
	a.Init(info)
	body := `{"id":"resp-test","object":"response","model":"muse-spark-1.3-contributor-free","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}`
	usage, apiErr := a.DoResponse(c, &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, info)
	require.Nil(t, apiErr)
	require.Equal(t, 7, usage.(*dto.Usage).TotalTokens)
	var result dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
	require.Equal(t, "chat.completion", result.Object)
	require.Len(t, result.Choices, 1)
	require.Equal(t, "hello", result.Choices[0].Message.Content)
}

func TestSystemOneRejectsChatAndGo(t *testing.T) {
	for _, model := range constant.OpenCodeZenSystemOneModels {
		info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: model}}
		a := &Adaptor{}
		a.Init(info)
		_, err := a.GetRequestURL(info)
		require.ErrorContains(t, err, "/v1/systemone")
		info.ChannelSetting.PassThroughBodyEnabled = true
		_, err = a.ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{Model: model})
		require.ErrorContains(t, err, "/v1/systemone")
	}
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatTypeSafe, ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: constant.OpenCodeGoBaseURLAlias}}
	a := &Adaptor{}
	a.Init(info)
	_, err := a.GetRequestURL(info)
	require.ErrorContains(t, err, "not supported")
	info.ChannelBaseUrl = constant.OpenCodeZenBaseURLAlias
	info.IsStream = true
	_, err = a.GetRequestURL(info)
	require.ErrorContains(t, err, "does not support streaming")
	require.Equal(t, constant.OpenCodeZenBaseURL, NormalizeRoot(constant.OpenCodeZenBaseURL+"/v1/systemone"))
}

func TestSystemOneRoundTripPreservesBodyUsageAndHeaderOverrides(t *testing.T) {
	service.InitHttpClient()
	payload := `{"model":"jev-1.13-free","state":{"n":0,"flag":false},"questions":{"q":{"type":"noul","instructions":"ok?"}},"extra":null}`
	response := `{"model":"jev-1.13-free","answers":{"q":{"type":"noul","noul":0}},"usage":{"input_tokens":7,"output_tokens":0},"extra":false}`
	type observed struct {
		path, body string
		headers    http.Header
	}
	got := make(chan observed, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- observed{r.URL.Path, string(body), r.Header.Clone()}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer upstream.Close()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(payload))
	info := &relaycommon.RelayInfo{
		RelayFormat:    types.RelayFormatTypeSafe,
		RelayMode:      relayconstant.RelayModeTypeSafeNative,
		RequestURLPath: "/v1/systemone",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenCode,
			ChannelBaseUrl:    upstream.URL + "/zen",
			UpstreamModelName: "jev-1.13-free",
			ApiKey:            "test-key",
			HeadersOverride:   map[string]interface{}{"User-Agent": "custom-agent", "x-opencode-project": "custom-project"},
		},
	}
	a := &Adaptor{}
	a.Init(info)
	result, err := a.DoRequest(c, info, strings.NewReader(payload))
	require.NoError(t, err)
	request := <-got
	require.Equal(t, "/zen/v1/systemone", request.path)
	require.Equal(t, payload, request.body)
	require.Equal(t, "Bearer test-key", request.headers.Get("Authorization"))
	require.Equal(t, "application/json", request.headers.Get("Content-Type"))
	require.Equal(t, "custom-agent", request.headers.Get("User-Agent"))
	require.Equal(t, "custom-project", request.headers.Get("x-opencode-project"))
	require.Regexp(t, openCodeSessionIDPattern, request.headers.Get("x-opencode-session"))
	require.Regexp(t, openCodeRequestIDPattern, request.headers.Get("x-opencode-request"))
	usage, apiErr := a.DoResponse(c, result.(*http.Response), info)
	require.Nil(t, apiErr)
	require.Equal(t, &dto.Usage{PromptTokens: 7, TotalTokens: 7}, usage)
	require.Equal(t, response, recorder.Body.String())
}

func TestNewModelChatRoundTripToNativeEndpoints(t *testing.T) {
	service.InitHttpClient()
	for _, tc := range []struct {
		model, path, reply, authHeader, requestField, excludedField, basePath string
	}{
		{"gpt-6-sol", "/v1/responses", `{"id":"resp_test","object":"response","model":"gpt-6-sol","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`, "Authorization", "input", "messages", "/zen"},
		{"claude-opus-5-5", "/v1/messages", `{"id":"msg_test","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`, "x-api-key", "messages", "input", "/zen"},
		{"space-bunny-free", "/v1/chat/completions", `{"id":"chat_test","object":"chat.completion","model":"space-bunny-free","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`, "Authorization", "messages", "input", "/zen"},
		{"qwen3.8-flash", "/v1/messages", `{"id":"msg_test","type":"message","role":"assistant","model":"qwen3.8-flash","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`, "x-api-key", "messages", "input", "/zen/go"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			type observed struct {
				path, body string
				headers    http.Header
			}
			sent := make(chan observed, 1)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				sent <- observed{r.URL.Path, string(body), r.Header.Clone()}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.reply)
			}))
			defer upstream.Close()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{
				RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeUnknown, RequestURLPath: "/v1/chat/completions",
				ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCode, ChannelBaseUrl: upstream.URL + tc.basePath, UpstreamModelName: tc.model, ApiKey: "test-key"},
			}
			a := &Adaptor{}
			a.Init(info)
			converted, err := a.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{Model: tc.model, Messages: []dto.Message{{Role: "user", Content: "hi"}}})
			require.NoError(t, err)
			body, err := common.Marshal(converted)
			require.NoError(t, err)
			result, err := a.DoRequest(c, info, strings.NewReader(string(body)))
			require.NoError(t, err)
			request := <-sent
			require.Equal(t, tc.basePath+tc.path, request.path)
			require.Equal(t, tc.model, gjson.Get(request.body, "model").String())
			require.True(t, gjson.Get(request.body, tc.requestField).Exists())
			require.False(t, gjson.Get(request.body, tc.excludedField).Exists())
			require.False(t, gjson.Get(request.body, "stream").Bool())
			require.NotEmpty(t, request.headers.Get(tc.authHeader))
			if tc.model == "space-bunny-free" {
				require.False(t, gjson.Get(request.body, "tools").Exists())
			}
			usage, apiErr := a.DoResponse(c, result.(*http.Response), info)
			require.Nil(t, apiErr)
			require.Equal(t, 5, usage.(*dto.Usage).TotalTokens)
			require.Equal(t, "chat.completion", gjson.GetBytes(recorder.Body.Bytes(), "object").String())
			require.Equal(t, "OK", gjson.GetBytes(recorder.Body.Bytes(), "choices.0.message.content").String())
		})
	}
}

func TestGoSpaceBunnyResponsesRoundTripViaChat(t *testing.T) {
	service.InitHttpClient()
	type observed struct{ path, body string }
	sent := make(chan observed, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sent <- observed{r.URL.Path, string(body)}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chat_test","object":"chat.completion","model":"space-bunny-free","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)
	}))
	defer server.Close()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAIResponses, RelayMode: relayconstant.RelayModeResponses, RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCode, ChannelBaseUrl: server.URL + "/zen/go", UpstreamModelName: "space-bunny-free", ApiKey: "test", ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: true}},
	}
	a := &Adaptor{}
	a.Init(info)
	converted, err := a.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{Model: "space-bunny-free", Input: []byte(`[{"role":"user","content":"hi"}]`)})
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)
	result, err := a.DoRequest(c, info, strings.NewReader(string(body)))
	require.NoError(t, err)
	request := <-sent
	require.Equal(t, "/zen/go/v1/chat/completions", request.path)
	require.True(t, gjson.Get(request.body, "messages").Exists())
	require.False(t, gjson.Get(request.body, "input").Exists())
	usage, apiErr := a.DoResponse(c, result.(*http.Response), info)
	require.Nil(t, apiErr)
	require.Equal(t, 5, usage.(*dto.Usage).TotalTokens)
	require.Equal(t, "response", gjson.GetBytes(recorder.Body.Bytes(), "object").String())
	require.Equal(t, "OK", gjson.GetBytes(recorder.Body.Bytes(), "output.0.content.0.text").String())
}

func TestGoSpaceBunnyClaudeRoundTripViaChat(t *testing.T) {
	service.InitHttpClient()
	for _, fromChannelTest := range []bool{false, true} {
		t.Run(fmt.Sprintf("channel-test=%t", fromChannelTest), func(t *testing.T) {
			sent := make(chan struct{ path, body string }, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				sent <- struct{ path, body string }{r.URL.Path, string(body)}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"chat_test","object":"chat.completion","model":"space-bunny-free","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)
			}))
			defer server.Close()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude, RelayMode: relayconstant.RelayModeUnknown, RequestURLPath: "/v1/messages", ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenCode, ChannelBaseUrl: server.URL + "/zen/go", UpstreamModelName: "space-bunny-free", ApiKey: "test", ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: true}}}
			a := &Adaptor{}
			a.Init(info)
			var converted any
			var err error
			if fromChannelTest {
				converted, err = a.ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{Model: "space-bunny-free", Messages: []dto.Message{{Role: "user", Content: "hi"}}})
			} else {
				maxTokens := uint(16)
				converted, err = a.ConvertClaudeRequest(c, info, &dto.ClaudeRequest{Model: "space-bunny-free", MaxTokens: &maxTokens, Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}}})
			}
			require.NoError(t, err)
			payload, err := common.Marshal(converted)
			require.NoError(t, err)
			resp, err := a.DoRequest(c, info, strings.NewReader(string(payload)))
			require.NoError(t, err)
			request := <-sent
			require.Equal(t, "/zen/go/v1/chat/completions", request.path)
			require.True(t, gjson.Get(request.body, "messages").Exists())
			require.NotEmpty(t, gjson.Get(request.body, "model").String())
			usage, apiErr := a.DoResponse(c, resp.(*http.Response), info)
			require.Nil(t, apiErr)
			require.Equal(t, 5, usage.(*dto.Usage).TotalTokens)
			require.Equal(t, "message", gjson.GetBytes(recorder.Body.Bytes(), "type").String())
			require.Equal(t, "OK", gjson.GetBytes(recorder.Body.Bytes(), "content.0.text").String())
		})
	}
}

func TestChatViaResponsesUnknownModeConvertsStreamBack(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat:        types.RelayFormatOpenAI,
		RelayMode:          relayconstant.RelayModeUnknown,
		IsStream:           true,
		ShouldIncludeUsage: true,
		DisablePing:        true,
		ChannelMeta:        &relaycommon.ChannelMeta{ChannelBaseUrl: constant.OpenCodeZenBaseURLAlias, UpstreamModelName: "muse-spark-1.3-contributor-free"},
	}
	a := &Adaptor{}
	a.Init(info)
	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_test","model":"muse-spark-1.3-contributor-free"}}`,
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		`data: [DONE]`,
		``,
	}, "\n")
	usage, apiErr := a.DoResponse(c, &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}, info)
	require.Nil(t, apiErr)
	require.Equal(t, 7, usage.(*dto.Usage).TotalTokens)
	require.Contains(t, recorder.Body.String(), `"content":"hello"`)
	require.Contains(t, recorder.Body.String(), `"object":"chat.completion.chunk"`)
	require.Contains(t, recorder.Body.String(), `data: [DONE]`)
}
