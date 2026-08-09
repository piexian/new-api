package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/setting/model_setting"
)

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	return relayconvert.ShouldChatCompletionsUseResponsesPolicy(policy, channelID, channelType, model)
}

func ShouldChatCompletionsUseResponsesGlobal(channelSetting dto.ChannelSettings, channelID int, channelType int, model string) bool {
	return relayconvert.ShouldChatCompletionsUseResponsesGlobal(channelSetting, channelID, channelType, model)
}
