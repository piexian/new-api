package relayconvert

import (
	"fmt"
	"io"
	"net/http"

	geminichat "github.com/QuantumNous/new-api/service/relayconvert/internal/gemini_chat"
	geminiinteractions "github.com/QuantumNous/new-api/service/relayconvert/internal/gemini_interactions"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// Gemini Interactions 直接转换器实现。
// 请求方向:一步到 interactions(保留 call_id,支持桥接续链);
// 响应方向:interaction JSON -> 入站格式(文本类复用 gemini_chat 中间形状的既有出口)。

// lookupFromInfo 从 RelayInfo 构建桥接查找(仅 interactions 渠道挂载)
var lookupFromInfo func(info *relaycommon.RelayInfo) BridgeLookup

// SetGeminiInteractionsBridgeLookup 注入桥接查找实现(由 gemini_interactions adaptor 调用)
func SetGeminiInteractionsBridgeLookup(fn func(info *relaycommon.RelayInfo) BridgeLookup) {
	lookupFromInfo = fn
}

func bridgeLookupFor(c *gin.Context, info *relaycommon.RelayInfo) BridgeLookup {
	if lookupFromInfo != nil {
		return lookupFromInfo(info)
	}
	return nil
}

// ---------------------------- 请求方向 ----------------------------

func convertOpenAIChatToInteractions(c *gin.Context, info *relaycommon.RelayInfo, request any) (any, error) {
	req, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		if v, ok := request.(dto.GeneralOpenAIRequest); ok {
			req = &v
		}
	}
	if req == nil {
		return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", request)
	}
	return geminiinteractions.ConvertOpenAIChatToInteractions(c, info, req, bridgeLookupFor(c, info))
}

func convertClaudeMessagesToInteractions(c *gin.Context, info *relaycommon.RelayInfo, request any) (any, error) {
	req, ok := request.(*dto.ClaudeRequest)
	if !ok {
		if v, ok := request.(dto.ClaudeRequest); ok {
			req = &v
		}
	}
	if req == nil {
		return nil, fmt.Errorf("expected Anthropic Messages request, got %T", request)
	}
	return geminiinteractions.ConvertClaudeToInteractions(c, info, req, bridgeLookupFor(c, info))
}

func convertGeminiContentToInteractions(c *gin.Context, info *relaycommon.RelayInfo, request any) (any, error) {
	req, ok := request.(*dto.GeminiChatRequest)
	if !ok {
		if v, ok := request.(dto.GeminiChatRequest); ok {
			req = &v
		}
	}
	if req == nil {
		return nil, fmt.Errorf("expected Gemini generateContent request, got %T", request)
	}
	return geminiinteractions.ConvertGeminiToInteractions(info, req, bridgeLookupFor(c, info))
}

func convertResponsesToInteractions(c *gin.Context, info *relaycommon.RelayInfo, request any) (any, error) {
	req, ok := request.(*dto.OpenAIResponsesRequest)
	if !ok {
		if v, ok := request.(dto.OpenAIResponsesRequest); ok {
			req = &v
		}
	}
	if req == nil {
		return nil, fmt.Errorf("expected OpenAI Responses request, got %T", request)
	}
	return geminiinteractions.ConvertResponsesToInteractions(info, req, bridgeLookupFor(c, info))
}

// ---------------------------- 响应方向 ----------------------------

// interactionsToGeminiChat 上游 interaction JSON -> generateContent 形状(公共前置)
func interactionsToGeminiChat(c *gin.Context, info *relaycommon.RelayInfo, response any) (*dto.GeminiChatResponse, error) {
	switch v := response.(type) {
	case *http.Response:
		body, err := io.ReadAll(v.Body)
		if err != nil {
			return nil, err
		}
		_ = v.Body.Close()
		var interaction dto.GeminiInteraction
		if err := common.Unmarshal(body, &interaction); err != nil {
			return nil, err
		}
		return geminiinteractions.InteractionToGeminiChatResponse(&interaction, info.GetEstimatePromptTokens()), nil
	case *dto.GeminiInteraction:
		return geminiinteractions.InteractionToGeminiChatResponse(v, info.GetEstimatePromptTokens()), nil
	default:
		return nil, fmt.Errorf("unsupported interactions response type %T", response)
	}
}

func convertGeminiInteractionsToOpenAIChat(c *gin.Context, info *relaycommon.RelayInfo, response any) (any, *dto.Usage, error) {
	geminiResp, err := interactionsToGeminiChat(c, info, response)
	if err != nil {
		return nil, nil, err
	}
	chatResp := geminichat.ResponseGeminiChat2OpenAI(info.UpstreamModelName, common.GetTimestamp(), geminiResp)
	chatResp.Model = info.UpstreamModelName
	usage := geminichat.UsageFromGeminiMetadata(&geminiResp.UsageMetadata, info.GetEstimatePromptTokens())
	return chatResp, usage, nil
}

func convertGeminiInteractionsStreamChunkToOpenAIChat(c *gin.Context, info *relaycommon.RelayInfo, response any, state any) ([]any, *dto.Usage, error) {
	// 流式由 adaptor 层 SSE 翻译器处理为 gemini 流后走既有 handler;此处仅非流式可达
	return nil, nil, fmt.Errorf("interactions stream conversion handled at adaptor layer")
}

func newGeminiInteractionsToOpenAIChatStreamState(options ResponseStreamOptions) any {
	return nil
}

func convertGeminiInteractionsToClaude(c *gin.Context, info *relaycommon.RelayInfo, response any) (any, *dto.Usage, error) {
	chatResp, usage, err := convertGeminiInteractionsToOpenAIChat(c, info, response)
	if err != nil {
		return nil, nil, err
	}
	converted, err := ConvertResponse(c, info, types.RelayFormatClaude, chatResp)
	if err != nil {
		return nil, nil, err
	}
	return converted.Value, usage, nil
}

func convertGeminiInteractionsStreamToClaude(c *gin.Context, info *relaycommon.RelayInfo, response any) (any, *dto.Usage, error) {
	return nil, nil, fmt.Errorf("interactions stream conversion handled at adaptor layer")
}

func convertGeminiInteractionsToGeminiContent(c *gin.Context, info *relaycommon.RelayInfo, response any) (any, *dto.Usage, error) {
	geminiResp, err := interactionsToGeminiChat(c, info, response)
	if err != nil {
		return nil, nil, err
	}
	usage := geminichat.UsageFromGeminiMetadata(&geminiResp.UsageMetadata, info.GetEstimatePromptTokens())
	return geminiResp, usage, nil
}

func convertGeminiInteractionsStreamToGeminiContent(c *gin.Context, info *relaycommon.RelayInfo, response any) (any, *dto.Usage, error) {
	return nil, nil, fmt.Errorf("interactions stream conversion handled at adaptor layer")
}

func convertGeminiInteractionsToOpenAIResponses(c *gin.Context, info *relaycommon.RelayInfo, response any) (any, *dto.Usage, error) {
	chatResp, usage, err := convertGeminiInteractionsToOpenAIChat(c, info, response)
	if err != nil {
		return nil, nil, err
	}
	converted, err := ConvertResponse(c, info, types.RelayFormatOpenAIResponses, chatResp)
	if err != nil {
		return nil, nil, err
	}
	return converted.Value, usage, nil
}

func convertGeminiInteractionsStreamToOpenAIResponses(c *gin.Context, info *relaycommon.RelayInfo, response any) (any, *dto.Usage, error) {
	return nil, nil, fmt.Errorf("interactions stream conversion handled at adaptor layer")
}
