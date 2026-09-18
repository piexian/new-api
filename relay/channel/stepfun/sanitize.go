package stepfun

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// StepFun Messages 官方仅承诺支持 docs 中列出的字段（"未在本文出现的字段请不要传入"），
// 且下游客户端（Claude Code 等）会带入 metadata / thinking / cache_control / tool_choice /
// context_management 等 Anthropic 私有字段。这里按官方形态裁剪：
//   - 保留：model、messages(text|image|tool_use|tool_result)、max_tokens、system、tools、
//     output_config.effort、stream、temperature、top_p、top_k、stop_sequences
//   - thinking 转换为官方字段 output_config.effort（预算 → high/medium/low）
//   - 其余顶层字段与块级 cache_control 一律剥离，避免上游 400
var stepFunMessagesBlockTypes = map[string]struct{}{
	"text":        {},
	"image":       {},
	"tool_use":    {},
	"tool_result": {},
}

func sanitizeStepFunMessagesRequest(req *dto.ClaudeRequest) {
	if req == nil {
		return
	}

	effort := stepFunEffortFromThinking(req.Thinking)
	req.Thinking = nil
	req.ToolChoice = nil
	req.Prompt = ""
	req.MaxTokensToSample = nil
	req.CacheControl = nil
	req.InferenceGeo = ""
	req.ServiceTier = ""
	req.Speed = nil
	req.ContextManagement = nil
	req.OutputFormat = nil
	req.Container = nil
	req.McpServers = nil
	req.Metadata = nil

	if len(req.OutputConfig) == 0 && effort != "" {
		if raw, err := common.Marshal(map[string]string{"effort": effort}); err == nil {
			req.OutputConfig = raw
		}
	}

	req.System = sanitizeStepFunSystem(req.System)
	req.Tools = sanitizeStepFunTools(req.Tools)
	for i := range req.Messages {
		req.Messages[i].Content = sanitizeStepFunContent(req.Messages[i].Content)
	}
}

// stepFunEffortFromThinking 把 Anthropic thinking 预算映射到官方 output_config.effort 档位。
func stepFunEffortFromThinking(thinking *dto.Thinking) string {
	if thinking == nil || !strings.EqualFold(strings.TrimSpace(thinking.Type), "enabled") {
		return ""
	}
	if thinking.BudgetTokens == nil {
		return "medium"
	}
	switch budget := *thinking.BudgetTokens; {
	case budget >= 32000:
		return "high"
	case budget >= 8192:
		return "medium"
	default:
		return "low"
	}
}

// sanitizeStepFunSystem 只保留文本 system 块（官方仅支持字符串或文本块数组）。
func sanitizeStepFunSystem(system any) any {
	switch sys := system.(type) {
	case nil:
		return nil
	case string:
		return sys
	case []dto.ClaudeMediaMessage:
		blocks := sanitizeStepFunTypedBlocks(sys, true)
		if len(blocks) == 0 {
			return nil
		}
		return blocks
	case []any:
		blocks := sanitizeStepFunGenericBlocks(sys, true)
		if len(blocks) == 0 {
			return nil
		}
		return blocks
	}
	return nil
}

// sanitizeStepFunTools 只保留函数工具（name/description/input_schema），
// 剥离 web_search 等服务器工具与工具级 cache_control。
func sanitizeStepFunTools(tools any) any {
	switch list := tools.(type) {
	case nil:
		return nil
	case []dto.Tool:
		if len(list) == 0 {
			return nil
		}
		return list
	case []any:
		out := make([]any, 0, len(list))
		for _, item := range list {
			switch tool := item.(type) {
			case dto.Tool:
				out = append(out, tool)
			case map[string]any:
				if toolType, ok := tool["type"].(string); ok && toolType != "" && toolType != "custom" {
					// 服务器工具（web_search_20250305 等）StepFun 不支持
					continue
				}
				cleaned := map[string]any{}
				for _, key := range []string{"name", "description", "input_schema"} {
					if value, ok := tool[key]; ok {
						cleaned[key] = value
					}
				}
				if _, ok := cleaned["name"]; !ok {
					continue
				}
				out = append(out, cleaned)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

// sanitizeStepFunContent 处理消息 content：字符串原样保留，块数组剥离非官方块与 cache_control。
func sanitizeStepFunContent(content any) any {
	switch value := content.(type) {
	case nil:
		return nil
	case string:
		return value
	case []dto.ClaudeMediaMessage:
		blocks := sanitizeStepFunTypedBlocks(value, false)
		if len(blocks) == 0 {
			return nil
		}
		return blocks
	case []any:
		blocks := sanitizeStepFunGenericBlocks(value, false)
		if len(blocks) == 0 {
			return nil
		}
		return blocks
	}
	return content
}

// sanitizeStepFunTypedBlocks 清理 DTO 形态的块数组；textOnly 为 true 时仅保留文本块。
func sanitizeStepFunTypedBlocks(blocks []dto.ClaudeMediaMessage, textOnly bool) []dto.ClaudeMediaMessage {
	out := make([]dto.ClaudeMediaMessage, 0, len(blocks))
	for _, block := range blocks {
		block.CacheControl = nil
		if textOnly && block.Type != "" && block.Type != dto.ContentTypeText {
			continue
		}
		if !textOnly {
			if _, ok := stepFunMessagesBlockTypes[block.Type]; !ok {
				// thinking / redacted_thinking / document 等 Anthropic 私有块上游不认
				continue
			}
		}
		if block.Type == "tool_result" {
			block.Content = sanitizeStepFunContent(block.Content)
		}
		out = append(out, block)
	}
	return out
}

// sanitizeStepFunGenericBlocks 清理 map 形态的块数组（JSON 反序列化为 any 的常见形态）。
func sanitizeStepFunGenericBlocks(blocks []any, textOnly bool) []any {
	out := make([]any, 0, len(blocks))
	for _, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		delete(block, "cache_control")
		blockType, _ := block["type"].(string)
		if textOnly {
			if blockType != "" && blockType != dto.ContentTypeText {
				continue
			}
			out = append(out, block)
			continue
		}
		if _, ok := stepFunMessagesBlockTypes[blockType]; !ok {
			continue
		}
		if blockType == "tool_result" {
			if inner, exists := block["content"]; exists {
				block["content"] = sanitizeStepFunContent(inner)
			}
		}
		out = append(out, block)
	}
	return out
}
