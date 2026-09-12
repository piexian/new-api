package moonshot

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
		if info != nil {
			// 记录发给上游的生效档位，供消费日志输出与实际生效值。
			info.ReasoningEffort = request.ReasoningEffort
		}
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

// removeConflictingKimiSamplingParameters 丢弃与固定值不符的采样参数。
// Kimi 官方模型参数说明：K3/K2.7-code 的 temperature 固定 1.0、top_p 固定 0.95、
// n 固定 1、presence/frequency_penalty 固定 0，显式传其他值上游直接报错，
// 因此只保留与固定值一致的取值，其余一律省略而不是报错。
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

// normalizeKimiK3Request 按 K3 的参数面清洗请求。
func normalizeKimiK3Request(request *dto.GeneralOpenAIRequest) {
	// K3 恒为思考模型，没有 thinking 参数，思考深度只能通过顶层 reasoning_effort 调。
	request.THINKING = nil
	request.Reasoning = nil
	request.ReasoningEffort = normalizeKimiK3Effort(request.ReasoningEffort)
	if request.MaxCompletionTokens == nil {
		request.MaxCompletionTokens = request.MaxTokens
	}
	request.MaxTokens = nil
	// 上游对超过上限的 max_completion_tokens 直接报错，留空回落到模型默认。
	if request.MaxCompletionTokens != nil && (*request.MaxCompletionTokens == 0 || *request.MaxCompletionTokens > kimiK3MaxCompletionTokens) {
		request.MaxCompletionTokens = nil
	}
}

// normalizeKimiK3Effort 把入站档位收敛到 K3 支持的 low/high/max。
// 上游默认 max，所以未识别的取值留空交给上游决定，而不是把客户端显式要的低档位悄悄抬到 max。
// K3 无法关闭思考，none/minimal 只能退到最低的 low。
func normalizeKimiK3Effort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "low", "minimal", "none":
		return "low"
	case "medium", "high":
		return "high"
	case "xhigh", "max":
		return "max"
	default:
		return ""
	}
}

// normalizeKimiK27Request 按 K2.7-code 的参数面清洗请求。
// K2.7-code 思考恒开、没有思考深度档位，也不支持顶层 reasoning_effort，传了会报错。
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
