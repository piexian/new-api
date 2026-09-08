package gemini_interactions

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaymedia "github.com/QuantumNous/new-api/service/relayconvert/internal/media"

	"github.com/tidwall/gjson"
)

// ResponsesToInteractions OpenAI Responses 入站直接转 Interactions create。
// 两协议高度同构:items<->steps、call_id<->step.id、previous_response_id<->previous_interaction_id、
// reasoning.effort<->thinking_level、text.format<->response_format、url_citation 注解同形。
// 命中桥接时改为有状态续链(previous_interaction_id + 仅提交 function_call_output 及其后的新输入)。
func ResponsesToInteractions(req *dto.OpenAIResponsesRequest, modelName string, isStream bool, lookup BridgeLookup) (*dto.GeminiInteractionsRequest, error) {
	if req == nil {
		return nil, nil
	}
	if req.Temperature != nil || req.TopP != nil {
		return nil, fmt.Errorf("Interactions does not support temperature or top_p")
	}
	copyRequest := *req
	req = &copyRequest
	out := &dto.GeminiInteractionsRequest{
		Model:  modelName,
		Stream: &isStream,
	}

	if len(req.Instructions) > 0 {
		if v := gjson.ParseBytes(req.Instructions); v.Type == gjson.String && v.String() != "" {
			out.SystemInstruction, _ = common.Marshal(v.String())
		}
	}
	if input := gjson.ParseBytes(req.Input); input.IsArray() {
		var retained []json.RawMessage
		var instructions []string
		if len(out.SystemInstruction) > 0 {
			instructions = append(instructions, gjson.ParseBytes(out.SystemInstruction).String())
		}
		for _, item := range input.Array() {
			role := item.Get("role").String()
			if role != "system" && role != "developer" {
				retained = append(retained, json.RawMessage(item.Raw))
				continue
			}
			blocks, err := responsesContentToBlocks(item.Get("content"))
			if err != nil {
				return nil, err
			}
			for _, block := range blocks {
				if block.Type != dto.GeminiInteractionContentText {
					return nil, fmt.Errorf("Interactions system instructions only support text")
				}
				instructions = append(instructions, block.Text)
			}
		}
		req.Input, _ = common.Marshal(retained)
		if len(instructions) > 0 {
			out.SystemInstruction, _ = common.Marshal(strings.Join(instructions, "\n"))
		}
	}
	if tools := responsesToolsToInteractions(req.Tools); len(tools) > 0 {
		out.Tools, _ = common.Marshal(tools)
	}
	if genCfg := responsesGenerationConfig(req); len(genCfg) > 0 {
		out.GenerationConfig, _ = common.Marshal(genCfg)
	}
	if len(req.Metadata) > 0 && gjson.ParseBytes(req.Metadata).IsObject() {
		out.Labels = req.Metadata
	}

	// 桥接:历史 function_call.call_id 命中已存 interaction 时走有状态续链
	if lookup != nil {
		chained, err := responsesBridgeStatefulInput(req.Input, lookup)
		if err != nil {
			return nil, err
		}
		if chained != nil {
			out.PreviousInteractionID = chained.interactionID
			out.Input, _ = common.Marshal(chained.steps)
			return out, nil
		}
	}

	steps, err := responsesInputToSteps(req.Input)
	if err != nil {
		return nil, err
	}
	if len(steps) > 0 {
		out.Input, _ = common.Marshal(steps)
	} else {
		out.Input, _ = common.Marshal([]dto.GeminiInteractionStep{
			{Type: dto.GeminiInteractionStepUserInput, Content: []dto.GeminiInteractionContent{{Type: dto.GeminiInteractionContentText, Text: " "}}},
		})
	}
	return out, nil
}

// responsesInputToSteps Responses input items -> steps 时间线(reasoning/web_search_call 等不可回放项跳过)
func responsesInputToSteps(raw json.RawMessage) ([]dto.GeminiInteractionStep, error) {
	root := gjson.ParseBytes(raw)
	if root.Type == gjson.String {
		return []dto.GeminiInteractionStep{
			{Type: dto.GeminiInteractionStepUserInput, Content: []dto.GeminiInteractionContent{{Type: dto.GeminiInteractionContentText, Text: root.String()}}},
		}, nil
	}
	if !root.IsArray() {
		return nil, fmt.Errorf("Interactions conversion requires a string or array input")
	}
	var steps []dto.GeminiInteractionStep
	for _, item := range root.Array() {
		switch item.Get("type").String() {
		case "", "message":
			stepType := dto.GeminiInteractionStepUserInput
			if item.Get("role").String() == "assistant" {
				stepType = dto.GeminiInteractionStepModelOutput
			}
			content, err := responsesContentToBlocks(item.Get("content"))
			if err != nil {
				return nil, err
			}
			if len(content) > 0 {
				steps = append(steps, dto.GeminiInteractionStep{Type: stepType, Content: content})
			}
		case "function_call":
			args := item.Get("arguments").String()
			if args == "" {
				args = "{}"
			}
			steps = append(steps, dto.GeminiInteractionStep{
				Type:      dto.GeminiInteractionStepFunctionCall,
				ID:        item.Get("call_id").String(),
				Name:      item.Get("name").String(),
				Arguments: json.RawMessage(args),
			})
		case "function_call_output":
			resultBlocks, err := responsesToolResultBlocks(item.Get("output"))
			if err != nil {
				return nil, err
			}
			steps = append(steps, dto.GeminiInteractionStep{
				Type:   dto.GeminiInteractionStepFunctionResult,
				CallID: item.Get("call_id").String(),
				Result: resultBlocks,
			})
		default:
			return nil, fmt.Errorf("Interactions conversion does not support Responses input item %q", item.Get("type").String())
		}
	}
	return steps, nil
}

// responsesBridgeStatefulInput 定位最后一个桥接命中的 function_call,其后的
// function_call_output 转为 function_result、后续用户 message 转为 user_input
func responsesBridgeStatefulInput(raw json.RawMessage, lookup BridgeLookup) (*bridgedInput, error) {
	root := gjson.ParseBytes(raw)
	if !root.IsArray() {
		return nil, nil
	}
	items := root.Array()
	// call_id -> name,用于 function_result 的 name
	callNames := map[string]string{}
	lastBridged := -1
	interactionID := ""
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.Get("type").String() != "function_call" {
			continue
		}
		callID := item.Get("call_id").String()
		if callID != "" {
			callNames[callID] = item.Get("name").String()
			if id, ok := lookup(callID); ok {
				interactionID = id
				lastBridged = i
			}
		}
		break // 只看最后一个 function_call
	}
	if lastBridged == -1 {
		return nil, nil
	}

	var steps []dto.GeminiInteractionStep
	var pendingUser []dto.GeminiInteractionContent
	for _, item := range items[lastBridged+1:] {
		switch item.Get("type").String() {
		case "function_call_output":
			callID := item.Get("call_id").String()
			resultBlocks, err := responsesToolResultBlocks(item.Get("output"))
			if err != nil {
				return nil, err
			}
			steps = append(steps, dto.GeminiInteractionStep{
				Type:   dto.GeminiInteractionStepFunctionResult,
				CallID: callID,
				Name:   callNames[callID],
				Result: resultBlocks,
			})
		case "", "message":
			if item.Get("role").String() != "user" {
				return nil, nil // 复杂形态回退无状态
			}
			content, err := responsesContentToBlocks(item.Get("content"))
			if err != nil {
				return nil, err
			}
			pendingUser = append(pendingUser, content...)
		default:
			return nil, nil
		}
	}
	if len(steps) == 0 {
		return nil, nil
	}
	if len(pendingUser) > 0 {
		steps = append(steps, dto.GeminiInteractionStep{Type: dto.GeminiInteractionStepUserInput, Content: pendingUser})
	}
	return &bridgedInput{interactionID: interactionID, steps: steps}, nil
}

// responsesContentToBlocks message.content 数组 -> interactions content 块
func responsesContentToBlocks(content gjson.Result) ([]dto.GeminiInteractionContent, error) {
	if content.Type == gjson.String {
		return []dto.GeminiInteractionContent{{Type: dto.GeminiInteractionContentText, Text: content.String()}}, nil
	}
	var out []dto.GeminiInteractionContent
	for _, part := range content.Array() {
		switch part.Get("type").String() {
		case "input_text", "output_text", "text", "summary_text":
			if t := part.Get("text").String(); t != "" {
				out = append(out, dto.GeminiInteractionContent{Type: dto.GeminiInteractionContentText, Text: t})
			}
		case "input_image", "input_file", "input_audio", "input_video":
			kind := part.Get("type").String()
			var raw, mime string
			switch kind {
			case "input_image":
				raw = part.Get("image_url").String()
				if part.Get("image_url.url").Exists() {
					raw = part.Get("image_url.url").String()
				}
				mime = part.Get("mime_type").String()
			case "input_file":
				raw = part.Get("file_url").String()
				if raw == "" {
					raw = part.Get("file_data").String()
				}
				mime = part.Get("mime_type").String()
				if mime == "" {
					mime = relaymedia.FileMimeType(&dto.MessageFile{FileName: part.Get("filename").String(), FileData: raw})
				}
			case "input_audio":
				raw = part.Get("input_audio.data").String()
				mime = "audio/" + part.Get("input_audio.format").String()
			case "input_video":
				raw = part.Get("video_url").String()
				if part.Get("video_url.url").Exists() {
					raw = part.Get("video_url.url").String()
				}
				mime = part.Get("mime_type").String()
			}
			if raw == "" {
				return nil, fmt.Errorf("Interactions conversion requires inline data or a URL for %s; provider file IDs cannot be transferred", kind)
			}
			block := dto.GeminiInteractionContent{Type: strings.TrimPrefix(kind, "input_"), MimeType: mime}
			if kind == "input_file" {
				block.Type = dto.GeminiInteractionContentDocument
			}
			if strings.HasPrefix(raw, "data:") {
				header, data, ok := strings.Cut(raw[len("data:"):], ",")
				if !ok || !strings.HasSuffix(header, ";base64") {
					return nil, fmt.Errorf("Interactions conversion requires a base64 data URI")
				}
				block.MimeType = strings.TrimSuffix(header, ";base64")
				block.Data = data
			} else if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
				block.URI = raw
			} else {
				block.Data = raw
			}
			out = append(out, block)
		default:
			return nil, fmt.Errorf("Interactions conversion does not support Responses content type %q", part.Get("type").String())
		}
	}
	return out, nil
}

func responsesToolResultBlocks(output gjson.Result) (json.RawMessage, error) {
	if output.IsArray() {
		blocks, err := responsesContentToBlocks(output)
		if err != nil {
			return nil, err
		}
		return common.Marshal(blocks)
	}
	text := output.String()
	if text == "" {
		text = "{}"
	}
	return common.Marshal([]dto.GeminiInteractionContent{{Type: dto.GeminiInteractionContentText, Text: text}})
}

// responsesToolsToInteractions Responses tools -> interactions tools(web_search->google_search 语义对齐)
func responsesToolsToInteractions(raw json.RawMessage) []map[string]any {
	root := gjson.ParseBytes(raw)
	if !root.IsArray() {
		return nil
	}
	var out []map[string]any
	for _, tool := range root.Array() {
		switch tool.Get("type").String() {
		case "function":
			t := map[string]any{
				"type":        "function",
				"name":        tool.Get("name").String(),
				"description": tool.Get("description").String(),
			}
			if params := tool.Get("parameters"); params.Exists() {
				var v any
				if err := common.Unmarshal([]byte(params.Raw), &v); err == nil {
					t["parameters"] = v
				}
			}
			out = append(out, t)
		case "web_search", "web_search_preview":
			out = append(out, map[string]any{"type": "google_search"})
		case "code_interpreter", "code_interpreter_preview":
			out = append(out, map[string]any{"type": "code_execution"})
		}
	}
	return out
}

// responsesGenerationConfig 保留 interactions 支持的参数;temperature/top_p 按官方弃用丢弃
func responsesGenerationConfig(req *dto.OpenAIResponsesRequest) map[string]any {
	out := map[string]any{}
	if req.MaxOutputTokens != nil && *req.MaxOutputTokens > 0 {
		out["max_output_tokens"] = *req.MaxOutputTokens
	}
	if req.Reasoning != nil {
		switch strings.ToLower(req.Reasoning.Effort) {
		case "minimal", "low", "medium", "high":
			out["thinking_level"] = strings.ToLower(req.Reasoning.Effort)
		}
		if strings.EqualFold(req.Reasoning.Summary, "none") {
			out["thinking_summaries"] = "none"
		}
	}
	if len(req.ToolChoice) > 0 {
		switch gjson.ParseBytes(req.ToolChoice).String() {
		case "auto", "none":
			out["tool_choice"] = gjson.ParseBytes(req.ToolChoice).String()
		case "required":
			out["tool_choice"] = "any"
		}
	}
	// text.format -> response_format
	format := gjson.GetBytes(req.Text, "format")
	if format.Exists() && (format.Get("type").String() == "json_schema" || format.Get("type").String() == "json_object") {
		f := map[string]any{"type": "text", "mime_type": "application/json"}
		if schema := format.Get("schema"); schema.Exists() {
			var v any
			if err := common.Unmarshal([]byte(schema.Raw), &v); err == nil {
				f["schema"] = v
			}
		}
		if name := format.Get("name").String(); name != "" {
			f["schema_name"] = name
		}
		out["response_format"] = []any{f}
	}
	return out
}
