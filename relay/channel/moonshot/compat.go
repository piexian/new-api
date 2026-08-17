package moonshot

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
)

type kimiModelFamily int

const (
	kimiModelUnknown kimiModelFamily = iota
	kimiModelK25
	kimiModelK26
	kimiModelK27
	kimiModelK3
)

const kimiK3MaxCompletionTokens = 1_048_576

const (
	kimiK3ShortContextTokenLimit     = 262_144
	kimiK3ProactiveFullContextCutoff = kimiK3ShortContextTokenLimit * 95 / 100 // 249036
)

// shouldRouteKimiK3DirectForEstimatedContext 预估 prompt token 超过 256K 档 95% 安全边际时
// 跳过 k3-256k 降级直接发 k3，省掉一次必然失败的上游往返。
// 预估为 0（CountToken 关闭或媒体低估）时返回 false，由报错兜底路径接管。
func shouldRouteKimiK3DirectForEstimatedContext(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	return info.GetEstimatePromptTokens() > kimiK3ProactiveFullContextCutoff
}

func normalizeKimiOpenAIRequest(info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) kimiModelFamily {
	if request == nil || relaycommon.IsRequestPassThroughEnabled(info) {
		return kimiModelUnknown
	}

	model := getUpstreamModelName(info, request.Model)
	family := classifyKimiModel(model, info != nil && info.ChannelMeta != nil && isKimiCodingBaseURL(info.ChannelBaseUrl))
	if family == kimiModelUnknown {
		return family
	}

	removeConflictingKimiSamplingParameters(request)
	switch family {
	case kimiModelK3:
		normalizeKimiK3Request(request)
	case kimiModelK27:
		normalizeKimiK27Request(request)
	}
	return family
}

func classifyKimiModel(model string, kimiCodingBase bool) kimiModelFamily {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case model == "kimi-k3", strings.HasPrefix(model, "kimi-k3-"), kimiCodingBase && (model == "k3" || model == "k3-256k"):
		return kimiModelK3
	case model == "kimi-for-coding", model == "kimi-for-coding-highspeed", strings.HasPrefix(model, "kimi-k2.7-code"):
		return kimiModelK27
	case strings.HasPrefix(model, "kimi-k2.6"):
		return kimiModelK26
	case strings.HasPrefix(model, "kimi-k2.5"):
		return kimiModelK25
	default:
		return kimiModelUnknown
	}
}

func shouldUseKimiK3ShortContext(info *relaycommon.RelayInfo, model string) bool {
	if info == nil || info.ChannelMeta == nil || relaycommon.IsRequestPassThroughEnabled(info) || !isKimiCodingBaseURL(info.ChannelBaseUrl) {
		return false
	}
	if info.RelayFormat != types.RelayFormatClaude &&
		info.RelayMode != constant.RelayModeChatCompletions &&
		info.RelayMode != constant.RelayModeResponses {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(model), "k3")
}

func removeConflictingKimiSamplingParameters(request *dto.GeneralOpenAIRequest) {
	if request.Temperature != nil && *request.Temperature != 1.0 {
		request.Temperature = nil
	}
	if request.TopP != nil && *request.TopP != 0.95 {
		request.TopP = nil
	}
	if request.N != nil && *request.N != 1 {
		request.N = nil
	}
	if request.PresencePenalty != nil && *request.PresencePenalty != 0 {
		request.PresencePenalty = nil
	}
	if request.FrequencyPenalty != nil && *request.FrequencyPenalty != 0 {
		request.FrequencyPenalty = nil
	}
}

func normalizeKimiK3Request(request *dto.GeneralOpenAIRequest) {
	request.THINKING = nil
	request.Reasoning = nil
	if strings.EqualFold(strings.TrimSpace(request.ReasoningEffort), "max") {
		request.ReasoningEffort = "max"
	} else {
		request.ReasoningEffort = ""
	}
	if request.MaxCompletionTokens == nil {
		request.MaxCompletionTokens = request.MaxTokens
	}
	request.MaxTokens = nil
	if request.MaxCompletionTokens != nil && (*request.MaxCompletionTokens == 0 || *request.MaxCompletionTokens > kimiK3MaxCompletionTokens) {
		request.MaxCompletionTokens = nil
	}
}

func normalizeKimiK27Request(request *dto.GeneralOpenAIRequest) {
	request.ReasoningEffort = ""
	request.Reasoning = nil
	request.THINKING = normalizeKimiK27Thinking(request.THINKING)
	request.ToolChoice = normalizeKimiK27ToolChoice(request.ToolChoice)
}

func normalizeKimiK27Thinking(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	var thinking map[string]any
	if err := common.Unmarshal(raw, &thinking); err != nil {
		return nil
	}
	if !strings.EqualFold(common.Interface2String(thinking["type"]), "enabled") {
		return nil
	}

	normalized := map[string]any{"type": "enabled"}
	if strings.EqualFold(common.Interface2String(thinking["keep"]), "all") {
		normalized["keep"] = "all"
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return nil
	}
	return data
}

func normalizeKimiK27ToolChoice(toolChoice any) any {
	value, ok := toolChoice.(string)
	if !ok {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto":
		return "auto"
	case "none":
		return "none"
	default:
		return nil
	}
}
