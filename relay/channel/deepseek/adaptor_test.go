package deepseek

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
)

func TestGetRequestURLUsesClaudeCompatibleEndpointForClaudeFormat(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.deepseek.com/v1",
			ChannelType:    constant.ChannelTypeDeepSeek,
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://api.deepseek.com/anthropic/v1/messages"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestGetRequestURLAcceptsAnthropicBaseURL(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.deepseek.com/anthropic",
			ChannelType:    constant.ChannelTypeDeepSeek,
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://api.deepseek.com/anthropic/v1/messages"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestGetRequestURLUsesBetaCompletionsEndpoint(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.deepseek.com/beta",
			ChannelType:    constant.ChannelTypeDeepSeek,
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://api.deepseek.com/beta/completions"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestSetupRequestHeaderUsesClaudeCompatibleHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("anthropic-beta", "tools-2024-04-04")

	adaptor := &Adaptor{}
	headers := make(http.Header)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:      "deepseek-key",
			ChannelType: constant.ChannelTypeDeepSeek,
		},
	}

	if err := adaptor.SetupRequestHeader(c, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if headers.Get("x-api-key") != "deepseek-key" {
		t.Fatalf("x-api-key = %q, want %q", headers.Get("x-api-key"), "deepseek-key")
	}
	if headers.Get("anthropic-version") != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want %q", headers.Get("anthropic-version"), "2023-06-01")
	}
	if headers.Get("anthropic-beta") != "tools-2024-04-04" {
		t.Fatalf("anthropic-beta = %q, want %q", headers.Get("anthropic-beta"), "tools-2024-04-04")
	}
	if headers.Get("Authorization") != "" {
		t.Fatalf("Authorization = %q, want empty for Claude-compatible requests", headers.Get("Authorization"))
	}
}

func TestConvertOpenAIRequestDoesNotRouteByModelName(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "claude-3-7-sonnet",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-7-sonnet",
			ChannelType:       constant.ChannelTypeDeepSeek,
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

	if converted != request {
		t.Fatalf("ConvertOpenAIRequest returned %T, want original OpenAI request", converted)
	}
}

func TestConvertOpenAIRequestReturnsClaudeRequestForClaudeFormat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatClaude,
		OriginModelName: "deepseek-chat",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "deepseek-chat",
			ChannelType:       constant.ChannelTypeDeepSeek,
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-chat",
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
