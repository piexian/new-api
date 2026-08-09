package oaichat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
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

func TestShouldChatCompletionsUseResponsesChannelPolicy(t *testing.T) {
	t.Parallel()

	policy := model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   true,
		ModelPatterns: []string{`^global-.*$`},
	}
	enabled := true
	disabled := false

	tests := []struct {
		name           string
		channelSetting dto.ChannelSettings
		model          string
		want           bool
	}{
		{name: "inherits global match", model: "global-model", want: true},
		{name: "inherits global miss", model: "channel-model", want: false},
		{
			name: "explicit disable wins",
			channelSetting: dto.ChannelSettings{
				ChatCompletionsToResponsesEnabled: &disabled,
			},
			model: "global-model",
			want:  false,
		},
		{
			name: "explicit enable overrides global channel selection",
			channelSetting: dto.ChannelSettings{
				ChatCompletionsToResponsesEnabled: &enabled,
			},
			model: "global-model",
			want:  true,
		},
		{
			name: "channel models override global models",
			channelSetting: dto.ChannelSettings{
				ChatCompletionsToResponsesModels: []string{`^channel-.*$`},
			},
			model: "channel-model",
			want:  true,
		},
		{
			name: "empty channel models inherit global models",
			channelSetting: dto.ChannelSettings{
				ChatCompletionsToResponsesModels: []string{"", "   "},
			},
			model: "global-model",
			want:  true,
		},
		{
			name: "new policy rejects compact models",
			channelSetting: dto.ChannelSettings{
				ChatCompletionsToResponsesEnabled: &enabled,
				ChatCompletionsToResponsesModels:  []string{`.*`},
			},
			model: "gpt-5-openai-compact",
			want:  false,
		},
		{
			name: "legacy switch remains all models",
			channelSetting: dto.ChannelSettings{
				UseResponsesApi: true,
			},
			model: "legacy-model",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ShouldChatCompletionsUseResponsesChannelPolicy(policy, tt.channelSetting, 1, 1, tt.model)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
