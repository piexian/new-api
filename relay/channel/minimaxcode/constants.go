package minimaxcode

const (
	ChannelName = "minimaxcode"

	// 生产域名：国内 cn，海外 global。
	RegionCN     = "cn"
	RegionGlobal = "global"

	DefaultBaseURL = "https://agent.minimax.cn"
	GlobalBaseURL  = "https://agent.minimax.io"
	LLMPathPrefix  = "/mavis/api/v1/llm/v1"
	MessagesPath   = LLMPathPrefix + "/messages"
	UserAgent      = "MiniMaxAgent"
	// 官方客户端未配置 agent_id 时固定发送 main。
	AgentID = "main"
)

var baseModelList = []string{
	"MiniMax-M3",
	"MiniMax-M3.1-Flash",
	"MiniMax-M3.1-Flash-Preview",
}

var ModelList = append([]string{}, baseModelList...)
