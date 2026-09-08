package geminichat

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	relaymedia "github.com/QuantumNous/new-api/service/relayconvert/internal/media"
	"github.com/QuantumNous/new-api/types"
)

func geminiMediaToOpenAI(part dto.GeminiPart) (dto.MediaContent, error) {
	var data, mimeType string
	remote := part.FileData != nil
	if remote {
		data, mimeType = part.FileData.FileUri, part.FileData.MimeType
		if !strings.HasPrefix(data, "https://") && !strings.HasPrefix(data, "http://") {
			return dto.MediaContent{}, fmt.Errorf("OpenAI conversion cannot access a Gemini file reference; supply inline data or an HTTP URL")
		}
	} else if part.InlineData != nil {
		data, mimeType = part.InlineData.Data, part.InlineData.MimeType
	}
	if data == "" {
		return dto.MediaContent{}, fmt.Errorf("Gemini media data is required")
	}
	if remote && mimeType == "" {
		source := types.NewURLFileSource(data)
		encoded, detected, err := relaymedia.ResolveBase64Data(nil, source, "detecting Gemini media type")
		if err != nil {
			return dto.MediaContent{}, err
		}
		data, mimeType, remote = encoded, detected, false
		if cache := source.GetCache(); cache != nil {
			cache.Close()
		}
	}
	if strings.HasPrefix(mimeType, "image/") {
		if !remote {
			data = fmt.Sprintf("data:%s;base64,%s", mimeType, data)
		}
		return dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: data, MimeType: mimeType}}, nil
	}
	if strings.HasPrefix(mimeType, "video/") {
		return dto.MediaContent{}, fmt.Errorf("OpenAI Chat conversion does not support Gemini video input")
	}
	if remote {
		source := types.NewURLFileSource(data)
		encoded, detected, err := relaymedia.ResolveBase64Data(nil, source, "formatting Gemini file for Chat")
		if err != nil {
			return dto.MediaContent{}, err
		}
		data = encoded
		if mimeType == "" {
			mimeType = detected
		}
		if cache := source.GetCache(); cache != nil {
			cache.Close()
		}
	}
	switch mimeType {
	case "audio/wav", "audio/x-wav", "audio/mpeg", "audio/mp3":
		format := "wav"
		if mimeType == "audio/mpeg" || mimeType == "audio/mp3" {
			format = "mp3"
		}
		return dto.MediaContent{Type: dto.ContentTypeInputAudio, InputAudio: &dto.MessageInputAudio{Data: data, Format: format}}, nil
	case "application/pdf":
		return dto.MediaContent{Type: dto.ContentTypeFile, File: &dto.MessageFile{FileName: "document.pdf", FileData: "data:application/pdf;base64," + data}}, nil
	default:
		if strings.HasPrefix(mimeType, "text/") {
			return dto.MediaContent{Type: dto.ContentTypeFile, File: &dto.MessageFile{FileName: "document.txt", FileData: fmt.Sprintf("data:%s;base64,%s", mimeType, data)}}, nil
		}
		return dto.MediaContent{}, fmt.Errorf("OpenAI Chat conversion does not support Gemini media type %q", mimeType)
	}
}
