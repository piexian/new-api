package claude

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaymedia "github.com/QuantumNous/new-api/service/relayconvert/internal/media"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// ConvertOpenAIContent is shared by normal messages and tool results so images
// have the same wire representation at either nesting level.
func ConvertOpenAIContent(c *gin.Context, content []dto.MediaContent) ([]dto.ClaudeMediaMessage, error) {
	parts := make([]dto.ClaudeMediaMessage, 0, len(content))
	for _, part := range content {
		if part.Type == dto.ContentTypeText {
			if part.Text != "" {
				parts = append(parts, dto.ClaudeMediaMessage{Type: "text", Text: common.GetPointer(part.Text), CacheControl: part.CacheControl})
			}
			continue
		}
		var source types.FileSource
		switch part.Type {
		case dto.ContentTypeImageURL:
			source = part.ToFileSource()
		case dto.ContentTypeFile:
			file := part.GetFile()
			if file != nil && file.FileData != "" {
				source = types.NewFileSourceFromData(file.FileData, relaymedia.FileMimeType(file))
			}
		default:
			return nil, fmt.Errorf("Claude Messages conversion does not support content type %q", part.Type)
		}
		if source == nil {
			return nil, fmt.Errorf("Claude Messages conversion requires inline data or a URL for %s; file IDs cannot be transferred between providers", part.Type)
		}
		converted, err := ConvertFileSource(c, source)
		if err != nil {
			return nil, err
		}
		converted.CacheControl = part.CacheControl
		parts = append(parts, converted)
	}
	return parts, nil
}

func ConvertFileSource(c *gin.Context, source types.FileSource) (dto.ClaudeMediaMessage, error) {
	data, mimeType, err := relaymedia.ResolveBase64Data(c, source, "formatting media for Claude")
	if err != nil {
		return dto.ClaudeMediaMessage{}, fmt.Errorf("get file data failed: %w", err)
	}
	part := dto.ClaudeMediaMessage{Source: &dto.ClaudeMessageSource{Type: "base64", MediaType: mimeType, Data: data}}
	switch mimeType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		part.Type = "image"
	case "application/pdf":
		part.Type = "document"
	default:
		if strings.HasPrefix(mimeType, "text/") || mimeType == "application/json" || mimeType == "application/yaml" || mimeType == "application/xml" {
			decoded, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return dto.ClaudeMediaMessage{}, fmt.Errorf("decode text file failed: %w", err)
			}
			return dto.ClaudeMediaMessage{Type: "text", Text: common.GetPointer(string(decoded))}, nil
		}
		return dto.ClaudeMediaMessage{}, fmt.Errorf("Claude Messages conversion does not support media type %q", mimeType)
	}
	return part, nil
}
