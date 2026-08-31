package gemini_interactions

import (
	"io"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestGeminiChatRequestToInteractionsBasic(t *testing.T) {
	budget := 4096
	maxTokens := uint(1024)
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "hello"}}},
			{Role: "model", Parts: []dto.GeminiPart{{Text: "hi there"}}},
			{Role: "user", Parts: []dto.GeminiPart{{Text: "what's the weather?"}}},
		},
		SystemInstructions: &dto.GeminiChatContent{Parts: []dto.GeminiPart{{Text: "be brief"}}},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			MaxOutputTokens: &maxTokens,
			Temperature:     common.GetPointer(0.7), // 应被丢弃(interactions 已弃用)
			ThinkingConfig:  &dto.GeminiThinkingConfig{ThinkingBudget: &budget},
		},
	}
	out, err := GeminiChatRequestToInteractions(req, "antigravity-preview-05-2026", true)
	if err != nil || out == nil {
		t.Fatalf("convert failed: %v", err)
	}
	if out.Model != "antigravity-preview-05-2026" {
		t.Fatalf("model = %s", out.Model)
	}
	if out.Stream == nil || !*out.Stream {
		t.Fatal("stream should be true")
	}
	if out.Store == nil || *out.Store {
		t.Fatal("store should be false (stateless replay)")
	}
	if sys := string(out.SystemInstruction); sys != `"be brief"` {
		t.Fatalf("system_instruction = %s", sys)
	}

	var steps []dto.GeminiInteractionStep
	if err := common.Unmarshal(out.Input, &steps); err != nil {
		t.Fatalf("unmarshal steps: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(steps))
	}
	if steps[0].Type != dto.GeminiInteractionStepUserInput || steps[0].Content[0].Text != "hello" {
		t.Fatalf("step0 = %+v", steps[0])
	}
	if steps[1].Type != dto.GeminiInteractionStepModelOutput || steps[1].Content[0].Text != "hi there" {
		t.Fatalf("step1 = %+v", steps[1])
	}

	var genCfg map[string]any
	if err := common.Unmarshal(out.GenerationConfig, &genCfg); err != nil {
		t.Fatalf("unmarshal generation_config: %v", err)
	}
	if genCfg["max_output_tokens"] != float64(1024) {
		t.Fatalf("max_output_tokens = %v", genCfg["max_output_tokens"])
	}
	if genCfg["thinking_level"] != "medium" {
		t.Fatalf("thinking_level = %v (budget 4096 -> medium)", genCfg["thinking_level"])
	}
	if _, ok := genCfg["temperature"]; ok {
		t.Fatal("temperature should be dropped")
	}
}

func TestGeminiChatRequestToInteractionsFunctionCalling(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "weather in boston?"}}},
			{Role: "model", Parts: []dto.GeminiPart{{
				FunctionCall: &dto.FunctionCall{FunctionName: "get_weather", Arguments: map[string]any{"location": "Boston"}},
			}}},
			{Role: "user", Parts: []dto.GeminiPart{{
				FunctionResponse: &dto.GeminiFunctionResponse{Name: "get_weather", Response: map[string]interface{}{"result": "52F"}},
			}}},
		},
		Tools: []byte(`[{"functionDeclarations":[{"name":"get_weather","description":"Get weather","parameters":{"type":"OBJECT","properties":{"location":{"type":"STRING"}},"required":["location"]}}]}]`),
	}
	out, err := GeminiChatRequestToInteractions(req, "deep-research-preview-04-2026", false)
	if err != nil || out == nil {
		t.Fatalf("convert failed: %v", err)
	}

	var steps []dto.GeminiInteractionStep
	if err := common.Unmarshal(out.Input, &steps); err != nil {
		t.Fatalf("unmarshal steps: %v", err)
	}
	// user_input / function_call / function_result 三个 step
	if len(steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(steps))
	}
	fc := steps[1]
	if fc.Type != dto.GeminiInteractionStepFunctionCall || fc.Name != "get_weather" || fc.ID == "" {
		t.Fatalf("function_call step = %+v", fc)
	}
	var args map[string]any
	if err := common.Unmarshal(fc.Arguments, &args); err != nil || args["location"] != "Boston" {
		t.Fatalf("arguments = %s", string(fc.Arguments))
	}
	fr := steps[2]
	if fr.Type != dto.GeminiInteractionStepFunctionResult || fr.CallID != fc.ID {
		t.Fatalf("function_result step = %+v, want call_id == %s", fr, fc.ID)
	}

	var tools []map[string]any
	if err := common.Unmarshal(out.Tools, &tools); err != nil {
		t.Fatalf("unmarshal tools: %v", err)
	}
	if tools[0]["type"] != "function" || tools[0]["name"] != "get_weather" {
		t.Fatalf("tool = %+v", tools[0])
	}
	params := tools[0]["parameters"].(map[string]any)
	if params["type"] != "object" {
		t.Fatalf("schema type should be downcased, got %v", params["type"])
	}
	props := params["properties"].(map[string]any)
	if props["location"].(map[string]any)["type"] != "string" {
		t.Fatal("nested schema type should be downcased")
	}
}

func TestGeminiChatRequestToInteractionsStructuredOutput(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "give me json"}}}},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema:   map[string]any{"type": "OBJECT", "properties": map[string]any{"a": map[string]any{"type": "NUMBER"}}},
		},
	}
	out, err := GeminiChatRequestToInteractions(req, "m", false)
	if err != nil || out == nil {
		t.Fatalf("convert failed: %v", err)
	}
	var genCfg map[string]any
	if err := common.Unmarshal(out.GenerationConfig, &genCfg); err != nil {
		t.Fatal(err)
	}
	formats, ok := genCfg["response_format"].([]any)
	if !ok || len(formats) != 1 {
		t.Fatalf("response_format = %v", genCfg["response_format"])
	}
	format := formats[0].(map[string]any)
	if format["mime_type"] != "application/json" {
		t.Fatalf("mime_type = %v", format["mime_type"])
	}
	schema := format["schema"].(map[string]any)
	if schema["type"] != "object" {
		t.Fatalf("schema type = %v", schema["type"])
	}
}

func TestInteractionToGeminiChatResponse(t *testing.T) {
	interaction := &dto.GeminiInteraction{
		ID:     "v1_x",
		Model:  "m",
		Status: "completed",
		Steps: []dto.GeminiInteractionStep{
			{Type: dto.GeminiInteractionStepThought, Content: []dto.GeminiInteractionContent{{Type: "text", Text: "thinking..."}}},
			{Type: dto.GeminiInteractionStepFunctionCall, Name: "get_weather", Arguments: []byte(`{"location":"Boston"}`)},
			{Type: dto.GeminiInteractionStepModelOutput, Content: []dto.GeminiInteractionContent{{Type: "text", Text: "It's 52F"}}},
		},
		Usage: &dto.GeminiInteractionUsage{
			TotalInputTokens:   100,
			TotalOutputTokens:  20,
			TotalThoughtTokens: 5,
			TotalToolUseTokens: 3,
			TotalTokens:        128,
		},
	}
	resp := InteractionToGeminiChatResponse(interaction, 0)
	if len(resp.Candidates) != 1 {
		t.Fatalf("candidates = %d", len(resp.Candidates))
	}
	parts := resp.Candidates[0].Content.Parts
	if len(parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(parts))
	}
	if !parts[0].Thought || parts[0].Text != "thinking..." {
		t.Fatalf("thought part = %+v", parts[0])
	}
	if parts[1].FunctionCall == nil || parts[1].FunctionCall.FunctionName != "get_weather" {
		t.Fatalf("function_call part = %+v", parts[1])
	}
	if parts[2].Text != "It's 52F" {
		t.Fatalf("text part = %+v", parts[2])
	}
	if *resp.Candidates[0].FinishReason != "STOP" {
		t.Fatalf("finishReason = %s", *resp.Candidates[0].FinishReason)
	}
	meta := resp.UsageMetadata
	if meta.PromptTokenCount != 100 || meta.ToolUsePromptTokenCount != 3 || meta.CandidatesTokenCount != 20 || meta.ThoughtsTokenCount != 5 || meta.TotalTokenCount != 128 {
		t.Fatalf("usageMetadata = %+v", meta)
	}
}

func TestInteractionToGeminiChatResponseIncomplete(t *testing.T) {
	resp := InteractionToGeminiChatResponse(&dto.GeminiInteraction{Status: "incomplete"}, 0)
	if *resp.Candidates[0].FinishReason != "MAX_TOKENS" {
		t.Fatalf("finishReason = %s", *resp.Candidates[0].FinishReason)
	}
}

// SSE 翻译黄金用例:官方 migration 指南样例事件流
func TestInteractionsSSETranslator(t *testing.T) {
	upstream := strings.Join([]string{
		`event: interaction.created`,
		`data: {"type": "interaction.created", "interaction": {"id": "int_xyz", "status": "in_progress"}}`,
		``,
		`event: step.start`,
		`data: {"type": "step.start", "index": 0, "step": {"type": "thought"}}`,
		``,
		`event: step.delta`,
		`data: {"type": "step.delta", "index": 0, "delta": {"type": "thought", "text": "User wants weather."}}`,
		``,
		`event: step.stop`,
		`data: {"type": "step.stop", "index": 0, "status": "done"}`,
		``,
		`event: step.start`,
		`data: {"type": "step.start", "index": 1, "step": {"type": "model_output"}}`,
		``,
		`event: step.delta`,
		`data: {"type": "step.delta", "index": 1, "delta": {"type": "text", "text": "Hello"}}`,
		``,
		`event: step.delta`,
		`data: {"type": "step.delta", "index": 1, "delta": {"type": "text", "text": " world"}}`,
		``,
		`event: step.stop`,
		`data: {"type": "step.stop", "index": 1, "status": "done"}`,
		``,
		`event: interaction.completed`,
		`data: {"type": "interaction.completed", "interaction": {"id": "int_xyz", "status": "completed", "usage": {"total_input_tokens": 10, "total_output_tokens": 5, "total_tokens": 15}}}`,
		``,
	}, "\n")

	reader := NewInteractionsSSEToGeminiSSE(strings.NewReader(upstream))
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read translated stream: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var datas []dto.GeminiChatResponse
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var r dto.GeminiChatResponse
		if err := common.UnmarshalJsonStr(strings.TrimPrefix(line, "data: "), &r); err != nil {
			t.Fatalf("bad translated line %q: %v", line, err)
		}
		datas = append(datas, r)
	}
	if len(datas) != 5 {
		t.Fatalf("translated chunks = %d, want 5 (thought/role/text/text/final)", len(datas))
	}
	// chunk0: thought 摘要(role=model + thought part)
	if !datas[0].Candidates[0].Content.Parts[0].Thought || datas[0].Candidates[0].Content.Parts[0].Text != "User wants weather." {
		t.Fatalf("thought chunk = %+v", datas[0].Candidates[0].Content.Parts[0])
	}
	// chunk1: model_output 首块(role 声明)
	if datas[1].Candidates[0].Content.Role != "model" || len(datas[1].Candidates[0].Content.Parts) != 0 {
		t.Fatalf("role chunk = %+v", datas[1].Candidates[0])
	}
	// chunk2/3: text
	if datas[2].Candidates[0].Content.Parts[0].Text != "Hello" || datas[3].Candidates[0].Content.Parts[0].Text != " world" {
		t.Fatalf("text chunks = %+v %+v", datas[2].Candidates[0].Content.Parts, datas[3].Candidates[0].Content.Parts)
	}
	// chunk4: final with finish + usage
	last := datas[4]
	if *last.Candidates[0].FinishReason != "STOP" {
		t.Fatalf("final finishReason = %s", *last.Candidates[0].FinishReason)
	}
	if last.UsageMetadata.PromptTokenCount != 10 || last.UsageMetadata.CandidatesTokenCount != 5 || last.UsageMetadata.TotalTokenCount != 15 {
		t.Fatalf("final usage = %+v", last.UsageMetadata)
	}
}

// function_call 参数分片聚合
func TestInteractionsSSETranslatorFunctionCall(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type": "step.start", "index": 1, "step": {"type": "function_call", "id": "fc_1", "name": "get_weather"}}`,
		`data: {"type": "step.delta", "index": 1, "delta": {"type": "arguments", "partial_arguments": "{\"location\":"}}`,
		`data: {"type": "step.delta", "index": 1, "delta": {"type": "arguments", "partial_arguments": " \"Boston\"}"}}`,
		`data: {"type": "step.stop", "index": 1, "status": "waiting"}`,
		`data: {"type": "interaction.requires_action", "interaction": {"id": "i", "status": "requires_action"}}`,
	}, "\n")

	reader := NewInteractionsSSEToGeminiSSE(strings.NewReader(upstream))
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(out)
	if !strings.Contains(content, `"functionCall"`) {
		t.Fatalf("no functionCall emitted: %s", content)
	}
	var r dto.GeminiChatResponse
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "data: ") {
			_ = common.UnmarshalJsonStr(strings.TrimPrefix(line, "data: "), &r)
			if r.Candidates[0].Content.Parts != nil && r.Candidates[0].Content.Parts[0].FunctionCall != nil {
				fc := r.Candidates[0].Content.Parts[0].FunctionCall
				if fc.FunctionName != "get_weather" {
					t.Fatalf("name = %s", fc.FunctionName)
				}
				args, _ := fc.Arguments.(map[string]any)
				if args["location"] != "Boston" {
					t.Fatalf("aggregated args = %v", fc.Arguments)
				}
			}
		}
	}
}
