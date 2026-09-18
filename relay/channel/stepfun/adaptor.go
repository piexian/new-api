package stepfun

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// StepFun（阶跃星辰）原生支持 OpenAI Chat / Anthropic Messages / OpenAI Responses 三种入站协议，
// 命中官方域名白名单时按入站格式直传对应上游路径，不做协议转换；
// 非白名单 base（自建代理等）回落到 openai.Adaptor 的既有转换逻辑。
type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.Adaptor.Init(info)
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return a.Adaptor.ConvertGeminiRequest(c, info, request)
}

// ConvertClaudeRequest 命中白名单时按 Anthropic Messages 直传：只做官方字段裁剪，
// 不做协议转换，并置 FinalRequestRelayFormat 供 GetRequestURL/DoResponse 分派。
func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if !nativeProtocolBase(info) {
		return a.Adaptor.ConvertClaudeRequest(c, info, request)
	}
	sanitizeStepFunMessagesRequest(request)
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatClaude
	}
	return request, nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return a.Adaptor.ConvertOpenAIRequest(c, info, request)
}

// ConvertOpenAIResponsesRequest 命中白名单时按 Responses 协议直传；
// 否则沿用 openai.Adaptor 的 Responses→Chat 降级管线。
func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	if !nativeProtocolBase(info) {
		return a.Adaptor.ConvertOpenAIResponsesRequest(c, info, request)
	}
	// 复用通用处理（-thinking/-effort 模型后缀解析、ReasoningEffort 记录），仅改写上游目标协议：
	// StepFun /v1/responses 原生兼容 OpenAI Responses，不做 Responses→Chat 降级。
	converted, err := a.Adaptor.ConvertOpenAIResponsesRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	if info != nil {
		info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses
	}
	return converted, nil
}

// GetRequestURL 构建上游地址。白名单 base 按「入站格式 → 官方路径」直传：
//
//	/v1/messages  /v1/responses  /v1/chat/completions
//
// Step Plan 通道统一加 /step_plan 前缀。音频/图像等非文本端点沿用其原始路径。
func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return a.Adaptor.GetRequestURL(info)
	}
	base, ok := resolveBase(info.ChannelBaseUrl)
	// WebSocket 原生端点（流式 TTS / 双向 ASR / 双向对话）：原始双向透传，仅要求白名单 base
	if info.RelayFormat == types.RelayFormatStepFunWss {
		if !ok {
			return "", fmt.Errorf("StepFun websocket endpoints require a whitelisted StepFun base, got %q", info.ChannelBaseUrl)
		}
		pathOnly, rawQuery := SplitPathQuery(info.RequestURLPath)
		if _, found := LookupWssEndpoint(pathOnly); !found {
			return "", fmt.Errorf("unsupported StepFun websocket endpoint %q", info.RequestURLPath)
		}
		// 查询串原样转发：?model= 与鉴权之外的可选参数都是上游真实字段
		url := WebsocketBaseURL(base.path(pathOnly))
		if rawQuery != "" {
			url += "?" + rawQuery
		}
		return url, nil
	}
	if !ok {
		// 非白名单 base 不假设 StepFun 私有路径，交回通用实现（含 Claude/Gemini→chat 兜底）
		return a.Adaptor.GetRequestURL(info)
	}
	// StepFun 原生端点（音频/音乐/音色/文件）：入站路径与上游 1:1，仅归一查询串中的路由用 model
	if pathOnly, _ := SplitPathQuery(info.RequestURLPath); IsNativePath(pathOnly) {
		return base.path(NativeForwardPath(info.RequestURLPath)), nil
	}
	if path, ok := stepFunNonTextPath(info.RelayMode); ok {
		return base.path(path), nil
	}
	switch info.RelayMode {
	case relayconstant.RelayModeRealtime:
		// 双向实时语音为上游自有 WS 协议，StepFun 渠道暂不支持 OpenAI Realtime 直传
		return "", fmt.Errorf("realtime endpoints are not supported on StepFun channels")
	case relayconstant.RelayModeEmbeddings, relayconstant.RelayModeRerank, relayconstant.RelayModeModerations:
		return "", fmt.Errorf("unsupported endpoint %q for StepFun channels", info.RequestURLPath)
	}
	switch info.GetFinalRequestRelayFormat() {
	case types.RelayFormatClaude:
		return base.path("/v1/messages"), nil
	case types.RelayFormatOpenAIResponses:
		return base.path("/v1/responses"), nil
	}
	// 其余（含 Claude/Gemini/Responses 已降级转换为 chat）统一走 chat 补全
	return base.path("/v1/chat/completions"), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if err := a.Adaptor.SetupRequestHeader(c, req, info); err != nil {
		return err
	}
	// Anthropic Messages 走 Bearer 鉴权（上游对 Bearer 与 x-api-key 均接受），
	// 仅补齐 anthropic-version 便于链路中的中间层识别；不下发任何客户端指纹头。
	if info != nil && info.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		if req.Get("anthropic-version") == "" {
			req.Set("anthropic-version", defaultAnthropicVersion)
		}
	}
	return nil
}

// DoRequest WebSocket 原生端点走 DoWssRequest（其余请求沿用 openai.Adaptor 的分派）。
func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info != nil && info.RelayFormat == types.RelayFormatStepFunWss {
		return channel.DoWssRequest(a, c, info, requestBody)
	}
	return a.Adaptor.DoRequest(c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	// TTS return_url=true 时上游返回 JSON（含音频下载 URL 与字幕），必须原样透传；
	// stream_format=sse 由通用实现按 SSE 处理，二进制音频同理由其处理。
	if info != nil && info.RelayMode == relayconstant.RelayModeAudioSpeech && resp != nil {
		if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
			return ttsJSONHandler(c, resp, info), nil
		}
	}
	if info != nil && info.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		claudeAdaptor := claude.Adaptor{}
		return claudeAdaptor.DoResponse(c, resp, info)
	}
	return a.Adaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

// stepFunNonTextPath 非文本端点的上游路径；StepFun 暂不支持 embedding/rerank。
func stepFunNonTextPath(relayMode int) (string, bool) {
	switch relayMode {
	case relayconstant.RelayModeAudioSpeech:
		return "/v1/audio/speech", true
	case relayconstant.RelayModeAudioTranscription:
		return "/v1/audio/transcriptions", true
	case relayconstant.RelayModeAudioTranslation:
		return "/v1/audio/translations", true
	case relayconstant.RelayModeImagesGenerations:
		return "/v1/images/generations", true
	case relayconstant.RelayModeImagesEdits:
		return "/v1/images/edits", true
	}
	return "", false
}

const defaultAnthropicVersion = "2023-06-01"

// nativeProtocolBase 判断渠道 base 是否命中白名单（原生三端点透传的前提）。
func nativeProtocolBase(info *relaycommon.RelayInfo) bool {
	return info != nil && isStepFunBase(info.ChannelBaseUrl)
}
