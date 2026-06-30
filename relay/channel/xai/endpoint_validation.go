package xai

import (
	"fmt"
	"net/http"
	"strings"

	appconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
)

type xAIRouteKind string

const (
	xAIRouteKindText  xAIRouteKind = "text"
	xAIRouteKindImage xAIRouteKind = "image"
	xAIRouteKindVideo xAIRouteKind = "video"
	xAIRouteKindVoice xAIRouteKind = "voice"
)

func ValidateEndpointForModel(info *relaycommon.RelayInfo) *types.NewAPIError {
	if info == nil || info.ChannelMeta == nil || info.ChannelType != appconstant.ChannelTypeXai {
		return nil
	}

	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = info.OriginModelName
	}
	modelKind, ok := xAIModelKind(modelName)
	if !ok {
		return nil
	}
	if modelKind == xAIRouteKindText {
		return nil
	}

	routeKind, ok := xAIRouteKindForInfo(info)
	if !ok {
		return nil
	}
	if routeKind == modelKind {
		return nil
	}

	message := fmt.Sprintf("xAI %s model %q must be used with %s endpoint, got %s", modelKind, modelName, expectedXAIEndpoint(modelKind), info.RequestURLPath)
	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s", message),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithSkipSensitiveMask(),
	)
}

func xAIRouteKindForInfo(info *relaycommon.RelayInfo) (xAIRouteKind, bool) {
	if info == nil {
		return "", false
	}
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		return xAIRouteKindImage, true
	case relayconstant.RelayModeVideoSubmit:
		return xAIRouteKindVideo, true
	case relayconstant.RelayModeAudioSpeech, relayconstant.RelayModeAudioTranscription, relayconstant.RelayModeAudioTranslation, relayconstant.RelayModeRealtime:
		return xAIRouteKindVoice, true
	case relayconstant.RelayModeXAINative:
		if strings.HasPrefix(info.RequestURLPath, "/v1/tts") ||
			strings.HasPrefix(info.RequestURLPath, "/v1/stt") ||
			strings.HasPrefix(info.RequestURLPath, "/v1/realtime/client_secrets") ||
			strings.HasPrefix(info.RequestURLPath, "/v1/custom-voices") {
			return xAIRouteKindVoice, true
		}
		if strings.HasPrefix(info.RequestURLPath, "/v1/responses") ||
			strings.HasPrefix(info.RequestURLPath, "/v1/chat/deferred-completion/") {
			return xAIRouteKindText, true
		}
	}
	return xAIRouteKindText, true
}

func expectedXAIEndpoint(kind xAIRouteKind) string {
	switch kind {
	case xAIRouteKindImage:
		return "/v1/images/generations or /v1/images/edits"
	case xAIRouteKindVideo:
		return "/v1/videos/generations, /v1/videos/edits, or /v1/videos/extensions"
	case xAIRouteKindVoice:
		return "/v1/tts, /v1/stt, /v1/realtime, /v1/realtime/client_secrets, or /v1/custom-voices"
	default:
		return "/v1/chat/completions or /v1/responses"
	}
}

func xAIModelKind(modelName string) (xAIRouteKind, bool) {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case modelName == "":
		return "", false
	case strings.HasPrefix(modelName, "grok-imagine-image"), modelName == "grok-2-image-1212":
		return xAIRouteKindImage, true
	case strings.HasPrefix(modelName, "grok-imagine-video"):
		return xAIRouteKindVideo, true
	case strings.HasPrefix(modelName, "grok-voice-"):
		return xAIRouteKindVoice, true
	default:
		return xAIRouteKindText, true
	}
}
