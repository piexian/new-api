package gemini_interactions

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

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
	out := &dto.GeminiInteractionsRequest{
		Model:  modelName,
		Stream: &isStream,
	}

	if len(req.Instructions) > 0 {
		if v := gjson.ParseBytes(req.Instructions); v.Type == gjson.String && v.String() != "" {
			out.SystemInstruction, _ = common.Marshal(v.String())
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
		if chained := responsesBridgeStatefulInput(req.Input, lookup); chained != nil {
			out.PreviousInteractionID = chained.interactionID
			out.Input, _ = common.Marshal(chained.steps)
			return out, nil
		}
	}

	steps := responsesInputToSteps(req.Input)
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
func responsesInputToSteps(raw json.RawMessage) []dto.GeminiInteractionStep {
	root := gjson.ParseBytes(raw)
	if root.Type == gjson.String {
		return []dto.GeminiInteractionStep{
			{Type: dto.GeminiInteractionStepUserInput, Content: []dto.GeminiInteractionContent{{Type: dto.GeminiInteractionContentText, Text: root.String()}}},
		}
	}
	if !root.IsArray() {
		return nil
	}
	var steps []dto.GeminiInteractionStep
	for _, item := range root.Array() {
		switch item.Get("type").String() {
		case "message":
			stepType := dto.GeminiInteractionStepUserInput
			if item.Get("role").String() == "assistant" {
				stepType = dto.GeminiInteractionStepModelOutput
			}
			if content := responsesContentToBlocks(item.Get("content")); len(content) > 0 {
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
			output := item.Get("output").String()
			if output == "" {
				output = "{}"
			}
			resultBlocks, _ := common.Marshal([]dto.GeminiInteractionContent{
				{Type: dto.GeminiInteractionContentText, Text: output},
			})
			steps = append(steps, dto.GeminiInteractionStep{
				Type:   dto.GeminiInteractionStepFunctionResult,
				CallID: item.Get("call_id").String(),
				Result: resultBlocks,
			})
		}
	}
	return steps
}

// responsesBridgeStatefulInput 定位最后一个桥接命中的 function_call,其后的
// function_call_output 转为 function_result、后续用户 message 转为 user_input
func responsesBridgeStatefulInput(raw json.RawMessage, lookup BridgeLookup) *bridgedInput {
	root := gjson.ParseBytes(raw)
	if !root.IsArray() {
		return nil
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
		return nil
	}

	var steps []dto.GeminiInteractionStep
	var pendingUser []dto.GeminiInteractionContent
	for _, item := range items[lastBridged+1:] {
		switch item.Get("type").String() {
		case "function_call_output":
			output := item.Get("output").String()
			if output == "" {
				output = "{}"
			}
			callID := item.Get("call_id").String()
			resultBlocks, _ := common.Marshal([]dto.GeminiInteractionContent{
				{Type: dto.GeminiInteractionContentText, Text: output},
			})
			steps = append(steps, dto.GeminiInteractionStep{
				Type:   dto.GeminiInteractionStepFunctionResult,
				CallID: callID,
				Name:   callNames[callID],
				Result: resultBlocks,
			})
		case "message":
			if item.Get("role").String() != "user" {
				return nil // 复杂形态回退无状态
			}
			pendingUser = append(pendingUser, responsesContentToBlocks(item.Get("content"))...)
		default:
			return nil
		}
	}
	if len(steps) == 0 {
		return nil
	}
	if len(pendingUser) > 0 {
		steps = append(steps, dto.GeminiInteractionStep{Type: dto.GeminiInteractionStepUserInput, Content: pendingUser})
	}
	return &bridgedInput{interactionID: interactionID, steps: steps}
}

// responsesContentToBlocks message.content 数组 -> interactions content 块
func responsesContentToBlocks(content gjson.Result) []dto.GeminiInteractionContent {
	if content.Type == gjson.String {
		return []dto.GeminiInteractionContent{{Type: dto.GeminiInteractionContentText, Text: content.String()}}
	}
	var out []dto.GeminiInteractionContent
	for _, part := range content.Array() {
		switch part.Get("type").String() {
		case "input_text", "output_text", "text", "summary_text":
			if t := part.Get("text").String(); t != "" {
				out = append(out, dto.GeminiInteractionContent{Type: dto.GeminiInteractionContentText, Text: t})
			}
		case "input_image":
			uri := part.Get("image_url").String()
			if u := part.Get("image_url.url"); u.Exists() {
				uri = u.String()
			}
			if uri != "" {
				out = append(out, dto.GeminiInteractionContent{
					Type:     dto.GeminiInteractionContentImage,
					URI:      uri,
					MimeType: "image/png",
				})
			}
		case "input_file":
			uri := part.Get("file_url").String()
			data := ""
			mime := part.Get("mime_type").String()
			if uri == "" {
				data = part.Get("file_data").String()
			}
			if uri == "" && data == "" {
				continue
			}
			out = append(out, dto.GeminiInteractionContent{
				Type:     dto.GeminiInteractionContentDocument,
				URI:      uri,
				Data:     data,
				MimeType: mime,
			})
		}
	}
	return out
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
