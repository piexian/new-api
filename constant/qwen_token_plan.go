package constant

const (
	QwenTokenPlanRootURL          = "https://token-plan.cn-beijing.maas.aliyuncs.com"
	QwenTokenPlanOpenAIBaseURL    = QwenTokenPlanRootURL + "/compatible-mode/v1"
	QwenTokenPlanAnthropicBaseURL = QwenTokenPlanRootURL + "/apps/anthropic"
)

var QwenTokenPlanModelList = []string{
	"qwen3.8-max-preview",
	"qwen3.7-max",
	"qwen3.7-plus",
	"qwen3.6-flash",
	"glm-5.2",
	"deepseek-v4-pro",
}
