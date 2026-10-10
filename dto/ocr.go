package dto

import (
	"encoding/json"
	"net/http"

	"github.com/QuantumNous/new-api/types"
)

// OCRRequest 对应 POST /v1/ocr 的 Mistral 原生请求。
// Document 是多态字段（document_url / image_url / file），保留原始 JSON 由上层透传。
type OCRRequest struct {
	Model    string          `json:"model"`
	Document json.RawMessage `json:"document"`
}

func (r *OCRRequest) IsStream(c *http.Request) bool {
	return false
}

func (r *OCRRequest) GetTokenCountMeta() *types.TokenCountMeta {
	// OCR 输入是文档而非文本，不做 tokenizer 估算，计费以响应 usage_info 的页数为准
	return &types.TokenCountMeta{}
}

func (r *OCRRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}
