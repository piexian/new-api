package moonshot

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
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

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := claude.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	adaptor := openai.Adaptor{}
	return adaptor.ConvertImageRequest(c, info, request)
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := info.ChannelBaseUrl
	if shouldUseKimiCodingClaudeEndpoint(info) {
		return fmt.Sprintf("%s/v1/messages", kimiCodingClaudeBaseURL(baseURL)), nil
	}
	if specialPlan, ok := channelconstant.ChannelSpecialBases[baseURL]; ok {
		if info.RelayFormat == types.RelayFormatClaude {
			return fmt.Sprintf("%s/v1/messages", specialPlan.ClaudeBaseURL), nil
		}
		if info.RelayFormat == types.RelayFormatOpenAI {
			return fmt.Sprintf("%s/chat/completions", specialPlan.OpenAIBaseURL), nil
		}
	}

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return fmt.Sprintf("%s/anthropic/v1/messages", info.ChannelBaseUrl), nil
	default:
		if info.RelayMode == constant.RelayModeRerank {
			return fmt.Sprintf("%s/v1/rerank", info.ChannelBaseUrl), nil
		} else if info.RelayMode == constant.RelayModeEmbeddings {
			return fmt.Sprintf("%s/v1/embeddings", info.ChannelBaseUrl), nil
		} else if info.RelayMode == constant.RelayModeChatCompletions {
			return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
		} else if info.RelayMode == constant.RelayModeCompletions {
			return fmt.Sprintf("%s/v1/completions", info.ChannelBaseUrl), nil
		}
		return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if shouldUseKimiCodingClaudeEndpoint(info) {
		adaptor := claude.Adaptor{}
		return adaptor.ConvertOpenAIRequest(c, info, request)
	}
	return request, nil
}

func getUpstreamModelName(info *relaycommon.RelayInfo, fallback string) string {
	if info != nil && info.ChannelMeta != nil && info.UpstreamModelName != "" {
		return info.UpstreamModelName
	}
	return fallback
}

func isTemperatureOneOnlyModel(model string) bool {
	return strings.EqualFold(model, "kimi-k2.6")
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

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info != nil && info.RelayMode == constant.RelayModeResponses && info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI {
		if info.IsStream {
			return openai.ChatCompletionResponsesStreamHandler(c, info, resp)
		}
		return openai.ChatCompletionResponsesHandler(c, info, resp)
	}
	if shouldUseKimiCodingClaudeEndpoint(info) {
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	default:
		adaptor := openai.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func shouldUseKimiCodingClaudeEndpoint(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if info.RelayFormat == types.RelayFormatClaude {
		return isKimiCodingBaseURL(info.ChannelBaseUrl)
	}
	return info.RelayMode == constant.RelayModeChatCompletions &&
		isKimiCodingBaseURL(info.ChannelBaseUrl)
}

func isKimiCodingBaseURL(baseURL string) bool {
	normalized := strings.ToLower(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if normalized == "" {
		return false
	}
	if normalized == "kimi-coding-plan" {
		return true
	}
	return strings.HasSuffix(normalized, "/coding") ||
		strings.HasSuffix(normalized, "/coding/v1")
}

func kimiCodingClaudeBaseURL(baseURL string) string {
	if specialPlan, ok := channelconstant.ChannelSpecialBases[baseURL]; ok && specialPlan.ClaudeBaseURL != "" {
		return strings.TrimRight(specialPlan.ClaudeBaseURL, "/")
	}
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(trimmed), "/v1") {
		trimmed = strings.TrimRight(trimmed[:len(trimmed)-len("/v1")], "/")
	}
	return trimmed
}
