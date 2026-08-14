package mistralconsole

var ModelList = []string{
	"glm-5-2",
}

const (
	ChannelName                 = "mistral-console"
	conversationsURL            = "/api-ui/bora/v1/conversations"
	defaultBoraMaxTokens   uint = 8192
	boraMaxReasoningEffort      = "high"
)
