package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/model_setting"
)

func TestShouldChatCompletionsUseResponsesPolicyRejectsCompactModels(t *testing.T) {
	t.Parallel()

	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   true,
		ModelPatterns: []string{`^gpt-5.*$`},
	}

	if ShouldChatCompletionsUseResponsesPolicy(policy, 1, 1, "gpt-5-openai-compact") {
		t.Fatal("expected compact models to bypass chat->responses compatibility policy")
	}
	if !ShouldChatCompletionsUseResponsesPolicy(policy, 1, 1, "gpt-5") {
		t.Fatal("expected regular gpt-5 model to match policy")
	}
}
