package geminichat

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func applyGeminiControls(req *dto.GeminiChatRequest, out *dto.GeneralOpenAIRequest) error {
	cfg := req.GenerationConfig
	if cfg.Seed != nil {
		out.Seed = common.GetPointer(float64(*cfg.Seed))
	}
	if cfg.PresencePenalty != nil {
		out.PresencePenalty = common.GetPointer(float64(*cfg.PresencePenalty))
	}
	if cfg.FrequencyPenalty != nil {
		out.FrequencyPenalty = common.GetPointer(float64(*cfg.FrequencyPenalty))
	}
	out.LogProbs = cfg.ResponseLogprobs
	if cfg.Logprobs != nil {
		out.TopLogProbs = common.GetPointer(int(*cfg.Logprobs))
	}
	if cfg.ThinkingConfig != nil {
		thinking := cfg.ThinkingConfig
		out.ReasoningEffort = strings.ToLower(thinking.ThinkingLevel)
		if thinking.ThinkingBudget != nil {
			if *thinking.ThinkingBudget == 0 {
				out.ReasoningEffort = "none"
			} else if *thinking.ThinkingBudget > 0 {
				out.Reasoning, _ = common.Marshal(map[string]any{"enabled": true, "max_tokens": *thinking.ThinkingBudget})
			}
		}
	}
	if cfg.ResponseMimeType == "application/json" || cfg.ResponseSchema != nil || len(cfg.ResponseJsonSchema) > 0 {
		schema := cfg.ResponseSchema
		if len(cfg.ResponseJsonSchema) > 0 {
			if err := common.Unmarshal(cfg.ResponseJsonSchema, &schema); err != nil {
				return err
			}
		}
		out.ResponseFormat = &dto.ResponseFormat{Type: "json_object"}
		if schema != nil {
			raw, err := common.Marshal(map[string]any{"name": "response", "schema": lowerSchemaTypes(schema)})
			if err != nil {
				return err
			}
			out.ResponseFormat = &dto.ResponseFormat{Type: "json_schema", JsonSchema: raw}
		}
	}
	if req.ToolConfig != nil && req.ToolConfig.FunctionCallingConfig != nil {
		choice := req.ToolConfig.FunctionCallingConfig
		switch strings.ToUpper(string(choice.Mode)) {
		case "", "AUTO":
			out.ToolChoice = "auto"
		case "NONE":
			out.ToolChoice = "none"
		case "ANY":
			out.ToolChoice = "required"
			if len(choice.AllowedFunctionNames) == 1 {
				out.ToolChoice = map[string]any{"type": "function", "function": map[string]any{"name": choice.AllowedFunctionNames[0]}}
			} else if len(choice.AllowedFunctionNames) > 1 {
				allowed := make(map[string]bool)
				for _, name := range choice.AllowedFunctionNames {
					allowed[name] = true
				}
				var tools []dto.ToolCallRequest
				for _, tool := range out.Tools {
					if allowed[tool.Function.Name] {
						tools = append(tools, tool)
					}
				}
				out.Tools = tools
			}
		default:
			return fmt.Errorf("OpenAI conversion cannot preserve Gemini function calling mode %q", choice.Mode)
		}
	}
	return nil
}

func lowerSchemaTypes(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if key == "nullable" {
				continue
			}
			if key == "type" {
				if name, ok := child.(string); ok {
					out[key] = strings.ToLower(name)
					continue
				}
			}
			out[key] = lowerSchemaTypes(child)
		}
		if nullable, _ := v["nullable"].(bool); nullable {
			if kind, ok := out["type"].(string); ok {
				out["type"] = []string{kind, "null"}
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = lowerSchemaTypes(child)
		}
		return out
	default:
		return value
	}
}
