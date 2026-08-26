package gmicloud

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}
func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("relay info is nil")
	}
	model := gmiModelName(info)
	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
		if isGMIMusicModel(model) {
			return "", errors.New("gmicloud music models only support /v1/music_generation")
		}
		if !isGMITTSModel(model) && !isGMIVoiceCloneModel(model) {
			return "", fmt.Errorf("gmicloud model %q does not support /v1/audio/speech", model)
		}
		return audioBaseURL(info) + submitRequestPath, nil
	case relayconstant.RelayModeMiniMaxMusicGeneration:
		if !IsSupportedMusicModel(model) {
			return "", fmt.Errorf("gmicloud model %q does not support /v1/music_generation", model)
		}
		return audioBaseURL(info) + submitRequestPath, nil
	}
	if isGMIAudioModel(model) {
		return "", fmt.Errorf("gmicloud audio model %q is not available on this endpoint", model)
	}

	base := llmBaseURL(info)
	if info.RelayFormat == types.RelayFormatClaude {
		return base + "/v1/messages", nil
	}
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		return base + "/v1/chat/completions", nil
	case relayconstant.RelayModeResponses:
		if info.GetFinalRequestRelayFormat() != types.RelayFormatOpenAI {
			return "", errors.New("gmicloud /v1/responses requires OpenAI chat-completions conversion")
		}
		return base + "/v1/chat/completions", nil
	case relayconstant.RelayModeResponsesInputTokens:
		return "", errors.New("gmicloud does not support /v1/responses/input_tokens")
	default:
		return relaycommon.GetFullRequestURL(base, info.RequestURLPath, info.ChannelType), nil
	}
}
func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	if info.RelayFormat == types.RelayFormatClaude {
		req.Set("x-api-key", info.ApiKey)
		req.Del("Authorization")
		claude.CommonClaudeHeadersOperation(c, req, info)
	} else {
		req.Set("Authorization", "Bearer "+info.ApiKey)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if info.RelayFormat == types.RelayFormatClaude {
		result, err := relayConvertToClaude(c, info, request)
		if err != nil {
			return nil, err
		}
		info.FinalRequestRelayFormat = types.RelayFormatClaude
		return result, nil
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	chatRequest, err := responsescompat.ConvertToOpenAIChatRequest(request)
	if err != nil {
		return nil, err
	}
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAI
	}
	return chatRequest, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}
	model := gmiModelName(info)
	if isGMIMusicModel(model) {
		return nil, errors.New("gmicloud music models require /v1/music_generation")
	}
	if !isGMITTSModel(model) && !isGMIVoiceCloneModel(model) {
		return nil, fmt.Errorf("gmicloud model %q does not support /v1/audio/speech", model)
	}
	return buildAudioRequestBody(c, info, &request)
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == relayconstant.RelayModeMiniMaxMusicGeneration {
		if !IsSupportedMusicModel(gmiModelName(info)) {
			return nil, fmt.Errorf("gmicloud model %q does not support /v1/music_generation", gmiModelName(info))
		}
		converted, err := buildMiniMaxMusicRequestBody(info, requestBody)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}
	response, err := channel.DoApiRequest(a, c, info, requestBody)
	if err != nil || !isAsyncMediaRequest(info) || response == nil || response.StatusCode == http.StatusOK {
		return response, err
	}
	defer service.CloseResponseBodyGracefully(response)

	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		body = []byte(fmt.Sprintf("failed to read error response: %v", readErr))
	}
	return nil, gmiUpstreamHTTPError("media submit", response.StatusCode, body)
}

func isAsyncMediaRequest(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	model := gmiModelName(info)
	return (info.RelayMode == relayconstant.RelayModeAudioSpeech && IsSupportedSpeechModel(model)) ||
		(info.RelayMode == relayconstant.RelayModeMiniMaxMusicGeneration && IsSupportedMusicModel(model))
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == relayconstant.RelayModeMiniMaxMusicGeneration && IsSupportedMusicModel(gmiModelName(info)) {
		return handleMiniMaxMusicResponse(c, resp, info)
	}
	if info.RelayMode == relayconstant.RelayModeAudioSpeech && (isGMITTSModel(gmiModelName(info)) || isGMIVoiceCloneModel(gmiModelName(info))) {
		return handleAudioResponse(c, resp, info)
	}

	// Responses API converted to chat completions.
	if info.RelayMode == relayconstant.RelayModeResponses && info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI {
		if info.IsStream {
			return openai.ChatCompletionResponsesStreamHandler(c, info, resp)
		}
		return openai.ChatCompletionResponsesHandler(c, info, resp)
	}

	// Anthropic Messages format.
	if info.RelayFormat == types.RelayFormatClaude {
		claudeAdaptor := claude.Adaptor{}
		return claudeAdaptor.DoResponse(c, resp, info)
	}

	// OpenAI-compatible chat.
	if info.IsStream {
		return openai.OaiStreamHandler(c, info, resp)
	}
	return openai.OpenaiHandler(c, info, resp)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func llmBaseURL(info *relaycommon.RelayInfo) string {
	base := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if base == "" {
		return defaultLLMBaseURL
	}
	return base
}

func audioBaseURL(info *relaycommon.RelayInfo) string {
	base := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if base == "" || strings.Contains(base, "gmi-serving.com") {
		return defaultAudioBaseURL
	}
	return base
}

func gmiModelName(info *relaycommon.RelayInfo) string {
	if info != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		return strings.TrimSpace(info.UpstreamModelName)
	}
	if info == nil {
		return ""
	}
	return strings.TrimSpace(info.OriginModelName)
}

// relayConvertToClaude reuses the framework's OpenAI→Claude converter.
func relayConvertToClaude(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	adaptor := claude.Adaptor{}
	return adaptor.ConvertOpenAIRequest(c, info, request)
}
