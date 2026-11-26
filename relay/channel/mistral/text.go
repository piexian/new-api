package mistral

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// moderationRequest 对应 Mistral /v1/moderations 的 ClassificationRequest，
// 与 OpenAI moderations 请求共用 {model, input} 形状。
type moderationRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
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
