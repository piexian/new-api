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
	case kimiModelK27:
		normalizeKimiK27Request(request)
	}
	return family
}

func classifyKimiModel(model string, kimiCodingBase bool) kimiModelFamily {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case model == "kimi-k3", strings.HasPrefix(model, "kimi-k3-"), kimiCodingBase && model == "k3":
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

func applyKimiCodingClaudeCompatibility(family kimiModelFamily, request *dto.ClaudeRequest) error {
	if request == nil {
		return nil
	}
	switch family {
	case kimiModelK3:
		outputConfig, err := common.Marshal(map[string]string{"effort": "max"})
		if err != nil {
			return err
		}
		request.Thinking = &dto.Thinking{Type: "adaptive"}
		request.OutputConfig = outputConfig
		request.Temperature = nil
		request.TopP = nil
		request.TopK = nil
	case kimiModelK27:
		ensureKimiK27ClaudeThinking(request)
		request.Temperature = nil
		request.TopP = nil
		request.TopK = nil
	}
	return nil
}

func ensureKimiK27ClaudeThinking(request *dto.ClaudeRequest) {
	if request.MaxTokens == nil || *request.MaxTokens < 1280 {
		minimum := uint(1280)
		request.MaxTokens = &minimum
	}
	budget := 4096
	if *request.MaxTokens <= uint(budget) {
		budget = int(*request.MaxTokens) - 256
	}
	if budget < 1024 {
		budget = 1024
	}
	request.Thinking = &dto.Thinking{Type: "enabled", BudgetTokens: &budget}
}
