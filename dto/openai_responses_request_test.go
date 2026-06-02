package dto

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
)

func TestOpenAIResponsesRequestParseInputSupportsMiniMaxM3Media(t *testing.T) {
	t.Parallel()

	req := OpenAIResponsesRequest{
		Model: "MiniMax-M3",
		Input: []byte(`[
			{
				"type": "message",
				"role": "user",
				"content": [
					{"type": "input_text", "text": "describe these files"},
					{"type": "input_image", "image_url": {"url": "mm_file://image-123", "detail": "high"}},
					{"type": "input_video", "video_url": {"url": "mm_file://video-456", "detail": "low"}},
					{"type": "input_file", "file_id": "file-789"}
				]
			}
		]`),
	}

	inputs := req.ParseInput()
	if len(inputs) != 4 {
		t.Fatalf("ParseInput returned %d items, want 4: %#v", len(inputs), inputs)
	}
	if inputs[0].Type != "input_text" || inputs[0].Text != "describe these files" {
		t.Fatalf("text input = %#v", inputs[0])
	}
	if inputs[1].Type != "input_image" || inputs[1].ImageUrl != "mm_file://image-123" || inputs[1].Detail != "high" {
		t.Fatalf("image input = %#v", inputs[1])
	}
	if inputs[2].Type != "input_video" || inputs[2].VideoUrl != "mm_file://video-456" || inputs[2].Detail != "low" {
		t.Fatalf("video input = %#v", inputs[2])
	}
	if inputs[3].Type != "input_file" || inputs[3].FileID != "file-789" {
		t.Fatalf("file input = %#v", inputs[3])
	}

	meta := req.GetTokenCountMeta()
	if !strings.Contains(meta.CombineText, "describe these files") {
		t.Fatalf("CombineText = %q, want text content", meta.CombineText)
	}
	if len(meta.Files) != 3 {
		t.Fatalf("Files returned %d items, want 3: %#v", len(meta.Files), meta.Files)
	}
	if meta.Files[0].FileType != types.FileTypeImage || meta.Files[0].Detail != "high" {
		t.Fatalf("image file meta = %#v", meta.Files[0])
	}
	if meta.Files[1].FileType != types.FileTypeVideo || meta.Files[1].Detail != "low" {
		t.Fatalf("video file meta = %#v", meta.Files[1])
	}
	if meta.Files[2].FileType != types.FileTypeFile {
		t.Fatalf("generic file meta = %#v", meta.Files[2])
	}
}
