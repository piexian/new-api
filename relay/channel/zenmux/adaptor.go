package zenmux

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	requestModeOpenAI = iota + 1
	requestModeClaude
	requestModeVertex
)

type Adaptor struct {
	RequestMode int
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.RequestMode = requestModeOpenAI
	if info == nil {
		return
	}
	if info.RelayFormat == types.RelayFormatClaude ||
		info.RelayMode == relayconstant.RelayModeClaudeCountTokens {
		a.RequestMode = requestModeClaude
		return
	}
	if info.RelayFormat == types.RelayFormatGemini ||
		info.RelayMode == relayconstant.RelayModeGemini {
		a.RequestMode = requestModeVertex
	}
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	geminiAdaptor := gemini.Adaptor{}
	return geminiAdaptor.ConvertGeminiRequest(c, info, request)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	claudeAdaptor := claude.Adaptor{}
	return claudeAdaptor.ConvertClaudeRequest(c, info, request)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("zenmux audio relay is not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("zenmux image relay is not implemented")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	switch a.RequestMode {
	case requestModeClaude:
		return zenMuxClaudeURL(info)
	case requestModeVertex:
		return zenMuxVertexURL(info)
	default:
		return zenMuxOpenAIURL(info), nil
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	switch a.RequestMode {
	case requestModeClaude:
		req.Set("x-api-key", info.ApiKey)
		anthropicVersion := c.Request.Header.Get("anthropic-version")
		if anthropicVersion == "" {
			anthropicVersion = "2023-06-01"
		}
		req.Set("anthropic-version", anthropicVersion)
		claude.CommonClaudeHeadersOperation(c, req, info)
	case requestModeVertex:
		req.Set("Authorization", "Bearer "+info.ApiKey)
	default:
		req.Set("Authorization", "Bearer "+info.ApiKey)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("zenmux rerank relay is not implemented")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
	}
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch a.RequestMode {
	case requestModeClaude:
		claudeAdaptor := claude.Adaptor{}
		return claudeAdaptor.DoResponse(c, resp, info)
	case requestModeVertex:
		if info.RelayMode == relayconstant.RelayModeGemini {
			if strings.Contains(info.RequestURLPath, ":embedContent") ||
				strings.Contains(info.RequestURLPath, ":batchEmbedContents") {
				return gemini.NativeGeminiEmbeddingHandler(c, resp, info)
			}
			if info.IsStream {
				return gemini.GeminiTextGenerationStreamHandler(c, info, resp)
			}
			return gemini.GeminiTextGenerationHandler(c, info, resp)
		}
		if info.IsStream {
			return gemini.GeminiChatStreamHandler(c, info, resp)
		}
		return gemini.GeminiChatHandler(c, info, resp)
	default:
		openaiAdaptor := openai.Adaptor{}
		return openaiAdaptor.DoResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func zenMuxRoot(info *relaycommon.RelayInfo) string {
	baseURL := ""
	if info != nil {
		baseURL = strings.TrimSpace(info.ChannelBaseUrl)
	}
	return NormalizeRoot(baseURL)
}

func NormalizeRoot(baseURL string) string {
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeZenMux]
	}
	baseURL = strings.TrimRight(baseURL, "/")
	for _, suffix := range []string{"/api/v1", "/api/anthropic", "/api/vertex-ai"} {
		if strings.HasSuffix(baseURL, suffix) {
			return strings.TrimSuffix(baseURL, suffix)
		}
	}
	return baseURL
}

func OpenAIBaseURL(baseURL string) string {
	return NormalizeRoot(baseURL) + "/api/v1"
}

func zenMuxOpenAIBase(info *relaycommon.RelayInfo) string {
	if info == nil {
		return OpenAIBaseURL("")
	}
	return OpenAIBaseURL(info.ChannelBaseUrl)
}

func zenMuxAnthropicBase(info *relaycommon.RelayInfo) string {
	return zenMuxRoot(info) + "/api/anthropic"
}

func zenMuxVertexBase(info *relaycommon.RelayInfo) string {
	return zenMuxRoot(info) + "/api/vertex-ai"
}

func zenMuxOpenAIURL(info *relaycommon.RelayInfo) string {
	if info == nil || info.RequestURLPath == "" {
		return zenMuxOpenAIBase(info) + "/chat/completions"
	}
	requestPath := info.RequestURLPath
	if strings.HasPrefix(requestPath, "/v1/") {
		requestPath = strings.TrimPrefix(requestPath, "/v1")
	} else if requestPath == "/v1" {
		requestPath = ""
	}
	return zenMuxOpenAIBase(info) + requestPath
}

func zenMuxClaudeURL(info *relaycommon.RelayInfo) (string, error) {
	requestPath := "/v1/messages"
	if info != nil && info.RelayMode == relayconstant.RelayModeClaudeCountTokens {
		requestPath = "/v1/messages/count_tokens"
	}
	requestURL := zenMuxAnthropicBase(info) + requestPath
	if info == nil || (!info.IsClaudeBetaQuery && !info.ChannelOtherSettings.ClaudeBetaQuery) {
		return requestURL, nil
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	query.Set("beta", "true")
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func zenMuxVertexURL(info *relaycommon.RelayInfo) (string, error) {
	modelName := zenMuxVertexModelName(info)
	provider, modelName, err := splitZenMuxVertexModel(modelName)
	if err != nil {
		return "", err
	}
	action := zenMuxVertexAction(info)
	return fmt.Sprintf("%s/v1/publishers/%s/models/%s:%s", zenMuxVertexBase(info), provider, modelName, action), nil
}

func zenMuxVertexModelName(info *relaycommon.RelayInfo) string {
	modelName := ""
	if info != nil {
		modelName = strings.TrimSpace(info.UpstreamModelName)
		if modelName == "" {
			requestPath := strings.Split(info.RequestURLPath, "?")[0]
			if idx := strings.Index(requestPath, "/models/"); idx >= 0 {
				modelName = requestPath[idx+len("/models/"):]
				if colonIdx := strings.Index(modelName, ":"); colonIdx >= 0 {
					modelName = modelName[:colonIdx]
				}
			}
		}
	}
	return strings.TrimPrefix(strings.TrimSpace(modelName), "models/")
}

func splitZenMuxVertexModel(modelName string) (string, string, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return "", "", errors.New("zenmux vertex model is required")
	}
	provider := "google"
	if slashIdx := strings.Index(modelName, "/"); slashIdx > 0 {
		provider = modelName[:slashIdx]
		modelName = modelName[slashIdx+1:]
	}
	if provider == "" || modelName == "" {
		return "", "", fmt.Errorf("invalid zenmux vertex model: %q", modelName)
	}
	return provider, modelName, nil
}

func zenMuxVertexAction(info *relaycommon.RelayInfo) string {
	requestPath := ""
	if info != nil {
		requestPath = info.RequestURLPath
	}
	switch {
	case strings.Contains(requestPath, ":batchEmbedContents"):
		return "batchEmbedContents"
	case strings.Contains(requestPath, ":embedContent"):
		return "embedContent"
	case strings.Contains(requestPath, ":predict"):
		return "predict"
	case strings.Contains(requestPath, ":streamGenerateContent") || (info != nil && info.IsStream):
		if info != nil && info.RelayMode == relayconstant.RelayModeGemini {
			info.DisablePing = true
		}
		return "streamGenerateContent?alt=sse"
	default:
		return "generateContent"
	}
}
