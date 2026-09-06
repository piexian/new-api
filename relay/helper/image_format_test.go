package helper

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/common"
)

func cerebrasInfo(model string) *common.RelayInfo {
	return &common.RelayInfo{
		OriginModelName: model,
		ChannelMeta:     &common.ChannelMeta{ChannelType: constant.ChannelTypeCerebras, UpstreamModelName: model},
	}
}

// 纯文本模型(gpt-oss-120b / qwen3.8-27b)携带任何图片都应被拒,
// 官方文档: 公共端点仅 gpt-oss-120b 与 qwen-3.8-27b, 均为纯文本
func TestValidateCerebrasImageInputRejectsTextModel(t *testing.T) {
	cases := []struct {
		name    string
		model   string
		request dto.Request
	}{
		{
			name:    "gpt-oss png data uri",
			model:   "gpt-oss-120b",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "data:image/png;base64,AAAA"}}}}}},
		},
		{
			name:    "qwen remote jpeg url",
			model:   "qwen3.8-27b",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "https://example.com/a.jpg"}}}}}},
		},
		{
			name:    "qwen gemini inline data",
			model:   "qwen-3.8-27b",
			request: &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: "AAAA"}}}}}},
		},
		{
			name:    "gpt-oss claude image block",
			model:   "gpt-oss-120b",
			request: &dto.ClaudeRequest{Messages: []dto.ClaudeMessage{{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: "image", Source: &dto.ClaudeMessageSource{Type: "base64", MediaType: "image/png", Data: "AAAA"}}}}}},
		},
		{
			name:  "gpt-oss responses input image",
			model: "gpt-oss-120b",
			request: &dto.OpenAIResponsesRequest{
				Model: "gpt-oss-120b",
				Input: json.RawMessage(`[{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]`),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCerebrasImageInput(nil, cerebrasInfo(tc.model), tc.request)
			if err == nil {
				t.Fatalf("text-only model must reject image input, got nil")
			}
			if !strings.Contains(err.Error(), "does not support image input") {
				t.Fatalf("error %q should explain the model is text-only", err.Error())
			}
			if err.StatusCode != 400 {
				t.Fatalf("status = %d, want 400", err.StatusCode)
			}
		})
	}
}

// gemma 为 Cerebras 唯一支持图片输入的模型, 但只收 PNG/JPEG 的 base64 data URI
func TestValidateCerebrasImageInputGemmaRules(t *testing.T) {
	cases := []struct {
		name    string
		model   string
		request dto.Request
		wantErr string
	}{
		{
			name:    "gif data uri rejected",
			model:   "gemma-4-31b-it",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "data:image/gif;base64,AAAA"}}}}}},
			wantErr: "only accepts PNG and JPEG",
		},
		{
			name:    "webp data uri rejected",
			model:   "gemma-4-31b-it",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "data:image/webp;base64,AAAA"}}}}}},
			wantErr: "only accepts PNG and JPEG",
		},
		{
			name:    "external url rejected even when extension is png",
			model:   "gemma-4-31b-it",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "https://example.com/a.png"}}}}}},
			wantErr: "external image URLs are not supported",
		},
		{
			name:    "gemini inline webp rejected",
			model:   "gemma-4-31b-it",
			request: &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{InlineData: &dto.GeminiInlineData{MimeType: "image/webp", Data: "AAAA"}}}}}},
			wantErr: "only accepts PNG and JPEG",
		},
		{
			name:    "interactions gif rejected",
			model:   "gemma-4-31b-it",
			request: &dto.GeminiInteractionsRequest{Model: "gemma-4-31b-it", Input: json.RawMessage(`[{"type":"image","mime_type":"image/gif","data":"AAAA"}]`)},
			wantErr: "only accepts PNG and JPEG",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCerebrasImageInput(nil, cerebrasInfo(tc.model), tc.request)
			if err == nil {
				t.Fatalf("expected rejection, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q should mention %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateCerebrasImageInputGemmaAllowedAndRejected(t *testing.T) {
	cases := []struct {
		name    string
		model   string
		request dto.Request
		wantErr string
	}{
		{
			name:    "gemma png data uri",
			model:   "gemma-4-31b-it",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "data:image/png;base64,AAAA"}}}}}},
		},
		{
			name:    "gemma jpeg data uri",
			model:   "gemma-4-31b-it",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "data:image/jpeg;base64,AAAA"}}}}}},
		},
		{
			name:    "gemma remote url always rejected, unknown mime does not rescue it",
			model:   "gemma-4-31b-it",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "https://example.com/noext"}}}}}},
			wantErr: "external image URLs are not supported",
		},
		{
			name:    "gemma bare base64 without declared mime passes through",
			model:   "gemma-4-31b-it",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: "AAAA"}}}}}},
		},
		{
			name:    "text model without image",
			model:   "gpt-oss-120b",
			request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: "plain text"}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCerebrasImageInput(nil, cerebrasInfo(tc.model), tc.request)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected pass, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateCerebrasImageInputSkipsOtherChannels(t *testing.T) {
	info := &common.RelayInfo{
		OriginModelName: "gpt-oss-120b",
		ChannelMeta:     &common.ChannelMeta{ChannelType: constant.ChannelTypeGemini, UpstreamModelName: "gpt-oss-120b"},
	}
	request := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{
		InlineData: &dto.GeminiInlineData{MimeType: "image/webp", Data: "AAAA"},
	}}}}}
	if err := ValidateCerebrasImageInput(nil, info, request); err != nil {
		t.Fatalf("non-cerebras channel must not be filtered, got %v", err)
	}
}
