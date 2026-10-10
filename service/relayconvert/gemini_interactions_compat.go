package relayconvert

import (
	"io"

	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	kitconvert "github.com/QuantumNous/new-api/relaykit/relayconvert"
)

// Gemini Interactions 兼容层:保持宿主既有 API 形态,实现委托 relaykit。

// GeminiChatRequestToInteractions generateContent 请求转 Interactions create 请求(无状态回放)
func GeminiChatRequestToInteractions(req *kitdto.GeminiChatRequest, modelName string, isStream bool) (*kitdto.GeminiInteractionsRequest, error) {
	return kitconvert.GeminiChatRequestToInteractions(req, modelName, isStream)
}

// BridgeLookup 有状态工具调用桥接:客户端可见 tool_call id -> interaction id
type BridgeLookup = kitconvert.BridgeLookup

// GeminiChatRequestToInteractionsWithBridge 带桥接查找的转换(命中则 previous_interaction_id 续链)
func GeminiChatRequestToInteractionsWithBridge(req *kitdto.GeminiChatRequest, modelName string, isStream bool, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	return kitconvert.GeminiChatRequestToInteractionsWithBridge(req, modelName, isStream, lookup)
}

// ResponsesToInteractionsWithBridge OpenAI Responses 入站直接转 Interactions(命中桥接走有状态续链)
func ResponsesToInteractionsWithBridge(req *kitdto.OpenAIResponsesRequest, modelName string, isStream bool, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	return kitconvert.ResponsesToInteractionsWithBridge(req, modelName, isStream, lookup)
}

// InteractionToGeminiChatResponse Interactions 资源转 generateContent 响应
func InteractionToGeminiChatResponse(interaction *kitdto.GeminiInteraction, fallbackPromptTokens int) *kitdto.GeminiChatResponse {
	return kitconvert.InteractionToGeminiChatResponse(interaction, fallbackPromptTokens)
}

// NewInteractionsSSETranslator Interactions SSE 事件流翻译为 generateContent 风格 data 行
func NewInteractionsSSETranslator(reader io.Reader) io.Reader {
	return kitconvert.NewInteractionsSSETranslator(reader)
}

// NewInteractionsSSETranslatorWithCallback 同上;requires_action/completed 时回调桥接信息
func NewInteractionsSSETranslatorWithCallback(reader io.Reader, onPending func(interactionID string, callIDs []string)) io.Reader {
	return kitconvert.NewInteractionsSSETranslatorWithCallback(reader, onPending)
}
