package common

import (
	dto "github.com/QuantumNous/new-api/dto"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"
)

func GuessRelayFormatFromRequest(req any) (types.RelayFormat, bool) {
	switch req.(type) {
	case *kitdto.GeneralOpenAIRequest, kitdto.GeneralOpenAIRequest:
		return types.RelayFormatOpenAI, true
	case *kitdto.OpenAIResponsesRequest, kitdto.OpenAIResponsesRequest:
		return types.RelayFormatOpenAIResponses, true
	case *kitdto.ClaudeRequest, kitdto.ClaudeRequest:
		return types.RelayFormatClaude, true
	case *kitdto.GeminiChatRequest, kitdto.GeminiChatRequest:
		return types.RelayFormatGemini, true
	case *dto.GeminiInteractionsRequest, dto.GeminiInteractionsRequest:
		return types.RelayFormatGeminiInteractions, true
	case *kitdto.EmbeddingRequest, kitdto.EmbeddingRequest:
		return types.RelayFormatEmbedding, true
	case *kitdto.RerankRequest, kitdto.RerankRequest:
		return types.RelayFormatRerank, true
	case *kitdto.ImageRequest, kitdto.ImageRequest:
		return types.RelayFormatOpenAIImage, true
	case *kitdto.AudioRequest, kitdto.AudioRequest:
		return types.RelayFormatOpenAIAudio, true
	default:
		return "", false
	}
}

func AppendRequestConversionFromRequest(info *RelayInfo, req any) {
	if info == nil {
		return
	}
	format, ok := GuessRelayFormatFromRequest(req)
	if !ok {
		return
	}
	info.AppendRequestConversion(format)
}
