// Package gemini_interactions 实现 Gemini 家族内部两套协议的互转:
// generateContent(GeminiChatRequest) <-> Interactions API。
// 正向:任意入站格式先转 gemini_chat(既有转换),再经本包转 interactions;
// 反向:interactions 响应/SSE 先还原为 gemini_chat,复用既有的 gemini_chat -> OpenAI/Claude 转换。
package gemini_interactions

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// GeminiChatRequestToInteractions 转换 create 请求。
// 无状态回放模式:历史全部转为 steps,store=false,由上游自行裁剪。
func GeminiChatRequestToInteractions(req *dto.GeminiChatRequest, modelName string, isStream bool) (*dto.GeminiInteractionsRequest, error) {
	if req == nil {
		return nil, nil
	}
	out := &dto.GeminiInteractionsRequest{
		Model:  modelName,
		Stream: &isStream,
		Store:  common.GetPointer(false),
	}

	// system instruction: 拼接 text parts
	if req.SystemInstructions != nil {
		var sb strings.Builder
		for _, part := range req.SystemInstructions.Parts {
			if part.Text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(part.Text)
			}
		}
		if sb.Len() > 0 {
			out.SystemInstruction, _ = common.Marshal(sb.String())
		}
	}

	// tools
	if tools := convertTools(req.Tools); len(tools) > 0 {
		out.Tools, _ = common.Marshal(tools)
	}

	// safety settings
	if len(req.SafetySettings) > 0 {
		out.SafetySettings, _ = common.Marshal(convertSafetySettings(req.SafetySettings))
	}

	// generation config
	if genCfg := convertGenerationConfig(&req.GenerationConfig); len(genCfg) > 0 {
		out.GenerationConfig, _ = common.Marshal(genCfg)
	}

	// contents -> steps(确定性 call id,保证每轮回放一致)
	steps := contentsToSteps(req.Contents)
	if len(steps) > 0 {
		out.Input, _ = common.Marshal(steps)
	} else {
		// interactions 要求 input 必填
		out.Input, _ = common.Marshal([]dto.GeminiInteractionStep{
			{Type: dto.GeminiInteractionStepUserInput, Content: []dto.GeminiInteractionContent{{Type: dto.GeminiInteractionContentText, Text: " "}}},
		})
	}
	return out, nil
}

// convertTools 转换工具声明,参数 schema 类型降为小写(interactions 用标准 JSON Schema)
func convertTools(raw json.RawMessage) []map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var geminiTools []dto.GeminiChatTool
	if err := common.Unmarshal(raw, &geminiTools); err != nil {
		return nil
	}
	var out []map[string]any
	for _, tool := range geminiTools {
		if tool.FunctionDeclarations != nil {
			var decls []struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
				Parameters  any    `json:"parameters,omitempty"`
			}
			if data, err := common.Marshal(tool.FunctionDeclarations); err == nil {
				if err := common.Unmarshal(data, &decls); err == nil {
					for _, d := range decls {
						t := map[string]any{"type": "function", "name": d.Name}
						if d.Description != "" {
							t["description"] = d.Description
						}
						if d.Parameters != nil {
							t["parameters"] = downcaseSchemaTypes(d.Parameters)
						}
						out = append(out, t)
					}
				}
			}
			continue
		}
		switch {
		case tool.GoogleSearch != nil || tool.GoogleSearchRetrieval != nil:
			out = append(out, map[string]any{"type": "google_search"})
		case tool.CodeExecution != nil:
			out = append(out, map[string]any{"type": "code_execution"})
		case tool.URLContext != nil:
			out = append(out, map[string]any{"type": "url_context"})
		}
	}
	return out
}

// downcaseSchemaTypes 递归把 Google 大写类型枚举(OBJECT/STRING/...)降为 JSON Schema 小写
func downcaseSchemaTypes(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if k == "type" {
				if s, ok := val.(string); ok {
					out[k] = strings.ToLower(s)
					continue
				}
			}
			out[k] = downcaseSchemaTypes(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = downcaseSchemaTypes(val)
		}
		return out
	default:
		return v
	}
}

func convertSafetySettings(settings []dto.GeminiChatSafetySettings) []map[string]string {
	out := make([]map[string]string, 0, len(settings))
	for _, s := range settings {
		out = append(out, map[string]string{
			"harm_category": downcaseHarmCategory(s.Category),
			"threshold":     s.Threshold,
		})
	}
	return out
}

func downcaseHarmCategory(category string) string {
	switch category {
	case "HARM_CATEGORY_HARASSMENT":
		return "harassment"
	case "HARM_CATEGORY_HATE_SPEECH":
		return "hate_speech"
	case "HARM_CATEGORY_SEXUALLY_EXPLICIT":
		return "sexually_explicit"
	case "HARM_CATEGORY_DANGEROUS_CONTENT":
		return "dangerous_content"
	case "HARM_CATEGORY_CIVIC_INTEGRITY":
		return "civic_integrity"
	case "HARM_CATEGORY_JAILBREAK":
		return "jailbreak"
	default:
		return strings.ToLower(strings.TrimPrefix(category, "HARM_CATEGORY_"))
	}
}

// convertGenerationConfig 保留 interactions 支持的字段;temperature/top_p/top_k 已被官方弃用,丢弃
func convertGenerationConfig(cfg *dto.GeminiChatGenerationConfig) map[string]any {
	if cfg == nil {
		return nil
	}
	out := map[string]any{}
	if cfg.MaxOutputTokens != nil && *cfg.MaxOutputTokens > 0 {
		out["max_output_tokens"] = *cfg.MaxOutputTokens
	}
	if cfg.Seed != nil {
		out["seed"] = *cfg.Seed
	}
	if len(cfg.StopSequences) > 0 {
		out["stop_sequences"] = cfg.StopSequences
	}
	if cfg.ThinkingConfig != nil {
		if level := thinkingLevelFromConfig(cfg.ThinkingConfig); level != "" {
			out["thinking_level"] = level
		}
		if !cfg.ThinkingConfig.IncludeThoughts {
			out["thinking_summaries"] = "none"
		}
	}
	// 结构化输出
	if cfg.ResponseSchema != nil {
		format := map[string]any{"type": "text"}
		if cfg.ResponseMimeType != "" {
			format["mime_type"] = cfg.ResponseMimeType
		} else {
			format["mime_type"] = "application/json"
		}
		format["schema"] = downcaseSchemaTypes(cfg.ResponseSchema)
		out2 := map[string]any{"response_format": []any{format}}
		for k, v := range out {
			out2[k] = v
		}
		return out2
	}
	return out
}

// thinkingLevelFromConfig budget -> level 枚举(0=关思考)
func thinkingLevelFromConfig(cfg *dto.GeminiThinkingConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.ThinkingLevel != "" {
		return strings.ToLower(cfg.ThinkingLevel)
	}
	if cfg.ThinkingBudget == nil {
		return ""
	}
	budget := *cfg.ThinkingBudget
	switch {
	case budget <= 0:
		return "minimal"
	case budget <= 2048:
		return "low"
	case budget <= 8192:
		return "medium"
	default:
		return "high"
	}
}

// contentsToSteps 历史转 steps 时间线;function call id 按函数名计数配对,保证每轮回放一致
func contentsToSteps(contents []dto.GeminiChatContent) []dto.GeminiInteractionStep {
	var steps []dto.GeminiInteractionStep
	callCounter := map[string]int{}
	respCounter := map[string]int{}
	for _, content := range contents {
		var plainParts []dto.GeminiPart
		isModel := content.Role == "model"
		for _, part := range content.Parts {
			switch {
			case part.FunctionCall != nil:
				callCounter[part.FunctionCall.FunctionName]++
				steps = append(steps, dto.GeminiInteractionStep{
					Type:      dto.GeminiInteractionStepFunctionCall,
					ID:        deterministicCallID(part.FunctionCall.FunctionName, callCounter[part.FunctionCall.FunctionName]),
					Name:      part.FunctionCall.FunctionName,
					Arguments: json.RawMessage(marshalArgs(part.FunctionCall.Arguments)),
				})
			case part.FunctionResponse != nil:
				respCounter[part.FunctionResponse.Name]++
				steps = append(steps, dto.GeminiInteractionStep{
					Type:   dto.GeminiInteractionStepFunctionResult,
					CallID: deterministicCallID(part.FunctionResponse.Name, respCounter[part.FunctionResponse.Name]),
					Name:   part.FunctionResponse.Name,
					Result: []dto.GeminiInteractionContent{
						{Type: dto.GeminiInteractionContentText, Text: marshalResponse(part.FunctionResponse.Response)},
					},
				})
			default:
				if !part.Thought {
					plainParts = append(plainParts, part)
				}
			}
		}
		if len(plainParts) > 0 {
			stepType := dto.GeminiInteractionStepUserInput
			if isModel {
				stepType = dto.GeminiInteractionStepModelOutput
			}
			steps = append(steps, dto.GeminiInteractionStep{
				Type:    stepType,
				Content: partsToContents(plainParts),
			})
		}
	}
	return steps
}

func deterministicCallID(name string, seq int) string {
	return "call_" + strings.ReplaceAll(name, "-", "_") + "_" + intToString(seq)
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func marshalArgs(args any) string {
	if args == nil {
		return "{}"
	}
	if data, err := common.Marshal(args); err == nil {
		return string(data)
	}
	return "{}"
}

func marshalResponse(resp map[string]interface{}) string {
	if resp == nil {
		return "{}"
	}
	if data, err := common.Marshal(resp); err == nil {
		return string(data)
	}
	return "{}"
}

// partsToContents gemini part -> interactions content 块
func partsToContents(parts []dto.GeminiPart) []dto.GeminiInteractionContent {
	out := make([]dto.GeminiInteractionContent, 0, len(parts))
	for _, part := range parts {
		switch {
		case part.Text != "":
			out = append(out, dto.GeminiInteractionContent{Type: dto.GeminiInteractionContentText, Text: part.Text})
		case part.InlineData != nil:
			out = append(out, dto.GeminiInteractionContent{
				Type:     contentTypeFromMime(part.InlineData.MimeType),
				Data:     part.InlineData.Data,
				MimeType: part.InlineData.MimeType,
			})
		case part.FileData != nil:
			out = append(out, dto.GeminiInteractionContent{
				Type:     contentTypeFromMime(part.FileData.MimeType),
				URI:      part.FileData.FileUri,
				MimeType: part.FileData.MimeType,
			})
		}
	}
	return out
}

func contentTypeFromMime(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return dto.GeminiInteractionContentImage
	case strings.HasPrefix(mimeType, "audio/"):
		return dto.GeminiInteractionContentAudio
	case strings.HasPrefix(mimeType, "video/"):
		return dto.GeminiInteractionContentVideo
	case strings.HasPrefix(mimeType, "application/pdf"), strings.HasPrefix(mimeType, "text/csv"):
		return dto.GeminiInteractionContentDocument
	default:
		return dto.GeminiInteractionContentDocument
	}
}
