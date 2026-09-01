// Package gemini_interactions 是独立的 Gemini Interactions 渠道适配器。
// 协议转换全部走 relayconvert 统一直接转换器(单步):
//   - 请求: openai/claude/gemini/responses -> interactions (ConverterOpenAIChatToInteractions 等)
//   - 响应: interactions -> 入站格式 (convertGeminiInteractionsToOpenAIChat 等)
//
// 入站 interactions 端点(RelayModeGeminiInteractions)直接透传(含映射表路由与异步结算)。
package gemini_interactions

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"

	gemini "github.com/QuantumNous/new-api/relay/channel/gemini"

	"github.com/gin-gonic/gin"
)

const ChannelName = "Gemini Interactions"

// ModelList Interactions 专属模型
var ModelList = []string{
	"gemini-3.7-flash",
	"gemini-3.6-flash",
	"gemini-3.5-flash",
	"gemini-3.5-flash-lite",
	"gemini-3.1-flash-lite",
	"gemini-3.1-pro-preview",
	"gemini-3-flash-preview",
	"gemini-2.5-pro",
	"gemini-2.5-flash",
	"gemini-2.5-flash-lite",
	"deep-research-pro-preview-12-2025",
	"deep-research-preview-04-2026",
	"deep-research-max-preview-04-2026",
	"antigravity-preview-05-2026",
	// embedding 不在 Interactions 协议内,仍走原生 embedContent 老路径
	"gemini-embedding-001",
	"gemini-embedding-2-preview",
}

type Adaptor struct {
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	// 注入桥接查找,使统一转换器能做工具调用续链
	relayconvert.SetGeminiInteractionsBridgeLookup(interactionsBridgeLookup)
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	if info.RelayMode == constant.RelayModeGeminiInteractions {
		requestPath := info.RequestURLPath
		query := ""
		if idx := strings.Index(requestPath, "?"); idx >= 0 {
			query = requestPath[idx:]
			requestPath = requestPath[:idx]
		}
		for _, prefix := range []string{"/v1beta/interactions", "/v1beta2/interactions", "/v1/interactions"} {
			if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
				return fmt.Sprintf("%s%s%s", baseURL, requestPath, query), nil
			}
		}
		return "", fmt.Errorf("unsupported gemini interactions path: %s", requestPath)
	}

	// 渠道级开关:OpenAI Chat 入站直传上游 OpenAI 兼容端点,不再转 Interactions
	if info.UpstreamOpenAICompatChat() {
		return fmt.Sprintf("%s/v1beta/openai/chat/completions", baseURL), nil
	}

	// Google 未将 embeddings 迁入 Interactions,embed 沿用原生 models/{model}:embedContent 老路径
	if info.RelayMode == constant.RelayModeEmbeddings || isEmbeddingModelName(info.UpstreamModelName) {
		return (&gemini.Adaptor{}).GetRequestURL(info)
	}
	version := common.GetStringIfEmpty(info.ApiVersion, "v1beta")
	return fmt.Sprintf("%s/%s/interactions", baseURL, version), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	req.Set("x-goog-api-key", info.ApiKey)
	return nil
}

// ConvertRequest 统一入口:按入站请求类型走 relayconvert 直转(单步)
func (a *Adaptor) ConvertRequest(c *gin.Context, info *relaycommon.RelayInfo, request any) (any, error) {
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatGeminiInteractions, request)
	if err != nil {
		return nil, err
	}
	return result.Value, nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if info.UpstreamOpenAICompatChat() {
		// 直传上游兼容层:请求保持 OpenAI 形状;流式必须显式索要 usage 供计费
		if request != nil && request.Stream != nil && *request.Stream && request.StreamOptions == nil {
			request.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
		}
		return request, nil
	}
	return a.ConvertRequest(c, info, request)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatGeminiInteractions
	}
	return a.ConvertRequest(c, info, req)
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return a.ConvertRequest(c, info, request)
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatGeminiInteractions
	}
	return a.ConvertRequest(c, info, &request)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, fmt.Errorf("gemini interactions channel does not support rerank")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	// embed 载荷构造复用 Gemini 原生实现(embedContent / batchEmbedContents)
	return (&gemini.Adaptor{}).ConvertEmbeddingRequest(c, info, request)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, fmt.Errorf("gemini interactions channel does not support audio")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, fmt.Errorf("gemini interactions channel does not support image generation")
}

// isEmbeddingModelName Google 未把 embeddings 迁入 Interactions,该类模型仍走原生 embedContent 老路径
func isEmbeddingModelName(name string) bool {
	return strings.HasPrefix(name, "text-embedding") ||
		strings.HasPrefix(name, "embedding") ||
		strings.HasPrefix(name, "gemini-embedding")
}

// interactionsBridgeLookup 工具调用桥接查找:校验归属,命中锁定原上游 key
func interactionsBridgeLookup(info *relaycommon.RelayInfo) relayconvert.BridgeLookup {
	return func(callID string) (string, bool) {
		bridge := service.GetGeminiInteractionToolCallBridge(callID)
		if bridge == nil || bridge.UserID != info.UserId || bridge.ChannelID != info.ChannelId {
			return "", false
		}
		if bridge.Key != "" {
			info.ApiKey = bridge.Key
		}
		return bridge.InteractionID, true
	}
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

// DoResponse 入站 interactions 透传;转换模式将上游 interaction 还原为
// gemini_chat 形状后,按入站 RelayFormat 走统一响应转换器
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeGeminiInteractions {
		if info.IsStream {
			return gemini.GeminiInteractionsStreamHandler(c, info, resp)
		}
		return gemini.GeminiInteractionsHandler(c, info, resp)
	}

	// 渠道级开关开启时上游返回 OpenAI 形状,直接走 OpenAI 响应处理
	if info.UpstreamOpenAICompatChat() {
		if info.IsStream {
			return openai.OaiStreamHandler(c, info, resp)
		}
		return openai.OpenaiHandler(c, info, resp)
	}

	// embed 响应解析复用 Gemini 原生实现(NativeGeminiEmbeddingHandler / GeminiEmbeddingHandler)
	if info.RelayMode == constant.RelayModeEmbeddings || isEmbeddingModelName(info.UpstreamModelName) {
		return (&gemini.Adaptor{}).DoResponse(c, resp, info)
	}
	converted := gemini.ConvertInteractionsUpstreamResponse(c, info, resp)
	switch info.RelayFormat {
	case types.RelayFormatGemini:
		if info.IsStream {
			return gemini.GeminiTextGenerationStreamHandler(c, info, converted)
		}
		return gemini.GeminiTextGenerationHandler(c, info, converted)
	case types.RelayFormatOpenAIResponses:
		if info.IsStream {
			return gemini.GeminiResponsesStreamHandler(c, info, converted)
		}
		return gemini.GeminiResponsesHandler(c, info, converted)
	default:
		if info.IsStream {
			return gemini.GeminiChatStreamHandler(c, info, converted)
		}
		return gemini.GeminiChatHandler(c, info, converted)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

// gemini 别名引用(handlers 位于 relay/channel/gemini 包)
var _ = common.GetStringIfEmpty
