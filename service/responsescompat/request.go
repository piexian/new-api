package responsescompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

func ConvertToOpenAIChatRequest(req dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	messages, err := responsesInputToMessages(req)
	if err != nil {
		return nil, err
	}

	tools, err := responsesToolsToChatTools(req.Tools)
	if err != nil {
		return nil, err
	}

	toolChoice, err := responsesToolChoiceToChatToolChoice(req.ToolChoice)
	if err != nil {
		return nil, err
	}

	parallelToolCalls, err := rawBoolPointer(req.ParallelToolCalls)
	if err != nil {
		return nil, fmt.Errorf("invalid parallel_tool_calls: %w", err)
	}

	responseFormat, err := responsesTextToChatResponseFormat(req.Text)
	if err != nil {
		return nil, err
	}

	out := &dto.GeneralOpenAIRequest{
		Model:                req.Model,
		Messages:             messages,
		Stream:               req.Stream,
		StreamOptions:        req.StreamOptions,
		MaxCompletionTokens:  req.MaxOutputTokens,
		Temperature:          req.Temperature,
		TopP:                 req.TopP,
		Tools:                tools,
		ToolChoice:           toolChoice,
		ParallelTooCalls:     parallelToolCalls,
		ResponseFormat:       responseFormat,
		User:                 req.User,
		Store:                req.Store,
		Metadata:             req.Metadata,
		ServiceTier:          rawServiceTier(req.ServiceTier),
		TopLogProbs:          req.TopLogProbs,
		PromptCacheRetention: req.PromptCacheRetention,
		ExtraBody:            req.ExtraBody,
	}
	if req.Reasoning != nil {
		out.ReasoningEffort = req.Reasoning.Effort
	}
	if len(req.PromptCacheKey) > 0 {
		var promptCacheKey string
		if err := common.Unmarshal(req.PromptCacheKey, &promptCacheKey); err == nil {
			out.PromptCacheKey = promptCacheKey
		}
	}
	if len(req.EnableThinking) > 0 {
		out.EnableThinking = req.EnableThinking
	}
	if len(req.Thinking) > 0 {
		out.THINKING = req.Thinking
	}
	return out, nil
}

func ConvertToNonStreamOpenAIChatRequest(req dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if lo.FromPtrOr(req.Stream, false) {
		return nil, errors.New("responses compatibility conversion does not support stream yet")
	}
	return ConvertToOpenAIChatRequest(req)
}

func responsesInputToMessages(req dto.OpenAIResponsesRequest) ([]dto.Message, error) {
	messages := make([]dto.Message, 0)
	if len(req.Instructions) > 0 {
		var instructions string
		if err := common.Unmarshal(req.Instructions, &instructions); err == nil && strings.TrimSpace(instructions) != "" {
			messages = append(messages, dto.Message{
				Role:    "system",
				Content: instructions,
			})
		}
	}
	if len(req.Input) == 0 {
		return messages, nil
	}
	switch common.GetJsonType(req.Input) {
	case "string":
		var text string
		if err := common.Unmarshal(req.Input, &text); err != nil {
			return nil, err
		}
		messages = append(messages, dto.Message{Role: "user", Content: text})
		return messages, nil
	case "array":
		var items []map[string]any
		if err := common.Unmarshal(req.Input, &items); err != nil {
			return nil, err
		}
		for _, item := range items {
			msgs, err := responsesInputItemToMessages(item)
			if err != nil {
				return nil, err
			}
			messages = append(messages, msgs...)
		}
		return messages, nil
	default:
		return nil, fmt.Errorf("unsupported responses input type: %s", common.GetJsonType(req.Input))
	}
}

func responsesInputItemToMessages(item map[string]any) ([]dto.Message, error) {
	itemType := strings.TrimSpace(common.Interface2String(item["type"]))
	switch itemType {
	case "", "message":
		role := strings.TrimSpace(common.Interface2String(item["role"]))
		if role == "" {
			role = "user"
		}
		content, err := responsesContentToChatContent(item["content"], role)
		if err != nil {
			return nil, err
		}
		return []dto.Message{{Role: role, Content: content}}, nil
	case "function_call":
		name := strings.TrimSpace(common.Interface2String(item["name"]))
		if name == "" {
			return nil, nil
		}
		callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
		if callID == "" {
			callID = strings.TrimSpace(common.Interface2String(item["id"]))
		}
		arguments := rawStringOrJSON(item["arguments"])
		msg := dto.Message{Role: "assistant", Content: ""}
		msg.SetToolCalls([]dto.ToolCallRequest{{
			ID:   callID,
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      name,
				Arguments: arguments,
			},
		}})
		return []dto.Message{msg}, nil
	case "function_call_output":
		callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
		output := rawStringOrJSON(item["output"])
		return []dto.Message{{
			Role:       "tool",
			Content:    output,
			ToolCallId: callID,
		}}, nil
	default:
		return nil, nil
	}
}

func responsesContentToChatContent(value any, role string) (any, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []any:
		parts := make([]any, 0, len(v))
		for _, partAny := range v {
			part, ok := partAny.(map[string]any)
			if !ok {
				continue
			}
			chatPart, ok := responsesContentPartToChatPart(part, role)
			if ok {
				parts = append(parts, chatPart)
			}
		}
		if len(parts) == 0 {
			return "", nil
		}
		return parts, nil
	default:
		return rawStringOrJSON(v), nil
	}
}

func responsesContentPartToChatPart(part map[string]any, role string) (map[string]any, bool) {
	partType := strings.TrimSpace(common.Interface2String(part["type"]))
	switch partType {
	case "input_text", "output_text", "text":
		return map[string]any{
			"type": dto.ContentTypeText,
			"text": common.Interface2String(part["text"]),
		}, true
	case "input_image", "image_url":
		imageURL := part["image_url"]
		if imageURL == nil {
			imageURL = part["file_id"]
		}
		chatImage := imageURL
		if detail := common.Interface2String(part["detail"]); detail != "" {
			chatImage = map[string]any{
				"url":    imageURL,
				"detail": detail,
			}
		}
		return map[string]any{
			"type":      dto.ContentTypeImageURL,
			"image_url": chatImage,
		}, true
	case "input_file", "file":
		file := map[string]any{}
		for _, key := range []string{"file_id", "file_data", "filename"} {
			if part[key] != nil {
				file[key] = part[key]
			}
		}
		if len(file) == 0 {
			return nil, false
		}
		return map[string]any{
			"type": dto.ContentTypeFile,
			"file": file,
		}, true
	case "input_audio":
		if inputAudio := part["input_audio"]; inputAudio != nil {
			return map[string]any{
				"type":        dto.ContentTypeInputAudio,
				"input_audio": inputAudio,
			}, true
		}
	}
	return nil, false
}

func responsesToolsToChatTools(raw json.RawMessage) ([]dto.ToolCallRequest, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil, err
	}
	out := make([]dto.ToolCallRequest, 0, len(tools))
	for _, tool := range tools {
		if common.Interface2String(tool["type"]) != "function" {
			continue
		}
		name := common.Interface2String(tool["name"])
		if name == "" {
			if function, ok := tool["function"].(map[string]any); ok {
				name = common.Interface2String(function["name"])
			}
		}
		if name == "" {
			continue
		}
		description := common.Interface2String(tool["description"])
		parameters := tool["parameters"]
		if function, ok := tool["function"].(map[string]any); ok {
			if description == "" {
				description = common.Interface2String(function["description"])
			}
			if parameters == nil {
				parameters = function["parameters"]
			}
		}
		out = append(out, dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        name,
				Description: description,
				Parameters:  parameters,
			},
		})
	}
	return out, nil
}

func responsesToolChoiceToChatToolChoice(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	switch common.GetJsonType(raw) {
	case "string":
		var choice string
		if err := common.Unmarshal(raw, &choice); err != nil {
			return nil, err
		}
		return choice, nil
	case "object":
		var choice map[string]any
		if err := common.Unmarshal(raw, &choice); err != nil {
			return nil, err
		}
		if common.Interface2String(choice["type"]) == "function" {
			if name := common.Interface2String(choice["name"]); name != "" {
				return map[string]any{
					"type": "function",
					"function": map[string]any{
						"name": name,
					},
				}, nil
			}
		}
		return choice, nil
	default:
		return nil, nil
	}
}

func responsesTextToChatResponseFormat(raw json.RawMessage) (*dto.ResponseFormat, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var text map[string]json.RawMessage
	if err := common.Unmarshal(raw, &text); err != nil {
		return nil, err
	}
	formatRaw := text["format"]
	if len(formatRaw) == 0 {
		return nil, nil
	}
	var format map[string]any
	if err := common.Unmarshal(formatRaw, &format); err != nil {
		return nil, err
	}
	formatType := common.Interface2String(format["type"])
	if formatType == "" {
		return nil, nil
	}
	respFormat := &dto.ResponseFormat{Type: formatType}
	if formatType == "json_schema" {
		schemaRaw, _ := common.Marshal(format)
		respFormat.JsonSchema = schemaRaw
	}
	return respFormat, nil
}

func rawBoolPointer(raw json.RawMessage) (*bool, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value bool
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return lo.ToPtr(value), nil
}

func rawStringOrJSON(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.RawMessage:
		return common.JsonRawMessageToString(v)
	default:
		b, err := common.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

func rawServiceTier(serviceTier string) json.RawMessage {
	if serviceTier == "" {
		return nil
	}
	data, _ := common.Marshal(serviceTier)
	return data
}
