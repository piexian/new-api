package ollama

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// ollamaAnthropicVersion is the only Anthropic version Ollama Cloud accepts.
const ollamaAnthropicVersion = "2023-06-01"

type Adaptor struct {
}

// NormalizeBaseURL accepts the client base URLs documented for Ollama Cloud
// (https://ollama.com, https://ollama.com/v1, https://ollama.com/api) and the
// local server form, and returns the root that the native /api and /v1 routes
// are appended to.
func NormalizeBaseURL(baseURL string) string {
	base := strings.TrimSpace(baseURL)
	for {
		trimmed := strings.TrimSuffix(base, "/")
		switch {
		case strings.HasSuffix(trimmed, "/v1"):
			trimmed = strings.TrimSuffix(trimmed, "/v1")
		case strings.HasSuffix(trimmed, "/api"):
			trimmed = strings.TrimSuffix(trimmed, "/api")
		}
		if trimmed == base {
			return base
		}
		base = trimmed
	}
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	// Ollama Cloud serves the native Anthropic Messages endpoint, so Claude
	// requests stay untouched instead of dropping through the chat protocol.
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := NormalizeBaseURL(info.ChannelBaseUrl)
	switch info.RelayMode {
	case relayconstant.RelayModeClaudeCountTokens:
		return base + "/v1/messages/count_tokens", nil
	case relayconstant.RelayModeEmbeddings:
		return base + "/api/embed", nil
	case relayconstant.RelayModeResponses:
		return base + "/v1/responses", nil
	case relayconstant.RelayModeResponsesCompact:
		return base + "/v1/responses/compact", nil
	case relayconstant.RelayModeResponsesInputTokens:
		return base + "/v1/responses/input_tokens", nil
	case relayconstant.RelayModeCompletions:
		return base + "/api/generate", nil
	}
	if info.RelayFormat == types.RelayFormatClaude {
		return base + "/v1/messages", nil
	}
	if strings.Contains(info.RequestURLPath, "/v1/completions") {
		return base + "/api/generate", nil
	}
	return base + "/api/chat", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	// Ollama Cloud requires bearer auth on every route and rejects x-api-key.
	req.Set("Authorization", "Bearer "+info.ApiKey)
	if info.RelayMode == relayconstant.RelayModeClaudeCountTokens ||
		(info.RelayFormat == types.RelayFormatClaude && info.RelayMode == relayconstant.RelayModeUnknown) {
		req.Set("anthropic-version", ollamaAnthropicVersion)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if info.RelayMode == relayconstant.RelayModeCompletions || strings.Contains(info.RequestURLPath, "/v1/completions") {
		return openAIToGenerate(c, request)
	}
	return openAIChatToOllamaChat(c, request)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return requestOpenAI2Embeddings(request), nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	converted, err := (&openai.Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
	}
	return converted, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		return ollamaEmbeddingHandler(c, info, resp)
	case relayconstant.RelayModeResponses:
		if info.IsStream {
			return openai.OaiResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesHandler(c, info, resp)
	}
	if info != nil && info.RelayFormat == types.RelayFormatClaude {
		return (&claude.Adaptor{}).DoResponse(c, resp, info)
	}
	if info.IsStream {
		return ollamaStreamHandler(c, info, resp)
	}
	return ollamaChatHandler(c, info, resp)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
