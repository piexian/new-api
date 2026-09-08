package gemini

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	relaymedia "github.com/QuantumNous/new-api/service/relayconvert/internal/media"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func OpenAIContentToParts(c *gin.Context, content []dto.MediaContent) ([]dto.GeminiPart, error) {
	parts := make([]dto.GeminiPart, 0, len(content))
	for _, part := range content {
		if part.Type == dto.ContentTypeText {
			if part.Text != "" {
				parts = append(parts, dto.GeminiPart{Text: part.Text})
			}
			continue
		}
		source := part.ToFileSource()
		if file := part.GetFile(); part.Type == dto.ContentTypeFile && file != nil && file.FileData != "" {
			source = types.NewFileSourceFromData(file.FileData, relaymedia.FileMimeType(file))
		}
		if source == nil {
			return nil, fmt.Errorf("Gemini conversion cannot transfer content type %q without inline data or a URL", part.Type)
		}
		data, mimeType, err := relaymedia.ResolveBase64Data(c, source, "formatting media for Gemini")
		if err != nil {
			return nil, fmt.Errorf("get file data failed: %w", err)
		}
		if _, ok := SupportedMimeTypes[strings.ToLower(mimeType)]; !ok {
			return nil, fmt.Errorf("Gemini conversion does not support media type %q", mimeType)
		}
		parts = append(parts, dto.GeminiPart{InlineData: &dto.GeminiInlineData{MimeType: mimeType, Data: data}})
	}
	return parts, nil
}
