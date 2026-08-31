package gemini_interactions

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	claudemessages "github.com/QuantumNous/new-api/service/relayconvert/internal/claude_messages"
	oaichat "github.com/QuantumNous/new-api/service/relayconvert/internal/oai_chat"

	"github.com/gin-gonic/gin"
)

// 直接转换器:generateContent 与 responses -> interactions,均支持桥接续链。
// openai chat / claude 的直转在 relayconvert 门面层注册(其一级转换在同包)。

// ConvertOpenAIChatToInteractions chat completions -> interactions
func ConvertOpenAIChatToInteractions(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest, lookup BridgeLookup) (*dto.GeminiInteractionsRequest, error) {
	if request == nil {
		return nil, nil
	}
	geminiReq, err := oaichat.OpenAIChatRequestToGeminiGenerateContent(c, *request, info)
	if err != nil {
		return nil, err
	}
	return GeminiChatRequestToInteractionsWithBridge(geminiReq, info.UpstreamModelName, info.IsStream, lookup)
}

// ConvertClaudeToInteractions claude messages -> interactions(经同源 gemini_chat 语义,call_id 保留)
func ConvertClaudeToInteractions(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest, lookup BridgeLookup) (*dto.GeminiInteractionsRequest, error) {
	if request == nil {
		return nil, nil
	}
	openAIReq, err := claudemessages.ClaudeMessagesRequestToOpenAIChat(*request, info)
	if err != nil {
		return nil, err
	}
	geminiReq, err := oaichat.OpenAIChatRequestToGeminiGenerateContent(c, *openAIReq, info)
	if err != nil {
		return nil, err
	}
	return GeminiChatRequestToInteractionsWithBridge(geminiReq, info.UpstreamModelName, info.IsStream, lookup)
}

// ConvertGeminiToInteractions generateContent -> interactions
func ConvertGeminiToInteractions(info *relaycommon.RelayInfo, request *dto.GeminiChatRequest, lookup BridgeLookup) (*dto.GeminiInteractionsRequest, error) {
	if request == nil {
		return nil, nil
	}
	return GeminiChatRequestToInteractionsWithBridge(request, info.UpstreamModelName, info.IsStream, lookup)
}

// ConvertResponsesToInteractions responses -> interactions
func ConvertResponsesToInteractions(info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest, lookup BridgeLookup) (*dto.GeminiInteractionsRequest, error) {
	if request == nil {
		return nil, nil
	}
	return ResponsesToInteractions(request, info.UpstreamModelName, info.IsStream, lookup)
}
