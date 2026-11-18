package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestSystemOneToOpenAIDecisionsQuestions(t *testing.T) {
	questions := []byte(`{
		"is_spam": {"type": "noul", "instructions": "Is this spam?", "criteria": {"true": "Promotes products.", "false": "Normal message."}},
		"route": {"type": "choice", "instructions": "Which team?", "criteria": {"billing": "Charges.", "shipping": "Delivery."}},
		"urgency": {"type": "score", "instructions": "How urgent?", "criteria": ["Routine.", "Same-day.", "Escalate now."]}
	}`)

	converted, err := systemOneToOpenAIDecisionsQuestions(questions)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	var parsed []map[string]any
	if err := common.Unmarshal(converted, &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("questions = %d, want 3", len(parsed))
	}
	if parsed[0]["name"] != "is_spam" || parsed[0]["type"] != "predicate" {
		t.Fatalf("first question = %v", parsed[0])
	}
	if parsed[0]["instructions"] != "Is this spam? {\"true\": \"Promotes products.\", \"false\": \"Normal message.\"}" {
		t.Fatalf("predicate instructions with criteria = %v", parsed[0]["instructions"])
	}
	if parsed[1]["name"] != "route" || parsed[1]["type"] != "choice" {
		t.Fatalf("second question = %v", parsed[1])
	}
	choices := parsed[1]["choices"].([]any)
	if len(choices) != 2 || choices[0].(map[string]any)["value"] != "billing" {
		t.Fatalf("choices = %v", choices)
	}
	if parsed[2]["name"] != "urgency" || parsed[2]["type"] != "score" {
		t.Fatalf("third question = %v", parsed[2])
	}
	levels := parsed[2]["levels"].([]any)
	if len(levels) != 3 || levels[2].(map[string]any)["label"] != "2" {
		t.Fatalf("levels = %v", levels)
	}

	for _, bad := range []string{
		`{"x": {"type": "box", "instructions": "where"}}`,
		`{"x": {"type": "choice", "instructions": "i", "criteria": {"only": "one"}}}`,
		`{"x": {"type": "score", "instructions": "i", "criteria": ["one"]}}`,
		`{"x": {"type": "mystery"}}`,
	} {
		if _, err := systemOneToOpenAIDecisionsQuestions([]byte(bad)); err == nil {
			t.Errorf("questions %s should be rejected", bad)
		}
	}
}

func TestBuildOpenAIDecisionsInputFromState(t *testing.T) {
	// 纯字符串 state 无资产:原样作为 input 字符串
	input, err := buildOpenAIDecisionsInputFromState([]byte(`"just a text state"`), nil)
	if err != nil {
		t.Fatalf("string state: %v", err)
	}
	if string(input) != `"just a text state"` {
		t.Fatalf("string state passthrough = %s", input)
	}

	// 整值引用:state 即引用串,转换为单图片消息(data URL 内联)
	assets := []byte(`{"ref-photo":{"type":"image","format":"png","data_base64":"aGk="}}`)
	input, err = buildOpenAIDecisionsInputFromState([]byte(`"ref-photo"`), assets)
	if err != nil {
		t.Fatalf("whole-ref state: %v", err)
	}
	var messages []map[string]any
	if err := common.Unmarshal(input, &messages); err != nil {
		t.Fatalf("whole-ref decode: %v", err)
	}
	content := messages[0]["content"].([]any)
	if messages[0]["role"] != "user" || len(content) != 2 {
		t.Fatalf("whole-ref message = %v", messages[0])
	}
	if content[0].(map[string]any)["text"] != "ref-photo:" ||
		content[1].(map[string]any)["image_url"] != "data:image/png;base64,aGk=" {
		t.Fatalf("whole-ref content = %v", content)
	}

	// 结构化 state:input_json 内容 + 每资产[标签块,图片块],引用顺序稳定
	structured := []byte(`{"ticket":{"subject":"Duplicate charge"}}`)
	twoAssets := []byte(`{"b-second":{"type":"image","format":"jpg","data_base64":"Yg=="},"a-first":{"type":"image","format":"jpeg","data_base64":"YT09"}}`)
	input, err = buildOpenAIDecisionsInputFromState(structured, twoAssets)
	if err != nil {
		t.Fatalf("structured state: %v", err)
	}
	if err := common.Unmarshal(input, &messages); err != nil {
		t.Fatalf("structured decode: %v", err)
	}
	content = messages[0]["content"].([]any)
	if len(content) != 5 {
		t.Fatalf("structured content = %d parts, want 5", len(content))
	}
	if content[0].(map[string]any)["type"] != "input_json" {
		t.Fatalf("state part = %v", content[0])
	}
	if content[1].(map[string]any)["text"] != "a-first:" || content[2].(map[string]any)["image_url"] != "data:image/jpeg;base64,YT09" {
		t.Fatalf("first labeled image = %v %v", content[1], content[2])
	}

	// 不支持的图片格式与 multipart part 形式:明确报错
	if _, err := buildOpenAIDecisionsInputFromState(structured, []byte(`{"x":{"type":"image","format":"webp","data_base64":"Zm9v"}}`)); err == nil {
		t.Fatal("webp format should be rejected")
	}
	if _, err := buildOpenAIDecisionsInputFromState(structured, []byte(`{"x":{"type":"image","format":"png","part":"file-1"}}`)); err == nil {
		t.Fatal("multipart part asset should be rejected")
	}
}
