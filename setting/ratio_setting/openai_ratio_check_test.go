package ratio_setting

import (
	"math"
	"testing"
)

// 验证 OpenAI 模型默认倍率与官方定价一致（https://developers.openai.com/api/docs/pricing）
// 约定：modelRatio = $/M input ÷ 2；实际输出价 = modelRatio × completionRatio × $2/M；
// 缓存读价 = modelRatio × cacheRatio × $2/M。价格取标准档、272K 内短上下文。
func TestOpenAIDefaultPricing(t *testing.T) {
	InitRatioSettings()
	cases := []struct {
		model       string
		wantInPerM  float64 // 官方输入价 $/M
		wantOutPerM float64 // 官方输出价 $/M
		wantCache   float64 // 官方缓存读价 $/M，无缓存价传 -1
	}{
		{"gpt-6-astra", 10, 50, 1},
		{"gpt-6-sol", 2, 10, 0.2},
		{"gpt-6.1-sol", 2, 10, 0.1},
		{"gpt-6.1-terra", 1, 5, 0.05},
		{"gpt-6-luna", 0.1, 0.5, 0.01},
		{"gpt-6-cyber", 20, 100, 2},
		{"gpt-5.6-sol", 4, 20, 0.4},
		{"gpt-5.6-terra", 2, 12, 0.2},
		{"gpt-5.6-luna", 0.2, 1.2, 0.02},
		{"gpt-5.6-cyber", 12.5, 75, 1.25},
		{"gpt-5.5", 5, 30, 0.5},
		{"gpt-5.5-pro", 30, 180, -1},
		{"gpt-5.4", 2.5, 15, 0.25},
		{"gpt-5.4-pro", 30, 180, -1},
		{"gpt-5.4-mini", 0.75, 4.5, 0.075},
		{"gpt-5.4-nano", 0.2, 1.25, 0.02},
		{"gpt-5.2", 1.75, 14, 0.175},
		{"gpt-5.2-pro", 21, 168, -1},
		{"gpt-5.1", 1.25, 10, 0.125},
		{"gpt-5-pro", 15, 120, -1},
		{"gpt-5.3-codex", 1.75, 14, 0.175},
		{"gpt-5-search-api", 1.25, 10, 0.125},
		{"chat-latest", 5, 30, 0.5},
		{"gpt-rosalind-research", 5, 25, 0.5},
	}
	for _, c := range cases {
		ratio, ok, _ := GetModelRatio(c.model)
		if !ok {
			t.Errorf("%s: missing builtin ratio", c.model)
			continue
		}
		inPerM := ratio * 2
		if math.Abs(inPerM-c.wantInPerM) > 1e-9 {
			t.Errorf("%s: input = $%v/M, want $%v/M", c.model, inPerM, c.wantInPerM)
		}
		outPerM := inPerM * GetCompletionRatio(c.model)
		if math.Abs(outPerM-c.wantOutPerM) > 1e-9 {
			t.Errorf("%s: output = $%v/M, want $%v/M", c.model, outPerM, c.wantOutPerM)
		}
		cacheRatio, _ := GetCacheRatio(c.model)
		if c.wantCache < 0 {
			if cacheRatio != 1 {
				t.Errorf("%s: cache ratio = %v, want default 1 (no cached price)", c.model, cacheRatio)
			}
			continue
		}
		cachePerM := inPerM * cacheRatio
		if math.Abs(cachePerM-c.wantCache) > 1e-9 {
			t.Errorf("%s: cached input = $%v/M, want $%v/M", c.model, cachePerM, c.wantCache)
		}
	}
}
