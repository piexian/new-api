package relayconvert

import (
	"context"
	"fmt"
	"io"
	"net/http"

	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	kitconvert "github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"

	"github.com/QuantumNous/new-api/common"
	dto "github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// Gemini Interactions 直接转换器（fork 专属第 5 协议），注册进 relaykit 统一注册表。
// 请求方向:一步到 interactions(保留 call_id,支持桥接续链);
// 响应方向:interaction JSON -> 入站格式(文本类复用 relaykit gemini_chat 中间形状的既有出口)。

const (
	ConverterOpenAIChatToInteractions      = "openai_chat_completions_to_gemini_interactions"
	ConverterClaudeToInteractions          = "claude_messages_to_gemini_interactions"
	ConverterGeminiContentToInteractions   = "gemini_generate_content_to_gemini_interactions"
	ConverterOpenAIResponsesToInteractions = "openai_responses_to_gemini_interactions"
)

// lookupFromInfo 从 RelayInfo 构建桥接查找(仅 interactions 渠道挂载)
var lookupFromInfo func(info *relaycommon.RelayInfo) BridgeLookup

// SetGeminiInteractionsBridgeLookup 注入桥接查找实现(由 gemini_interactions adaptor 调用)
func SetGeminiInteractionsBridgeLookup(fn func(info *relaycommon.RelayInfo) BridgeLookup) {
	lookupFromInfo = fn
}

func bridgeLookupFor(info *relaycommon.RelayInfo) BridgeLookup {
	if lookupFromInfo != nil {
		return lookupFromInfo(info)
	}
	return nil
}

// relayInfoOf 还原宿主 RelayInfo，供桥接查找注入使用；转换本体已收 convmeta/context 签名。
func relayInfoOf(meta convmeta.Meta) *relaycommon.RelayInfo {
	info, _ := meta.(*relaycommon.RelayInfo)
	return info
}

// ---------------------------- 请求方向 ----------------------------

func convertOpenAIChatToInteractions(c context.Context, meta convmeta.Meta, request any) (any, error) {
	req, ok := request.(*kitdto.GeneralOpenAIRequest)
	if !ok {
		if v, ok := request.(kitdto.GeneralOpenAIRequest); ok {
			req = &v
		}
	}
	if req == nil {
		return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", request)
	}
	info := relayInfoOf(meta)
	return kitconvert.ConvertOpenAIChatToInteractions(c, meta, req, bridgeLookupFor(info))
}

func convertClaudeMessagesToInteractions(c context.Context, meta convmeta.Meta, request any) (any, error) {
	req, ok := request.(*kitdto.ClaudeRequest)
	if !ok {
		if v, ok := request.(kitdto.ClaudeRequest); ok {
			req = &v
		}
	}
	if req == nil {
		return nil, fmt.Errorf("expected Anthropic Messages request, got %T", request)
	}
	info := relayInfoOf(meta)
	return kitconvert.ConvertClaudeToInteractions(c, meta, req, bridgeLookupFor(info))
}

func convertGeminiContentToInteractions(c context.Context, meta convmeta.Meta, request any) (any, error) {
	req, ok := request.(*kitdto.GeminiChatRequest)
	if !ok {
		if v, ok := request.(kitdto.GeminiChatRequest); ok {
			req = &v
		}
	}
	if req == nil {
		return nil, fmt.Errorf("expected Gemini generateContent request, got %T", request)
	}
	info := relayInfoOf(meta)
	return kitconvert.ConvertGeminiToInteractions(meta, req, bridgeLookupFor(info))
}

func convertResponsesToInteractions(c context.Context, meta convmeta.Meta, request any) (any, error) {
	req, ok := request.(*kitdto.OpenAIResponsesRequest)
	if !ok {
		if v, ok := request.(kitdto.OpenAIResponsesRequest); ok {
			req = &v
		}
	}
	if req == nil {
		return nil, fmt.Errorf("expected OpenAI Responses request, got %T", request)
	}
	info := relayInfoOf(meta)
	return kitconvert.ConvertResponsesToInteractions(meta, req, bridgeLookupFor(info))
}

// ---------------------------- 响应方向 ----------------------------

// interactionsToGeminiChat 上游 interaction JSON -> generateContent 形状(公共前置)
func interactionsToGeminiChat(meta convmeta.Meta, response any) (*kitdto.GeminiChatResponse, error) {
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
		return kitconvert.InteractionToGeminiChatResponse(&interaction, meta.GetEstimatePromptTokens()), nil
	case *dto.GeminiInteraction:
		return kitconvert.InteractionToGeminiChatResponse(v, meta.GetEstimatePromptTokens()), nil
	default:
		return nil, fmt.Errorf("unsupported interactions response type %T", response)
	}
}

func convertGeminiInteractionsToOpenAIChat(c context.Context, meta convmeta.Meta, response any) (any, *kitdto.Usage, error) {
	geminiResp, err := interactionsToGeminiChat(meta, response)
	if err != nil {
		return nil, nil, err
	}
	chatResp := kitconvert.ResponseGeminiChat2OpenAI(meta.GetUpstreamModelName(), common.GetTimestamp(), geminiResp)
	chatResp.Model = meta.GetUpstreamModelName()
	usage := kitconvert.UsageFromGeminiMetadata(&geminiResp.UsageMetadata, meta.GetEstimatePromptTokens())
	return chatResp, usage, nil
}

func convertGeminiInteractionsStreamChunkToOpenAIChat(c context.Context, meta convmeta.Meta, response any, state any) ([]any, *kitdto.Usage, error) {
	// 流式由 adaptor 层 SSE 翻译器处理为 gemini 流后走既有 handler;此处仅非流式可达
	return nil, nil, fmt.Errorf("interactions stream conversion handled at adaptor layer")
}

func newGeminiInteractionsToOpenAIChatStreamState(options kitconvert.ResponseStreamOptions) any {
	return nil
}

func convertGeminiInteractionsToClaude(c context.Context, meta convmeta.Meta, response any) (any, *kitdto.Usage, error) {
	chatResp, usage, err := convertGeminiInteractionsToOpenAIChat(c, meta, response)
	if err != nil {
		return nil, nil, err
	}
	converted, err := kitconvert.ConvertResponse(c, meta, types.RelayFormatClaude, chatResp)
	if err != nil {
		return nil, nil, err
	}
	return converted.Value, usage, nil
}

func convertGeminiInteractionsStreamToClaude(c context.Context, meta convmeta.Meta, response any) (any, *kitdto.Usage, error) {
	return nil, nil, fmt.Errorf("interactions stream conversion handled at adaptor layer")
}

func convertGeminiInteractionsToGeminiContent(c context.Context, meta convmeta.Meta, response any) (any, *kitdto.Usage, error) {
	geminiResp, err := interactionsToGeminiChat(meta, response)
	if err != nil {
		return nil, nil, err
	}
	usage := kitconvert.UsageFromGeminiMetadata(&geminiResp.UsageMetadata, meta.GetEstimatePromptTokens())
	return geminiResp, usage, nil
}

func convertGeminiInteractionsStreamToGeminiContent(c context.Context, meta convmeta.Meta, response any) (any, *kitdto.Usage, error) {
	return nil, nil, fmt.Errorf("interactions stream conversion handled at adaptor layer")
}

func convertGeminiInteractionsToOpenAIResponses(c context.Context, meta convmeta.Meta, response any) (any, *kitdto.Usage, error) {
	chatResp, usage, err := convertGeminiInteractionsToOpenAIChat(c, meta, response)
	if err != nil {
		return nil, nil, err
	}
	converted, err := kitconvert.ConvertResponse(c, meta, types.RelayFormatOpenAIResponses, chatResp)
	if err != nil {
		return nil, nil, err
	}
	return converted.Value, usage, nil
}

func convertGeminiInteractionsStreamToOpenAIResponses(c context.Context, meta convmeta.Meta, response any) (any, *kitdto.Usage, error) {
	return nil, nil, fmt.Errorf("interactions stream conversion handled at adaptor layer")
}

// ---------------------------- 注册表挂载 ----------------------------

func init() {
	kitconvert.RegisterTextConverter(kitconvert.TextConverterSpec{
		ID:      ConverterOpenAIChatToInteractions,
		From:    types.RelayFormatOpenAI,
		To:      types.RelayFormatGeminiInteractions,
		Quality: kitconvert.TextConverterQualityGood,
		Req: kitconvert.TextRequestSide{
			Convert: convertOpenAIChatToInteractions,
		},
		Resp: kitconvert.TextResponseSide{
			Convert:            convertGeminiInteractionsToOpenAIChat,
			NewStreamState:     newGeminiInteractionsToOpenAIChatStreamState,
			ConvertStreamChunk: convertGeminiInteractionsStreamChunkToOpenAIChat,
		},
	})
	kitconvert.RegisterTextConverter(kitconvert.TextConverterSpec{
		ID:      ConverterClaudeToInteractions,
		From:    types.RelayFormatClaude,
		To:      types.RelayFormatGeminiInteractions,
		Quality: kitconvert.TextConverterQualityGood,
		Req: kitconvert.TextRequestSide{
			Convert: convertClaudeMessagesToInteractions,
		},
		Resp: kitconvert.TextResponseSide{
			Convert:       convertGeminiInteractionsToClaude,
			ConvertStream: convertGeminiInteractionsStreamToClaude,
		},
	})
	kitconvert.RegisterTextConverter(kitconvert.TextConverterSpec{
		ID:      ConverterGeminiContentToInteractions,
		From:    types.RelayFormatGemini,
		To:      types.RelayFormatGeminiInteractions,
		Quality: kitconvert.TextConverterQualityGood,
		Req: kitconvert.TextRequestSide{
			Convert: convertGeminiContentToInteractions,
		},
		Resp: kitconvert.TextResponseSide{
			Convert:       convertGeminiInteractionsToGeminiContent,
			ConvertStream: convertGeminiInteractionsStreamToGeminiContent,
		},
	})
	kitconvert.RegisterTextConverter(kitconvert.TextConverterSpec{
		ID:      ConverterOpenAIResponsesToInteractions,
		From:    types.RelayFormatOpenAIResponses,
		To:      types.RelayFormatGeminiInteractions,
		Quality: kitconvert.TextConverterQualityGood,
		Req: kitconvert.TextRequestSide{
			Convert: convertResponsesToInteractions,
		},
		Resp: kitconvert.TextResponseSide{
			Convert:       convertGeminiInteractionsToOpenAIResponses,
			ConvertStream: convertGeminiInteractionsStreamToOpenAIResponses,
		},
	})
}
