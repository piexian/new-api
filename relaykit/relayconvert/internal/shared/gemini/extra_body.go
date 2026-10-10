package gemini

// fork 特性回植：extra_body.google.cached_content / image_config 的兼容处理。
// thinking_config 由 kit 的 reasoning 管线负责，此处不重复处理。

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// ApplyExtraBodyConfig applies Gemini-specific compatibility options carried in
// extra_body.google.*. It never touches thinking config; the reasoning
// pipeline owns that field.
func ApplyExtraBodyConfig(geminiRequest *dto.GeminiChatRequest, extraBody json.RawMessage) error {
	if geminiRequest == nil || len(extraBody) == 0 {
		return nil
	}

	var body map[string]any
	if err := kitutil.Unmarshal(extraBody, &body); err != nil {
		return fmt.Errorf("invalid extra body: %w", err)
	}
	googleBody, ok := body["google"].(map[string]any)
	if !ok {
		return nil
	}

	if _, exists := googleBody["cachedContent"]; exists {
		return errors.New("extra_body.google.cachedContent is not supported, use extra_body.google.cached_content instead")
	}
	if cachedContent, exists := googleBody["cached_content"]; exists {
		value, ok := cachedContent.(string)
		if !ok {
			return errors.New("extra_body.google.cached_content must be a string")
		}
		geminiRequest.CachedContent = strings.TrimSpace(value)
	}

	if _, exists := googleBody["imageConfig"]; exists {
		return errors.New("extra_body.google.imageConfig is not supported, use extra_body.google.image_config instead")
	}
	if imageConfig, ok := googleBody["image_config"].(map[string]any); ok {
		if _, exists := imageConfig["aspectRatio"]; exists {
			return errors.New("extra_body.google.image_config.aspectRatio is not supported, use extra_body.google.image_config.aspect_ratio instead")
		}
		if _, exists := imageConfig["imageSize"]; exists {
			return errors.New("extra_body.google.image_config.imageSize is not supported, use extra_body.google.image_config.image_size instead")
		}

		geminiImageConfig := make(map[string]any, 2)
		if aspectRatio, exists := imageConfig["aspect_ratio"]; exists {
			geminiImageConfig["aspectRatio"] = aspectRatio
		}
		if imageSize, exists := imageConfig["image_size"]; exists {
			geminiImageConfig["imageSize"] = imageSize
		}
		if len(geminiImageConfig) > 0 {
			encoded, err := kitutil.Marshal(geminiImageConfig)
			if err != nil {
				return fmt.Errorf("failed to marshal image_config: %w", err)
			}
			geminiRequest.GenerationConfig.ImageConfig = encoded
		}
	}

	return nil
}
