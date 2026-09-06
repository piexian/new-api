package gemini

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/stretchr/testify/require"
)

func TestThinkingAdaptorGemini3UsesThinkingLevel(t *testing.T) {
	prev := model_setting.GetGeminiSettings().ThinkingAdapterEnabled
	model_setting.GetGeminiSettings().ThinkingAdapterEnabled = true
	defer func() { model_setting.GetGeminiSettings().ThinkingAdapterEnabled = prev }()

	newInfo := func(modelName string) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: modelName,
			},
		}
	}

	t.Run("3.x budget suffix maps to level", func(t *testing.T) {
		req := &dto.GeminiChatRequest{}
		require.NoError(t, ThinkingAdaptor(req, newInfo("gemini-3.5-flash-thinking-8192")))
		require.NotNil(t, req.GenerationConfig.ThinkingConfig)
		require.Equal(t, "medium", req.GenerationConfig.ThinkingConfig.ThinkingLevel)
		require.Nil(t, req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	})

	t.Run("3.x nothinking is rejected", func(t *testing.T) {
		req := &dto.GeminiChatRequest{}
		require.Error(t, ThinkingAdaptor(req, newInfo("gemini-3.7-flash-nothinking")))
	})

	t.Run("3.8 nothinking is rejected", func(t *testing.T) {
		req := &dto.GeminiChatRequest{}
		require.Error(t, ThinkingAdaptor(req, newInfo("gemini-3.8-flash-nothinking")))
	})

	t.Run("3.x reasoning_effort maps to level", func(t *testing.T) {
		info := newInfo("gemini-3.5-flash")
		req := &dto.GeminiChatRequest{}
		require.NoError(t, ThinkingAdaptor(req, info, dto.GeneralOpenAIRequest{ReasoningEffort: "high"}))
		require.NotNil(t, req.GenerationConfig.ThinkingConfig)
		require.Equal(t, "high", req.GenerationConfig.ThinkingConfig.ThinkingLevel)
		require.Equal(t, "high", info.ReasoningEffort)
	})

	t.Run("3.x plain request without effort stays untouched", func(t *testing.T) {
		req := &dto.GeminiChatRequest{}
		require.NoError(t, ThinkingAdaptor(req, newInfo("gemini-3.5-flash")))
		require.Nil(t, req.GenerationConfig.ThinkingConfig)
	})

	t.Run("2.5 keeps numeric budget", func(t *testing.T) {
		req := &dto.GeminiChatRequest{}
		require.NoError(t, ThinkingAdaptor(req, newInfo("gemini-2.5-flash-thinking-8192")))
		require.NotNil(t, req.GenerationConfig.ThinkingConfig)
		require.Equal(t, "", req.GenerationConfig.ThinkingConfig.ThinkingLevel)
		require.NotNil(t, req.GenerationConfig.ThinkingConfig.ThinkingBudget)
		require.Equal(t, 8192, *req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	})

	t.Run("2.5 nothinking keeps budget 0", func(t *testing.T) {
		req := &dto.GeminiChatRequest{}
		require.NoError(t, ThinkingAdaptor(req, newInfo("gemini-2.5-flash-nothinking")))
		require.NotNil(t, req.GenerationConfig.ThinkingConfig)
		require.NotNil(t, req.GenerationConfig.ThinkingConfig.ThinkingBudget)
		require.Equal(t, 0, *req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	})
}
