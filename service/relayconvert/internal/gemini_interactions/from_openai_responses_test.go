package gemini_interactions

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestResponsesToInteractionsBasic(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:        "gemini-3.1-flash-lite",
		Input:        []byte(`"用一句话介绍自己"`),
		Instructions: []byte(`"你是简短助手"`),
		Tools:        []byte(`[{"type":"function","name":"get_weather","description":"查天气","parameters":{"type":"object","properties":{"location":{"type":"string"}},"required":["location"]}},{"type":"web_search"}]`),
		Reasoning:    &dto.Reasoning{Effort: "low"},
	}
	out, err := ResponsesToInteractions(req, "gemini-3.1-flash-lite", false, nil)
	if err != nil || out == nil {
		t.Fatalf("convert failed: %v", err)
	}
	if sys := string(out.SystemInstruction); sys != `"你是简短助手"` {
		t.Fatalf("system_instruction = %s", sys)
	}

	var tools []map[string]any
	if err := common.Unmarshal(out.Tools, &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 || tools[0]["name"] != "get_weather" || tools[1]["type"] != "google_search" {
		t.Fatalf("tools = %+v", tools)
	}
	params := tools[0]["parameters"].(map[string]any)
	if params["type"] != "object" {
		t.Fatalf("parameters type = %v", params["type"])
	}

	var genCfg map[string]any
	if err := common.Unmarshal(out.GenerationConfig, &genCfg); err != nil {
		t.Fatal(err)
	}
	if genCfg["thinking_level"] != "low" {
		t.Fatalf("thinking_level = %v", genCfg["thinking_level"])
	}

	var steps []dto.GeminiInteractionStep
	if err := common.Unmarshal(out.Input, &steps); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Type != dto.GeminiInteractionStepUserInput {
		t.Fatalf("steps = %+v", steps)
	}
	if steps[0].Content[0].Text != "用一句话介绍自己" {
		t.Fatalf("text = %s", steps[0].Content[0].Text)
	}
}

func TestResponsesToInteractionsItemsWithCalls(t *testing.T) {
	input := `[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"天气?"}]},
		{"type":"function_call","call_id":"call_abc","name":"get_weather","arguments":"{\"location\":\"东京\"}"},
		{"type":"function_call_output","call_id":"call_abc","output":"26度"},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"谢谢"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "m", Input: []byte(input)}
	out, err := ResponsesToInteractions(req, "m", false, nil)
	if err != nil || out == nil {
		t.Fatalf("convert failed: %v", err)
	}
	var steps []dto.GeminiInteractionStep
	if err := common.Unmarshal(out.Input, &steps); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 4 {
		t.Fatalf("steps = %d, want 4", len(steps))
	}
	fc := steps[1]
	if fc.Type != dto.GeminiInteractionStepFunctionCall || fc.ID != "call_abc" || fc.Name != "get_weather" {
		t.Fatalf("function_call step = %+v", fc)
	}
	var args map[string]any
	if err := common.Unmarshal(fc.Arguments, &args); err != nil || args["location"] != "东京" {
		t.Fatalf("arguments = %s", string(fc.Arguments))
	}
	fr := steps[2]
	if fr.Type != dto.GeminiInteractionStepFunctionResult || fr.CallID != "call_abc" {
		t.Fatalf("function_result step = %+v", fr)
	}
	var resultBlocks []dto.GeminiInteractionContent
	if err := common.Unmarshal(fr.Result, &resultBlocks); err != nil || resultBlocks[0].Text != "26度" {
		t.Fatalf("result = %s", string(fr.Result))
	}
}

func TestResponsesToInteractionsBridge(t *testing.T) {
	input := `[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"天气?"}]},
		{"type":"function_call","call_id":"call_known","name":"get_weather","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_known","output":"26度"},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"换算成华氏度"}]}
	]`
	req := &dto.OpenAIResponsesRequest{Model: "m", Input: []byte(input)}
	lookup := func(callID string) (string, bool) {
		if callID == "call_known" {
			return "v1_inter_123", true
		}
		return "", false
	}
	out, err := ResponsesToInteractions(req, "m", false, lookup)
	if err != nil || out == nil {
		t.Fatalf("convert failed: %v", err)
	}
	if out.PreviousInteractionID != "v1_inter_123" {
		t.Fatalf("previous_interaction_id = %s", out.PreviousInteractionID)
	}
	var steps []dto.GeminiInteractionStep
	if err := common.Unmarshal(out.Input, &steps); err != nil {
		t.Fatal(err)
	}
	// 仅 function_result + 后续 user_input,历史不回放
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2 (function_result + user_input)", len(steps))
	}
	if steps[0].Type != dto.GeminiInteractionStepFunctionResult || steps[0].CallID != "call_known" || steps[0].Name != "get_weather" {
		t.Fatalf("step0 = %+v", steps[0])
	}
	if steps[1].Type != dto.GeminiInteractionStepUserInput || steps[1].Content[0].Text != "换算成华氏度" {
		t.Fatalf("step1 = %+v", steps[1])
	}
}

func TestResponsesToInteractionsStructuredOutput(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "m",
		Input: []byte(`"给json"`),
		Text:  []byte(`{"format":{"type":"json_schema","name":"recipe","schema":{"type":"object","properties":{"name":{"type":"string"}}}}}`),
	}
	out, err := ResponsesToInteractions(req, "m", false, nil)
	if err != nil || out == nil {
		t.Fatalf("convert failed: %v", err)
	}
	var genCfg map[string]any
	if err := common.Unmarshal(out.GenerationConfig, &genCfg); err != nil {
		t.Fatal(err)
	}
	formats, ok := genCfg["response_format"].([]any)
	if !ok || len(formats) != 1 {
		t.Fatalf("response_format = %v", genCfg)
	}
	f := formats[0].(map[string]any)
	if f["mime_type"] != "application/json" {
		t.Fatalf("mime_type = %v", f["mime_type"])
	}
	schema := f["schema"].(map[string]any)
	if schema["type"] != "object" {
		t.Fatalf("schema = %v", schema)
	}
}
