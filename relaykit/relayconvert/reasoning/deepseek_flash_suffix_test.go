package reasoning

import (
	"testing"
)

func TestParseDeepSeekV4ThinkingSuffixFlash(t *testing.T) {
	// 旗舰名 deepseek-flash 接受思考后缀
	base, thinkingType, effort, ok := ParseDeepSeekV4ThinkingSuffix("deepseek-flash-none")
	if !ok || base != "deepseek-flash" || thinkingType != "disabled" || effort != "" {
		t.Fatalf("flash-none = %q %q %q %v", base, thinkingType, effort, ok)
	}
	base, thinkingType, effort, ok = ParseDeepSeekV4ThinkingSuffix("deepseek-flash-max")
	if !ok || base != "deepseek-flash" || thinkingType != "enabled" || effort != "max" {
		t.Fatalf("flash-max = %q %q %q %v", base, thinkingType, effort, ok)
	}
	// 裸名不带后缀不命中
	if _, _, _, ok := ParseDeepSeekV4ThinkingSuffix("deepseek-flash"); ok {
		t.Fatal("bare deepseek-flash should not match")
	}
	// v4 系行为不变
	base, thinkingType, _, ok = ParseDeepSeekV4ThinkingSuffix("deepseek-v4-pro-none")
	if !ok || base != "deepseek-v4-pro" || thinkingType != "disabled" {
		t.Fatalf("v4-pro-none = %q %q %v", base, thinkingType, ok)
	}
	// 非本族模型不误伤
	if _, _, _, ok := ParseDeepSeekV4ThinkingSuffix("deepseek-chat-max"); ok {
		t.Fatal("deepseek-chat-max should not match")
	}
}
