package gemini

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// ApplyExtraBodyConfig applies Gemini-specific OpenAI compatibility options.
// The bool result reports whether the caller should skip the default thinking adapter.
func ApplyExtraBodyConfig(request *dto.GeminiChatRequest, extraBody json.RawMessage, upstreamModelName string) (bool, error) {
	if request == nil || len(extraBody) == 0 {
		return false, nil
	}

	var body map[string]interface{}
	if err := common.Unmarshal(extraBody, &body); err != nil {
		return false, fmt.Errorf("invalid extra body: %w", err)
	}
	googleBody, ok := body["google"].(map[string]interface{})
	if !ok {
		return false, nil
	}

	if _, exists := googleBody["cachedContent"]; exists {
		return false, errors.New("extra_body.google.cachedContent is not supported, use extra_body.google.cached_content instead")
	}
	if cachedContent, exists := googleBody["cached_content"]; exists {
		value, ok := cachedContent.(string)
		if !ok {
			return false, errors.New("extra_body.google.cached_content must be a string")
		}
		request.CachedContent = strings.TrimSpace(value)
	}

	overrideThinking := !strings.HasSuffix(upstreamModelName, "-nothinking")
	if overrideThinking {
		if _, exists := googleBody["thinkingConfig"]; exists {
			return false, errors.New("extra_body.google.thinkingConfig is not supported, use extra_body.google.thinking_config instead")
		}
		if thinkingConfig, ok := googleBody["thinking_config"].(map[string]interface{}); ok {
			if _, exists := thinkingConfig["thinkingBudget"]; exists {
				return false, errors.New("extra_body.google.thinking_config.thinkingBudget is not supported, use extra_body.google.thinking_config.thinking_budget instead")
			}

			var hasThinkingConfig bool
			var next dto.GeminiThinkingConfig
			if thinkingBudget, exists := thinkingConfig["thinking_budget"]; exists {
				switch value := thinkingBudget.(type) {
				case float64:
					budget := int(value)
					next.ThinkingBudget = common.GetPointer(budget)
					next.IncludeThoughts = budget > 0
					hasThinkingConfig = true
				default:
					return false, errors.New("extra_body.google.thinking_config.thinking_budget must be an integer")
				}
			}
			if includeThoughts, exists := thinkingConfig["include_thoughts"]; exists {
				value, ok := includeThoughts.(bool)
				if !ok {
					return false, errors.New("extra_body.google.thinking_config.include_thoughts must be a boolean")
				}
				next.IncludeThoughts = value
				hasThinkingConfig = true
			}
			if thinkingLevel, exists := thinkingConfig["thinking_level"]; exists {
				value, ok := thinkingLevel.(string)
				if !ok {
					return false, errors.New("extra_body.google.thinking_config.thinking_level must be a string")
				}
				next.ThinkingLevel = value
				hasThinkingConfig = true
			}

			if hasThinkingConfig {
				if request.GenerationConfig.ThinkingConfig == nil {
					request.GenerationConfig.ThinkingConfig = &next
				} else {
					if next.ThinkingBudget != nil {
						request.GenerationConfig.ThinkingConfig.ThinkingBudget = next.ThinkingBudget
					}
					request.GenerationConfig.ThinkingConfig.IncludeThoughts = next.IncludeThoughts
					if next.ThinkingLevel != "" {
						request.GenerationConfig.ThinkingConfig.ThinkingLevel = next.ThinkingLevel
					}
				}
			}
		}
	}

	if _, exists := googleBody["imageConfig"]; exists {
		return false, errors.New("extra_body.google.imageConfig is not supported, use extra_body.google.image_config instead")
	}
	if imageConfig, ok := googleBody["image_config"].(map[string]interface{}); ok {
		if _, exists := imageConfig["aspectRatio"]; exists {
			return false, errors.New("extra_body.google.image_config.aspectRatio is not supported, use extra_body.google.image_config.aspect_ratio instead")
		}
		if _, exists := imageConfig["imageSize"]; exists {
			return false, errors.New("extra_body.google.image_config.imageSize is not supported, use extra_body.google.image_config.image_size instead")
		}

		geminiImageConfig := make(map[string]interface{}, 2)
		if aspectRatio, exists := imageConfig["aspect_ratio"]; exists {
			geminiImageConfig["aspectRatio"] = aspectRatio
		}
		if imageSize, exists := imageConfig["image_size"]; exists {
			geminiImageConfig["imageSize"] = imageSize
		}
		if len(geminiImageConfig) > 0 {
			encoded, err := common.Marshal(geminiImageConfig)
			if err != nil {
				return false, fmt.Errorf("failed to marshal image_config: %w", err)
			}
			request.GenerationConfig.ImageConfig = encoded
		}
	}

	return overrideThinking, nil
}
