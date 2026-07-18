package opencode

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	requestModeOpenAI = iota + 1
	requestModeResponses
	requestModeClaude
	requestModeGemini
)

type Adaptor struct {
	RequestMode  int
	RouteByModel bool
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.RequestMode = requestModeOpenAI
	a.RouteByModel = false
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
		a.RequestMode = requestModeGemini
		return
	}
	if info.RelayMode == relayconstant.RelayModeResponses ||
		info.RelayMode == relayconstant.RelayModeResponsesCompact ||
		info.RelayMode == relayconstant.RelayModeResponsesInputTokens ||
		info.RelayFormat == types.RelayFormatOpenAIResponses ||
		info.RelayFormat == types.RelayFormatOpenAIResponsesCompaction {
		a.RequestMode = requestModeResponses
	}
	if shouldRouteOpenCodeByModel(info) {
		if requestMode, ok := requestModeForModel(info.ChannelBaseUrl, info.UpstreamModelName); ok {
			a.RequestMode = requestMode
			a.RouteByModel = true
		}
	}
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return a.convertRequest(c, info, request)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return a.convertRequest(c, info, request)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("opencode audio relay is not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("opencode image relay is not implemented")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info != nil && IsGoBase(info.ChannelBaseUrl) {
		switch a.RequestMode {
		case requestModeResponses:
			return "", errors.New("opencode go does not support OpenAI Responses endpoint")
		case requestModeGemini:
			return "", errors.New("opencode go does not support Gemini endpoint")
		}
	}

	switch a.RequestMode {
	case requestModeResponses:
		return openCodeResponsesURL(info), nil
	case requestModeClaude:
		return openCodeClaudeURL(info)
	case requestModeGemini:
		return openCodeGeminiURL(info)
	default:
		if a.RouteByModel {
			return openCodeRoot(info) + "/v1/chat/completions", nil
		}
		return openCodeOpenAIURL(info), nil
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
	default:
		req.Set("Authorization", "Bearer "+info.ApiKey)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return a.convertRequest(c, info, request)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("opencode rerank relay is not implemented")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return a.convertRequest(c, info, &request)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch a.RequestMode {
	case requestModeResponses:
		if info.RelayMode == relayconstant.RelayModeChatCompletions {
			if info.IsStream {
				return openai.OaiResponsesToChatStreamHandler(c, info, resp)
			}
			return openai.OaiResponsesToChatHandler(c, info, resp)
		}
		openaiAdaptor := openai.Adaptor{}
		return openaiAdaptor.DoResponse(c, resp, info)
	case requestModeClaude:
		claudeAdaptor := claude.Adaptor{}
		return claudeAdaptor.DoResponse(c, resp, info)
	case requestModeGemini:
		geminiAdaptor := gemini.Adaptor{}
		return geminiAdaptor.DoResponse(c, resp, info)
	default:
		if info.RelayMode == relayconstant.RelayModeResponses {
			if info.IsStream {
				return openai.OaiChatToResponsesStreamHandler(c, info, resp)
			}
			return openai.OaiChatToResponsesHandler(c, info, resp)
		}
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

func shouldRouteOpenCodeByModel(info *relaycommon.RelayInfo) bool {
	if info == nil || relaycommon.IsRequestPassThroughEnabled(info) {
		return false
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions && info.RelayMode != relayconstant.RelayModeResponses {
		return false
	}
	return info.RelayFormat == types.RelayFormatOpenAI || info.RelayFormat == types.RelayFormatOpenAIResponses
}

func (a *Adaptor) convertRequest(c *gin.Context, info *relaycommon.RelayInfo, request any) (any, error) {
	if relaycommon.IsRequestPassThroughEnabled(info) {
		return request, nil
	}
	target, err := a.targetRelayFormat()
	if err != nil {
		return nil, err
	}
	result, err := service.ConvertRequest(c, info, target, request)
	if err != nil {
		return nil, err
	}
	if info != nil {
		info.FinalRequestRelayFormat = target
	}
	return result.Value, nil
}

func (a *Adaptor) targetRelayFormat() (types.RelayFormat, error) {
	switch a.RequestMode {
	case requestModeOpenAI:
		return types.RelayFormatOpenAI, nil
	case requestModeResponses:
		return types.RelayFormatOpenAIResponses, nil
	case requestModeClaude:
		return types.RelayFormatClaude, nil
	case requestModeGemini:
		return types.RelayFormatGemini, nil
	default:
		return "", fmt.Errorf("unsupported opencode request mode: %d", a.RequestMode)
	}
}

func openCodeRoot(info *relaycommon.RelayInfo) string {
	baseURL := ""
	if info != nil {
		baseURL = info.ChannelBaseUrl
	}
	return NormalizeRoot(baseURL)
}

func openCodeOpenAIURL(info *relaycommon.RelayInfo) string {
	if info == nil || info.RequestURLPath == "" {
		return openCodeRoot(info) + "/v1/chat/completions"
	}
	requestPath := info.RequestURLPath
	if requestPath == "/v1" {
		requestPath = ""
	}
	return openCodeRoot(info) + requestPath
}

func openCodeResponsesURL(info *relaycommon.RelayInfo) string {
	requestPath := "/v1/responses"
	if info != nil && info.RelayMode == relayconstant.RelayModeResponsesCompact {
		requestPath = "/v1/responses/compact"
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeResponsesInputTokens {
		requestPath = "/v1/responses/input_tokens"
	}
	return openCodeRoot(info) + requestPath
}

func openCodeClaudeURL(info *relaycommon.RelayInfo) (string, error) {
	requestPath := "/v1/messages"
	if info != nil && info.RelayMode == relayconstant.RelayModeClaudeCountTokens {
		requestPath = "/v1/messages/count_tokens"
	}
	requestURL := openCodeRoot(info) + requestPath
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

func openCodeGeminiURL(info *relaycommon.RelayInfo) (string, error) {
	modelName := openCodeGeminiModelName(info)
	if modelName == "" {
		return "", errors.New("opencode gemini model is required")
	}
	action := openCodeGeminiAction(info)
	return fmt.Sprintf("%s/v1/models/%s:%s", openCodeRoot(info), modelName, action), nil
}

func openCodeGeminiModelName(info *relaycommon.RelayInfo) string {
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

func openCodeGeminiAction(info *relaycommon.RelayInfo) string {
	requestPath := ""
	if info != nil {
		requestPath = info.RequestURLPath
	}
	switch {
	case strings.Contains(requestPath, ":batchEmbedContents"):
		return "batchEmbedContents"
	case strings.Contains(requestPath, ":embedContent"):
		return "embedContent"
	case strings.Contains(requestPath, ":streamGenerateContent") || (info != nil && info.IsStream):
		if info != nil && info.RelayMode == relayconstant.RelayModeGemini {
			info.DisablePing = true
		}
		return "streamGenerateContent?alt=sse"
	default:
		return "generateContent"
	}
}
