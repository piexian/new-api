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

	// MCP 媒体工具路径(MiniMax Code 订阅的语音/图片/音乐生成面)
	MCPPrefix     = "/mavis/api/v1/mcp"
	MCPSpeechPath = MCPPrefix + "/synthesize_speech"
	MCPMusicPath  = MCPPrefix + "/batch_text_to_music"
	MCPImagePath  = MCPPrefix + "/image_synthesize"
)

var baseModelList = []string{
	"MiniMax-M3",
	"MiniMax-M3.1-Flash",
	// 旧名称已下线,上游自动路由到 MiniMax-M3.1-Flash
	"MiniMax-M3.1-Flash-Preview",
	// 订阅附带的语音/图片/音乐模型(经 MCP 工具接口)
	"speech-2.8-hd",
	"MiniMax-H3-Image",
	"music-3.0",
}

var ModelList = append([]string{}, baseModelList...)
