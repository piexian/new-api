package dto

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// GeminiInteractionsRequest Gemini Interactions API 创建请求。
// 透传导向设计:仅强类型化路由/计费/流式所需字段,其余以 RawMessage 保留。
type GeminiInteractionsRequest struct {
	Model                 string          `json:"model,omitempty"`
	Agent                 string          `json:"agent,omitempty"`
	Input                 json.RawMessage `json:"input,omitempty"`
	SystemInstruction     json.RawMessage `json:"system_instruction,omitempty"`
	Tools                 json.RawMessage `json:"tools,omitempty"`
	ResponseFormat        json.RawMessage `json:"response_format,omitempty"`
	Stream                *bool           `json:"stream,omitempty"`
	Store                 *bool           `json:"store,omitempty"`
	Background            *bool           `json:"background,omitempty"`
	GenerationConfig      json.RawMessage `json:"generation_config,omitempty"`
	AgentConfig           json.RawMessage `json:"agent_config,omitempty"`
	Environment           json.RawMessage `json:"environment,omitempty"`
	Labels                json.RawMessage `json:"labels,omitempty"`
	PreviousInteractionID string          `json:"previous_interaction_id,omitempty"`
	SafetySettings        json.RawMessage `json:"safety_settings,omitempty"`
	ServiceTier           string          `json:"service_tier,omitempty"`
	Webhook               json.RawMessage `json:"webhook,omitempty"`
}

// GetRequestModel 返回参与路由与计费的模型名;agent 请求以 agent 名作为伪模型名
func (r *GeminiInteractionsRequest) GetRequestModel() string {
	if r.Model != "" {
		return r.Model
	}
	return r.Agent
}

func (r *GeminiInteractionsRequest) IsStream(c *gin.Context) bool {
	return r.Stream != nil && *r.Stream
}

func (r *GeminiInteractionsRequest) SetModelName(modelName string) {
	if r.Model != "" || r.Agent == "" {
		r.Model = modelName
		return
	}
	r.Agent = modelName
}

// GetTokenCountMeta 从 input 中提取文本与文件,供 token 估算与敏感词检查
func (r *GeminiInteractionsRequest) GetTokenCountMeta() *types.TokenCountMeta {
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	if len(r.GenerationConfig) > 0 {
		if v := gjson.GetBytes(r.GenerationConfig, "max_output_tokens"); v.Exists() && v.Int() > 0 {
			meta.MaxTokens = int(v.Int())
		}
	}
	var texts []string
	var files []*types.FileMeta
	collectInteractionInputMeta(r.Input, &texts, &files)
	meta.CombineText = strings.Join(texts, "\n")
	meta.Files = files
	return meta
}

// collectInteractionInputMeta 递归提取 input 中的文本与文件。
// input 形态: string | Content | []Content | []Step(Step 内含 content)。
func collectInteractionInputMeta(raw json.RawMessage, texts *[]string, files *[]*types.FileMeta) {
	if len(raw) == 0 {
		return
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return
	}
	if strings.HasPrefix(trimmed, "\"") {
		var s string
		if err := common.Unmarshal(raw, &s); err == nil && s != "" {
			*texts = append(*texts, s)
		}
		return
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []json.RawMessage
		if err := common.Unmarshal(raw, &items); err != nil {
			return
		}
		for _, item := range items {
			collectInteractionContentMeta(item, texts, files)
		}
		return
	}
	collectInteractionContentMeta(raw, texts, files)
}

// collectInteractionContentMeta 提取单个 Content 或 Step。
// Content: {type: text|image|audio|video|document, text?, data?, uri?, mime_type?}
// Step: {type: user_input|model_output|..., content: [...]},function_result 的 result 也是 Content 数组
func collectInteractionContentMeta(raw json.RawMessage, texts *[]string, files *[]*types.FileMeta) {
	if len(raw) == 0 {
		return
	}
	typeVal := gjson.GetBytes(raw, "type").String()
	// Step 形态: 含 content 数组则递归(function_result 除外,其 result 为内容)
	if content := gjson.GetBytes(raw, "content"); content.Exists() && content.IsArray() {
		for _, item := range content.Array() {
			collectInteractionContentMeta(json.RawMessage(item.Raw), texts, files)
		}
		return
	}
	if typeVal == "function_result" {
		if result := gjson.GetBytes(raw, "result"); result.Exists() && result.IsArray() {
			for _, item := range result.Array() {
				collectInteractionContentMeta(json.RawMessage(item.Raw), texts, files)
			}
		}
		return
	}
	if text := gjson.GetBytes(raw, "text"); text.Exists() && text.String() != "" {
		*texts = append(*texts, text.String())
	}
	mimeType := gjson.GetBytes(raw, "mime_type").String()
	data := gjson.GetBytes(raw, "data").String()
	uri := gjson.GetBytes(raw, "uri").String()
	source := data
	if source == "" {
		source = uri
	}
	if source == "" {
		return
	}
	var fileType types.FileType
	switch {
	case strings.HasPrefix(mimeType, "image/") || typeVal == "image":
		fileType = types.FileTypeImage
	case strings.HasPrefix(mimeType, "audio/") || typeVal == "audio":
		fileType = types.FileTypeAudio
	case strings.HasPrefix(mimeType, "video/") || typeVal == "video":
		fileType = types.FileTypeVideo
	default:
		fileType = types.FileTypeFile
	}
	*files = append(*files, &types.FileMeta{
		FileType: fileType,
		Source:   types.NewFileSourceFromData(source, mimeType),
	})
}

// GeminiInteractionUsage Interactions API 用量结构
type GeminiInteractionUsage struct {
	TotalInputTokens        int                               `json:"total_input_tokens,omitempty"`
	TotalOutputTokens       int                               `json:"total_output_tokens,omitempty"`
	TotalThoughtTokens      int                               `json:"total_thought_tokens,omitempty"`
	TotalCachedTokens       int                               `json:"total_cached_tokens,omitempty"`
	TotalToolUseTokens      int                               `json:"total_tool_use_tokens,omitempty"`
	TotalTokens             int                               `json:"total_tokens,omitempty"`
	InputTokensByModality   []GeminiInteractionModalityTokens `json:"input_tokens_by_modality,omitempty"`
	OutputTokensByModality  []GeminiInteractionModalityTokens `json:"output_tokens_by_modality,omitempty"`
	ToolUseTokensByModality []GeminiInteractionModalityTokens `json:"tool_use_tokens_by_modality,omitempty"`
}

type GeminiInteractionModalityTokens struct {
	Modality string `json:"modality,omitempty"`
	Tokens   int    `json:"tokens,omitempty"`
}

// GeminiInteraction Interactions API 资源(解析计费/转换所需字段)
type GeminiInteraction struct {
	ID     string                  `json:"id,omitempty"`
	Object string                  `json:"object,omitempty"`
	Model  string                  `json:"model,omitempty"`
	Agent  string                  `json:"agent,omitempty"`
	Status string                  `json:"status,omitempty"`
	Steps  []GeminiInteractionStep `json:"steps,omitempty"`
	Usage  *GeminiInteractionUsage `json:"usage,omitempty"`
}

// GeminiInteractionSseEvent SSE 事件载荷;event_type 为判别字段,兼容旧名 type
type GeminiInteractionSseEvent struct {
	EventType   string             `json:"event_type,omitempty"`
	Type        string             `json:"type,omitempty"`
	EventID     string             `json:"event_id,omitempty"`
	Interaction *GeminiInteraction `json:"interaction,omitempty"`
}

// EventName 归一化事件名
func (e *GeminiInteractionSseEvent) EventName() string {
	if e.EventType != "" {
		return e.EventType
	}
	return e.Type
}

// 终态状态集合(用于异步计费结算)
var geminiInteractionTerminalStatus = map[string]bool{
	"completed":       true,
	"failed":          true,
	"cancelled":       true,
	"incomplete":      true,
	"budget_exceeded": true,
}

// IsGeminiInteractionTerminal 判断 interaction 状态是否终态
func IsGeminiInteractionTerminal(status string) bool {
	return geminiInteractionTerminalStatus[status]
}

// HasTokens 是否含可计费 token
func (u *GeminiInteractionUsage) HasTokens() bool {
	if u == nil {
		return false
	}
	return u.TotalInputTokens > 0 || u.TotalOutputTokens > 0 || u.TotalThoughtTokens > 0 || u.TotalTokens > 0
}

// ToUsage 映射为站内 Usage,口径对齐 generateContent:
// prompt = input + tool_use, completion = output + thought, cached 计入 prompt 明细
func (u *GeminiInteractionUsage) ToUsage(fallbackPromptTokens int) *Usage {
	if u == nil {
		if fallbackPromptTokens <= 0 {
			return nil
		}
		usage := &Usage{PromptTokens: fallbackPromptTokens}
		usage.PromptTokensDetails.TextTokens = fallbackPromptTokens
		return usage
	}
	promptTokens := u.TotalInputTokens + u.TotalToolUseTokens
	if promptTokens <= 0 && fallbackPromptTokens > 0 {
		promptTokens = fallbackPromptTokens
	}
	usage := &Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: u.TotalOutputTokens + u.TotalThoughtTokens,
		TotalTokens:      u.TotalTokens,
	}
	usage.CompletionTokenDetails.ReasoningTokens = u.TotalThoughtTokens
	usage.PromptTokensDetails.CachedTokens = u.TotalCachedTokens
	for _, detail := range u.InputTokensByModality {
		switch detail.Modality {
		case "audio":
			usage.PromptTokensDetails.AudioTokens += detail.Tokens
		case "image":
			usage.PromptTokensDetails.ImageTokens += detail.Tokens
		case "text":
			usage.PromptTokensDetails.TextTokens += detail.Tokens
		}
	}
	for _, detail := range u.ToolUseTokensByModality {
		switch detail.Modality {
		case "audio":
			usage.PromptTokensDetails.AudioTokens += detail.Tokens
		case "image":
			usage.PromptTokensDetails.ImageTokens += detail.Tokens
		case "text":
			usage.PromptTokensDetails.TextTokens += detail.Tokens
		}
	}
	for _, detail := range u.OutputTokensByModality {
		switch detail.Modality {
		case "image":
			usage.CompletionTokenDetails.ImageTokens += detail.Tokens
		case "audio":
			usage.CompletionTokenDetails.AudioTokens += detail.Tokens
		case "text":
			usage.CompletionTokenDetails.TextTokens += detail.Tokens
		}
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

// GeminiInteractionStep 构建请求时的强类型 Step
type GeminiInteractionStep struct {
	Type      string                     `json:"type"`
	ID        string                     `json:"id,omitempty"`
	Name      string                     `json:"name,omitempty"`
	CallID    string                     `json:"call_id,omitempty"`
	Arguments json.RawMessage            `json:"arguments,omitempty"`
	Content   []GeminiInteractionContent `json:"content,omitempty"`
	Result    []GeminiInteractionContent `json:"result,omitempty"`
}

// GeminiInteractionContent 构建请求时的强类型 Content 块
type GeminiInteractionContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// 常量
const (
	GeminiInteractionStepUserInput      = "user_input"
	GeminiInteractionStepModelOutput    = "model_output"
	GeminiInteractionStepThought        = "thought"
	GeminiInteractionStepFunctionCall   = "function_call"
	GeminiInteractionStepFunctionResult = "function_result"

	GeminiInteractionContentText     = "text"
	GeminiInteractionContentImage    = "image"
	GeminiInteractionContentAudio    = "audio"
	GeminiInteractionContentVideo    = "video"
	GeminiInteractionContentDocument = "document"
)
