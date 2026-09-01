package mistral

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

var mistralToolCallIdRegexp = regexp.MustCompile("^[a-zA-Z0-9]{9}$")

// moderationRequest 对应 Mistral /v1/moderations 的 ClassificationRequest，
// 与 OpenAI moderations 请求共用 {model, input} 形状。
type moderationRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

func requestOpenAI2Mistral(request *dto.GeneralOpenAIRequest) *dto.GeneralOpenAIRequest {
	messages := make([]dto.Message, 0, len(request.Messages))
	idMap := make(map[string]string)
	for _, message := range request.Messages {
		// 1. tool_calls.id
		toolCalls := message.ParseToolCalls()
		if toolCalls != nil {
			for i := range toolCalls {
				if !mistralToolCallIdRegexp.MatchString(toolCalls[i].ID) {
					if newId, ok := idMap[toolCalls[i].ID]; ok {
						toolCalls[i].ID = newId
					} else {
						newId, err := common.GenerateRandomCharsKey(9)
						if err == nil {
							idMap[toolCalls[i].ID] = newId
							toolCalls[i].ID = newId
						}
					}
				}
			}
			message.SetToolCalls(toolCalls)
		}

		// 2. tool_call_id
		if message.ToolCallId != "" {
			if newId, ok := idMap[message.ToolCallId]; ok {
				message.ToolCallId = newId
			} else {
				if !mistralToolCallIdRegexp.MatchString(message.ToolCallId) {
					newId, err := common.GenerateRandomCharsKey(9)
					if err == nil {
						idMap[message.ToolCallId] = newId
						message.ToolCallId = newId
					}
				}
			}
		}

		mediaMessages := message.ParseContent()
		for j, mediaMessage := range mediaMessages {
			if mediaMessage.Type == dto.ContentTypeImageURL {
				imageUrl := mediaMessage.GetImageMedia()
				if imageUrl == nil {
					continue
				}
				mediaMessage.ImageUrl = imageUrl.Url
				mediaMessages[j] = mediaMessage
			}
		}
		convertedMessage := dto.Message{
			Role:       message.Role,
			Name:       message.Name,
			ToolCalls:  message.ToolCalls,
			ToolCallId: message.ToolCallId,
		}
		setMistralMessageContent(&convertedMessage, &message, mediaMessages)
		messages = append(messages, convertedMessage)
	}
	out := &dto.GeneralOpenAIRequest{
		Model:           request.Model,
		Stream:          request.Stream,
		Messages:        messages,
		ReasoningEffort: request.ReasoningEffort,
		Temperature:     request.Temperature,
		TopP:            request.TopP,
		Tools:           normalizeMistralToolTypes(request.Tools),
		ToolChoice:      request.ToolChoice,
	}
	if request.MaxTokens != nil || request.MaxCompletionTokens != nil {
		maxTokens := request.GetMaxTokens()
		out.MaxTokens = &maxTokens
	}
	return out
}

// normalizeMistralToolTypes 把内置工具类型名归一到 Mistral 官方名称（web_search /
// web_search_premium），避免 OpenAI/Claude 风格别名被上游严格 schema 拒绝（422）。
// 复制切片，不改写调用方的 request.Tools。
func normalizeMistralToolTypes(tools []dto.ToolCallRequest) []dto.ToolCallRequest {
	if len(tools) == 0 {
		return tools
	}
	out := make([]dto.ToolCallRequest, len(tools))
	copy(out, tools)
	for i := range out {
		if out[i].Type == "" || out[i].Type == "function" || out[i].Type == dto.CustomType {
			continue
		}
		out[i].Type = common.CanonicalBuildInToolName(out[i].Type)
	}
	return out
}

// mistralBuiltInToolsFromResponses 从 Responses 请求的 tools 中挑出 Mistral 支持的内置
// 工具（web_search 系列）。responsescompat.ConvertToOpenAIChatRequest 会丢弃所有非
// function 工具，这里在 Mistral 渠道内补回，不影响其他渠道的转换行为。
// 类型名随后由 requestOpenAI2Mistral 归一到官方名称。
func mistralBuiltInToolsFromResponses(raw json.RawMessage) []dto.ToolCallRequest {
	if len(raw) == 0 {
		return nil
	}
	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	var out []dto.ToolCallRequest
	for _, tool := range tools {
		toolType := common.Interface2String(tool["type"])
		switch common.CanonicalBuildInToolName(toolType) {
		case dto.BuildInToolWebSearch, dto.BuildInToolWebSearchPremium:
			out = append(out, dto.ToolCallRequest{Type: toolType})
		}
	}
	return out
}

func setMistralMessageContent(converted *dto.Message, original *dto.Message, mediaMessages []dto.MediaContent) {
	if original.Role == "assistant" && original.ToolCalls != nil && original.IsStringContent() && original.StringContent() == "" {
		converted.SetMediaContent([]dto.MediaContent{})
		return
	}
	if original.IsStringContent() {
		converted.SetStringContent(original.StringContent())
		return
	}

	filtered := make([]dto.MediaContent, 0, len(mediaMessages))
	allText := true
	var textContent strings.Builder
	for _, mediaMessage := range mediaMessages {
		if mediaMessage.Type == dto.ContentTypeText {
			textContent.WriteString(mediaMessage.Text)
			if mediaMessage.Text == "" {
				continue
			}
		} else {
			allText = false
		}
		filtered = append(filtered, mediaMessage)
	}
	if len(filtered) == 0 {
		converted.SetStringContent("")
	} else if allText {
		converted.SetStringContent(textContent.String())
	} else {
		converted.SetMediaContent(filtered)
	}
}

func normalizeMistralStreamData(data string) (string, error) {
	normalized, err := normalizeMistralResponseData([]byte(data))
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

func normalizeMistralResponseData(data []byte) ([]byte, error) {
	var response map[string]any
	if err := common.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	choices, ok := response["choices"].([]any)
	if !ok {
		return data, nil
	}

	changed := false
	for _, choiceValue := range choices {
		choice, ok := choiceValue.(map[string]any)
		if !ok {
			continue
		}
		for _, field := range []string{"message", "delta"} {
			message, ok := choice[field].(map[string]any)
			if !ok {
				continue
			}
			if normalizeMistralMessageContent(message) {
				changed = true
			}
		}
	}

	if !changed {
		return data, nil
	}
	return common.Marshal(response)
}

func normalizeMistralMessageContent(message map[string]any) bool {
	contentBlocks, ok := message["content"].([]any)
	if !ok {
		return false
	}

	var content strings.Builder
	var reasoningContent strings.Builder
	for _, blockValue := range contentBlocks {
		switch block := blockValue.(type) {
		case string:
			content.WriteString(block)
		case map[string]any:
			if block["type"] == "thinking" {
				appendMistralThinkingText(&reasoningContent, block["thinking"])
				continue
			}
			if text, ok := block["text"].(string); ok {
				content.WriteString(text)
			}
		}
	}

	message["content"] = content.String()
	if reasoningContent.Len() > 0 {
		existingReasoning, _ := message["reasoning_content"].(string)
		message["reasoning_content"] = existingReasoning + reasoningContent.String()
	}
	return true
}

func appendMistralThinkingText(builder *strings.Builder, thinkingValue any) {
	switch thinking := thinkingValue.(type) {
	case string:
		builder.WriteString(thinking)
	case []any:
		for _, itemValue := range thinking {
			switch item := itemValue.(type) {
			case string:
				builder.WriteString(item)
			case map[string]any:
				if text, ok := item["text"].(string); ok {
					builder.WriteString(text)
				}
			}
		}
	}
}
