package cerebras

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaymode "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	openai.Adaptor
}

var cerebrasChatFields = map[string]struct{}{
	"model":                 {},
	"messages":              {},
	"clear_thinking":        {},
	"frequency_penalty":     {},
	"logit_bias":            {},
	"logprobs":              {},
	"max_completion_tokens": {},
	"parallel_tool_calls":   {},
	"prediction":            {},
	"presence_penalty":      {},
	"prompt_cache_key":      {},
	"reasoning_effort":      {},
	"response_format":       {},
	"seed":                  {},
	"service_tier":          {},
	"stop":                  {},
	"stream":                {},
	"temperature":           {},
	"tool_choice":           {},
	"tools":                 {},
	"top_logprobs":          {},
	"top_p":                 {},
	"user":                  {},
}

var cerebrasReasoningEffortSuffixes = []string{"-medium", "-high", "-none", "-low"}

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
	if info != nil && info.RelayMode != relaymode.RelayModeUnknown && info.RelayMode != relaymode.RelayModeChatCompletions {
		return nil, errors.New("cerebras channel: only /v1/chat/completions is supported")
	}

	normalizeCerebrasRequest(info, request)

	payload := request.ToMap()
	if original, ok := originalJSONFields(c); ok {
		mergeSupportedRawFields(payload, original)
		if raw, exists := original["extra_body"]; exists {
			mergeExtraBody(payload, raw)
		}
	}
	mergeExtraBody(payload, request.ExtraBody)

	payload["model"] = request.Model
	return filterCerebrasPayload(payload), nil
}

func normalizeCerebrasRequest(info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) {
	if request.MaxCompletionTokens == nil && request.MaxTokens != nil {
		request.MaxCompletionTokens = request.MaxTokens
	}
	request.MaxTokens = nil
	request.StreamOptions = nil

	modelName := strings.TrimSpace(request.Model)
	if info != nil {
		ensureChannelMeta(info)
	}
	if info != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		modelName = strings.TrimSpace(info.UpstreamModelName)
	}
	if baseModel, effort, ok := trimCerebrasReasoningEffortSuffix(modelName); ok {
		request.Model = baseModel
		request.ReasoningEffort = effort
		if info != nil {
			info.UpstreamModelName = baseModel
			info.ReasoningEffort = effort
		}
		return
	}
	if modelName != "" {
		request.Model = modelName
	}
	if info != nil {
		info.ReasoningEffort = request.ReasoningEffort
	}
}

func ensureChannelMeta(info *relaycommon.RelayInfo) {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &relaycommon.ChannelMeta{}
	}
}

func trimCerebrasReasoningEffortSuffix(modelName string) (string, string, bool) {
	for _, suffix := range cerebrasReasoningEffortSuffixes {
		if strings.HasSuffix(modelName, suffix) {
			return strings.TrimSuffix(modelName, suffix), strings.TrimPrefix(suffix, "-"), true
		}
	}
	return modelName, "", false
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

func mergeSupportedRawFields(target map[string]any, fields map[string]json.RawMessage) {
	for key, raw := range fields {
		if key == "extra_body" {
			continue
		}
		if _, supported := cerebrasChatFields[key]; !supported {
			continue
		}
		if _, exists := target[key]; exists {
			continue
		}
		if value, ok := rawJSONValue(raw); ok {
			target[key] = value
		}
	}
}

func mergeExtraBody(target map[string]any, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var extra map[string]json.RawMessage
	if err := common.Unmarshal(raw, &extra); err != nil {
		return
	}
	for key, valueRaw := range extra {
		if _, supported := cerebrasChatFields[key]; !supported {
			continue
		}
		if value, ok := rawJSONValue(valueRaw); ok {
			target[key] = value
		}
	}
}

func rawJSONValue(raw json.RawMessage) (any, bool) {
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	return value, true
}

func filterCerebrasPayload(payload map[string]any) map[string]any {
	filtered := make(map[string]any, len(cerebrasChatFields))
	for key, value := range payload {
		if _, supported := cerebrasChatFields[key]; !supported {
			continue
		}
		filtered[key] = value
	}
	return filtered
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("cerebras channel: /v1/rerank endpoint is not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("cerebras channel: /v1/embeddings endpoint is not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("cerebras channel: audio endpoints are not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("cerebras channel: image endpoints are not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("cerebras channel: /v1/responses endpoint is not supported")
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	return a.Adaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
