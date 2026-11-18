package dto

import (
	"encoding/json"
	"net/http"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// OpenAIDecisionsRequest 是 OpenAI Decisions API(POST /v1/decisions)的请求信封。
// input 与 questions 原样透传,只强校验 model 供渠道分发与计费。
type OpenAIDecisionsRequest struct {
	Model     string          `json:"model,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Questions json.RawMessage `json:"questions,omitempty"`
}

// GetRequestModel 返回渠道分发用的模型名。
func (r *OpenAIDecisionsRequest) GetRequestModel() string {
	if r == nil {
		return ""
	}
	return r.Model
}

func (r *OpenAIDecisionsRequest) IsStream(c *http.Request) bool {
	return false
}

func (r *OpenAIDecisionsRequest) SetModelName(modelName string) {
	if r != nil && modelName != "" {
		r.Model = modelName
	}
}

// GetTokenCountMeta 提供 token 估算口径;Decisions 无 max_tokens 配置,仅声明 tokenizer 计数
func (r *OpenAIDecisionsRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
}

// OpenAIDecisionsResponse 对齐官方 Decision 返回;answers 原样透传(数组,按题序回显 name)。
type OpenAIDecisionsResponse struct {
	Answers []json.RawMessage     `json:"answers,omitempty"`
	Model   string                `json:"model,omitempty"`
	Usage   *OpenAIDecisionsUsage `json:"usage,omitempty"`
}

// OpenAIDecisionsUsage 为官方标准 OpenAI usage 形状(input/cached/cache_write/reasoning/total)。
type OpenAIDecisionsUsage struct {
	InputTokens        int `json:"input_tokens,omitempty"`
	OutputTokens       int `json:"output_tokens,omitempty"`
	TotalTokens        int `json:"total_tokens,omitempty"`
	InputTokensDetails struct {
		CachedTokens     int `json:"cached_tokens,omitempty"`
		CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
	} `json:"input_tokens_details,omitempty"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	} `json:"output_tokens_details,omitempty"`
}

// ToUsage 映射为站内 Usage,缓存命中与缓存写计入 prompt 明细供缓存计费。
func (u *OpenAIDecisionsUsage) ToUsage(fallbackPromptTokens int) *Usage {
	if u == nil {
		if fallbackPromptTokens <= 0 {
			return nil
		}
		usage := &Usage{PromptTokens: fallbackPromptTokens}
		usage.PromptTokensDetails.TextTokens = fallbackPromptTokens
		return usage
	}
	promptTokens := u.InputTokens
	if promptTokens <= 0 && fallbackPromptTokens > 0 {
		promptTokens = fallbackPromptTokens
	}
	usage := &Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.TotalTokens,
	}
	usage.PromptTokensDetails.CachedTokens = u.InputTokensDetails.CachedTokens
	usage.PromptTokensDetails.CacheWriteTokens = u.InputTokensDetails.CacheWriteTokens
	usage.CompletionTokenDetails.ReasoningTokens = u.OutputTokensDetails.ReasoningTokens
	return usage
}
