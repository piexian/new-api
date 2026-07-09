package cerebras

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestConvertOpenAIRequestMapsCerebrasFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{
		"model":"gpt-oss-120b-high",
		"messages":[{"role":"user","content":"hello"}],
		"max_tokens":128,
		"clear_thinking":false,
		"stream_options":{"include_usage":true},
		"n":2,
		"extra_body":{"service_tier":"flex","unsupported":true}
	}`
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	maxTokens := uint(128)
	stream := true
	n := 2
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-oss-120b-high",
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-oss-120b-high",
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
		},
		Stream:        &stream,
		StreamOptions: &dto.StreamOptions{IncludeUsage: true},
		MaxTokens:     &maxTokens,
		N:             &n,
		ExtraBody:     []byte(`{"prompt_cache_key":"conversation-1","unsupported_extra":true}`),
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	payload, ok := converted.(map[string]any)
	if !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want map[string]any", converted)
	}

	if got := payload["model"]; got != "gpt-oss-120b" {
		t.Fatalf("model = %v, want gpt-oss-120b", got)
	}
	if got := payload["reasoning_effort"]; got != "high" {
		t.Fatalf("reasoning_effort = %v, want high", got)
	}
	if got := payload["max_completion_tokens"]; got != float64(128) {
		t.Fatalf("max_completion_tokens = %#v, want 128", got)
	}
	if got := payload["clear_thinking"]; got != false {
		t.Fatalf("clear_thinking = %#v, want false", got)
	}
	if got := payload["prompt_cache_key"]; got != "conversation-1" {
		t.Fatalf("prompt_cache_key = %#v, want conversation-1", got)
	}
	if got := payload["service_tier"]; got != "flex" {
		t.Fatalf("service_tier = %#v, want flex", got)
	}
	for _, key := range []string{"max_tokens", "stream_options", "n", "extra_body", "unsupported", "unsupported_extra"} {
		if _, exists := payload[key]; exists {
			t.Fatalf("payload contains unsupported key %q: %#v", key, payload[key])
		}
	}
	if info.UpstreamModelName != "gpt-oss-120b" {
		t.Fatalf("UpstreamModelName = %q, want gpt-oss-120b", info.UpstreamModelName)
	}
}

func TestConvertOpenAIRequestRejectsUnsupportedEndpoint(t *testing.T) {
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeEmbeddings}
	_, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{})
	if err == nil {
		t.Fatal("ConvertOpenAIRequest error = nil, want unsupported endpoint error")
	}
}

func TestConvertOpenAIRequestAllowsNilRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	maxTokens := uint(16)
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(
		c,
		&relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions},
		&dto.GeneralOpenAIRequest{
			Model:     "gpt-oss-120b",
			Messages:  []dto.Message{{Role: "user", Content: "hi"}},
			MaxTokens: &maxTokens,
		},
	)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	payload, ok := converted.(map[string]any)
	if !ok {
		t.Fatalf("ConvertOpenAIRequest returned %T, want map[string]any", converted)
	}
	if got := payload["max_completion_tokens"]; got != float64(16) {
		t.Fatalf("max_completion_tokens = %#v, want 16", got)
	}
}
