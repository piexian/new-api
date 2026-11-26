package mistral

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
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
		// chat 端点：官方主机走公开预览的 OpenAI 兼容端点；代理/自定义 base 只保证原生路径
		chatPath := "/v1/chat/completions"
		if isOfficialMistralBase(info.ChannelBaseUrl) {
			chatPath = "/openai/v1/chat/completions"
		}
		return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, chatPath, info.ChannelType), nil
	}
	// Responses 及 input_tokens 走原生 /v1/responses*，OCR/embeddings/audio 直接透传入站路径
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
}

// isOfficialMistralBase 判断 base 是否为官方 API 主机；代理与自定义网关只有
// 原生 /v1/chat/completions 路由，OpenAI 兼容端点仅官方主机可用。
func isOfficialMistralBase(baseURL string) bool {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return parsed.Hostname() == "api.mistral.ai"
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
	// chat 端点即 OpenAI 兼容端点，请求原样透传
	if info != nil {
		info.ReasoningEffort = request.ReasoningEffort
		info.FinalRequestRelayFormat = types.RelayFormatOpenAI
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// Mistral 原生支持 OpenAI Responses，请求直传。无状态端点：带状态字段一律
	// 清空，携带非法值会被上游 422 拒绝
	request.Conversation = nil
	request.Prompt = nil
	request.Moderation = nil
	request.Store = nil
	request.Truncation = nil
	request.PreviousResponseID = ""
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
	}
	return request, nil
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
	case relayconstant.RelayModeResponses:
		// 原生 Responses 端点的事件流与响应对象均为 OpenAI 形状，openai 处理器可直接渲染
		adaptor := openai.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	case relayconstant.RelayModeResponsesInputTokens:
		return mistralResponsesInputTokensHandler(c, resp)
	}
	// chat（含 Claude 转 chat）走 OpenAI 兼容端点，响应即 OpenAI 格式；
	// normalizeMistral* 仅在上游回吐原生 content 块时改写，正常情况零改动
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
