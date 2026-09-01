package mistral

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	result, err := service.ConvertRequest(c, info, types.RelayFormatOpenAI, request)
	if err != nil {
		return nil, err
	}
	chatRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value)
	}
	return a.ConvertOpenAIRequest(c, info, chatRequest)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
		return convertSpeechRequest(&request)
	case relayconstant.RelayModeAudioTranscription, relayconstant.RelayModeAudioTranslation:
		return convertTranscriptionRequest(c, &request)
	default:
		return nil, errors.New("unsupported audio relay mode")
	}
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info != nil && info.RelayMode == relayconstant.RelayModeModerations {
		// moderation 模型不在 chat 端点上，必须走独立的 /v1/moderations
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, "/v1/moderations", info.ChannelType), nil
	}
	if info != nil && info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI {
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, "/v1/chat/completions", info.ChannelType), nil
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeModerations {
		// ClassificationRequest 仅收 model/input，独立类型避免聊天字段混入（additionalProperties: false）
		return moderationRequest{Model: request.Model, Input: request.Input}, nil
	}
	convertedRequest := requestOpenAI2Mistral(request)
	if info != nil {
		info.ReasoningEffort = convertedRequest.ReasoningEffort
		info.FinalRequestRelayFormat = types.RelayFormatOpenAI
	}
	return convertedRequest, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	chatRequest, err := responsescompat.ConvertToOpenAIChatRequest(request)
	if err != nil {
		return nil, err
	}
	// responsescompat 只保留 function 工具，这里补回 Mistral 支持的内置工具（web_search 系列）
	chatRequest.Tools = append(chatRequest.Tools, mistralBuiltInToolsFromResponses(request.Tools)...)
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAI
	}
	return requestOpenAI2Mistral(chatRequest), nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == relayconstant.RelayModeAudioTranscription || info.RelayMode == relayconstant.RelayModeAudioTranslation {
		return channel.DoFormRequest(a, c, info, requestBody)
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
		return MistralTTSHandler(c, resp, info)
	case relayconstant.RelayModeAudioTranscription, relayconstant.RelayModeAudioTranslation:
		sttErr, sttUsage := openai.OpenaiSTTHandler(c, resp, info, "json")
		return sttUsage, sttErr
	case relayconstant.RelayModeOCR:
		return MistralOCRHandler(c, resp, info)
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeResponses && info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI {
		if info.IsStream {
			return openai.ChatCompletionResponsesStreamHandlerWithDataTransformer(c, info, resp, normalizeMistralStreamData)
		}
		return openai.ChatCompletionResponsesHandlerWithBodyTransformer(c, info, resp, normalizeMistralResponseData)
	}
	if info.IsStream {
		usage, err = openai.OaiStreamHandlerWithDataTransformer(c, info, resp, normalizeMistralStreamData)
	} else {
		usage, err = openai.OpenaiHandlerWithBodyTransformer(c, info, resp, normalizeMistralResponseData)
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
