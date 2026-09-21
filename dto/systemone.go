package dto

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// SystemOneRequest 是 TypeSafe System One (/v1/systemone) 的原生请求体,
// state/questions 用 RawMessage 保真透传
type SystemOneRequest struct {
	BaseRequest
	Model     string          `json:"model"`
	State     json.RawMessage `json:"state,omitempty"`
	Questions json.RawMessage `json:"questions,omitempty"`
}

func (r *SystemOneRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *SystemOneRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

// SystemOneResponse 是 /v1/systemone 的原生响应, answers 保持原始 JSON
type SystemOneResponse struct {
	Model   string                     `json:"model,omitempty"`
	Answers map[string]json.RawMessage `json:"answers,omitempty"`
	Usage   *SystemOneUsage            `json:"usage,omitempty"`
}

type SystemOneUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
