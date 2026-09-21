package xiaomimimo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := NormalizeBaseURL(info.ChannelBaseUrl)
	if shouldUseClaudeCompatibleAPI(info) {
		return fmt.Sprintf("%s/anthropic/v1/messages", baseURL), nil
	}
	if info.RelayMode == relayconstant.RelayModeResponses {
		// MiMo 原生支持 OpenAI Responses 协议
		return fmt.Sprintf("%s/v1/responses", baseURL), nil
	}
	return fmt.Sprintf("%s/v1/chat/completions", baseURL), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	if shouldUseClaudeCompatibleAPI(info) {
		anthropicVersion := c.Request.Header.Get("anthropic-version")
		if anthropicVersion == "" {
			anthropicVersion = "2023-06-01"
		}
		req.Set("anthropic-version", anthropicVersion)
		claude.CommonClaudeHeadersOperation(c, req, info)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	// MiMo is OpenAI-compatible, pass through
	return request, nil
}

// normalizeResponsesReasoningEffort 将 OpenAI 扩展档位映射到 MiMo 接受的 none/low/medium/high
func normalizeResponsesReasoningEffort(effort string) string {
	switch effort {
	case "", "none", "low", "medium", "high":
		return effort
	case "minimal":
		return "low"
	case "xhigh":
		return "high"
	default:
		return effort
	}
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
	case relayconstant.RelayModeAudioTranscription:
		// MiMo ASR(mimo-v2.5-asr) 复用 chat/completions 端点
		return convertOpenAISTTToMiMo(c, request, info.UpstreamModelName)
	default:
		return nil, errors.New("unsupported audio relay mode")
	}

	stream := info.IsStream

	mimoReq := ConvertOpenAITTSToMiMo(request, info.UpstreamModelName, stream)
	jsonData, err := common.Marshal(mimoReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling xiaomi mimo request: %w", err)
	}

	// Store response format for later use in DoResponse
	responseFormat := request.ResponseFormat
	if responseFormat == "" {
		responseFormat = "wav"
	}
	if stream {
		responseFormat = "pcm16"
	}
	c.Set("response_format", responseFormat)

	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// MiMo 原生支持 OpenAI Responses(/v1/responses), 直接透传;
	// 官方声明不支持 previous_response_id/context_management, 携带会被忽略或报错, 统一清洗;
	// reasoning.effort 仅接受 none/low/medium/high, 归一 OpenAI 扩展档位防上游 400
	if request.PreviousResponseID != "" {
		request.PreviousResponseID = ""
	}
	request.ContextManagement = nil
	if request.Reasoning != nil {
		request.Reasoning = &dto.Reasoning{Effort: normalizeResponsesReasoningEffort(request.Reasoning.Effort)}
	}
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == relayconstant.RelayModeAudioTranscription {
		return handleASRResponse(c, resp, info)
	}
	if info.RelayMode == relayconstant.RelayModeAudioSpeech {
		if info.IsStream {
			return handleStreamTTSResponse(c, resp, info)
		}
		return handleTTSResponse(c, resp, info)
	}

	if shouldUseClaudeCompatibleAPI(info) {
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}

	adaptor := openai.Adaptor{}
	return adaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	// MiMo has native Anthropic-compatible API, pass through
	return request, nil
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func shouldUseClaudeCompatibleAPI(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	return info.RelayFormat == types.RelayFormatClaude
}
