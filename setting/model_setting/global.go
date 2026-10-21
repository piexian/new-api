package model_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type GlobalSettings struct {
	PassThroughRequestEnabled bool     `json:"pass_through_request_enabled"`
	ThinkingModelBlacklist    []string `json:"thinking_model_blacklist"`
	// EffortTailModelIDs 列出名称本身以 effort 词结尾的真实模型 ID，转换器不再追加/剥离 effort 后缀。
	EffortTailModelIDs []string `json:"effort_tail_model_ids"`
}

// 默认配置
var defaultOpenaiSettings = GlobalSettings{
	PassThroughRequestEnabled: false,
	ThinkingModelBlacklist: []string{
		"moonshotai/kimi-k2-thinking",
		"kimi-k2-thinking",
	},
	EffortTailModelIDs: []string{
		"gpt-5.1-codex-max",
		"qwen-image-edit-max",
		"qwen-max",
		"stable-diffusion-3-medium",
	},
}

// 全局实例
var globalSettings = defaultOpenaiSettings

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("global", &globalSettings)
}

func GetGlobalSettings() *GlobalSettings {
	return &globalSettings
}

// ShouldPreserveThinkingSuffix 判断模型是否配置为保留 thinking/-nothinking/-low/-high/-medium 后缀
func ShouldPreserveThinkingSuffix(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}

	for _, entry := range globalSettings.ThinkingModelBlacklist {
		if strings.TrimSpace(entry) == target {
			return true
		}
	}
	return false
}

// ShouldPreserveEffortTail 判断 modelName 是否为名称本身以 effort 词结尾的真实模型 ID。
func ShouldPreserveEffortTail(modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}
	bare := target
	if slash := strings.LastIndex(target, "/"); slash >= 0 {
		bare = target[slash+1:]
	}

	for _, entry := range globalSettings.EffortTailModelIDs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == target || entry == bare {
			return true
		}
	}
	return false
}
