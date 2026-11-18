package dto

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// GeminiDecisionsRequest 是 Decisions API(POST /v1beta/decisions)的请求信封。
// input 与 questions 原样透传,只强校验 model 供渠道分发与计费。
type GeminiDecisionsRequest struct {
	Model     string          `json:"model,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Questions json.RawMessage `json:"questions,omitempty"`
}

// GetRequestModel 返回渠道分发用的模型名。
func (r *GeminiDecisionsRequest) GetRequestModel() string {
	if r == nil {
		return ""
	}
	return r.Model
}

func (r *GeminiDecisionsRequest) IsStream(c *http.Request) bool {
	return false
}

func (r *GeminiDecisionsRequest) SetModelName(modelName string) {
	if r != nil && modelName != "" {
		r.Model = modelName
	}
}

// GetTokenCountMeta 提供 token 估算口径;Decisions 无 max_tokens 配置,仅声明 tokenizer 计数
func (r *GeminiDecisionsRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
}

// GeminiDecisionsResponse 对齐官方 CreateDecisionResponse;answers 原样透传,
// usage 与 Interactions 完全同形故复用其结构。
type GeminiDecisionsResponse struct {
	Answers        map[string]json.RawMessage    `json:"answers,omitempty"`
	Usage          *GeminiInteractionUsage       `json:"usage,omitempty"`
	PromptFeedback *GeminiDecisionPromptFeedback `json:"prompt_feedback,omitempty"`
}

// GeminiDecisionPromptFeedback 输入内容过滤反馈:block_reason 非空表示整单被拦且无 answers。
type GeminiDecisionPromptFeedback struct {
	BlockReason        string `json:"block_reason,omitempty"`
	BlockReasonMessage string `json:"block_reason_message,omitempty"`
}

// IsGeminiDecisionsModel 判断模型是否属于 Gemini Decisions 家族。
func IsGeminiDecisionsModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gemini-decision")
}
