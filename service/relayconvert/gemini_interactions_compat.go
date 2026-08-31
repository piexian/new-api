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

// InteractionToGeminiChatResponse Interactions 资源转 generateContent 响应
func InteractionToGeminiChatResponse(interaction *dto.GeminiInteraction, fallbackPromptTokens int) *dto.GeminiChatResponse {
	return gemini_interactions.InteractionToGeminiChatResponse(interaction, fallbackPromptTokens)
}

// NewInteractionsSSETranslator Interactions SSE 事件流翻译为 generateContent 风格 data 行
func NewInteractionsSSETranslator(reader io.Reader) io.Reader {
	return gemini_interactions.NewInteractionsSSEToGeminiSSE(reader)
}
