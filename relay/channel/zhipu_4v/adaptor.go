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
	// FinalRequestRelayFormat 仅在 Convert 之后才被赋值；Convert 前此 guard 不生效，
	// Responses 兼容路径此时依赖 ConvertOpenAIResponsesRequest 里的 info.RelayFormat 判断兜底。
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
	// 白名单而非黑名单：直连 /v1/chat/completions 与 /v1/messages 不经 Path2RelayMode，RelayMode 为 Unknown，
	// 必须放行；count_tokens/compact/input_tokens/moderations/edits 等显式模式不再默认送 Claude 端点。
	switch info.RelayMode {
	case relayconstant.RelayModeUnknown,
		relayconstant.RelayModeChatCompletions,
		relayconstant.RelayModeCompletions,
		relayconstant.RelayModeResponses:
		return true
	default:
		return false
	}
}

func zhipuCodingPlanAliases() []string {
	return []string{"glm-coding-plan", "glm-coding-plan-international"}
}

func zhipuCodingPlanBases() map[string]channelconstant.ChannelSpecialBase {
	aliases := zhipuCodingPlanAliases()
	bases := make(map[string]channelconstant.ChannelSpecialBase, len(aliases))
	for _, alias := range aliases {
		if base, ok := channelconstant.ChannelSpecialBases[alias]; ok {
			bases[alias] = base
		}
	}
	return bases
}

func isZhipuCodingPlan(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	baseURL := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	for _, alias := range zhipuCodingPlanAliases() {
		if baseURL == alias {
			return true
		}
	}
	for _, specialBase := range zhipuCodingPlanBases() {
		if baseURL == strings.TrimRight(specialBase.ClaudeBaseURL, "/") ||
			baseURL == strings.TrimRight(specialBase.OpenAIBaseURL, "/") {
			return true
		}
	}
	return false
}

// isZhipuZcodeMode 判定 Coding Plan 渠道是否开启 ZCode 模式：开启后全部 LLM 请求
// 固定转 /v1/messages 并注入 ZCode 设备指纹；关闭保持原有透传逻辑。
func isZhipuZcodeMode(info *relaycommon.RelayInfo) bool {
	if !isZhipuCodingPlan(info) {
		return false
	}
	return info.ChannelSetting.ZcodeModeEnabled
}

func zhipuSpecialBase(baseURL string) (channelconstant.ChannelSpecialBase, bool) {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if specialBase, ok := channelconstant.ChannelSpecialBases[normalized]; ok {
		return specialBase, true
	}
	for _, specialBase := range zhipuCodingPlanBases() {
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
	if isZhipuZcodeMode(info) {
		setupZCodeCompatibilityHeaders(req)
	} else if isZhipuCodingPlan(info) {
		setupZCodeTraceHeaders(req)
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
		claudePath := "/v1/messages"
		if info.RelayMode == relayconstant.RelayModeClaudeCountTokens {
			claudePath = "/v1/messages/count_tokens"
		}
		if hasSpecialPlan && specialPlan.ClaudeBaseURL != "" {
			return fmt.Sprintf("%s%s", specialPlan.ClaudeBaseURL, claudePath), nil
		}
		return fmt.Sprintf("%s/api/anthropic%s", baseURL, claudePath), nil
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
	// shouldUseZhipuClaudeCompatibleAPI 对 nil 返回 false，短路后 info 必非 nil
	// ZCode 模式下 Responses 入站也固定转 Claude Messages，不走 OpenAI 直连。
	if shouldUseZhipuClaudeCompatibleAPI(info) && (info.RelayFormat != types.RelayFormatOpenAI || isZhipuZcodeMode(info)) {
		info.FinalRequestRelayFormat = types.RelayFormatClaude
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
