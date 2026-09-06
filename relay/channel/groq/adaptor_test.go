package groq

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func TestSanitizeGroqRequestStripsUnsupportedFields(t *testing.T) {
	maxTokens := uint(999999)
	n := 3
	request := &dto.GeneralOpenAIRequest{
		Model:            "qwen/qwen3.8-27b",
		MaxTokens:        &maxTokens,
		N:                &n,
		ReasoningEffort:  "minimal",
		FrequencyPenalty: ptrFloat64(0.5),
		LogitBias:        json.RawMessage(`{"a":1}`),
		Metadata:         json.RawMessage(`{"k":"v"}`),
		Messages: []dto.Message{
			{Role: "assistant", ReasoningContent: ptrString("think"), Reasoning: ptrString("raw"), Name: ptrString("n")},
			{Role: "user", Content: "hi"},
		},
	}
	info := &relaycommon.RelayInfo{}
	sanitizeGroqRequest(info, request)

	if request.MaxTokens != nil {
		t.Fatalf("max_tokens should be normalized to max_completion_tokens")
	}
	if request.MaxCompletionTokens == nil || *request.MaxCompletionTokens != 16384 {
		t.Fatalf("max_completion_tokens should be clamped to 16384, got %v", request.MaxCompletionTokens)
	}
	if request.N == nil || *request.N != 1 {
		t.Fatalf("n should be clamped to 1")
	}
	if request.ReasoningEffort != "low" {
		t.Fatalf("reasoning_effort minimal should map to low, got %q", request.ReasoningEffort)
	}
	if request.FrequencyPenalty != nil || request.LogitBias != nil || request.Metadata != nil {
		t.Fatalf("unsupported fields should be cleared")
	}
	if request.Messages[0].ReasoningContent != nil || request.Messages[0].Reasoning != nil || request.Messages[0].Name != nil {
		t.Fatalf("assistant history should drop reasoning/name fields")
	}
	if request.Messages[1].Content != "hi" {
		t.Fatalf("user message content should be untouched")
	}
}

func TestConvertOpenAIRequestFiltersAndRestoresRawFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{
		"model":"groq/compound",
		"messages":[{"role":"user","content":"hi"}],
		"prompt_cache_key":"sess-1",
		"reasoning_format":"parsed",
		"search_settings":{"exclude_domains":["example.com"]}
	}`
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	request := &dto.GeneralOpenAIRequest{
		Model:          "groq/compound",
		PromptCacheKey: "sess-1",
		Messages:       []dto.Message{{Role: "user", Content: "hi"}},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "groq/compound"}}
	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	payload := converted.(map[string]any)

	if _, exists := payload["prompt_cache_key"]; exists {
		t.Fatalf("prompt_cache_key should be filtered out")
	}
	if payload["reasoning_format"] != "parsed" {
		t.Fatalf("reasoning_format should be restored from raw body")
	}
	settings, ok := payload["search_settings"].(map[string]any)
	if !ok || len(settings["exclude_domains"].([]any)) != 1 {
		t.Fatalf("search_settings should be restored for compound model")
	}

	// 非 Compound 模型不应恢复检索类字段
	c2 := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c2.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	c2.Request.Header.Set("Content-Type", "application/json")
	request2 := &dto.GeneralOpenAIRequest{Model: "llama-3.3-70b-versatile", Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	info2 := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "llama-3.3-70b-versatile"}}
	converted2, err := (&Adaptor{}).ConvertOpenAIRequest(c2, info2, request2)
	if err != nil {
		t.Fatalf("ConvertOpenAIRequest error = %v", err)
	}
	payload2 := converted2.(map[string]any)
	if _, exists := payload2["search_settings"]; exists {
		t.Fatalf("search_settings must not be restored for non-compound model")
	}
	if payload2["model"] != "llama-3.3-70b-versatile" {
		t.Fatalf("model should stay upstream name, got %v", payload2["model"])
	}
}

func ptrString(s string) *string { return &s }

func ptrFloat64(f float64) *float64 { return &f }
