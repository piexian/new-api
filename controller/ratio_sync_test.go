package controller

import (
	"strings"
	"testing"
)

func TestConvertModelsDevToRatioDataProviderFilter(t *testing.T) {
	fixture := `{
		"openrouter": {"models": {
			"gemini-4-argon": {"cost": {"input": 0.001, "output": 0.001}},
			"openrouter-only": {"cost": {"input": 1}}
		}},
		"google": {"models": {
			"gemini-4-argon": {"cost": {"input": 2, "output": 10, "cache_read": 0.1}}
		}}
	}`

	converted, err := convertModelsDevToRatioData(strings.NewReader(fixture), "google")
	if err != nil {
		t.Fatalf("convert with filter: %v", err)
	}
	modelRatios := converted["model_ratio"].(map[string]any)
	if got := modelRatios["gemini-4-argon"].(float64); got != 1.0 {
		t.Fatalf("model_ratio = %v, want 1.0 (google $2/M)", got)
	}
	if _, ok := modelRatios["openrouter-only"]; ok {
		t.Fatal("non-google provider entries should be excluded by the filter")
	}
	completionRatios := converted["completion_ratio"].(map[string]any)
	if got := completionRatios["gemini-4-argon"].(float64); got != 5 {
		t.Fatalf("completion_ratio = %v, want 5", got)
	}
	cacheRatios := converted["cache_ratio"].(map[string]any)
	if got := cacheRatios["gemini-4-argon"].(float64); got != 0.05 {
		t.Fatalf("cache_ratio = %v, want 0.05", got)
	}

	all, err := convertModelsDevToRatioData(strings.NewReader(fixture), "")
	if err != nil {
		t.Fatalf("convert without filter: %v", err)
	}
	if got := all["model_ratio"].(map[string]any)["gemini-4-argon"].(float64); got != 0.0005 {
		t.Fatalf("unfiltered model_ratio = %v, want cheapest 0.0005", got)
	}
	if _, ok := all["model_ratio"].(map[string]any)["openrouter-only"]; !ok {
		t.Fatal("unfiltered conversion should keep all providers")
	}
}

func TestConvertLiteLLMToRatioData(t *testing.T) {
	fixture := `{
		"sample_spec": {"mode": "one of: chat, embedding", "input_cost_per_token": 0.0},
		"gemini/gemini-4-argon": {
			"mode": "chat",
			"input_cost_per_token": 2e-06,
			"output_cost_per_token": 1e-05,
			"cache_read_input_token_cost": 1e-07
		},
		"text-embedding-test": {
			"mode": "embedding",
			"input_cost_per_token": 1.5e-07
		},
		"free/model": {"mode": "chat", "input_cost_per_token": 0, "output_cost_per_token": 0},
		"bad/no-baseline": {"mode": "chat", "input_cost_per_token": 0, "output_cost_per_token": 1},
		"weird/image": {"mode": "image_generation", "input_cost_per_token": 1},
		"neg/cost": {"mode": "chat", "input_cost_per_token": -1, "output_cost_per_token": -1}
	}`

	converted, err := convertLiteLLMToRatioData(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	modelRatios := converted["model_ratio"].(map[string]any)
	if got := modelRatios["gemini/gemini-4-argon"].(float64); got != 1.0 {
		t.Fatalf("model_ratio = %v, want 1.0 ($2/M / 2)", got)
	}
	if got := modelRatios["text-embedding-test"].(float64); got != 0.075 {
		t.Fatalf("embedding model_ratio = %v, want 0.075 ($0.15/M / 2)", got)
	}
	if got, ok := modelRatios["free/model"]; !ok || got.(float64) != 0 {
		t.Fatalf("free model_ratio = %v, want 0", got)
	}
	for _, name := range []string{"sample_spec", "bad/no-baseline", "weird/image", "neg/cost"} {
		if _, ok := modelRatios[name]; ok {
			t.Fatalf("%s should be skipped", name)
		}
	}

	completionRatios := converted["completion_ratio"].(map[string]any)
	if got := completionRatios["gemini/gemini-4-argon"].(float64); got != 5 {
		t.Fatalf("completion_ratio = %v, want 5", got)
	}
	if _, ok := completionRatios["text-embedding-test"]; ok {
		t.Fatal("embedding without output cost should not get a completion_ratio")
	}

	cacheRatios := converted["cache_ratio"].(map[string]any)
	if got := cacheRatios["gemini/gemini-4-argon"].(float64); got != 0.05 {
		t.Fatalf("cache_ratio = %v, want 0.05", got)
	}
}
