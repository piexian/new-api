package openai

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// SplitToolContent keeps Chat Completions tool messages text-only. Attachments
// are sent in the following user message, after all parallel tool results.
func SplitToolContent(callID string, parts []dto.MediaContent) (string, []dto.MediaContent) {
	var text []string
	var media []dto.MediaContent
	for _, part := range parts {
		if part.Type == dto.ContentTypeText {
			text = append(text, part.Text)
		} else {
			media = append(media, part)
		}
	}
	if len(media) > 0 {
		media = append([]dto.MediaContent{{Type: dto.ContentTypeText, Text: "Attachments from tool result " + callID + ":"}}, media...)
	}
	return strings.Join(text, "\n"), media
}
