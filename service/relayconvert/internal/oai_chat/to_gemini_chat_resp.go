package oaichat

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"sort"
)

// ResponseOpenAI2Gemini 将 OpenAI 响应转换为 Gemini 格式
func ResponseOpenAI2Gemini(openAIResponse *dto.OpenAITextResponse, info *relaycommon.RelayInfo) *dto.GeminiChatResponse {
	totalTokens := openAIResponse.TotalTokens
	if totalTokens == 0 {
		totalTokens = openAIResponse.PromptTokens + openAIResponse.CompletionTokens
	}
	geminiResponse := &dto.GeminiChatResponse{
		Candidates:       make([]dto.GeminiChatCandidate, 0, len(openAIResponse.Choices)),
		HasUsageMetadata: true,
		UsageMetadata: dto.GeminiUsageMetadata{
			PromptTokenCount:     openAIResponse.PromptTokens,
			CandidatesTokenCount: openAIResponse.CompletionTokens,
			TotalTokenCount:      totalTokens,
			BillingUsage:         openAIBillingUsageFromUsage(&openAIResponse.Usage),
		},
	}
	if metadata, ok := geminiBillingMetadataFromOpenAIUsage(&openAIResponse.Usage); ok {
		geminiResponse.UsageMetadata = metadata
	}

	for _, choice := range openAIResponse.Choices {
		candidate := dto.GeminiChatCandidate{
			Index:         int64(choice.Index),
			SafetyRatings: []dto.GeminiChatSafetyRating{},
		}

		// 设置结束原因
		var finishReason string
		switch choice.FinishReason {
		case "stop":
			finishReason = "STOP"
		case "length":
			finishReason = "MAX_TOKENS"
		case "content_filter":
			finishReason = "SAFETY"
		case "tool_calls":
			finishReason = "STOP"
		default:
			finishReason = "STOP"
		}
		candidate.FinishReason = &finishReason

		// 转换消息内容
		content := dto.GeminiChatContent{
			Role:  "model",
			Parts: make([]dto.GeminiPart, 0),
		}

		textContent := choice.Message.StringContent()
		if choice.Message.Refusal != nil {
			textContent += *choice.Message.Refusal
		}
		if reasoning := choice.Message.GetReasoningContent(); reasoning != "" {
			content.Parts = append(content.Parts, dto.GeminiPart{Text: reasoning, Thought: true})
		}
		if textContent != "" {
			part := dto.GeminiPart{
				Text: textContent,
			}
			content.Parts = append(content.Parts, part)
		}

		toolCalls := choice.Message.ParseToolCalls()
		for _, toolCall := range toolCalls {
			var args map[string]interface{}
			if toolCall.Function.Arguments != "" {
				if err := common.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					args = map[string]interface{}{"arguments": toolCall.Function.Arguments}
				}
			} else {
				args = make(map[string]interface{})
			}

			part := dto.GeminiPart{
				FunctionCall: &dto.FunctionCall{
					ID:           toolCall.ID,
					FunctionName: toolCall.Function.Name,
					Arguments:    args,
				},
			}
			content.Parts = append(content.Parts, part)
		}

		candidate.Content = content
		geminiResponse.Candidates = append(geminiResponse.Candidates, candidate)
	}

	return geminiResponse
}

// StreamResponseOpenAI2Gemini 将 OpenAI 流式响应转换为 Gemini 格式
func StreamResponseOpenAI2Gemini(openAIResponse *dto.ChatCompletionsStreamResponse, info *relaycommon.RelayInfo) (*dto.GeminiChatResponse, error) {
	response := &dto.GeminiChatResponse{}
	if openAIResponse.Usage != nil {
		usage := openAIResponse.Usage
		response.HasUsageMetadata = true
		response.UsageMetadata = dto.GeminiUsageMetadata{PromptTokenCount: usage.PromptTokens, CandidatesTokenCount: usage.CompletionTokens, TotalTokenCount: usage.TotalTokens, BillingUsage: openAIBillingUsageFromUsage(usage)}
		if metadata, ok := geminiBillingMetadataFromOpenAIUsage(usage); ok {
			response.UsageMetadata = metadata
		}
	}
	for _, choice := range openAIResponse.Choices {
		candidate := dto.GeminiChatCandidate{Index: int64(choice.Index), Content: dto.GeminiChatContent{Role: "model"}}
		if reasoning := choice.Delta.GetReasoningContent(); reasoning != "" {
			candidate.Content.Parts = append(candidate.Content.Parts, dto.GeminiPart{Text: reasoning, Thought: true})
		}
		if text := choice.Delta.GetContentString(); text != "" {
			candidate.Content.Parts = append(candidate.Content.Parts, dto.GeminiPart{Text: text})
		}
		if choice.Delta.Refusal != nil {
			candidate.Content.Parts = append(candidate.Content.Parts, dto.GeminiPart{Text: *choice.Delta.Refusal})
		}
		if len(choice.Delta.ToolCalls) > 0 {
			if info == nil {
				return nil, fmt.Errorf("Gemini stream conversion requires state for tool calls")
			}
			if info.GeminiToolCalls == nil {
				info.GeminiToolCalls = make(map[int]map[int]*dto.ToolCallResponse)
			}
			if info.GeminiToolCalls[choice.Index] == nil {
				info.GeminiToolCalls[choice.Index] = make(map[int]*dto.ToolCallResponse)
			}
			for i, delta := range choice.Delta.ToolCalls {
				if delta.Custom != nil {
					return nil, fmt.Errorf("Gemini conversion cannot preserve custom tool calls")
				}
				index := i
				if delta.Index != nil {
					index = *delta.Index
				}
				tool := info.GeminiToolCalls[choice.Index][index]
				if tool == nil {
					tool = &dto.ToolCallResponse{}
					info.GeminiToolCalls[choice.Index][index] = tool
				}
				if delta.ID != "" {
					tool.ID = delta.ID
				}
				if delta.Function.Name != "" {
					tool.Function.Name += delta.Function.Name
				}
				tool.Function.Arguments += delta.Function.Arguments
			}
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			parts, err := flushGeminiToolCalls(info, choice.Index)
			if err != nil {
				return nil, err
			}
			candidate.Content.Parts = append(candidate.Content.Parts, parts...)
			reason := "STOP"
			switch *choice.FinishReason {
			case "length":
				reason = "MAX_TOKENS"
			case "content_filter":
				reason = "SAFETY"
			}
			candidate.FinishReason = &reason
		}
		if len(candidate.Content.Parts) > 0 || candidate.FinishReason != nil {
			response.Candidates = append(response.Candidates, candidate)
		}
	}
	if len(openAIResponse.Choices) == 0 && openAIResponse.Usage != nil && info != nil {
		var indices []int
		for index := range info.GeminiToolCalls {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for _, index := range indices {
			parts, err := flushGeminiToolCalls(info, index)
			if err != nil {
				return nil, err
			}
			if len(parts) > 0 {
				response.Candidates = append(response.Candidates, dto.GeminiChatCandidate{Index: int64(index), Content: dto.GeminiChatContent{Role: "model", Parts: parts}, FinishReason: common.GetPointer("STOP")})
			}
		}
	}
	if len(response.Candidates) == 0 && !response.HasUsageMetadata {
		return nil, nil
	}
	return response, nil
}

func flushGeminiToolCalls(info *relaycommon.RelayInfo, choice int) ([]dto.GeminiPart, error) {
	if info == nil {
		return nil, nil
	}
	tools := info.GeminiToolCalls[choice]
	var indices []int
	for index := range tools {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	var parts []dto.GeminiPart
	for _, index := range indices {
		tool := tools[index]
		args := make(map[string]any)
		if tool.Function.Arguments != "" {
			if err := common.Unmarshal([]byte(tool.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("invalid streamed arguments for tool %q: %w", tool.Function.Name, err)
			}
		}
		if tool.Function.Name == "" || args == nil {
			return nil, fmt.Errorf("Gemini conversion requires a tool name and JSON object arguments")
		}
		parts = append(parts, dto.GeminiPart{FunctionCall: &dto.FunctionCall{ID: tool.ID, FunctionName: tool.Function.Name, Arguments: args}})
	}
	delete(info.GeminiToolCalls, choice)
	return parts, nil
}

func geminiBillingMetadataFromOpenAIUsage(usage *dto.Usage) (dto.GeminiUsageMetadata, bool) {
	if usage == nil || usage.BillingUsage == nil || usage.BillingUsage.GeminiUsageMetadata == nil {
		return dto.GeminiUsageMetadata{}, false
	}
	if usage.BillingUsage.Source != dto.BillingUsageSourceGeminiChat && usage.BillingUsage.Semantic != dto.BillingUsageSemanticGemini {
		return dto.GeminiUsageMetadata{}, false
	}
	billingUsage := dto.CloneBillingUsage(usage.BillingUsage)
	if billingUsage == nil || billingUsage.GeminiUsageMetadata == nil {
		return dto.GeminiUsageMetadata{}, false
	}
	return *billingUsage.GeminiUsageMetadata, true
}

func openAIBillingUsageFromUsage(usage *dto.Usage) *dto.BillingUsage {
	if usage == nil {
		return nil
	}
	if existingBillingUsage := dto.CloneBillingUsage(usage.BillingUsage); existingBillingUsage != nil && existingBillingUsage.OpenAIUsage != nil {
		if existingBillingUsage.Source == dto.BillingUsageSourceOAIChat ||
			existingBillingUsage.Source == dto.BillingUsageSourceOAIResponses ||
			existingBillingUsage.Semantic == dto.BillingUsageSemanticOpenAI {
			return existingBillingUsage
		}
	}
	return dto.NewOpenAIChatBillingUsage(usage)
}
