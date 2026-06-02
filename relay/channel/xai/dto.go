package xai

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/dto"
)

// ChatCompletionResponse represents the response from XAI chat completion API
type ChatCompletionResponse struct {
	Id                string                         `json:"id"`
	Object            string                         `json:"object"`
	Created           int64                          `json:"created"`
	Model             string                         `json:"model"`
	Choices           []dto.OpenAITextResponseChoice `json:"choices"`
	Usage             *dto.Usage                     `json:"usage"`
	SystemFingerprint string                         `json:"system_fingerprint"`
}

// ImageRequest represents xAI's JSON image request. xAI image edits use JSON
// even when clients call the OpenAI-compatible multipart edit endpoint.
type ImageRequest struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt" binding:"required"`
	N              *uint           `json:"n,omitempty"`
	ResponseFormat string          `json:"response_format,omitempty"`
	User           json.RawMessage `json:"user,omitempty"`
	AspectRatio    string          `json:"aspect_ratio,omitempty"`
	Resolution     string          `json:"resolution,omitempty"`
	Image          json.RawMessage `json:"image,omitempty"`
	Images         json.RawMessage `json:"images,omitempty"`
}

type xAIImageSource struct {
	Type   string `json:"type,omitempty"`
	URL    string `json:"url,omitempty"`
	FileID string `json:"file_id,omitempty"`
}
