package minimax

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
)

func GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := miniMaxRootBaseURL(info)
	if info.RelayMode == constant.RelayModeClaudeCountTokens {
		return fmt.Sprintf("%s/anthropic/v1/messages/count_tokens", baseURL), nil
	}
	if shouldUseMiniMaxClaudeCompatibleAPI(info) {
		return fmt.Sprintf("%s/anthropic/v1/messages", baseURL), nil
	}
	if info.RelayMode == constant.RelayModeResponses && info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI {
		return fmt.Sprintf("%s/v1/chat/completions", baseURL), nil
	}
	switch info.RelayMode {
	case constant.RelayModeChatCompletions:
		return fmt.Sprintf("%s/v1/chat/completions", baseURL), nil
	case constant.RelayModeResponses:
		return fmt.Sprintf("%s/v1/responses", baseURL), nil
	case constant.RelayModeResponsesInputTokens:
		return fmt.Sprintf("%s/v1/responses/input_tokens", baseURL), nil
	case constant.RelayModeImagesGenerations:
		return fmt.Sprintf("%s/v1/image_generation", baseURL), nil
	case constant.RelayModeMiniMaxMusicGeneration:
		return fmt.Sprintf("%s/v1/music_generation", baseURL), nil
	case constant.RelayModeMiniMaxMusicCoverPreprocess:
		return fmt.Sprintf("%s/v1/music_cover_preprocess", baseURL), nil
	case constant.RelayModeMiniMaxLyricsGeneration:
		return fmt.Sprintf("%s/v1/lyrics_generation", baseURL), nil
	case constant.RelayModeAudioSpeech:
		if isMiniMaxMusicModel(info.OriginModelName) {
			return fmt.Sprintf("%s/v1/music_generation", baseURL), nil
		}
		return fmt.Sprintf("%s/v1/t2a_v2", baseURL), nil
	default:
		return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
}

func shouldUseMiniMaxClaudeCompatibleAPI(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if info.RelayFormat == types.RelayFormatClaude {
		return true
	}
	if info.ChannelMeta == nil {
		return false
	}
	return common.IsClaudeCompatibleModel(info.UpstreamModelName)
}

func miniMaxRootBaseURL(info *relaycommon.RelayInfo) string {
	baseURL := ""
	if info != nil {
		baseURL = info.ChannelBaseUrl
	}
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeMiniMax]
	}
	return NormalizeBaseURL(baseURL)
}

func NormalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	for _, suffix := range []string{"/v1", "/anthropic"} {
		baseURL = strings.TrimSuffix(baseURL, suffix)
	}
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeMiniMax]
	}
	return baseURL
}
