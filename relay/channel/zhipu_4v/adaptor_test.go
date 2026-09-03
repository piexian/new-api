package zhipu_4v

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
	"github.com/google/uuid"
)

func TestGetRequestURLUsesClaudeCompatibleEndpointForClaudeModel(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet",
			ChannelBaseUrl:    "https://open.bigmodel.cn",
			ChannelType:       constant.ChannelTypeZhipu_v4,
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://open.bigmodel.cn/api/anthropic/v1/messages"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestGetRequestURLUsesClaudeCompatibleEndpointForCodingPlan(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	for _, testCase := range []struct {
		baseURL string
		want    string
	}{
		{baseURL: "glm-coding-plan", want: "https://open.bigmodel.cn/api/anthropic/v1/messages"},
		{baseURL: "glm-coding-plan/", want: "https://open.bigmodel.cn/api/anthropic/v1/messages"},
		{baseURL: "https://open.bigmodel.cn/api/coding/paas/v4", want: "https://open.bigmodel.cn/api/anthropic/v1/messages"},
		{baseURL: "glm-coding-plan-international", want: "https://api.z.ai/api/anthropic/v1/messages"},
		{baseURL: "https://api.z.ai/api/coding/paas/v4", want: "https://api.z.ai/api/anthropic/v1/messages"},
	} {
		info := &relaycommon.RelayInfo{
			RelayFormat: types.RelayFormatOpenAI,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl:    testCase.baseURL,
				UpstreamModelName: "glm-4.6",
			},
		}

		got, err := adaptor.GetRequestURL(info)
		if err != nil {
			t.Fatalf("GetRequestURL(%q) returned error: %v", testCase.baseURL, err)
		}
		if got != testCase.want {
			t.Fatalf("GetRequestURL(%q) = %q, want %q", testCase.baseURL, got, testCase.want)
		}
	}
}

func TestSetupRequestHeaderUsesClaudeCompatibleHeadersForClaudeModel(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	adaptor := &Adaptor{}
	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet",
			ApiKey:            "zhipu-v4-key",
			ChannelType:       constant.ChannelTypeZhipu_v4,
		},
	}

	if err := adaptor.SetupRequestHeader(c, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if headers.Get("x-api-key") != "zhipu-v4-key" {
		t.Fatalf("x-api-key = %q, want %q", headers.Get("x-api-key"), "zhipu-v4-key")
	}
	if headers.Get("anthropic-version") != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want %q", headers.Get("anthropic-version"), "2023-06-01")
	}
	if headers.Get("Authorization") != "" {
		t.Fatalf("Authorization = %q, want empty for Claude-compatible requests", headers.Get("Authorization"))
	}
}

func TestSetupRequestHeaderAddsZCodeFingerprintForCodingPlan(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "glm-coding-plan",
			ApiKey:            "coding-plan-key",
			UpstreamModelName: "glm-4.6",
			ChannelSetting:    dto.ChannelSettings{ZcodeModeEnabled: true},
		},
	}

	if err := (&Adaptor{}).SetupRequestHeader(c, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if headers.Get("Authorization") != "" {
		t.Fatalf("Authorization = %q, want empty for Claude-compatible requests", headers.Get("Authorization"))
	}
	if headers.Get("x-api-key") != "coding-plan-key" {
		t.Fatalf("x-api-key = %q, want coding-plan-key", headers.Get("x-api-key"))
	}
	zcodeFingerprint := map[string]string{
		"User-Agent":           "ZCode/" + zcodeClientVersion,
		"HTTP-Referer":         "https://zcode.z.ai",
		"X-Title":              "Z Code@electron",
		"X-ZCode-App-Version":  zcodeClientVersion,
		"X-Platform":           "win32-x64",
		"X-Release-Channel":    "production",
		"X-Client-Language":    "zh-CN",
		"X-Client-Timezone":    "Asia/Shanghai",
		"X-Os-Category":        "windows",
		"x-zcode-session-type": "main",
	}
	for name, want := range zcodeFingerprint {
		if value := headers.Get(name); value != want {
			t.Fatalf("%s = %q, want %q", name, value, want)
		}
	}
	for _, name := range []string{"x-request-id", "x-zcode-trace-id", "x-query-id", "x-session-id"} {
		if value := headers.Get(name); value == "" {
			t.Fatalf("%s is empty", name)
		}
	}
	for _, name := range []string{"X-Stainless-Runtime", "X-Stainless-Package-Version", "x-app", "X-Claude-Code-Session-Id", "x-client-request-id", "anthropic-client-platform", "anthropic-client-version", "anthropic-dangerous-direct-browser-access"} {
		if value := headers.Get(name); value != "" {
			t.Fatalf("%s = %q, want cleared from Claude fingerprint", name, value)
		}
	}
	for _, name := range []string{"x-request-id", "x-zcode-trace-id", "x-query-id", "x-session-id"} {
		if _, err := uuid.Parse(headers.Get(name)); err != nil {
			t.Fatalf("%s = %q is not a UUID: %v", name, headers.Get(name), err)
		}
	}
}

func TestConvertOpenAIRequestReturnsClaudeRequestForClaudeModel(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "claude-3-7-sonnet",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet",
			ChannelType:       constant.ChannelTypeZhipu_v4,
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model: "claude-3-7-sonnet",
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	}

	converted, err := adaptor.ConvertOpenAIRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}

	if _, ok := converted.(*dto.ClaudeRequest); !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.ClaudeRequest", converted)
	}
}

func TestConvertOpenAIResponsesRequestUsesClaudeRequestForCodingPlan(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	stream := true
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "glm-coding-plan",
			UpstreamModelName: "glm-4.6",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:  "glm-4.6",
		Input:  []byte(`"hello"`),
		Stream: &stream,
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}
	claudeReq, ok := converted.(*dto.ClaudeRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.ClaudeRequest", converted)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatClaude {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatClaude)
	}
	if claudeReq.Stream == nil || !*claudeReq.Stream {
		t.Fatalf("stream = %#v, want true", claudeReq.Stream)
	}
}

func TestConvertInternalChatResponsesRequestKeepsOpenAIFormat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "glm-coding-plan",
			UpstreamModelName: "glm-4.6",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "glm-4.6",
		Input: []byte(`"hello"`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}
	if _, ok := converted.(*dto.GeneralOpenAIRequest); !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatOpenAI {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatOpenAI)
	}
}

func TestSetupRequestHeaderPassthroughForResponsesOpenAIPathWhenZcodeModeOff(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "glm-coding-plan",
			ApiKey:            "coding-plan-key",
			UpstreamModelName: "glm-5.3-flash",
		},
	}

	// ZCode 模式关闭时 Responses→OpenAI chat 直连保持透传，不注入任何 ZCode 头。
	info.RelayMode = relayconstant.RelayModeResponses
	info.FinalRequestRelayFormat = types.RelayFormatOpenAI

	if err := (&Adaptor{}).SetupRequestHeader(c, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if headers.Get("Authorization") != "Bearer coding-plan-key" {
		t.Fatalf("Authorization = %q, want Bearer coding-plan-key", headers.Get("Authorization"))
	}
	for _, name := range []string{"User-Agent", "HTTP-Referer", "X-ZCode-App-Version", "X-Release-Channel", "x-zcode-session-type", "x-request-id"} {
		if value := headers.Get(name); value != "" {
			t.Fatalf("%s = %q, want empty for passthrough when ZCode mode is off", name, value)
		}
	}
}

func TestSetupRequestHeaderSkipsZCodeFingerprintForEmbeddings(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "glm-coding-plan",
			ApiKey:            "coding-plan-key",
			UpstreamModelName: "embedding-3",
		},
	}
	info.RelayMode = relayconstant.RelayModeEmbeddings

	if err := (&Adaptor{}).SetupRequestHeader(c, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if headers.Get("Authorization") != "Bearer coding-plan-key" {
		t.Fatalf("Authorization = %q, want Bearer coding-plan-key", headers.Get("Authorization"))
	}
	for _, name := range []string{"User-Agent", "X-ZCode-App-Version", "X-Release-Channel", "x-zcode-session-type"} {
		if value := headers.Get(name); value != "" {
			t.Fatalf("%s = %q, want empty for non-LLM coding plan requests", name, value)
		}
	}
}

func TestSetupRequestHeaderTraceOnlyForCodingPlanWhenZcodeModeOff(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "glm-coding-plan",
			ApiKey:            "coding-plan-key",
			UpstreamModelName: "glm-4.6",
		},
	}

	if err := (&Adaptor{}).SetupRequestHeader(c, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if headers.Get("x-api-key") != "coding-plan-key" {
		t.Fatalf("x-api-key = %q, want coding-plan-key", headers.Get("x-api-key"))
	}
	// tracing 头保留（原有透传逻辑），ZCode 设备指纹不注入
	for _, name := range []string{"x-request-id", "x-zcode-trace-id", "x-query-id", "x-session-id"} {
		if value := headers.Get(name); value == "" {
			t.Fatalf("%s is empty", name)
		}
		if _, err := uuid.Parse(headers.Get(name)); err != nil {
			t.Fatalf("%s = %q is not a UUID: %v", name, headers.Get(name), err)
		}
	}
	if headers.Get("x-zcode-session-type") != "main" {
		t.Fatalf("x-zcode-session-type = %q, want main", headers.Get("x-zcode-session-type"))
	}
	for _, name := range []string{"User-Agent", "HTTP-Referer", "X-ZCode-App-Version", "X-Release-Channel", "X-Platform", "X-Os-Category"} {
		if value := headers.Get(name); value != "" {
			t.Fatalf("%s = %q, want empty when ZCode mode is off", name, value)
		}
	}
}

func TestConvertOpenAIResponsesRequestForcesClaudeWhenZcodeModeOn(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "glm-coding-plan",
			UpstreamModelName: "glm-5.3-flash",
			ChannelSetting:    dto.ChannelSettings{ZcodeModeEnabled: true},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "glm-5.3-flash",
		Input: []byte(`"hello"`),
	})
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatClaude {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatClaude)
	}
	if _, ok := converted.(*dto.ClaudeRequest); !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.ClaudeRequest", converted)
	}
}
