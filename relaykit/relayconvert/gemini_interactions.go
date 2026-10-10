package relayconvert

import (
	"context"
	"io"

	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"

	geminiinteractions "github.com/QuantumNous/new-api/relaykit/relayconvert/internal/gemini_interactions"
)

// Gemini Interactions(fork 专属第 5 协议)公开门面:宿主注册表与兼容层经此调用,
// 转换实现在 internal/gemini_interactions,协议 DTO 在 relaykit/dto。
// 请求方向四条 1 步直达;响应方向 interaction -> gemini_chat 后复用既有出口,
// -> Claude / -> Responses 两跳经 ConvertResponse。流式由 adaptor 层 SSE 翻译器处理。

// BridgeLookup 有状态工具调用桥接:客户端可见 tool_call id -> interaction id
type BridgeLookup = geminiinteractions.BridgeLookup

// ConvertOpenAIChatToInteractions chat completions -> interactions
func ConvertOpenAIChatToInteractions(ctx context.Context, info convmeta.Meta, request *kitdto.GeneralOpenAIRequest, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	return geminiinteractions.ConvertOpenAIChatToInteractions(ctx, info, request, lookup)
}

// ConvertClaudeToInteractions claude messages -> interactions(经同源 gemini_chat 语义,call_id 保留)
func ConvertClaudeToInteractions(ctx context.Context, info convmeta.Meta, request *kitdto.ClaudeRequest, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	return geminiinteractions.ConvertClaudeToInteractions(ctx, info, request, lookup)
}

// ConvertGeminiToInteractions generateContent -> interactions
func ConvertGeminiToInteractions(info convmeta.Meta, request *kitdto.GeminiChatRequest, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	return geminiinteractions.ConvertGeminiToInteractions(info, request, lookup)
}

// ConvertResponsesToInteractions responses -> interactions
func ConvertResponsesToInteractions(info convmeta.Meta, request *kitdto.OpenAIResponsesRequest, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	return geminiinteractions.ConvertResponsesToInteractions(info, request, lookup)
}

// GeminiChatRequestToInteractions 转换 create 请求(无状态回放)
func GeminiChatRequestToInteractions(req *kitdto.GeminiChatRequest, modelName string, isStream bool) (*kitdto.GeminiInteractionsRequest, error) {
	return geminiinteractions.GeminiChatRequestToInteractions(req, modelName, isStream)
}

// GeminiChatRequestToInteractionsWithBridge 带桥接查找的转换(命中则 previous_interaction_id 续链)
func GeminiChatRequestToInteractionsWithBridge(req *kitdto.GeminiChatRequest, modelName string, isStream bool, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	return geminiinteractions.GeminiChatRequestToInteractionsWithBridge(req, modelName, isStream, lookup)
}

// ResponsesToInteractions OpenAI Responses 入站直接转 Interactions create(无状态回放)
func ResponsesToInteractions(req *kitdto.OpenAIResponsesRequest, modelName string, isStream bool) (*kitdto.GeminiInteractionsRequest, error) {
	return geminiinteractions.ResponsesToInteractions(req, modelName, isStream, nil)
}

// ResponsesToInteractionsWithBridge 同上,命中桥接走有状态续链
func ResponsesToInteractionsWithBridge(req *kitdto.OpenAIResponsesRequest, modelName string, isStream bool, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	return geminiinteractions.ResponsesToInteractions(req, modelName, isStream, lookup)
}

// InteractionToGeminiChatResponse Interactions 资源转 generateContent 响应
func InteractionToGeminiChatResponse(interaction *kitdto.GeminiInteraction, fallbackPromptTokens int) *kitdto.GeminiChatResponse {
	return geminiinteractions.InteractionToGeminiChatResponse(interaction, fallbackPromptTokens)
}

// NewInteractionsSSETranslator Interactions SSE 事件流翻译为 generateContent 风格 data 行
func NewInteractionsSSETranslator(reader io.Reader) io.Reader {
	return geminiinteractions.NewInteractionsSSEToGeminiSSE(reader)
}

// NewInteractionsSSETranslatorWithCallback 同上;requires_action/completed 时回调桥接信息
func NewInteractionsSSETranslatorWithCallback(reader io.Reader, onPending func(interactionID string, callIDs []string)) io.Reader {
	return geminiinteractions.NewInteractionsSSEToGeminiSSEWithCallback(reader, onPending)
}
