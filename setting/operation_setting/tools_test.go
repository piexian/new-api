package operation_setting

import "testing"

func TestGetToolPriceForModelCanonicalName(t *testing.T) {
	RebuildToolPriceIndex()
	t.Cleanup(func() { currentIndex.Store(nil) })

	cases := []struct {
		tool  string
		model string
		want  float64
	}{
		{"web_search", "", 10.0},
		{"web_search", "gpt-4o-2024-08-06", 25.0},
		{"web_search", "gpt-4.1-mini-x", 25.0},
		{"web_search", "claude-sonnet-4-5", 10.0},
		// OpenAI 旧版别名归一到 web_search
		{"web_search_preview", "", 10.0},
		{"web_search_preview", "gpt-4o-mini-2024-07-18", 25.0},
		{"web_search_preview_2025_03_11", "gpt-4.1-nano-x", 25.0},
		// Claude 带日期变体归一到 web_search
		{"web_search_20250305", "", 10.0},
		// 无默认价格配置的工具
		{"web_search_premium", "", 0},
		{"unknown_tool", "", 0},
	}
	for _, tc := range cases {
		if got := GetToolPriceForModel(tc.tool, tc.model); got != tc.want {
			t.Errorf("GetToolPriceForModel(%q, %q) = %v, want %v", tc.tool, tc.model, got, tc.want)
		}
	}
}
