// Package cliproxyapi CLIProxyAPI 本地代理渠道:入站各协议原样透传到上游同路径端点,不做格式转换。
package cliproxyapi

import (
	"errors"
	"io"
	"net/http"
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

type Adaptor struct {
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

// GetRequestURL 入站路径透传;仅 Gemini 原生路径按段替换模型名(兼容渠道自测的 {model} 占位符)
func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := strings.TrimRight(info.ChannelBaseUrl, "/")
	if base == "" {
		return "", errors.New("cliproxyapi channel requires a base URL")
	}
	requestPath := info.RequestURLPath
	query := ""
	if idx := strings.Index(requestPath, "?"); idx >= 0 {
		query = requestPath[idx:]
		requestPath = requestPath[:idx]
	}
	if idx := strings.Index(requestPath, "/models/"); idx >= 0 && info.UpstreamModelName != "" {
		start := idx + len("/models/")
		if end := strings.Index(requestPath[start:], ":"); end >= 0 {
			requestPath = requestPath[:start] + info.UpstreamModelName + requestPath[start+end:]
		} else {
			requestPath = requestPath[:start] + info.UpstreamModelName
		}
	}
	if info.RelayMode == relayconstant.RelayModeGemini && info.IsStream {
		// 与 gemini 渠道对齐:原生流式禁用下行 Ping,上游强制 alt=sse 保证 SSE 输出
		info.DisablePing = true
		if strings.Contains(requestPath, ":streamGenerateContent") && !strings.Contains(query, "alt=") {
			if query == "" {
				query = "?alt=sse"
			} else {
				query += "&alt=sse"
			}
		}
	}
	return base + requestPath + query, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	switch info.RelayFormat {
	case types.RelayFormatGemini:
		// 渠道自测合成请求:借 Gemini 适配器转换后按原生 gemini 路径透传
		return (&gemini.Adaptor{}).ConvertOpenAIRequest(c, info, request)
	case types.RelayFormatClaude:
		// 渠道自测合成请求:借 Claude 适配器转换后按 /v1/messages 透传
		return (&claude.Adaptor{}).ConvertOpenAIRequest(c, info, request)
	}
	// 原生 OpenAI 透传;流式必须显式索要 usage 供计费
	if info.SupportStreamOptions && request.Stream != nil && *request.Stream && request.StreamOptions == nil {
		request.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// codex 专有字段由 CLIProxyAPI 服务端归一化
	return request, nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		// edits 入站可能是 multipart 表单,复用 openai 适配器的表单重建逻辑
		return (&openai.Adaptor{}).ConvertImageRequest(c, info, request)
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("cliproxyapi channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("cliproxyapi channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("cliproxyapi channel: audio endpoint not supported")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

// DoResponse 按入站协议委托对应适配器,复用既有 usage 解析与计费链路
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayFormat {
	case types.RelayFormatGemini:
		// 覆盖 generateContent 与 interactions 两类 RelayMode(gemini 适配器内部分流)
		return (&gemini.Adaptor{}).DoResponse(c, resp, info)
	case types.RelayFormatClaude:
		return (&claude.Adaptor{}).DoResponse(c, resp, info)
	default:
		return (&openai.Adaptor{}).DoResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
