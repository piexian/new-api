package constant

const (
	OpenCodeZenBaseURLAlias = "opencode-zen"
	OpenCodeGoBaseURLAlias  = "opencode-go"
	OpenCodeZenBaseURL      = "https://opencode.ai/zen"
	OpenCodeGoBaseURL       = "https://opencode.ai/zen/go"
)

var OpenCodeZenResponsesModels = []string{
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
	"grok-4.6",
	"grok-4.5",
	"grok-build-0.1",
	"muse-spark-1.2",
	"muse-spark-1.2-contributor-free",
}

var OpenCodeZenClaudeModels = []string{
	"claude-fable-5",
	"claude-opus-5",
	"claude-opus-4-8",
	"claude-opus-4-7",
	"claude-opus-4-6",
	"claude-opus-4-5",
	"claude-sonnet-5",
	"claude-sonnet-4-6",
	"claude-sonnet-4-5",
	"claude-haiku-4-5",
	"qwen3.7-max",
	"qwen3.7-plus",
	"qwen3.6-plus",
	"qwen3.5-plus",
}

var OpenCodeZenGeminiModels = []string{
	"gemini-3.7-flash",
	"gemini-3.6-flash",
	"gemini-3.5-flash",
	"gemini-3.5-flash-lite",
	"gemini-3.1-pro",
	"gemini-3-flash",
}

var OpenCodeZenChatModels = []string{
	"deepseek-v4-pro",
	"deepseek-v4-flash",
	"minimax-m3",
	"minimax-m2.7",
	"minimax-m2.5",
	"glm-5.2",
	"glm-5.1",
	"glm-5",
	"kimi-k2.5",
	"kimi-k2.6",
	"kimi-k2.7-code",
	"kimi-k3",
	"big-pickle",
	"x-preview-f-free",
	"mimo-v2.5-free",
	"hy3-free",
	"nemotron-3-ultra-free",
	"nemotron-3.5-lightning-free",
	"deepseek-v4-flash-free",
}

var OpenCodeGoResponsesModels = []string{
	"grok-4.5",
	"gpt-5.6-luna",
	"muse-spark-1.2-contributor",
}

var OpenCodeGoChatModels = []string{
	"glm-5.3",
	"glm-5.2",
	"glm-5.1",
	"kimi-k3",
	"kimi-k2.7-code",
	"kimi-k2.6",
	"deepseek-v4-pro",
	"deepseek-v4-flash",
	"mimo-v2.5",
	"mimo-v2.5-pro",
	"hy3",
}

var OpenCodeGoClaudeModels = []string{
	"minimax-m3",
	"minimax-m2.7",
	"minimax-m2.5",
	"qwen3.8-max",
	"qwen3.7-max",
	"qwen3.7-plus",
	"qwen3.6-plus",
}

var OpenCodeGoModels = uniqueStringList(
	OpenCodeGoResponsesModels,
	OpenCodeGoChatModels,
	OpenCodeGoClaudeModels,
)
var OpenCodeModelList = uniqueStringList(
	OpenCodeZenResponsesModels,
	OpenCodeZenClaudeModels,
	OpenCodeZenGeminiModels,
	OpenCodeZenChatModels,
	OpenCodeGoResponsesModels,
	OpenCodeGoChatModels,
	OpenCodeGoClaudeModels,
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
