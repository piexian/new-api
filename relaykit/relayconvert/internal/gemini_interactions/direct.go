package gemini_interactions

import (
	"context"

	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"

	claudemessages "github.com/QuantumNous/new-api/relaykit/relayconvert/internal/claude_messages"
	oaichat "github.com/QuantumNous/new-api/relaykit/relayconvert/internal/oai_chat"
)

// 直接转换器:generateContent 与 responses -> interactions,均支持桥接续链。
// openai chat / claude 的直转经同源 gemini_chat 中间形态完成。

// ConvertOpenAIChatToInteractions chat completions -> interactions
func ConvertOpenAIChatToInteractions(ctx context.Context, info convmeta.Meta, request *kitdto.GeneralOpenAIRequest, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	if request == nil {
		return nil, nil
	}
	geminiReq, err := oaichat.OpenAIChatRequestToGeminiGenerateContent(ctx, *request, info)
	if err != nil {
		return nil, err
	}
	return GeminiChatRequestToInteractionsWithBridge(geminiReq, info.GetUpstreamModelName(), info.GetIsStream(), lookup)
}

// ConvertClaudeToInteractions claude messages -> interactions(经同源 gemini_chat 语义,call_id 保留)
func ConvertClaudeToInteractions(ctx context.Context, info convmeta.Meta, request *kitdto.ClaudeRequest, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	if request == nil {
		return nil, nil
	}
	openAIReq, err := claudemessages.ClaudeMessagesRequestToOpenAIChat(ctx, *request, info)
	if err != nil {
		return nil, err
	}
	geminiReq, err := oaichat.OpenAIChatRequestToGeminiGenerateContent(ctx, *openAIReq, info)
	if err != nil {
		return nil, err
	}
	return GeminiChatRequestToInteractionsWithBridge(geminiReq, info.GetUpstreamModelName(), info.GetIsStream(), lookup)
}

// ConvertGeminiToInteractions generateContent -> interactions
func ConvertGeminiToInteractions(info convmeta.Meta, request *kitdto.GeminiChatRequest, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	if request == nil {
		return nil, nil
	}
	return GeminiChatRequestToInteractionsWithBridge(request, info.GetUpstreamModelName(), info.GetIsStream(), lookup)
}

// ConvertResponsesToInteractions responses -> interactions
func ConvertResponsesToInteractions(info convmeta.Meta, request *kitdto.OpenAIResponsesRequest, lookup BridgeLookup) (*kitdto.GeminiInteractionsRequest, error) {
	if request == nil {
		return nil, nil
	}
	return ResponsesToInteractions(request, info.GetUpstreamModelName(), info.GetIsStream(), lookup)
}
