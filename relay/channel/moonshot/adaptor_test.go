package moonshot

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func moonshotPointer[T any](value T) *T {
	return &value
}

func TestGetRequestURLUsesInputProtocolForKimiCoding(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		baseURL     string
		relayFormat types.RelayFormat
		want        string
	}{
		{
			name:        "special base OpenAI",
			baseURL:     "kimi-coding-plan",
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://api.kimi.com/coding/v1/chat/completions",
		},
		{
			name:        "special base Claude",
			baseURL:     "kimi-coding-plan",
			relayFormat: types.RelayFormatClaude,
			want:        "https://api.kimi.com/coding/v1/messages",
		},
		{
			name:        "custom coding base OpenAI",
			baseURL:     "https://example.com/coding",
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://example.com/coding/v1/chat/completions",
		},
		{
			name:        "custom coding base Claude",
			baseURL:     "https://example.com/coding",
			relayFormat: types.RelayFormatClaude,
			want:        "https://example.com/coding/v1/messages",
		},
		{
			name:        "custom coding v1 base OpenAI",
			baseURL:     "https://example.com/coding/v1",
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://example.com/coding/v1/chat/completions",
		},
		{
			name:        "custom coding v1 base Claude",
			baseURL:     "https://example.com/coding/v1",
			relayFormat: types.RelayFormatClaude,
			want:        "https://example.com/coding/v1/messages",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			adaptor := &Adaptor{}
			got, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
				RelayMode:   relayconstant.RelayModeChatCompletions,
				RelayFormat: testCase.relayFormat,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: testCase.baseURL,
				},
			})
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("GetRequestURL() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestGetRequestURLKeepsOpenAIEndpointForRegularMoonshot(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"https://api.moonshot.cn",
		"https://api.moonshot.cn/v1",
		"https://api.moonshot.cn/v1/",
	}
	for _, baseURL := range testCases {
		baseURL := baseURL
		t.Run(baseURL, func(t *testing.T) {
			t.Parallel()

			adaptor := &Adaptor{}
			got, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
				RelayMode:   relayconstant.RelayModeChatCompletions,
				RelayFormat: types.RelayFormatOpenAI,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: baseURL,
				},
			})
			if err != nil {
				t.Fatalf("GetRequestURL returned error: %v", err)
			}

			want := "https://api.moonshot.cn/v1/chat/completions"
			if got != want {
				t.Fatalf("GetRequestURL() = %q, want %q", got, want)
			}
		})
	}
}

func TestGetRequestURLUsesConvertedOpenAIEndpointForKimiCodingResponses(t *testing.T) {
	t.Parallel()

	got, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		RelayMode:               relayconstant.RelayModeResponses,
		RelayFormat:             types.RelayFormatOpenAIResponses,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "kimi-coding-plan",
		},
	})
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	const want = "https://api.kimi.com/coding/v1/chat/completions"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestConvertOpenAIRequestKeepsOpenAIRequestForKimiCodingPlan(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	adaptor := &Adaptor{}
	request := &dto.GeneralOpenAIRequest{
		Model: "kimi-k2.5",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	}
	converted, err := adaptor.ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "kimi-k2.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "kimi-coding-plan",
			UpstreamModelName: "kimi-k2.5",
		},
	}, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	if converted != request {
		t.Fatalf("ConvertOpenAIRequest returned %T, want the original OpenAI request", converted)
	}
}

func TestSetupRequestHeaderKeepsBearerAuthForKimiCodingPlan(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	adaptor := &Adaptor{}
	headers := make(http.Header)
	err := adaptor.SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         "kimi-key",
			ChannelBaseUrl: "kimi-coding-plan",
		},
	})
	if err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if headers.Get("Authorization") != "Bearer kimi-key" {
		t.Fatalf("Authorization = %q, want Bearer kimi-key", headers.Get("Authorization"))
	}
	if got := headers.Get("Content-Type"); got != gin.MIMEJSON {
		t.Fatalf("Content-Type = %q, want %q", got, gin.MIMEJSON)
	}
	if got := headers.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q, want text/event-stream", got)
	}
}

func TestGetRequestURLUsesOpenAIEndpointForKimiCodingPassThrough(t *testing.T) {
	adaptor := &Adaptor{}
	got, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "kimi-coding-plan",
			ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: true},
		},
	})
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	const want = "https://api.kimi.com/coding/v1/chat/completions"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestSetupRequestHeaderAppliesKimiCLICompatibilityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("User-Agent", "client-kimi-agent")
	c.Request.Header.Set("X-Msh-Platform", "client-platform")
	c.Request.Header.Set("X-Msh-Version", "client-version")
	c.Request.Header.Set("anthropic-version", "client-version")
	c.Request.Header.Set("anthropic-beta", "client-beta")
	c.Request.Header.Set("anthropic-dangerous-direct-browser-access", "false")
	c.Request.Header.Set("x-app", "client")
	c.Request.Header.Set("Content-Type", "text/plain")
	c.Request.Header.Set("Accept", "application/x-client")
	c.Request.Header.Set("Cookie", "session=client-cookie")
	c.Request.Header.Set("X-Client-Only", "do-not-forward")
	c.Request.Header.Set("X-Claude-Code-Session-Id", "session-123")

	headers := c.Request.Header.Clone()
	err := (&Adaptor{}).SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         "kimi-key",
			ChannelBaseUrl: "kimi-coding-plan",
		},
	})
	if err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	wantHeaders := map[string]string{
		"Authorization":  "Bearer kimi-key",
		"Content-Type":   gin.MIMEJSON,
		"Accept":         gin.MIMEJSON,
		"User-Agent":     "kimi-code-cli/0.34.0",
		"X-Msh-Platform": "kimi_code_cli",
		"X-Msh-Version":  "0.34.0",
	}
	for name, want := range wantHeaders {
		if got := headers.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"X-Msh-Device-Name", "X-Msh-Device-Model", "X-Msh-Os-Version", "X-Msh-Device-Id"} {
		if headers.Get(name) == "" {
			t.Errorf("%s should not be empty", name)
		}
	}
	for _, name := range []string{"anthropic-version", "anthropic-beta", "anthropic-dangerous-direct-browser-access", "x-app"} {
		if got := headers.Get(name); got != "" {
			t.Errorf("%s = %q, want no Anthropic header on OpenAI request", name, got)
		}
	}
	for _, name := range []string{"Cookie", "X-Client-Only", "X-Claude-Code-Session-Id"} {
		if got := headers.Get(name); got != "" {
			t.Errorf("%s = %q, want client header removed from upstream request", name, got)
		}
	}
	if got := c.Request.Header.Get("X-Claude-Code-Session-Id"); got != "session-123" {
		t.Fatalf("incoming session header = %q, want it preserved for local affinity/cache lookup", got)
	}
}

func TestSetupRequestHeaderUsesClaudeHeadersForKimiCodingClaudeRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("anthropic-beta", "client-beta")

	headers := make(http.Header)
	err := (&Adaptor{}).SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         "kimi-key",
			ChannelBaseUrl: "kimi-coding-plan",
		},
	})
	if err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if got := headers.Get("Authorization"); got != "Bearer kimi-key" {
		t.Fatalf("Authorization = %q, want Bearer kimi-key", got)
	}
	if got := headers.Get("User-Agent"); !strings.HasPrefix(got, "claude-cli/") {
		t.Fatalf("User-Agent = %q, want Claude Code fingerprint", got)
	}
	if got := headers.Get("anthropic-version"); got != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want 2023-06-01", got)
	}
	if got := headers.Get("anthropic-beta"); got != "client-beta" {
		t.Fatalf("anthropic-beta = %q, want client-beta", got)
	}
	for _, name := range kimiCLIHeaderNames[1:] {
		if got := headers.Get(name); got != "" {
			t.Errorf("%s = %q, want no Kimi Code header on Claude request", name, got)
		}
	}
}

func TestDoRequestAppliesKimiCodingHeaderPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	testCases := []struct {
		name              string
		headerOverride    map[string]interface{}
		wantUserAgent     string
		wantAnthropicBeta string
		wantConfigured    string
	}{
		{
			name:              "empty override uses built-in headers",
			headerOverride:    map[string]interface{}{},
			wantUserAgent:     "kimi-code-cli/0.34.0",
			wantAnthropicBeta: "",
		},
		{
			name: "explicit override wins",
			headerOverride: map[string]interface{}{
				"User-Agent":     "configured-agent",
				"anthropic-beta": "configured-beta",
				"X-Configured":   "configured-value",
			},
			wantUserAgent:     "configured-agent",
			wantAnthropicBeta: "configured-beta",
			wantConfigured:    "configured-value",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var upstreamHeaders http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamHeaders = r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
			c.Request.Header.Set("User-Agent", "client-kimi-agent")
			c.Request.Header.Set("anthropic-beta", "client-beta")
			c.Request.Header.Set("Content-Type", "text/plain")
			c.Request.Header.Set("Accept", "application/x-client")
			c.Request.Header.Set("Cookie", "session=client-cookie")
			c.Request.Header.Set("X-Client-Only", "do-not-forward")
			c.Request.Header.Set("X-Claude-Code-Session-Id", "session-123")

			result, err := (&Adaptor{}).DoRequest(c, &relaycommon.RelayInfo{
				RelayMode:   relayconstant.RelayModeChatCompletions,
				RelayFormat: types.RelayFormatOpenAI,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiKey:          "kimi-key",
					ChannelBaseUrl:  server.URL + "/coding",
					HeadersOverride: testCase.headerOverride,
				},
			}, strings.NewReader(`{}`))
			if err != nil {
				t.Fatalf("DoRequest returned error: %v", err)
			}
			resp, ok := result.(*http.Response)
			if !ok {
				t.Fatalf("DoRequest returned %T, want *http.Response", result)
			}
			defer resp.Body.Close()

			if got := upstreamHeaders.Get("User-Agent"); got != testCase.wantUserAgent {
				t.Fatalf("User-Agent = %q, want %q", got, testCase.wantUserAgent)
			}
			if got := upstreamHeaders.Get("anthropic-beta"); got != testCase.wantAnthropicBeta {
				t.Fatalf("anthropic-beta = %q, want %q", got, testCase.wantAnthropicBeta)
			}
			if got := upstreamHeaders.Get("X-Configured"); got != testCase.wantConfigured {
				t.Fatalf("X-Configured = %q, want %q", got, testCase.wantConfigured)
			}
			if got := upstreamHeaders.Get("Content-Type"); got != gin.MIMEJSON {
				t.Fatalf("Content-Type = %q, want %q", got, gin.MIMEJSON)
			}
			if got := upstreamHeaders.Get("Accept"); got != gin.MIMEJSON {
				t.Fatalf("Accept = %q, want %q", got, gin.MIMEJSON)
			}
			for _, name := range []string{"Cookie", "X-Client-Only", "X-Claude-Code-Session-Id"} {
				if got := upstreamHeaders.Get(name); got != "" {
					t.Errorf("%s = %q, want client header removed from upstream request", name, got)
				}
			}
			if got := c.Request.Header.Get("X-Claude-Code-Session-Id"); got != "session-123" {
				t.Fatalf("incoming session header = %q, want it preserved for local affinity/cache lookup", got)
			}
		})
	}
}

func TestSetupRequestHeaderPassThroughUsesClientKimiHeadersWithoutDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("User-Agent", "custom-kimi-client")
	c.Request.Header.Set("X-Msh-Version", "custom-version")

	headers := make(http.Header)
	err := (&Adaptor{}).SetupRequestHeader(c, &headers, &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         "kimi-key",
			ChannelBaseUrl: "kimi-coding-plan",
			ChannelSetting: dto.ChannelSettings{PassThroughBodyEnabled: true},
		},
	})
	if err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if got := headers.Get("User-Agent"); got != "custom-kimi-client" {
		t.Fatalf("User-Agent = %q, want client value", got)
	}
	if got := headers.Get("X-Msh-Version"); got != "custom-version" {
		t.Fatalf("X-Msh-Version = %q, want client value", got)
	}
	for _, name := range []string{"X-Msh-Platform", "X-Msh-Device-Id", "anthropic-version", "x-app"} {
		if got := headers.Get(name); got != "" {
			t.Errorf("%s = %q, want no fixed pass-through value", name, got)
		}
	}
}

func TestLocalKimiDeviceIDUsesOfficialCLIHome(t *testing.T) {
	homeDir := t.TempDir()
	const want = "11111111-2222-4333-8444-555555555555"
	if err := os.WriteFile(filepath.Join(homeDir, "device_id"), []byte(want+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("KIMI_CODE_HOME", homeDir)

	if got := localKimiDeviceID("host", "Linux test x64"); got != want {
		t.Fatalf("localKimiDeviceID() = %q, want %q", got, want)
	}
}

func TestConvertOpenAIRequestNormalizesKimiK3Parameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	request := &dto.GeneralOpenAIRequest{
		Model:               "kimi-k3",
		MaxTokens:           moonshotPointer[uint](4096),
		ReasoningEffort:     "high",
		Temperature:         moonshotPointer(0.7),
		TopP:                moonshotPointer(0.8),
		N:                   moonshotPointer(2),
		FrequencyPenalty:    moonshotPointer(0.2),
		PresencePenalty:     moonshotPointer(0.3),
		THINKING:            []byte(`{"type":"enabled"}`),
		Reasoning:           []byte(`{"effort":"high"}`),
		MaxCompletionTokens: nil,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "kimi-k3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.moonshot.cn",
			UpstreamModelName: "kimi-k3",
		},
	}, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if got.MaxTokens != nil || got.MaxCompletionTokens == nil || *got.MaxCompletionTokens != 4096 {
		t.Fatalf("max token fields were not normalized: max_tokens=%v max_completion_tokens=%v", got.MaxTokens, got.MaxCompletionTokens)
	}
	if got.ReasoningEffort != "" || got.THINKING != nil || got.Reasoning != nil {
		t.Fatalf("K3 reasoning fields were not normalized: effort=%q thinking=%s reasoning=%s", got.ReasoningEffort, got.THINKING, got.Reasoning)
	}
	if got.Temperature != nil || got.TopP != nil || got.N != nil || got.FrequencyPenalty != nil || got.PresencePenalty != nil {
		t.Fatal("conflicting fixed K3 sampling parameters should be removed")
	}
}

func TestConvertOpenAIRequestKeepsValidKimiFixedParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	request := &dto.GeneralOpenAIRequest{
		Model:            "kimi-k3",
		ReasoningEffort:  "MAX",
		Temperature:      moonshotPointer(1.0),
		TopP:             moonshotPointer(0.95),
		N:                moonshotPointer(1),
		FrequencyPenalty: moonshotPointer(0.0),
		PresencePenalty:  moonshotPointer(0.0),
		ToolChoice:       "required",
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "kimi-k3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.moonshot.cn",
			UpstreamModelName: "kimi-k3",
		},
	}, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got := converted.(*dto.GeneralOpenAIRequest)
	if got.ReasoningEffort != "max" || got.Temperature == nil || got.TopP == nil || got.N == nil || got.FrequencyPenalty == nil || got.PresencePenalty == nil {
		t.Fatal("valid K3 fixed parameters should be preserved")
	}
	if got.ToolChoice != "required" {
		t.Fatalf("K3 tool_choice = %#v, want required", got.ToolChoice)
	}
}

func TestConvertOpenAIRequestNormalizesKimiK27Parameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	request := &dto.GeneralOpenAIRequest{
		Model:           "kimi-k2.7-code",
		ReasoningEffort: "high",
		Temperature:     moonshotPointer(0.5),
		ToolChoice:      "required",
		THINKING:        []byte(`{"type":"disabled","keep":"invalid"}`),
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "kimi-k2.7-code",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.moonshot.cn",
			UpstreamModelName: "kimi-k2.7-code",
		},
	}, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got := converted.(*dto.GeneralOpenAIRequest)
	if got.ReasoningEffort != "" || got.Temperature != nil || got.ToolChoice != nil || got.THINKING != nil {
		t.Fatalf("K2.7 incompatible fields were not removed: effort=%q temperature=%v tool_choice=%#v thinking=%s", got.ReasoningEffort, got.Temperature, got.ToolChoice, got.THINKING)
	}
}

func TestConvertOpenAIRequestPreservesKimiK27EnabledThinkingKeepAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	request := &dto.GeneralOpenAIRequest{
		Model:      "kimi-k2.7-code",
		ToolChoice: "auto",
		THINKING:   []byte(`{"type":"enabled","keep":"all","budget_tokens":9999}`),
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "kimi-k2.7-code",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.moonshot.cn",
			UpstreamModelName: "kimi-k2.7-code",
		},
	}, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	got := converted.(*dto.GeneralOpenAIRequest)
	var thinking map[string]any
	if err := common.Unmarshal(got.THINKING, &thinking); err != nil {
		t.Fatalf("normalized thinking is invalid: %v", err)
	}
	if len(thinking) != 2 || thinking["type"] != "enabled" || thinking["keep"] != "all" {
		t.Fatalf("normalized thinking = %#v, want enabled + keep all", thinking)
	}
	if got.ToolChoice != "auto" {
		t.Fatalf("tool_choice = %#v, want auto", got.ToolChoice)
	}
}

func TestConvertOpenAIRequestPassThroughSkipsKimiNormalizationAndConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	request := &dto.GeneralOpenAIRequest{
		Model:           "kimi-for-coding",
		ReasoningEffort: "high",
		Temperature:     moonshotPointer(0.5),
		ToolChoice:      "required",
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "kimi-for-coding",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "kimi-coding-plan",
			UpstreamModelName: "kimi-for-coding",
			ChannelSetting:    dto.ChannelSettings{PassThroughBodyEnabled: true},
		},
	}, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	if converted != request {
		t.Fatalf("pass-through returned %T, want the original request pointer", converted)
	}
	if request.ReasoningEffort != "high" || request.Temperature == nil || request.ToolChoice != "required" {
		t.Fatal("pass-through request was unexpectedly normalized")
	}
}

func TestConvertOpenAIRequestKeepsKimiCodingModelsInOpenAIFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name      string
		model     string
		wantModel string
		maxTokens *uint
		reasoning string
	}{
		{name: "K3", model: "k3", wantModel: kimiK3ShortContextModel, reasoning: "max"},
		{name: "K2.7", model: "kimi-for-coding", wantModel: "kimi-for-coding", maxTokens: moonshotPointer[uint](2048)},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
			request := &dto.GeneralOpenAIRequest{
				Model:           testCase.model,
				Messages:        []dto.Message{{Role: "user", Content: "hi"}},
				MaxTokens:       testCase.maxTokens,
				ReasoningEffort: testCase.reasoning,
				Temperature:     moonshotPointer(0.5),
			}
			converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
				RelayMode:       relayconstant.RelayModeChatCompletions,
				RelayFormat:     types.RelayFormatOpenAI,
				OriginModelName: testCase.model,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl:    "kimi-coding-plan",
					UpstreamModelName: testCase.model,
				},
			}, request)
			if err != nil {
				t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
			}
			if converted != request {
				t.Fatalf("ConvertOpenAIRequest returned %T, want the original OpenAI request", converted)
			}
			if request.Model != testCase.wantModel {
				t.Fatalf("model = %q, want %q", request.Model, testCase.wantModel)
			}
			if request.Temperature != nil {
				t.Fatal("Kimi Coding OpenAI normalization should omit conflicting temperature")
			}
			if testCase.model == "k3" && request.ReasoningEffort != "max" {
				t.Fatalf("reasoning_effort = %q, want max", request.ReasoningEffort)
			}
		})
	}
}

func TestKimiK3MessageParametersSurviveTypedMarshal(t *testing.T) {
	request := dto.GeneralOpenAIRequest{
		Model: "kimi-k3",
		Messages: []dto.Message{
			{
				Role:    "assistant",
				Partial: moonshotPointer(true),
				Tools:   []byte(`[{"type":"function","function":{"name":"calculate"}}]`),
			},
		},
	}
	data, err := common.Marshal(request)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var payload map[string]any
	if err := common.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	messages := payload["messages"].([]any)
	message := messages[0].(map[string]any)
	if message["partial"] != true || message["tools"] == nil {
		t.Fatalf("K3 message parameters were dropped: %#v", message)
	}
	if _, exists := message["content"]; exists {
		t.Fatalf("content should be omitted for dynamic tool declarations: %#v", message)
	}
}

func TestConvertOpenAIRequestUsesKimiK3ShortContextByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "k3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "kimi-coding-plan",
			UpstreamModelName: "k3",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    "k3",
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	request, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if request.Model != kimiK3ShortContextModel {
		t.Fatalf("upstream model = %q, want %q", request.Model, kimiK3ShortContextModel)
	}
	if value, ok := c.Get(kimiK3FallbackContextKey); !ok || value != true {
		t.Fatalf("K3 fallback marker = %#v, want true", value)
	}
	if len(info.RequestModelRoutingChain) != 1 || info.RequestModelRoutingChain[0] != kimiK3ShortContextRouteLabel {
		t.Fatalf("model routing chain = %#v, want automatic 256K route", info.RequestModelRoutingChain)
	}
}

func TestConvertOpenAIRequestKeepsExplicitKimiK3ShortContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: kimiK3ShortContextModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "kimi-coding-plan",
			UpstreamModelName: kimiK3ShortContextModel,
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    kimiK3ShortContextModel,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	request, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if request.Model != kimiK3ShortContextModel {
		t.Fatalf("upstream model = %q, want %q", request.Model, kimiK3ShortContextModel)
	}
	if _, ok := c.Get(kimiK3FallbackContextKey); ok {
		t.Fatal("explicit k3-256k request should not enable full-context fallback")
	}
	if len(info.RequestModelRoutingChain) != 0 {
		t.Fatalf("explicit k3-256k model routing chain = %#v, want empty", info.RequestModelRoutingChain)
	}
}

func TestConvertOpenAIRequestDoesNotAutoRouteKimiK3OutsideKimiCoding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	request := &dto.GeneralOpenAIRequest{
		Model:    kimiK3FullContextModel,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: kimiK3FullContextModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.moonshot.cn",
			UpstreamModelName: kimiK3FullContextModel,
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	if converted != request {
		t.Fatalf("ConvertOpenAIRequest returned %T, want the original request pointer", converted)
	}
	if request.Model != kimiK3FullContextModel {
		t.Fatalf("upstream model = %q, want %q", request.Model, kimiK3FullContextModel)
	}
	if _, ok := c.Get(kimiK3FallbackContextKey); ok {
		t.Fatal("regular Moonshot k3 request should not enable Kimi Coding fallback")
	}
}

func TestConvertOpenAIRequestDoesNotAutoRouteKimiK3InPassThroughMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	request := &dto.GeneralOpenAIRequest{
		Model:    kimiK3FullContextModel,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: kimiK3FullContextModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "kimi-coding-plan",
			UpstreamModelName: kimiK3FullContextModel,
			ChannelSetting:    dto.ChannelSettings{PassThroughBodyEnabled: true},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	if converted != request {
		t.Fatalf("ConvertOpenAIRequest returned %T, want the original request pointer", converted)
	}
	if request.Model != kimiK3FullContextModel {
		t.Fatalf("upstream model = %q, want %q", request.Model, kimiK3FullContextModel)
	}
	if _, ok := c.Get(kimiK3FallbackContextKey); ok {
		t.Fatal("pass-through k3 request should not enable full-context fallback")
	}
}

func TestConvertClaudeRequestUsesKimiK3ShortContextByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatClaude,
		OriginModelName: "k3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "kimi-coding-plan",
			UpstreamModelName: "k3",
		},
	}

	converted, err := (&Adaptor{}).ConvertClaudeRequest(c, info, &dto.ClaudeRequest{Model: "k3"})
	if err != nil {
		t.Fatalf("ConvertClaudeRequest returned error: %v", err)
	}
	request, ok := converted.(*dto.ClaudeRequest)
	if !ok {
		t.Fatalf("ConvertClaudeRequest returned %T, want *dto.ClaudeRequest", converted)
	}
	if request.Model != kimiK3ShortContextModel {
		t.Fatalf("upstream model = %q, want %q", request.Model, kimiK3ShortContextModel)
	}
	if value, ok := c.Get(kimiK3FallbackContextKey); !ok || value != true {
		t.Fatalf("K3 fallback marker = %#v, want true", value)
	}
	if len(info.RequestModelRoutingChain) != 1 || info.RequestModelRoutingChain[0] != kimiK3ShortContextRouteLabel {
		t.Fatalf("model routing chain = %#v, want automatic 256K route", info.RequestModelRoutingChain)
	}
}

func TestDoRequestFallsBackToKimiK3AfterShortContextOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	var requestBodies [][]byte
	var requestPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll returned error: %v", err)
		}
		requestBodies = append(requestBodies, body)
		w.Header().Set("Content-Type", "application/json")
		if len(requestBodies) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid request: Your request exceeded model token limit: 262144 (requested: 300000)"}}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "k3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "kimi-key",
			ChannelBaseUrl:    server.URL + "/coding",
			UpstreamModelName: "k3",
		},
	}
	requestBody := convertKimiK3OpenAIRequestForDoRequest(t, c, info)
	outboundBody, size, closer, err := relaycommon.NewOutboundJSONBody(requestBody)
	if err != nil {
		t.Fatalf("NewOutboundJSONBody returned error: %v", err)
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	resp, err := (&Adaptor{}).DoRequest(c, info, outboundBody)
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}
	response, ok := resp.(*http.Response)
	if !ok {
		t.Fatalf("DoRequest returned %T, want *http.Response", resp)
	}
	defer response.Body.Close()
	if len(requestBodies) != 2 {
		t.Fatalf("upstream request count = %d, want 2", len(requestBodies))
	}
	for _, path := range requestPaths {
		if path != "/coding/v1/chat/completions" {
			t.Fatalf("upstream request path = %q, want /coding/v1/chat/completions", path)
		}
	}

	var first, second map[string]any
	if err := common.Unmarshal(requestBodies[0], &first); err != nil {
		t.Fatalf("first request is invalid JSON: %v", err)
	}
	if err := common.Unmarshal(requestBodies[1], &second); err != nil {
		t.Fatalf("second request is invalid JSON: %v", err)
	}
	if first["model"] != kimiK3ShortContextModel || second["model"] != kimiK3FullContextModel {
		t.Fatalf("models = %q then %q, want %q then %q", first["model"], second["model"], kimiK3ShortContextModel, kimiK3FullContextModel)
	}
	wantRouting := []string{kimiK3ShortContextRouteLabel, kimiK3FullContextFallbackRouteLabel}
	if len(info.RequestModelRoutingChain) != len(wantRouting) ||
		info.RequestModelRoutingChain[0] != wantRouting[0] ||
		info.RequestModelRoutingChain[1] != wantRouting[1] {
		t.Fatalf("model routing chain = %#v, want %#v", info.RequestModelRoutingChain, wantRouting)
	}
}

func TestDoRequestDoesNotFallbackForExplicitKimiK3ShortContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Your request exceeded model token limit: 262144"}}`))
	}))
	defer server.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: kimiK3ShortContextModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "kimi-key",
			ChannelBaseUrl:    server.URL + "/coding",
			UpstreamModelName: kimiK3ShortContextModel,
		},
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    kimiK3ShortContextModel,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	request, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if request.Model != kimiK3ShortContextModel {
		t.Fatalf("converted model = %q, want %q", request.Model, kimiK3ShortContextModel)
	}
	requestBody, err := common.Marshal(converted)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	outboundBody, size, closer, err := relaycommon.NewOutboundJSONBody(requestBody)
	if err != nil {
		t.Fatalf("NewOutboundJSONBody returned error: %v", err)
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	resp, err := (&Adaptor{}).DoRequest(c, info, outboundBody)
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}
	response, ok := resp.(*http.Response)
	if !ok {
		t.Fatalf("DoRequest returned %T, want *http.Response", resp)
	}
	defer response.Body.Close()
	if requestCount != 1 {
		t.Fatalf("upstream request count = %d, want 1", requestCount)
	}
}

func TestDoRequestDoesNotFallbackForOtherKimiError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"unsupported image url"}}`))
	}))
	defer server.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "kimi-key",
			ChannelBaseUrl:    server.URL + "/coding",
			UpstreamModelName: "k3",
		},
	}
	requestBody := convertKimiK3OpenAIRequestForDoRequest(t, c, info)
	outboundBody, size, closer, err := relaycommon.NewOutboundJSONBody(requestBody)
	if err != nil {
		t.Fatalf("NewOutboundJSONBody returned error: %v", err)
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size
	resp, err := (&Adaptor{}).DoRequest(c, info, outboundBody)
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}
	response, ok := resp.(*http.Response)
	if !ok {
		t.Fatalf("DoRequest returned %T, want *http.Response", resp)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		t.Fatalf("ReadAll returned error: %v", readErr)
	}
	if !strings.Contains(string(body), "unsupported image url") {
		t.Fatalf("response body = %q, want original upstream error", body)
	}
	if requestCount != 1 {
		t.Fatalf("upstream request count = %d, want 1", requestCount)
	}
}

func TestKimiK3ShortContextOverflowDetection(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{
			name:       "documented 256K token overflow",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"Invalid request: Your request exceeded model token limit: 262144 (requested: 558009)"}}`,
			want:       true,
		},
		{
			name:       "two megabyte message size limit",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"total message size 5943865 exceeds limit 2097152"}}`,
		},
		{
			name:       "different token limit",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"Your request exceeded model token limit: 131072"}}`,
		},
		{
			name:       "requested tokens equal 256K but model limit differs",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"Your request exceeded model token limit: 131072 (requested: 262144)"}}`,
		},
		{
			name:       "unsupported media",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"unsupported video input"}}`,
		},
		{
			name:       "permission error",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"message":"Your current plan supports only kimi-k3 up to 256K context"}}`,
			want:       true,
		},
		{
			name:       "short context model plan limit",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"message":"k3-256k supports only 256K context"}}`,
			want:       true,
		},
		{
			name:       "same text with non-400 status",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":{"message":"Your request exceeded model token limit: 262144"}}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: testCase.statusCode,
				Body:       io.NopCloser(strings.NewReader(testCase.body)),
			}
			if got := isKimiK3ShortContextOverflow(resp); got != testCase.want {
				t.Fatalf("isKimiK3ShortContextOverflow() = %t, want %t", got, testCase.want)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("ReadAll returned error: %v", err)
			}
			if string(body) != testCase.body {
				t.Fatalf("response body = %q, want %q", body, testCase.body)
			}
		})
	}
}

func convertKimiK3OpenAIRequestForDoRequest(t *testing.T, c *gin.Context, info *relaycommon.RelayInfo) []byte {
	t.Helper()
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    kimiK3FullContextModel,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	if request, ok := converted.(*dto.GeneralOpenAIRequest); !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	} else if request.Model != kimiK3ShortContextModel {
		t.Fatalf("converted model = %q, want %q", request.Model, kimiK3ShortContextModel)
	}
	body, err := common.Marshal(converted)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	return body
}

func TestDoRequestRewritesAutoRouteOverflowTo429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Your current plan supports only kimi-k3 up to 256K context"}}`))
	}))
	defer server.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "k3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "kimi-key",
			ChannelBaseUrl:    server.URL + "/coding",
			UpstreamModelName: "k3",
		},
	}
	requestBody := convertKimiK3OpenAIRequestForDoRequest(t, c, info)
	outboundBody, size, closer, err := relaycommon.NewOutboundJSONBody(requestBody)
	if err != nil {
		t.Fatalf("NewOutboundJSONBody returned error: %v", err)
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	resp, err := (&Adaptor{}).DoRequest(c, info, outboundBody)
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}
	response, ok := resp.(*http.Response)
	if !ok {
		t.Fatalf("DoRequest returned %T, want *http.Response", resp)
	}
	defer response.Body.Close()
	// 第一次 k3-256k 溢出后 inline 兜底发 k3，上游仍 401 限档 → 两次请求
	if requestCount != 2 {
		t.Fatalf("upstream request count = %d, want 2", requestCount)
	}
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status code = %d, want %d", response.StatusCode, http.StatusTooManyRequests)
	}
	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		t.Fatalf("ReadAll returned error: %v", readErr)
	}
	if !strings.Contains(string(body), "supports only") {
		t.Fatalf("response body = %q, want upstream message preserved", body)
	}
}

func TestDoRequestKeepsOriginalStatusForExplicitKimiK3ShortContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"k3-256k supports only 256K context"}}`))
	}))
	defer server.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: kimiK3ShortContextModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "kimi-key",
			ChannelBaseUrl:    server.URL + "/coding",
			UpstreamModelName: kimiK3ShortContextModel,
		},
	}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    kimiK3ShortContextModel,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	requestBody, err := common.Marshal(converted)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	outboundBody, size, closer, err := relaycommon.NewOutboundJSONBody(requestBody)
	if err != nil {
		t.Fatalf("NewOutboundJSONBody returned error: %v", err)
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	resp, err := (&Adaptor{}).DoRequest(c, info, outboundBody)
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}
	response, ok := resp.(*http.Response)
	if !ok {
		t.Fatalf("DoRequest returned %T, want *http.Response", resp)
	}
	defer response.Body.Close()
	if requestCount != 1 {
		t.Fatalf("upstream request count = %d, want 1 (no fallback for explicit k3-256k)", requestCount)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want original %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func TestConvertOpenAIRequestRoutesK3DirectWhenEstimateExceedsCutoff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "k3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "kimi-key",
			ChannelBaseUrl:    "https://example.com/coding",
			UpstreamModelName: "k3",
		},
	}
	info.SetEstimatePromptTokens(kimiK3ProactiveFullContextCutoff + 1)

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model:    "k3",
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	request, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if request.Model != kimiK3FullContextModel {
		t.Fatalf("converted model = %q, want direct %q", request.Model, kimiK3FullContextModel)
	}
	if value, marked := c.Get(kimiK3FallbackContextKey); marked && value == true {
		t.Fatalf("fallback marker should not be set for direct k3 route")
	}
	if len(info.RequestModelRoutingChain) != 1 || info.RequestModelRoutingChain[0] != kimiK3DirectContextRouteLabel {
		t.Fatalf("model routing chain = %#v, want direct route label", info.RequestModelRoutingChain)
	}

	// 阈值边界：恰好等于 cutoff 时仍走 256K 降级
	info2 := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "k3",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "kimi-key",
			ChannelBaseUrl:    "https://example.com/coding",
			UpstreamModelName: "k3",
		},
	}
	info2.SetEstimatePromptTokens(kimiK3ProactiveFullContextCutoff)
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	converted2, err := (&Adaptor{}).ConvertOpenAIRequest(c2, info2, &dto.GeneralOpenAIRequest{
		Model:    "k3",
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	request2 := converted2.(*dto.GeneralOpenAIRequest)
	if request2.Model != kimiK3ShortContextModel {
		t.Fatalf("converted model at cutoff = %q, want %q", request2.Model, kimiK3ShortContextModel)
	}
}
