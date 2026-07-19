package qwentokenplan

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
	if info.RelayMode == relayconstant.RelayModeResponses ||
		info.RelayMode == relayconstant.RelayModeResponsesCompact ||
		info.RelayMode == relayconstant.RelayModeResponsesInputTokens ||
		info.RelayFormat == types.RelayFormatOpenAIResponses ||
		info.RelayFormat == types.RelayFormatOpenAIResponsesCompaction {
		a.RequestMode = requestModeResponses
	}
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("qwen token plan gemini relay is not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("qwen token plan audio relay is not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("qwen token plan image relay uses a separate API")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	switch a.RequestMode {
	case requestModeResponses:
		return qwenTokenPlanResponsesURL(info), nil
	case requestModeClaude:
		return qwenTokenPlanClaudeURL(info)
	default:
		return qwenTokenPlanChatURL(info)
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	credential, err := ParseCredential(info.ApiKey)
	if err != nil {
		return err
	}
	apiKey := credential.APIKey
	if a.RequestMode == requestModeClaude {
		req.Set("x-api-key", apiKey)
		anthropicVersion := c.Request.Header.Get("anthropic-version")
		if anthropicVersion == "" {
			anthropicVersion = "2023-06-01"
		}
		req.Set("anthropic-version", anthropicVersion)
		claude.CommonClaudeHeadersOperation(c, req, info)
		return nil
	}
	req.Set("Authorization", "Bearer "+apiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("qwen token plan rerank relay is not implemented")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("qwen token plan embedding relay is not implemented")
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
	if a.RequestMode == requestModeClaude {
		claudeAdaptor := claude.Adaptor{}
		return claudeAdaptor.DoResponse(c, resp, info)
	}
	openaiAdaptor := openai.Adaptor{}
	return openaiAdaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return channelconstant.QwenTokenPlanModelList
}

func (a *Adaptor) GetChannelName() string {
	return "Qwen Token Plan"
}

func NormalizeRoot(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return channelconstant.QwenTokenPlanRootURL
	}
	for _, suffix := range []string{
		"/compatible-mode/v1/responses/input_tokens",
		"/compatible-mode/v1/responses/compact",
		"/compatible-mode/v1/chat/completions",
		"/compatible-mode/v1/responses",
		"/apps/anthropic/v1/messages/count_tokens",
		"/apps/anthropic/v1/messages",
		"/compatible-mode/v1",
		"/apps/anthropic",
	} {
		if strings.HasSuffix(baseURL, suffix) {
			return strings.TrimSuffix(baseURL, suffix)
		}
	}
	return baseURL
}

func OpenAIBaseURL(baseURL string) string {
	return NormalizeRoot(baseURL) + "/compatible-mode/v1"
}

func AnthropicBaseURL(baseURL string) string {
	return NormalizeRoot(baseURL) + "/apps/anthropic"
}

func qwenTokenPlanChatURL(info *relaycommon.RelayInfo) (string, error) {
	requestPath := ""
	baseURL := ""
	if info != nil {
		requestPath = strings.Split(info.RequestURLPath, "?")[0]
		baseURL = info.ChannelBaseUrl
	}
	if requestPath != "" && requestPath != "/v1/chat/completions" && requestPath != "/chat/completions" {
		return "", fmt.Errorf("qwen token plan does not support OpenAI endpoint %q", requestPath)
	}
	return OpenAIBaseURL(baseURL) + "/chat/completions", nil
}

func qwenTokenPlanResponsesURL(info *relaycommon.RelayInfo) string {
	baseURL := ""
	requestPath := "/responses"
	if info != nil {
		baseURL = info.ChannelBaseUrl
		switch info.RelayMode {
		case relayconstant.RelayModeResponsesCompact:
			requestPath = "/responses/compact"
		case relayconstant.RelayModeResponsesInputTokens:
			requestPath = "/responses/input_tokens"
		}
	}
	return OpenAIBaseURL(baseURL) + requestPath
}

func qwenTokenPlanClaudeURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := ""
	requestPath := "/v1/messages"
	if info != nil {
		baseURL = info.ChannelBaseUrl
		if info.RelayMode == relayconstant.RelayModeClaudeCountTokens {
			requestPath = "/v1/messages/count_tokens"
		}
	}
	requestURL := AnthropicBaseURL(baseURL) + requestPath
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
