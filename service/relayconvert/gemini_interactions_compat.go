package relayconvert

import (
	"io"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/relayconvert/internal/gemini_interactions"
)

// GeminiChatRequestToInteractions generateContent 请求转 Interactions create 请求(无状态回放)
func GeminiChatRequestToInteractions(req *dto.GeminiChatRequest, modelName string, isStream bool) (*dto.GeminiInteractionsRequest, error) {
	return gemini_interactions.GeminiChatRequestToInteractions(req, modelName, isStream)
}

// BridgeLookup 有状态工具调用桥接:客户端可见 tool_call id -> interaction id
type BridgeLookup = gemini_interactions.BridgeLookup

// GeminiChatRequestToInteractionsWithBridge 带桥接查找的转换(命中则 previous_interaction_id 续链)
func GeminiChatRequestToInteractionsWithBridge(req *dto.GeminiChatRequest, modelName string, isStream bool, lookup BridgeLookup) (*dto.GeminiInteractionsRequest, error) {
	return gemini_interactions.GeminiChatRequestToInteractionsWithBridge(req, modelName, isStream, lookup)
}

// ResponsesToInteractionsWithBridge OpenAI Responses 入站直接转 Interactions(命中桥接走有状态续链)
func ResponsesToInteractionsWithBridge(req *dto.OpenAIResponsesRequest, modelName string, isStream bool, lookup BridgeLookup) (*dto.GeminiInteractionsRequest, error) {
	return gemini_interactions.ResponsesToInteractions(req, modelName, isStream, lookup)
}

// InteractionToGeminiChatResponse Interactions 资源转 generateContent 响应
func InteractionToGeminiChatResponse(interaction *dto.GeminiInteraction, fallbackPromptTokens int) *dto.GeminiChatResponse {
	return gemini_interactions.InteractionToGeminiChatResponse(interaction, fallbackPromptTokens)
}

// NewInteractionsSSETranslator Interactions SSE 事件流翻译为 generateContent 风格 data 行
func NewInteractionsSSETranslator(reader io.Reader) io.Reader {
	return gemini_interactions.NewInteractionsSSEToGeminiSSE(reader)
}

// NewInteractionsSSETranslatorWithCallback 同上;requires_action/completed 时回调桥接信息
func NewInteractionsSSETranslatorWithCallback(reader io.Reader, onPending func(interactionID string, callIDs []string)) io.Reader {
	return gemini_interactions.NewInteractionsSSEToGeminiSSEWithCallback(reader, onPending)
}
