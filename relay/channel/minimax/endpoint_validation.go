package minimax

import (
	"fmt"
	"net/http"
	"strings"

	appconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
)

type EndpointGuide struct {
	Endpoint string
	DocURL   string
	Label    string
}

func ValidateEndpointForModel(info *relaycommon.RelayInfo) *types.NewAPIError {
	if info == nil || info.ChannelType != appconstant.ChannelTypeMiniMax {
		return nil
	}
	if info.RelayMode == relayconstant.RelayModeMiniMaxMusicCoverPreprocess && info.UpstreamModelName == "music-cover" {
		return nil
	}
	guide, ok := ExpectedEndpointForModel(info)
	if !ok {
		return nil
	}
	if endpointMatchesRelayMode(info, guide.Endpoint) {
		return nil
	}
	message := fmt.Sprintf(
		"MiniMax %s must be called with %s. See MiniMax documentation: %s",
		guide.Label,
		guide.Endpoint,
		guide.DocURL,
	)
	return types.NewOpenAIError(
		fmt.Errorf("%s", message),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithSkipSensitiveMask(),
	)
}

func ExpectedEndpointForModel(info *relaycommon.RelayInfo) (EndpointGuide, bool) {
	if info == nil {
		return EndpointGuide{}, false
	}
	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = info.OriginModelName
	}
	modelName = strings.TrimSuffix(modelName, ratio_setting.CompactModelSuffix)
	return expectedEndpointForModel(modelName, info.RelayFormat)
}

func expectedEndpointForModel(modelName string, relayFormat types.RelayFormat) (EndpointGuide, bool) {
	switch {
	case isMiniMaxMusicModel(modelName):
		return EndpointGuide{Endpoint: MusicGenerationEndpoint, DocURL: MusicGenerationDocURL, Label: "music model"}, true
	case modelName == MusicCoverPreprocessModel:
		return EndpointGuide{Endpoint: MusicCoverPreprocessEndpoint, DocURL: MusicCoverPreprocessDocURL, Label: "music cover preprocess model"}, true
	case modelName == LyricsGenerationModel:
		return EndpointGuide{Endpoint: LyricsGenerationEndpoint, DocURL: LyricsGenerationDocURL, Label: "lyrics generation model"}, true
	case isMiniMaxSpeechModel(modelName):
		return EndpointGuide{Endpoint: SpeechEndpoint, DocURL: SpeechDocURL, Label: "speech model"}, true
	case isMiniMaxImageModel(modelName):
		return EndpointGuide{Endpoint: ImageGenerationEndpoint, DocURL: ImageGenerationDocURL, Label: "image model"}, true
	case isMiniMaxTextModel(modelName):
		if relayFormat == types.RelayFormatClaude {
			return EndpointGuide{Endpoint: AnthropicMessagesEndpoint, DocURL: AnthropicMessagesDocURL, Label: "text model"}, true
		}
		return EndpointGuide{Endpoint: ChatCompletionsEndpoint, DocURL: OpenAIChatCompletionsDocURL, Label: "text model"}, true
	default:
		return EndpointGuide{}, false
	}
}

func endpointMatchesRelayMode(info *relaycommon.RelayInfo, endpoint string) bool {
	if info == nil {
		return false
	}
	switch endpoint {
	case MusicGenerationEndpoint:
		return info.RelayMode == relayconstant.RelayModeMiniMaxMusicGeneration
	case MusicCoverPreprocessEndpoint:
		return info.RelayMode == relayconstant.RelayModeMiniMaxMusicCoverPreprocess
	case LyricsGenerationEndpoint:
		return info.RelayMode == relayconstant.RelayModeMiniMaxLyricsGeneration
	case ChatCompletionsEndpoint:
		return (info.RelayMode == relayconstant.RelayModeChatCompletions || info.RelayMode == relayconstant.RelayModeResponses) &&
			info.RelayFormat != types.RelayFormatClaude
	case AnthropicMessagesEndpoint:
		return info.RelayFormat == types.RelayFormatClaude && strings.HasPrefix(info.RequestURLPath, AnthropicMessagesEndpoint)
	case SpeechEndpoint:
		return info.RelayMode == relayconstant.RelayModeAudioSpeech
	case ImageGenerationEndpoint:
		return info.RelayMode == relayconstant.RelayModeImagesGenerations && strings.HasPrefix(info.RequestURLPath, ImageGenerationEndpoint)
	default:
		return false
	}
}
