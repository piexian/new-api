package minimax

import (
	"fmt"
	"strings"

	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
)

func GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := miniMaxRootBaseURL(info)
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return fmt.Sprintf("%s/anthropic/v1/messages", baseURL), nil
	default:
		switch info.RelayMode {
		case constant.RelayModeChatCompletions:
			return fmt.Sprintf("%s/v1/chat/completions", baseURL), nil
		case constant.RelayModeImagesGenerations:
			return fmt.Sprintf("%s/v1/image_generation", baseURL), nil
		case constant.RelayModeAudioSpeech:
			if isMiniMaxMusicModel(info.OriginModelName) {
				return fmt.Sprintf("%s/v1/music_generation", baseURL), nil
			}
			return fmt.Sprintf("%s/v1/t2a_v2", baseURL), nil
		default:
			return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
		}
	}
}

func miniMaxRootBaseURL(info *relaycommon.RelayInfo) string {
	baseURL := ""
	if info != nil {
		baseURL = info.ChannelBaseUrl
	}
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeMiniMax]
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	for _, suffix := range []string{"/v1", "/anthropic"} {
		baseURL = strings.TrimSuffix(baseURL, suffix)
	}
	return baseURL
}
