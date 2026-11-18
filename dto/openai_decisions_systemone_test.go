package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestSystemOneAnswersFromOpenAIDecisions(t *testing.T) {
	answers := []json.RawMessage{
		json.RawMessage(`{"type":"predicate","name":"damaged","probability":0.95}`),
		json.RawMessage(`{"type":"choice","name":"route","choice":"billing","confidence":0.93,"probabilities":[{"value":"billing","probability":0.93},{"value":"shipping","probability":0.07}]}`),
		json.RawMessage(`{"type":"score","name":"urgency","score":1.9,"confidence":0.91,"probabilities":[{"label":"0","value":0,"probability":0.01},{"label":"1","value":1,"probability":0.08},{"label":"2","value":2,"probability":0.91}]}`),
		json.RawMessage(`{"type":"refusal","name":"secret"}`),
		json.RawMessage(`{"type":"predicate","name":null,"probability":0.5}`),
	}

	converted := SystemOneAnswersFromOpenAIDecisions(answers)
	if len(converted) != 4 {
		t.Fatalf("converted = %d entries, want 4 (unnamed skipped)", len(converted))
	}

	var noul map[string]any
	if err := common.Unmarshal(converted["damaged"], &noul); err != nil {
		t.Fatalf("decode noul: %v", err)
	}
	if noul["type"] != "noul" || noul["noul"] != 0.95 || noul["confidence"] != 0.95 {
		t.Fatalf("noul answer = %v", noul)
	}

	var choice map[string]any
	if err := common.Unmarshal(converted["route"], &choice); err != nil {
		t.Fatalf("decode choice: %v", err)
	}
	if choice["type"] != "choice" || choice["choice"] != "billing" || choice["confidence"] != 0.93 {
		t.Fatalf("choice answer = %v", choice)
	}
	probabilities := choice["probabilities"].(map[string]any)
	if probabilities["billing"] != 0.93 || probabilities["shipping"] != 0.07 {
		t.Fatalf("choice probabilities = %v", probabilities)
	}

	var score map[string]any
	if err := common.Unmarshal(converted["urgency"], &score); err != nil {
		t.Fatalf("decode score: %v", err)
	}
	if score["type"] != "score" || score["score"] != 1.9 {
		t.Fatalf("score answer = %v", score)
	}
	scoreProbabilities := score["probabilities"].(map[string]any)
	if scoreProbabilities["2"] != 0.91 {
		t.Fatalf("score probabilities = %v", scoreProbabilities)
	}

	if !strings.Contains(string(converted["secret"]), `"type":"refusal"`) {
		t.Fatalf("refusal passthrough = %s", converted["secret"])
	}
}
