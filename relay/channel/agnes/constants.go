package agnes

const (
	ChannelName         = "agnes-ai"
	ModelText15Flash    = "agnes-1.5-flash"
	ModelText20Flash    = "agnes-2.0-flash"
	ModelText25Flash    = "agnes-2.5-flash"
	ModelText25Pro      = "agnes-2.5-pro"
	ModelText25ProAlpha = "agnes-2.5-pro-alpha"
	ModelText25ProBeta  = "agnes-2.5-pro-beta"
	ModelText30Flash    = "agnes-3.0-flash"
	ModelImage20Flash   = "agnes-image-2.0-flash"
	ModelImage21Flash   = "agnes-image-2.1-flash"
	ModelImage25Flash   = "agnes-image-2.5-flash"
	ModelVideoV20       = "agnes-video-v2.0"
	ModelVideo25Flash   = "agnes-video-2.5-flash"
)

var ModelList = []string{
	ModelText15Flash,
	ModelText20Flash,
	ModelText25Flash,
	ModelText25Pro,
	ModelText25ProAlpha,
	ModelText25ProBeta,
	ModelText30Flash,
	ModelImage20Flash,
	ModelImage21Flash,
	ModelImage25Flash,
	ModelVideoV20,
	ModelVideo25Flash,
}

// maxOutputTokensByModel 官方文档标注的各文本模型最大输出 token 数。
// agnes-1.5-flash 文档已下架、上限未知，故不收录（不注入默认值）。
var maxOutputTokensByModel = map[string]uint{
	ModelText20Flash:    65536,
	ModelText25Flash:    65536,
	ModelText25Pro:      65536,
	ModelText25ProAlpha: 65536,
	ModelText25ProBeta:  65536,
	ModelText30Flash:    65536,
}

// defaultMaxTokens 取模型最大输出的一半；未收录模型返回 0（不注入，保持上游默认）。
func defaultMaxTokens(model string) uint {
	if maxOutput, ok := maxOutputTokensByModel[model]; ok {
		return maxOutput / 2
	}
	return 0
}
