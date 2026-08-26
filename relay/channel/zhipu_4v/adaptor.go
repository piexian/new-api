package zhipu_4v

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func shouldUseZhipuClaudeCompatibleAPI(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if info.RelayMode == relayconstant.RelayModeResponses && info.FinalRequestRelayFormat == types.RelayFormatOpenAI {
		return false
	}
	if isZhipuCodingPlanClaudeRequest(info) {
		return true
	}
	if info.RelayFormat == types.RelayFormatClaude {
		return true
	}
	return common.IsClaudeCompatibleModel(info.UpstreamModelName)
}

func isZhipuCodingPlanClaudeRequest(info *relaycommon.RelayInfo) bool {
	if !isZhipuCodingPlan(info) {
		return false
	}
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings,
		relayconstant.RelayModeImagesGenerations,
		relayconstant.RelayModeImagesEdits,
		relayconstant.RelayModeAudioSpeech,
		relayconstant.RelayModeAudioTranscription,
		relayconstant.RelayModeAudioTranslation,
		relayconstant.RelayModeRerank:
		return false
	default:
		return true
	}
}

func isZhipuCodingPlan(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	baseURL := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if baseURL == "glm-coding-plan" || baseURL == "glm-coding-plan-international" {
		return true
	}
	for alias, specialBase := range channelconstant.ChannelSpecialBases {
		if alias != "glm-coding-plan" && alias != "glm-coding-plan-international" {
			continue
		}
		if baseURL == strings.TrimRight(specialBase.ClaudeBaseURL, "/") ||
			baseURL == strings.TrimRight(specialBase.OpenAIBaseURL, "/") {
			return true
		}
	}
	return false
}

func zhipuSpecialBase(baseURL string) (channelconstant.ChannelSpecialBase, bool) {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if specialBase, ok := channelconstant.ChannelSpecialBases[normalized]; ok {
		return specialBase, true
	}
	for alias, specialBase := range channelconstant.ChannelSpecialBases {
		if alias != "glm-coding-plan" && alias != "glm-coding-plan-international" {
			continue
		}
		if normalized == strings.TrimRight(specialBase.ClaudeBaseURL, "/") ||
			normalized == strings.TrimRight(specialBase.OpenAIBaseURL, "/") {
			return specialBase, true
		}
	}
	return channelconstant.ChannelSpecialBase{}, false
}

func setupZhipuClaudeCompatibleHeaders(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("x-api-key", info.ApiKey)
	anthropicVersion := c.Request.Header.Get("anthropic-version")
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	req.Set("anthropic-version", anthropicVersion)
	claude.CommonClaudeHeadersOperation(c, req, info)
	if isZhipuCodingPlan(info) {
		setupZCodeCompatibilityHeaders(req)
	}
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	return req, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeZhipu_v4]
	}
	specialPlan, hasSpecialPlan := zhipuSpecialBase(baseURL)

	switch {
	case shouldUseZhipuClaudeCompatibleAPI(info):
		if hasSpecialPlan && specialPlan.ClaudeBaseURL != "" {
			return fmt.Sprintf("%s/v1/messages", specialPlan.ClaudeBaseURL), nil
		}
		return fmt.Sprintf("%s/api/anthropic/v1/messages", baseURL), nil
	default:
		switch info.RelayMode {
		case relayconstant.RelayModeEmbeddings:
			if hasSpecialPlan && specialPlan.OpenAIBaseURL != "" {
				return fmt.Sprintf("%s/embeddings", specialPlan.OpenAIBaseURL), nil
			}
			return fmt.Sprintf("%s/api/paas/v4/embeddings", baseURL), nil
		case relayconstant.RelayModeImagesGenerations:
			if hasSpecialPlan && specialPlan.OpenAIBaseURL != "" {
				return fmt.Sprintf("%s/images/generations", specialPlan.OpenAIBaseURL), nil
			}
			return fmt.Sprintf("%s/api/paas/v4/images/generations", baseURL), nil
		default:
			if hasSpecialPlan && specialPlan.OpenAIBaseURL != "" {
				return fmt.Sprintf("%s/chat/completions", specialPlan.OpenAIBaseURL), nil
			}
			return fmt.Sprintf("%s/api/paas/v4/chat/completions", baseURL), nil
		}
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if shouldUseZhipuClaudeCompatibleAPI(info) {
		setupZhipuClaudeCompatibleHeaders(c, req, info)
		return nil
	}
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if shouldUseZhipuClaudeCompatibleAPI(info) {
		adaptor := claude.Adaptor{}
		return adaptor.ConvertOpenAIRequest(c, info, request)
	}
	if lo.FromPtrOr(request.TopP, 0) >= 1 {
		request.TopP = lo.ToPtr(0.99)
	}
	return requestOpenAI2Zhipu(*request), nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// Chat completions can be internally routed through the Responses relay.
	// Keep that compatibility path on OpenAI Chat so its response handler can
	// aggregate the upstream stream back into the original Chat contract.
	if shouldUseZhipuClaudeCompatibleAPI(info) && (info == nil || info.RelayFormat != types.RelayFormatOpenAI) {
		if info != nil {
			info.FinalRequestRelayFormat = types.RelayFormatClaude
		}
		return relayconvert.OpenAIResponsesRequestToClaudeMessages(c, &request)
	}
	chatRequest, err := responsescompat.ConvertToOpenAIChatRequest(request)
	if err != nil {
		return nil, err
	}
	if lo.FromPtrOr(chatRequest.TopP, 0) >= 1 {
		chatRequest.TopP = lo.ToPtr(0.99)
	}
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAI
	}
	return requestOpenAI2Zhipu(*chatRequest), nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info != nil && info.RelayMode == relayconstant.RelayModeResponses && info.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		if info.IsStream {
			return zhipuClaudeResponsesStreamHandler(c, info, resp)
		}
		return claude.ClaudeResponsesHandler(c, resp, info)
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeResponses && info.GetFinalRequestRelayFormat() == types.RelayFormatOpenAI {
		if info.IsStream {
			return openai.ChatCompletionResponsesStreamHandler(c, info, resp)
		}
		return openai.ChatCompletionResponsesHandler(c, info, resp)
	}
	switch {
	case shouldUseZhipuClaudeCompatibleAPI(info):
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	default:
		if info.RelayMode == relayconstant.RelayModeImagesGenerations {
			return zhipu4vImageHandler(c, resp, info)
		}
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
