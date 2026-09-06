package groq

// ModelList 为渠道"填入所有模型"按钮的预置清单, 来源 console.groq.com/docs/models
var ModelList = []string{
	"llama-3.1-8b-instant",
	"llama-3.3-70b-versatile",
	"openai/gpt-oss-120b",
	"openai/gpt-oss-20b",
	"groq/compound",
	"groq/compound-mini",
	"qwen/qwen3.6-27b",
	"qwen/qwen3.8-27b",
	"whisper-large-v3",
	"whisper-large-v3-turbo",
	"playai-tts",
}

var ChannelName = "groq"

// 上游对超过 max_completion_tokens 的请求直接 400, 未收录模型不钳制
var maxCompletionTokensByModel = map[string]uint{
	"llama-3.1-8b-instant":         131072,
	"llama-3.3-70b-versatile":      32768,
	"openai/gpt-oss-120b":          65536,
	"openai/gpt-oss-20b":           65536,
	"openai/gpt-oss-safeguard-20b": 65536,
	"groq/compound":                8192,
	"groq/compound-mini":           8192,
	"qwen/qwen3.6-27b":             16384,
	"qwen/qwen3.8-27b":             16384,
	"minimaxai/minimax-m2.7":       131072,
}
