package moonshot

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func moonshotPointer[T any](value T) *T {
	return &value
}

func TestGetRequestURLUsesMessagesForKimiCodingPlan(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "special base",
			baseURL: "kimi-coding-plan",
			want:    "https://api.kimi.com/coding/v1/messages",
		},
		{
			name:    "custom coding base",
			baseURL: "https://example.com/coding",
			want:    "https://example.com/coding/v1/messages",
		},
		{
			name:    "custom coding v1 base",
			baseURL: "https://example.com/coding/v1",
			want:    "https://example.com/coding/v1/messages",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			adaptor := &Adaptor{}
			got, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
				RelayMode:   relayconstant.RelayModeChatCompletions,
				RelayFormat: types.RelayFormatOpenAI,
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

func TestConvertOpenAIRequestReturnsClaudeRequestForKimiCodingPlan(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	adaptor := &Adaptor{}
	converted, err := adaptor.ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "kimi-k2.5",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "kimi-coding-plan",
			UpstreamModelName: "kimi-k2.5",
		},
	}, &dto.GeneralOpenAIRequest{
		Model: "kimi-k2.5",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}
	if _, ok := converted.(*dto.ClaudeRequest); !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.ClaudeRequest", converted)
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
	c.Request.Header.Set("anthropic-beta", "client-beta")

	headers := make(http.Header)
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
		"Authorization":     "Bearer kimi-key",
		"User-Agent":        "kimi-code-cli/0.27.0",
		"X-Msh-Platform":    "kimi_code_cli",
		"X-Msh-Version":     "0.27.0",
		"anthropic-version": "2023-06-01",
		"anthropic-beta":    "client-beta",
		"anthropic-dangerous-direct-browser-access": "true",
		"x-app": "cli",
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

func TestConvertOpenAIRequestAdaptsKimiCodingModelsToClaudeThinking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		name       string
		model      string
		wantType   string
		wantEffort string
		wantBudget bool
		toolChoice any
		maxTokens  *uint
		reasoning  string
	}{
		{name: "K3 adaptive", model: "k3", wantType: "adaptive", wantEffort: "max", toolChoice: "required", reasoning: "max"},
		{name: "K2.7 enabled", model: "kimi-for-coding", wantType: "enabled", wantBudget: true, toolChoice: "auto", maxTokens: moonshotPointer[uint](2048)},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
			converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, &relaycommon.RelayInfo{
				RelayMode:       relayconstant.RelayModeChatCompletions,
				RelayFormat:     types.RelayFormatOpenAI,
				OriginModelName: testCase.model,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl:    "kimi-coding-plan",
					UpstreamModelName: testCase.model,
				},
			}, &dto.GeneralOpenAIRequest{
				Model:           testCase.model,
				Messages:        []dto.Message{{Role: "user", Content: "hi"}},
				MaxTokens:       testCase.maxTokens,
				ReasoningEffort: testCase.reasoning,
				Temperature:     moonshotPointer(0.5),
				ToolChoice:      testCase.toolChoice,
			})
			if err != nil {
				t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
			}
			got, ok := converted.(*dto.ClaudeRequest)
			if !ok {
				t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.ClaudeRequest", converted)
			}
			if got.Thinking == nil || got.Thinking.Type != testCase.wantType {
				t.Fatalf("thinking = %#v, want type %q", got.Thinking, testCase.wantType)
			}
			if testCase.wantBudget && got.Thinking.BudgetTokens == nil {
				t.Fatal("K2.7 Claude thinking should include budget_tokens")
			}
			if got.Temperature != nil || got.TopP != nil || got.TopK != nil {
				t.Fatal("Kimi Coding Claude thinking should omit sampling parameters")
			}
			if testCase.wantEffort != "" {
				var outputConfig map[string]string
				if err := common.Unmarshal(got.OutputConfig, &outputConfig); err != nil {
					t.Fatalf("output_config is invalid: %v", err)
				}
				if outputConfig["effort"] != testCase.wantEffort {
					t.Fatalf("output_config effort = %q, want %q", outputConfig["effort"], testCase.wantEffort)
				}
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
