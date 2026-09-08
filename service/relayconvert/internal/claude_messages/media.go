package claudemessages

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaymedia "github.com/QuantumNous/new-api/service/relayconvert/internal/media"
)

func claudeMediaToOpenAI(part dto.ClaudeMediaMessage) (dto.MediaContent, error) {
	if part.Type == "text" || part.Type == "input_text" {
		return dto.MediaContent{Type: dto.ContentTypeText, Text: part.GetText(), CacheControl: part.CacheControl}, nil
	}
	if part.Type != "image" && part.Type != "document" {
		return dto.MediaContent{}, fmt.Errorf("OpenAI Chat conversion does not support Claude content type %q", part.Type)
	}
	if part.Source == nil {
		return dto.MediaContent{}, fmt.Errorf("Claude %s source is required", part.Type)
	}
	var data string
	switch part.Source.Type {
	case "url":
		data = part.Source.Url
	case "base64":
		if raw, ok := part.Source.Data.(string); ok && raw != "" {
			data = fmt.Sprintf("data:%s;base64,%s", part.Source.MediaType, raw)
		}
	case "text":
		if part.Type == "document" {
			return dto.MediaContent{Type: dto.ContentTypeText, Text: common.Interface2String(part.Source.Data)}, nil
		}
	default:
		return dto.MediaContent{}, fmt.Errorf("OpenAI Chat conversion does not support Claude source type %q; provider file references cannot be transferred", part.Source.Type)
	}
	if data == "" {
		return dto.MediaContent{}, fmt.Errorf("Claude %s source data is required", part.Type)
	}
	if part.Type == "image" {
		return dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: data}, CacheControl: part.CacheControl}, nil
	}
	if part.Source.Type == "url" {
		source := part.ToFileSource()
		encoded, mimeType, err := relaymedia.ResolveBase64Data(nil, source, "formatting Claude document for Chat")
		if err != nil {
			return dto.MediaContent{}, err
		}
		data = fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
		if cache := source.GetCache(); cache != nil {
			cache.Close()
		}
	}
	return dto.MediaContent{Type: dto.ContentTypeFile, File: &dto.MessageFile{FileName: "document.pdf", FileData: data}, CacheControl: part.CacheControl}, nil
}
