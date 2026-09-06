package groq

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Groq 为 OpenAI 兼容上游, 但对未声明字段严格 400 且有私有扩展参数, 出站请求需白名单过滤
type Adaptor struct {
	openai.Adaptor
}

var groqChatFields = map[string]struct{}{
	"model":                 {},
	"messages":              {},
	"temperature":           {},
	"top_p":                 {},
	"n":                     {},
	"stream":                {},
	"stream_options":        {},
	"stop":                  {},
	"max_completion_tokens": {},
	"tools":                 {},
	"tool_choice":           {},
	"parallel_tool_calls":   {},
	"response_format":       {},
	"seed":                  {},
	"user":                  {},
	"service_tier":          {},
	"reasoning_effort":      {},
}

// Groq 私有参数不在 GeneralOpenAIRequest DTO 中, ToMap 会丢弃, 需从原始请求体恢复
var groqRawPassthroughFields = []string{"reasoning_format", "include_reasoning", "disable_tool_validation"}

// 文档检索/引用类参数仅 Compound 系统模型支持, 其余模型传了会被 400
var groqCompoundRawFields = []string{"citation_options", "compound_custom", "search_settings", "documents"}

func isGroqCompoundModel(model string) bool {
	return strings.HasPrefix(model, "groq/compound")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	openaiRequest, err := service.GeminiToOpenAIRequest(request, info)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, openaiRequest)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	openaiRequest, err := service.ClaudeToOpenAIRequest(*request, info)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, openaiRequest)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	sanitizeGroqRequest(info, request)
	payload := filterGroqPayload(request.ToMap())
	if original, ok := originalJSONFields(c); ok {
		restoreGroqRawFields(payload, original, request.Model)
	}
	return payload, nil
}

// sanitizeGroqRequest 清洗 Groq 不接受或值越界的字段, 全部来源于生产 400 报错
func sanitizeGroqRequest(info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) {
	if request.MaxCompletionTokens == nil && request.MaxTokens != nil {
		request.MaxCompletionTokens = request.MaxTokens
	}
	request.MaxTokens = nil

	modelName := strings.TrimSpace(request.Model)
	if info != nil {
		if info.ChannelMeta == nil {
			info.ChannelMeta = &relaycommon.ChannelMeta{}
		}
		if strings.TrimSpace(info.UpstreamModelName) != "" {
			modelName = strings.TrimSpace(info.UpstreamModelName)
			request.Model = modelName
		}
	}
	if limit, ok := maxCompletionTokensByModel[modelName]; ok {
		if request.MaxCompletionTokens != nil && *request.MaxCompletionTokens > limit {
			clamped := limit
			request.MaxCompletionTokens = &clamped
		}
	}

	// 上游仅支持 n=1, 大于 1 直接 400
	if request.N != nil && *request.N > 1 {
		one := 1
		request.N = &one
	}

	// 多轮对话重放思维链字段会被上游 400, name 上游不支持
	for i := range request.Messages {
		request.Messages[i].ReasoningContent = nil
		request.Messages[i].Reasoning = nil
		request.Messages[i].Name = nil
	}

	request.ReasoningEffort = normalizeReasoningEffort(request.ReasoningEffort)

	// 上游文档明确不支持的顶层参数
	request.LogProbs = nil
	request.TopLogProbs = nil
	request.LogitBias = nil
	request.FrequencyPenalty = nil
	request.PresencePenalty = nil
	request.Metadata = nil
	request.Store = nil
	request.PromptCacheKey = ""

	if info != nil {
		info.ReasoningEffort = request.ReasoningEffort
	}
}

func normalizeReasoningEffort(effort string) string {
	switch effort {
	case "", "none", "default", "low", "medium", "high":
		return effort
	case "minimal":
		// OpenAI 新枚举, Groq 无此值
		return "low"
	case "xhigh":
		return "high"
	default:
		return ""
	}
}

func originalJSONFields(c *gin.Context) (map[string]json.RawMessage, bool) {
	if c == nil ||
		c.Request == nil ||
		c.Request.Body == nil ||
		c.Request.Body == http.NoBody ||
		!strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "application/json") {
		return nil, false
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, false
	}
	data, err := storage.Bytes()
	if err != nil || len(data) == 0 {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(data, &fields); err != nil || len(fields) == 0 {
		return nil, false
	}
	return fields, true
}

func restoreGroqRawFields(target map[string]any, fields map[string]json.RawMessage, model string) {
	for _, key := range groqRawPassthroughFields {
		restoreRawField(target, fields, key)
	}
	if isGroqCompoundModel(model) {
		for _, key := range groqCompoundRawFields {
			restoreRawField(target, fields, key)
		}
	}
}

func restoreRawField(target map[string]any, fields map[string]json.RawMessage, key string) {
	raw, ok := fields[key]
	if !ok || len(raw) == 0 {
		return
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return
	}
	target[key] = value
}

func filterGroqPayload(payload map[string]any) map[string]any {
	filtered := make(map[string]any, len(groqChatFields))
	for key, value := range payload {
		if _, supported := groqChatFields[key]; !supported {
			continue
		}
		filtered[key] = value
	}
	return filtered
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("/v1/rerank is not supported on this channel")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("/v1/embeddings is not supported on this channel")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("image endpoints are not supported on this channel")
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
