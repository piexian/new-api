package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestGeminiInteractionsRequestUnmarshal(t *testing.T) {
	body := `{
		"model": "gemini-3.6-flash",
		"input": [
			{"type": "text", "text": "hello"},
			{"type": "image", "mime_type": "image/png", "data": "aGVsbG8="}
		],
		"stream": true,
		"background": false,
		"previous_interaction_id": "v1_abc",
		"generation_config": {"max_output_tokens": 1024, "thinking_level": "low"},
		"tools": [{"type": "google_search"}]
	}`
	req := &GeminiInteractionsRequest{}
	if err := common.Unmarshal([]byte(body), req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.Model != "gemini-3.6-flash" {
		t.Fatalf("model = %s", req.Model)
	}
	if req.Stream == nil || !*req.Stream {
		t.Fatal("stream should be true")
	}
	if req.PreviousInteractionID != "v1_abc" {
		t.Fatalf("previous_interaction_id = %s", req.PreviousInteractionID)
	}
	if req.GetRequestModel() != "gemini-3.6-flash" {
		t.Fatalf("GetRequestModel = %s", req.GetRequestModel())
	}

	meta := req.GetTokenCountMeta()
	if meta.CombineText != "hello" {
		t.Fatalf("CombineText = %s", meta.CombineText)
	}
	if len(meta.Files) != 1 || meta.Files[0].FileType != "image" {
		t.Fatalf("files = %+v", meta.Files)
	}
	if meta.MaxTokens != 1024 {
		t.Fatalf("MaxTokens = %d", meta.MaxTokens)
	}
}

func TestGeminiInteractionsRequestAgentModel(t *testing.T) {
	req := &GeminiInteractionsRequest{Agent: "deep-research-preview-04-2026"}
	if req.GetRequestModel() != "deep-research-preview-04-2026" {
		t.Fatalf("agent routing model = %s", req.GetRequestModel())
	}
	req.SetModelName("deep-research-max-preview-04-2026")
	if req.Agent != "deep-research-max-preview-04-2026" || req.Model != "" {
		t.Fatalf("SetModelName should rewrite agent, got agent=%s model=%s", req.Agent, req.Model)
	}
}

func TestGeminiInteractionsTokenMetaStepInput(t *testing.T) {
	// stateless 多轮: input 为 Step 数组(user_input.content + function_result.result)
	body := `[
		{"type": "user_input", "content": [{"type": "text", "text": "weather?"}]},
		{"type": "function_call", "id": "fc1", "name": "get_weather", "arguments": {"location": "Boston"}},
		{"type": "function_result", "call_id": "fc1", "result": [{"type": "text", "text": "52F"}]}
	]`
	req := &GeminiInteractionsRequest{Input: []byte(body)}
	meta := req.GetTokenCountMeta()
	if meta.CombineText != "weather?\n52F" {
		t.Fatalf("CombineText = %s", meta.CombineText)
	}
}

func TestGeminiInteractionUsageToUsage(t *testing.T) {
	u := &GeminiInteractionUsage{
		TotalInputTokens:   100,
		TotalOutputTokens:  20,
		TotalThoughtTokens: 30,
		TotalCachedTokens:  10,
		TotalToolUseTokens: 5,
		TotalTokens:        155,
		InputTokensByModality: []GeminiInteractionModalityTokens{
			{Modality: "text", Tokens: 90},
			{Modality: "image", Tokens: 10},
		},
		OutputTokensByModality: []GeminiInteractionModalityTokens{
			{Modality: "text", Tokens: 20},
		},
	}
	usage := u.ToUsage(0)
	if usage.PromptTokens != 105 {
		t.Fatalf("PromptTokens = %d, want 105 (input+tool_use)", usage.PromptTokens)
	}
	if usage.CompletionTokens != 50 {
		t.Fatalf("CompletionTokens = %d, want 50 (output+thought)", usage.CompletionTokens)
	}
	if usage.TotalTokens != 155 {
		t.Fatalf("TotalTokens = %d", usage.TotalTokens)
	}
	if usage.PromptTokensDetails.CachedTokens != 10 {
		t.Fatalf("CachedTokens = %d", usage.PromptTokensDetails.CachedTokens)
	}
	if usage.CompletionTokenDetails.ReasoningTokens != 30 {
		t.Fatalf("ReasoningTokens = %d", usage.CompletionTokenDetails.ReasoningTokens)
	}
	if usage.PromptTokensDetails.ImageTokens != 10 || usage.PromptTokensDetails.TextTokens != 90 {
		t.Fatalf("input modality details = %+v", usage.PromptTokensDetails)
	}
}

func TestGeminiInteractionUsageToUsageFallback(t *testing.T) {
	var u *GeminiInteractionUsage
	usage := u.ToUsage(50)
	if usage == nil || usage.PromptTokens != 50 {
		t.Fatalf("fallback usage = %+v", usage)
	}
}

func TestGeminiInteractionSseEventName(t *testing.T) {
	e := &GeminiInteractionSseEvent{Type: "step.delta"}
	if e.EventName() != "step.delta" {
		t.Fatal("legacy type field should work")
	}
	e2 := &GeminiInteractionSseEvent{EventType: "interaction.completed", Type: "ignored"}
	if e2.EventName() != "interaction.completed" {
		t.Fatal("event_type should take precedence")
	}
}

func TestIsGeminiInteractionTerminal(t *testing.T) {
	for _, s := range []string{"completed", "failed", "cancelled", "incomplete", "budget_exceeded"} {
		if !IsGeminiInteractionTerminal(s) {
			t.Fatalf("%s should be terminal", s)
		}
	}
	for _, s := range []string{"in_progress", "requires_action", "queued", ""} {
		if IsGeminiInteractionTerminal(s) {
			t.Fatalf("%s should not be terminal", s)
		}
	}
}
