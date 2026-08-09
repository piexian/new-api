package oaichat

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/relayconvert/internal/matcher"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	return ShouldChatCompletionsUseResponsesChannelPolicy(policy, dto.ChannelSettings{}, channelID, channelType, model)
}

func ShouldChatCompletionsUseResponsesChannelPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelSetting dto.ChannelSettings, channelID int, channelType int, model string) bool {
	// Preserve the legacy per-channel switch exactly until the channel is saved
	// through the new visual policy editor.
	if channelSetting.ChatCompletionsToResponsesEnabled == nil && channelSetting.UseResponsesApi {
		return true
	}

	if common.IsOpenAIResponseCompactModel(model) {
		return false
	}

	enabled := policy.IsChannelEnabled(channelID, channelType)
	if channelSetting.ChatCompletionsToResponsesEnabled != nil {
		enabled = *channelSetting.ChatCompletionsToResponsesEnabled
	}
	if !enabled {
		return false
	}

	patterns := policy.ModelPatterns
	if hasConfiguredChatCompletionsToResponsesModels(channelSetting.ChatCompletionsToResponsesModels) {
		patterns = channelSetting.ChatCompletionsToResponsesModels
	}
	return matcher.MatchAnyRegex(patterns, model)
}

func ShouldChatCompletionsUseResponsesGlobal(channelSetting dto.ChannelSettings, channelID int, channelType int, model string) bool {
	return ShouldChatCompletionsUseResponsesChannelPolicy(
		model_setting.GetGlobalSettings().ChatCompletionsToResponsesPolicy,
		channelSetting,
		channelID,
		channelType,
		model,
	)
}

func hasConfiguredChatCompletionsToResponsesModels(patterns []string) bool {
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) != "" {
			return true
		}
	}
	return false
}
