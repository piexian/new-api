package constant

const (
	OpenCodeZenBaseURLAlias = "opencode-zen"
	OpenCodeGoBaseURLAlias  = "opencode-go"
	OpenCodeZenBaseURL      = "https://opencode.ai/inference"
	OpenCodeGoBaseURL       = "https://opencode.ai/inference/go"
	// OpenCodeZenLegacyBaseURL is the console origin that still serves the
	// anonymous models listing after chat traffic moved to /inference.
	OpenCodeZenLegacyBaseURL = "https://opencode.ai/zen"
)

var OpenCodeZenResponsesModels = []string{
	"gpt-6-astra",
	"gpt-6.1-sol",
	"gpt-6-sol",
	"gpt-6-luna",
	"gpt-5.6-sol",
	"gpt-5.6-terra",
	"gpt-5.6-luna",
	"gpt-5.5",
	"gpt-5.5-pro",
	"gpt-5.4",
	"gpt-5.4-pro",
	"gpt-5.4-mini",
	"gpt-5.4-nano",
	"gpt-5.3-codex",
	"gpt-5.3-codex-spark",
	"gpt-5.2",
	"gpt-5.2-codex",
	"gpt-5.1",
	"gpt-5.1-codex",
	"gpt-5.1-codex-max",
	"gpt-5.1-codex-mini",
	"gpt-5",
	"gpt-5-codex",
	"gpt-5-nano",
	"grok-4.7",
	"grok-4.6",
	"grok-4.5",
	"grok-build-0.1",
	"muse-wonder-1.0",
	"muse-spark-1.3",
	"muse-spark-1.2",
	"muse-spark-1.2-contributor-free",
	"muse-spark-1.3-contributor-free",
	"gpt-6.2-sol",
	"gpt-6.2-astra",
	"muse-ember-1.0",
}

var OpenCodeZenClaudeModels = []string{
	"claude-fable-5-1",
	"claude-fable-5",
	"claude-opus-5-5-backup",
	"claude-opus-5-5",
	"claude-opus-5",
	"claude-opus-4-8",
	"claude-opus-4-7",
	"claude-opus-4-6",
	"claude-opus-4-5",
	"claude-sonnet-5-5",
	"claude-sonnet-5",
	"claude-sonnet-4-6",
	"claude-sonnet-4-5",
	"claude-haiku-5-5",
	"claude-haiku-4-5",
	"qwen3.8-flash",
	"qwen3.6-plus",
	"qwen3.5-plus",
	"claude-fable-5-2",
}
var OpenCodeZenGeminiModels = []string{
	"gemini-4-argon",
	"gemini-3.8-flash",
	"gemini-3.7-flash",
	"gemini-3.6-flash",
	"gemini-3.5-flash",
	"gemini-3.5-flash-lite",
	"gemini-3.1-pro",
	"gemini-3-flash",
	"gemini-4-carbon",
	"gemini-4-hydrogen",
	"gemini-4-neon",
}

var OpenCodeZenChatModels = []string{
	"deepseek-v4.1-flash-free",
	"deepseek-v4.1-flash",
	"deepseek-v4-pro",
	"deepseek-v4-flash",
	"mistral-large-4",
	"minimax-m3",
	"minimax-m2.7",
	"minimax-m2.5",
	"glm-5.3-flash",
	"glm-5.3",
	"glm-5.2",
	"glm-5.1",
	"glm-5",
	"kimi-k2.5",
	"kimi-k2.6",
	"kimi-k2.7-code",
	"kimi-k3",
	"qwen3.8-max",
	"big-pickle",
	"space-bunny-free",
	"cat-jam-free",
	"longcat-2.5-preview-free",
	"mimo-v2.6-flash-free",
	"nemotron-3.5-lightning-free",
	"nemotron-3.5-super-vl-free",
	"glm-5.3-flash-free",
	"ll-alpha-free",
	// 2026-10~11 新增(zen /v1/models)
	"beam-free",
	"hy4-flash-free",
	"tidepool-alpha-free",
	"inkling-2-small-free",
	"flint-40b-free",
	"iceman-free",
	"longcat-2.5-free",
}

var OpenCodeZenSystemOneModels = []string{
	"jev-1.13",
	"jev-1.13-free",
}

var OpenCodeGoResponsesModels = []string{
	"grok-4.7",
	"grok-4.6",
	"gpt-6-luna",
	"gpt-5.6-luna",
	"muse-spark-1.3-contributor",
	"muse-spark-1.2-contributor",
}

var OpenCodeGoChatModels = []string{
	"glm-5.3-flash",
	"glm-5.3",
	"glm-5.2",
	"glm-5.1",
	"kimi-k3",
	"kimi-k2.7-code",
	"kimi-k2.6",
	"longcat-2.0",
	"longcat-2.5-preview-free",
	"deepseek-v4.1-flash",
	"deepseek-v4-pro",
	"mimo-v2.6-flash",
	"mimo-v2.6-pro",
	"mimo-v2.5",
	"mimo-v2.5-pro",
	"step-5-preview-free",
	"step-5-preview",
	"beam-free",
	"minimax-m3.1-flash-free",
	"hy4-preview",
	"hy3",
	"cat-jam-free",
	"omen-alpha",
	"space-bunny",
	// 2026-10~11 新增(go /v1/models 与控制台清单)
	"kimi-k3.1-pro",
	"hy4",
	"minimax-m3.1-flash",
	"longcat-2.5-free",
	"iceman-contributor-free",
}

// Go accepts this model on Chat, but does not list it in its public /v1/models inventory.
var OpenCodeGoChatRouteOnlyModels = []string{
	"space-bunny-free",
	// 控制台在售但未列入 go /v1/models(2026-11-16 上架)
	"hy4-flash-free",
}

var OpenCodeGoClaudeModels = []string{
	"claude-haiku-5-5",
	"minimax-m3",
	"minimax-m2.7",
	"minimax-m2.5",
	"qwen3.8-flash",
	"qwen3.8-max",
	"qwen3.7-max",
	"qwen3.7-plus",
	"qwen3.6-plus",
	"qwen4-max",
}
var OpenCodeGoGeminiModels = []string{
	"gemini-4-argon-contributor",
	"gemini-4-argon",
}
var OpenCodeGoModels = uniqueStringList(
	OpenCodeGoResponsesModels,
	OpenCodeGoChatModels,
	OpenCodeGoClaudeModels,
	OpenCodeGoGeminiModels,
)
var OpenCodeModelList = uniqueStringList(
	OpenCodeZenResponsesModels,
	OpenCodeZenClaudeModels,
	OpenCodeZenGeminiModels,
	OpenCodeZenChatModels,
	OpenCodeZenSystemOneModels,
	OpenCodeGoResponsesModels,
	OpenCodeGoChatModels,
	OpenCodeGoClaudeModels,
	OpenCodeGoGeminiModels,
)

func uniqueStringList(lists ...[]string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, list := range lists {
		for _, item := range list {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
