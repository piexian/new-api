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
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("opencode rerank relay is not implemented")
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
	case requestModeGemini:
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
