package common

import "strings"

// CanonicalBuildInToolName 归一内置工具类型名到官方名称：OpenAI 旧版 web_search_preview
// （含 web_search_preview_2025_03_11 等带日期变体）与 Claude 的 web_search_20250305
// 等带日期变体统一归一到 web_search；web_search_premium 与其余类型保持不变。
// 与 dto.BuildInToolWebSearch* 常量保持一致；common 不 import dto 以避免循环依赖。
func CanonicalBuildInToolName(name string) string {
	switch name {
	case "web_search", "web_search_premium":
		return name
	case "web_search_preview":
		return "web_search"
	}
	if strings.HasPrefix(name, "web_search_preview_") {
		return "web_search"
	}
	// premium 已在上面精确返回，此处覆盖 web_search_20250305 等 Claude 带日期变体
	if strings.HasPrefix(name, "web_search_") {
		return "web_search"
	}
	return name
}
