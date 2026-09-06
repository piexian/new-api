package cerebras

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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

func TestFlattenAssistantArrayContentForQwen(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"model":"qwen-3.8-27b","messages":[],"reasoning_format":"parsed"}`
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	request := &dto.GeneralOpenAIRequest{
		Model: "qwen-3.8-27b",
		Messages: []dto.Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: []dto.MediaContent{
				{Type: "text", Text: "part one "},
				{Type: "text", Text: "part two"},
			}},
		},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "qwen-3.8-27b"}}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	payload := converted.(map[string]any)

	if payload["reasoning_format"] != "parsed" {
		t.Fatalf("reasoning_format should be restored from raw body, got %#v", payload["reasoning_format"])
	}
	messages, ok := payload["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("messages should round-trip with 2 entries, got %#v", payload["messages"])
	}
	assistant := messages[1].(map[string]any)
	if assistant["content"] != "part one part two" {
		t.Fatalf("assistant array content should flatten to string, got %#v", assistant["content"])
	}
}

func TestReasoningFormatHiddenRejectedForQwenOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	build := func(model string) (map[string]any, *gin.Context) {
		body := `{"model":"` + model + `","messages":[],"reasoning_format":"hidden"}`
		c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
		c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		request := &dto.GeneralOpenAIRequest{
			Model:    model,
			Messages: []dto.Message{{Role: "user", Content: "hi"}},
		}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: model}}
		converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
		if err != nil {
			t.Fatalf("ConvertOpenAIRequest error = %v", err)
		}
		return converted.(map[string]any), c
	}

	qwenPayload, _ := build("qwen-3.8-27b")
	if _, exists := qwenPayload["reasoning_format"]; exists {
		t.Fatalf("reasoning_format=hidden must be dropped for qwen models")
	}
	ossPayload, _ := build("gpt-oss-120b")
	if ossPayload["reasoning_format"] != "hidden" {
		t.Fatalf("reasoning_format=hidden must be preserved for gpt-oss models")
	}
}

func TestAssistantReasoningContentMigratedToReasoning(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"model":"gpt-oss-120b","messages":[]}`
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-oss-120b",
		Messages: []dto.Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "first answer", ReasoningContent: common.GetPointer("deep thought")},
			{Role: "user", Content: "again"},
			{Role: "assistant", Content: "second", ReasoningContent: common.GetPointer("stale"), Reasoning: common.GetPointer("canonical")},
		},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-oss-120b"}}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	messages := converted.(map[string]any)["messages"].([]any)

	migrated := messages[1].(map[string]any)
	if _, exists := migrated["reasoning_content"]; exists {
		t.Fatalf("assistant reasoning_content must be stripped, got %#v", migrated)
	}
	if migrated["reasoning"] != "deep thought" {
		t.Fatalf("reasoning should carry the reasoning_content value, got %#v", migrated["reasoning"])
	}

	kept := messages[3].(map[string]any)
	if _, exists := kept["reasoning_content"]; exists {
		t.Fatalf("assistant reasoning_content must be stripped even when reasoning is set, got %#v", kept)
	}
	if kept["reasoning"] != "canonical" {
		t.Fatalf("existing reasoning must win, got %#v", kept["reasoning"])
	}
}
