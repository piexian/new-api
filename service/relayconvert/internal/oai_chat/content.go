package oaichat

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func chatContentToResponses(parts []dto.MediaContent, role string) ([]map[string]any, error) {
	converted := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		var value map[string]any
		switch part.Type {
		case dto.ContentTypeText:
			kind := "input_text"
			if role == "assistant" {
				kind = "output_text"
			}
			value = map[string]any{"type": kind, "text": part.Text}
		case dto.ContentTypeImageURL:
			img := part.GetImageMedia()
			if img == nil || img.Url == "" {
				return nil, fmt.Errorf("Responses conversion requires an image URL or data URI")
			}
			value = map[string]any{"type": "input_image", "image_url": img.Url}
			if img.Detail != "" {
				value["detail"] = img.Detail
			}
		case dto.ContentTypeFile:
			file := part.GetFile()
			if file == nil || (file.FileData == "" && file.FileId == "") {
				return nil, fmt.Errorf("Responses conversion requires file data or a file ID")
			}
			value = map[string]any{"type": "input_file"}
			if file.FileName != "" {
				value["filename"] = file.FileName
			}
			if file.FileData != "" {
				value["file_data"] = file.FileData
			} else {
				value["file_id"] = file.FileId
			}
		case dto.ContentTypeInputAudio:
			value = map[string]any{"type": "input_audio", "input_audio": part.InputAudio}
		case dto.ContentTypeVideoUrl:
			value = map[string]any{"type": "input_video", "video_url": part.VideoUrl}
		default:
			return nil, fmt.Errorf("Responses conversion does not support content type %q", part.Type)
		}
		converted = append(converted, value)
	}
	return converted, nil
}

func chatToolCallsToResponses(calls []dto.ToolCallRequest) ([]map[string]any, error) {
	items := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		if call.ID == "" {
			return nil, fmt.Errorf("Responses conversion requires a tool call ID")
		}
		item := map[string]any{"call_id": call.ID}
		switch call.Type {
		case "", "function":
			item["type"], item["name"], item["arguments"] = "function_call", call.Function.Name, call.Function.Arguments
		case "custom":
			var custom dto.CustomToolCall
			if err := common.Unmarshal(call.Custom, &custom); err != nil {
				return nil, err
			}
			item["type"], item["name"], item["input"] = "custom_tool_call", custom.Name, custom.Input
		default:
			return nil, fmt.Errorf("Responses conversion does not support tool call type %q", call.Type)
		}
		items = append(items, item)
	}
	return items, nil
}

func responsesAnnotationsFromChat(annotations []any) []any {
	result := make([]any, 0, len(annotations))
	for _, value := range annotations {
		annotation, ok := value.(map[string]any)
		if ok && annotation["type"] == "url_citation" {
			if citation, ok := annotation["url_citation"].(map[string]any); ok {
				flat := make(map[string]any, len(citation)+1)
				for key, value := range citation {
					flat[key] = value
				}
				flat["type"] = "url_citation"
				result = append(result, flat)
				continue
			}
		}
		result = append(result, value)
	}
	return result
}
