package gemini

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/stretchr/testify/require"
)

func newThinkingRelayInfo(modelName string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: modelName,
		},
	}
}

func applyThinking(t *testing.T, modelName string, effort string) (*dto.GeminiThinkingConfig, error) {
	t.Helper()
	info := newThinkingRelayInfo(modelName)
	req := &dto.GeminiChatRequest{}
	var err error
	if effort == "" {
		err = ApplyThinkingConfig(req, info)
	} else {
		err = ApplyThinkingConfig(req, info, dto.GeneralOpenAIRequest{ReasoningEffort: effort})
	}
	return req.GenerationConfig.ThinkingConfig, err
}

func TestApplyThinkingConfigGemini3UsesThinkingLevel(t *testing.T) {
	prev := model_setting.GetGeminiSettings().ThinkingAdapterEnabled
	model_setting.GetGeminiSettings().ThinkingAdapterEnabled = true
	defer func() { model_setting.GetGeminiSettings().ThinkingAdapterEnabled = prev }()

	cases := []struct {
		name          string
		model         string
		effort        string
		wantLevel     string
		wantBudgetNil bool
		wantBudget    int
		wantErr       bool
	}{
		{"budget 8192 maps to medium", "gemini-3.5-flash-thinking-8192", "", "medium", true, 0, false},
		{"budget 2048 maps to low", "gemini-3.5-flash-thinking-2048", "", "low", true, 0, false},
		{"budget 500 maps to minimal", "gemini-3.5-flash-thinking-500", "", "minimal", true, 0, false},
		{"bare -thinking defaults to medium", "gemini-3.5-flash-thinking", "", "medium", true, 0, false},
		{"-max suffix normalizes to high", "gemini-3.5-flash-max", "", "high", true, 0, false},
		{"-xhigh suffix normalizes to high", "gemini-3.5-flash-xhigh", "", "high", true, 0, false},
		{"reasoning_effort high maps directly", "gemini-3.5-flash", "high", "high", true, 0, false},
		{"reasoning_effort minimal maps directly", "gemini-3.5-flash", "minimal", "minimal", true, 0, false},
		{"reasoning_effort none floors to minimal", "gemini-3.5-flash", "none", "minimal", true, 0, false},
		{"3.8 -minimal suffix clamps to low", "gemini-3.8-flash-minimal", "", "low", true, 0, false},
		{"3.8 small budget clamps to low", "gemini-3.8-flash-thinking-500", "", "low", true, 0, false},
		{"3.8 reasoning_effort minimal clamps to low", "gemini-3.8-flash", "minimal", "low", true, 0, false},
		{"3.8 reasoning_effort high maps directly", "gemini-3.8-flash", "high", "high", true, 0, false},
		{"3.5 nothinking is rejected", "gemini-3.5-flash-nothinking", "", "", true, 0, true},
		{"3.8 nothinking is rejected", "gemini-3.8-flash-nothinking", "", "", true, 0, true},
		{"2.5 keeps numeric budget", "gemini-2.5-flash-thinking-8192", "", "", false, 8192, false},
		{"2.5 nothinking keeps budget 0", "gemini-2.5-flash-nothinking", "", "", false, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := applyThinking(t, tc.model, tc.effort)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			require.Equal(t, tc.wantLevel, cfg.ThinkingLevel)
			if tc.wantBudgetNil {
				require.Nil(t, cfg.ThinkingBudget)
			} else {
				require.NotNil(t, cfg.ThinkingBudget)
				require.Equal(t, tc.wantBudget, *cfg.ThinkingBudget)
			}
		})
	}
}

func TestApplyThinkingConfigDisabledLeavesRequestUntouched(t *testing.T) {
	prev := model_setting.GetGeminiSettings().ThinkingAdapterEnabled
	model_setting.GetGeminiSettings().ThinkingAdapterEnabled = false
	defer func() { model_setting.GetGeminiSettings().ThinkingAdapterEnabled = prev }()

	cfg, err := applyThinking(t, "gemini-3.5-flash-thinking-8192", "")
	require.NoError(t, err)
	require.Nil(t, cfg)
}
