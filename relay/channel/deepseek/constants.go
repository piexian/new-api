package deepseek

var ModelList = []string{
	// 官方旗舰名:deepseek-flash = DeepSeek-V4.1-Flash-1017(1M ctx/384K max out/视觉/FIM 限非思考)
	"deepseek-flash", "deepseek-flash-none", "deepseek-flash-max",
	// 退役旧名仍兼容,上游路由至 V4.1-Flash-1017 并按 Flash 价计费
	"deepseek-v4-flash", "deepseek-v4-flash-none", "deepseek-v4-flash-max",
	"deepseek-v4-flash-vision-exp",
	"deepseek-v4-pro", "deepseek-v4-pro-none", "deepseek-v4-pro-max",
	// V3 系旧名仍接受,实测路由至 deepseek-flash(思考模式)
	"deepseek-chat", "deepseek-reasoner",
	// 工具 API(2026-11-13 全家桶):/v1/tools/*
	"deepseek-tts", "deepseek-asr", "deepseek-search", "deepseek-fetch",
}

var ChannelName = "deepseek"
