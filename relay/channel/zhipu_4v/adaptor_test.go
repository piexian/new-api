package zhipu_4v

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
