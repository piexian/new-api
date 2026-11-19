package opencode

import (
	"strings"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	dto "github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

// 上游 chat/responses 端点把 reasoning_effort 当固定枚举校验,取值集外直接 400:
// expected one of "max"|"xhigh"|"high"|"medium"|"low"|"minimal"|"none"(实测 2026-11)。
// 端点级取值集与模型级枚举都来自官方模型清单(/inference/v1/models 与
// /inference/go/v1/models 的 capabilities.reasoning.effort)。不在声明集内的取值
// 一律剥离交给上游默认档,不替客户端猜一个档位。
var openCodeReasoningEffortUnion = map[string]bool{
	"max": true, "xhigh": true, "high": true, "medium": true,
	"low": true, "minimal": true, "none": true,
}

// openCodeReasoningEffortAliases 收敛跨厂商的等价拼写到上游枚举内。
var openCodeReasoningEffortAliases = map[string]string{
	"ultra": "max",
}

// openCodeGoChatEfforts 是 go 清单里 chat-completions 模型声明的 effort 枚举。
var openCodeGoChatEfforts = map[string][]string{
	"deepseek-v4.1-flash": {"low", "high", "max"},
	"deepseek-v4-pro":     {"high", "max"},
	"glm-5.2":             {"high", "max"},
	"glm-5.3":             {"low", "high", "max"},
	"glm-5.3-flash":       {"low", "high", "max"},
	"hy3":                 {"none", "low", "high"},
	"hy4":                 {"none", "high"},
	"hy4-flash-free":      {"none", "high"},
	"hy4-preview":         {"none", "high"},
	"kimi-k3":             {"max"},
	"kimi-k3.1-pro":       {"low", "high", "max"},
	"minimax-m3.1-flash":  {"low", "medium", "high", "xhigh", "max"},
	"step-5-preview":      {"low", "medium", "high"},
	"beam-free":           {"low", "medium", "high", "xhigh", "max"},
}

// openCodeZenChatEfforts 是 zen 清单里 chat-completions 模型声明的 effort 枚举。
var openCodeZenChatEfforts = map[string][]string{
	"beam-free":                  {"low", "medium", "high", "xhigh", "max"},
	"deepseek-v4.1-flash":        {"low", "high", "max"},
	"deepseek-v4.1-flash-free":   {"low", "high", "max"},
	"deepseek-v4-pro":            {"high", "max"},
	"flint-40b-free":             {"low", "medium", "high", "xhigh", "max"},
	"glm-5.2":                    {"high", "max"},
	"glm-5.3":                    {"low", "high", "max"},
	"glm-5.3-flash":              {"low", "high", "max"},
	"hy4-flash-free":             {"none", "high"},
	"inkling-2-small-free":       {"none", "minimal", "low", "medium", "high", "max"},
	"kimi-k3":                    {"low", "high", "max"},
	"mistral-large-4":            {"none", "high"},
	"nemotron-3.5-super-vl-free": {"none", "minimal", "low", "medium", "high", "xhigh"},
	"qwen3.8-max":                {"low", "medium", "xhigh"},
	"tidepool-alpha-free":        {"low", "medium", "high", "xhigh", "max"},
}

func normalizeOpenCodeReasoningEffort(baseURL string, model string, effort string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	if normalized == "" {
		return "", false
	}
	if aliased, ok := openCodeReasoningEffortAliases[normalized]; ok {
		normalized = aliased
	}
	if !openCodeReasoningEffortUnion[normalized] {
		return "", false
	}

	model = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(model, "models/")))
	var declared []string
	if IsGoBase(baseURL) {
		declared = openCodeGoChatEfforts[model]
	} else {
		declared = openCodeZenChatEfforts[model]
	}
	// 模型未声明枚举(如 mimo/kimi-k2.7-code)时按端点级取值集放行。
	if len(declared) > 0 && !stringListContains(declared, normalized) {
		return "", false
	}
	return normalized, true
}

// normalizeOutboundReasoningEffort 在出站前收敛 chat/responses 的思考档位,
// 剥离上游会拒绝的取值并记录一次,交由模型默认档生效。
func (a *Adaptor) normalizeOutboundReasoningEffort(c *gin.Context, info *relaycommon.RelayInfo, request any) {
	if info == nil {
		return
	}
	switch value := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if value == nil || value.ReasoningEffort == "" {
			return
		}
		if normalized, ok := normalizeOpenCodeReasoningEffort(info.ChannelBaseUrl, info.UpstreamModelName, value.ReasoningEffort); ok {
			value.ReasoningEffort = normalized
		} else {
			if c != nil {
				logger.LogInfo(c, "opencode stripped unsupported reasoning_effort: "+value.ReasoningEffort+" (model: "+info.UpstreamModelName+")")
			}
			value.ReasoningEffort = ""
		}
	case *dto.OpenAIResponsesRequest:
		if value == nil || value.Reasoning == nil || value.Reasoning.Effort == "" {
			return
		}
		if normalized, ok := normalizeOpenCodeReasoningEffort(info.ChannelBaseUrl, info.UpstreamModelName, value.Reasoning.Effort); ok {
			value.Reasoning.Effort = normalized
		} else {
			if c != nil {
				logger.LogInfo(c, "opencode stripped unsupported reasoning.effort: "+value.Reasoning.Effort+" (model: "+info.UpstreamModelName+")")
			}
			value.Reasoning.Effort = ""
		}
	}
}
