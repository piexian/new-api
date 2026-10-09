package mistralconsole

// ModelList 为 Mistral Console（Bora）当前可用的模型目录。
var ModelList = []string{
	"codestral-latest",
	"ministral-14b-latest",
	"ministral-3b-latest",
	"ministral-8b-latest",
	"mistral-medium-latest",
	"mistral-small-latest",
	"mistral-large-4",
	"labs-leanstral-1.5",
}

const (
	ChannelName           = "mistral-console"
	boraSessionCookieName = "ory_session_coolcurranf83m3srkfl"
	conversationsURL      = "/api-ui/bora/v1/conversations"

	// 客户端未指定 max_tokens 时的默认输出预算。
	defaultBoraMaxTokens uint = 8192
	// bora 输出预算的上限：思考链与正文共用 max_tokens，压缩客户端显式请求会让
	// 批量任务的正文被思考挤空，因此仅在超过该上限时兜底。
	maxBoraMaxTokens uint = 32768

	// bora 的 reasoning_effort 枚举只有 none/high。
	boraMaxReasoningEffort = "high"
	boraNoReasoningEffort  = "none"
)
