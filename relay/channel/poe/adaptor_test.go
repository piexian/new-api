package poe

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestGetRequestURLNormalizesOpenAIBaseURL(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RequestURLPath: "/v1/chat/completions",
		RelayFormat:    types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.poe.com/v1",
			ChannelType:    constant.ChannelTypePoe,
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://api.poe.com/v1/chat/completions"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	t.Parallel()

	if got := NormalizeBaseURL("https://api.poe.com/v1"); got != "https://api.poe.com" {
		t.Fatalf("NormalizeBaseURL() = %q, want %q", got, "https://api.poe.com")
	}
}

func TestGetRequestURLNormalizesClaudeBaseURL(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.poe.com/v1",
			ChannelType:    constant.ChannelTypePoe,
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://api.poe.com/v1/messages"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestSetupRequestHeaderForClaude(t *testing.T) {
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
			ApiKey:      "poe-key",
			ChannelType: constant.ChannelTypePoe,
		},
	}

	if err := adaptor.SetupRequestHeader(c, &headers, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}

	if headers.Get("x-api-key") != "poe-key" {
		t.Fatalf("x-api-key = %q, want %q", headers.Get("x-api-key"), "poe-key")
	}
	if headers.Get("anthropic-version") != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want %q", headers.Get("anthropic-version"), "2023-06-01")
	}
	if headers.Get("anthropic-beta") != "tools-2024-04-04" {
		t.Fatalf("anthropic-beta = %q, want %q", headers.Get("anthropic-beta"), "tools-2024-04-04")
	}
}

func TestConvertOpenAIRequestPreservesStreamOptions(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	request := &dto.GeneralOpenAIRequest{
		Model: "GPT-5.4",
		StreamOptions: &dto.StreamOptions{
			IncludeUsage: true,
		},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "GPT-5.4",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypePoe,
			UpstreamModelName: "GPT-5.4",
		},
	}

	converted, err := adaptor.ConvertOpenAIRequest(gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()), info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest returned error: %v", err)
	}

	got, ok := converted.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want *dto.GeneralOpenAIRequest", converted)
	}
	if got.StreamOptions == nil || !got.StreamOptions.IncludeUsage {
		t.Fatalf("stream_options = %#v, want include_usage=true", got.StreamOptions)
	}
}
