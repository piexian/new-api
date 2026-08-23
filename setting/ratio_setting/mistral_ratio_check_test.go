package ratio_setting

import (
	"math"
	"testing"
)

// 验证 Mistral 模型默认倍率与官方定价一致（https://docs.mistral.ai/inference/pricing）
// 约定：modelRatio = $/M input ÷ 2；实际输出价 = modelRatio × completionRatio × $2/M
func TestMistralDefaultPricing(t *testing.T) {
	InitRatioSettings()
	cases := []struct {
		model       string
		wantInPerM  float64 // 官方输入价 $/M（或等效价）
		wantOutPerM float64 // 官方输出价 $/M（或等效价）
	}{
		{"mistral-large-2512", 0.5, 1.5},
		{"mistral-large-latest", 0.5, 1.5},
		{"mistral-medium-3-5", 1.5, 7.5},
		{"mistral-medium-3", 1.5, 7.5},
		{"mistral-medium-latest", 1.5, 7.5},
		{"mistral-small-2603", 0.15, 0.6},
		{"mistral-small-latest", 0.15, 0.6},
		{"ministral-14b-2512", 0.2, 0.2},
		{"ministral-14b-latest", 0.2, 0.2},
		{"ministral-8b-2512", 0.15, 0.15},
		{"ministral-8b-latest", 0.15, 0.15},
		{"ministral-3b-2512", 0.1, 0.1},
		{"ministral-3b-latest", 0.1, 0.1},
		{"codestral-2508", 0.3, 0.9},
		{"codestral-latest", 0.3, 0.9},
		{"devstral-2512", 0.4, 2.0},
		{"devstral-latest", 0.4, 2.0},
		{"devstral-medium-latest", 0.4, 2.0},
		{"devstral-small-2507", 0.1, 0.3},
		{"devstral-small-latest", 0.1, 0.3},
		{"magistral-medium-2509", 2.0, 5.0},
		{"magistral-medium-latest", 2.0, 5.0},
		{"magistral-small-2509", 0.5, 1.5},
		{"magistral-small-latest", 0.5, 1.5},
		{"zai-glm-5-2", 1.4, 4.4},
		// 旧世代
		{"mistral-large-2411", 2.0, 6.0},
		{"mistral-medium-2508", 0.4, 2.0},
		{"mistral-small-2506", 0.1, 0.3},
		{"ministral-8b-2410", 0.1, 0.1},
		{"ministral-3b-2410", 0.04, 0.04},
		{"open-mistral-nemo", 0.15, 0.15},
		{"pixtral-large-2411", 2.0, 6.0},
		{"pixtral-large-latest", 2.0, 6.0},
		{"pixtral-12b-2409", 0.15, 0.15},
		// embedding（仅输入）
		{"codestral-embed-2505", 0.15, 0.15},
		{"codestral-embed", 0.15, 0.15},
		{"mistral-embed-2312", 0.1, 0},
		{"mistral-embed", 0.1, 0},
		// free
		{"mistral-moderation-2603", 0, 0},
		{"labs-leanstral-2603", 0, 0},
		// OCR: 1 页 = 1000 prompt tokens，官方 $x/1000 pages → 等效 $x/M
		{"mistral-ocr-4-1", 4.0, 0},
		{"mistral-ocr-latest", 4.0, 0},
		{"mistral-ocr-4-0", 4.0, 0},
		{"mistral-ocr-2512", 2.0, 0},
		{"mistral-ocr-2505", 1.0, 0},
		// STT: 1 分钟 = 1000 tokens，官方 $x/min → 等效 $(x*1000)/M
		{"voxtral-mini-2602", 3.0, 0},
		{"voxtral-mini-latest", 3.0, 0},
		{"voxtral-mini-2507", 2.0, 0},
		{"voxtral-mini-transcribe-realtime-2602", 6.0, 0},
		{"voxtral-mini-transcribe-realtime-latest", 6.0, 0},
		{"voxtral-small-2507", 4.0, 0},
		{"voxtral-small-latest", 4.0, 0},
		// TTS: 输出按分钟折算，目标输出侧 $15/M（官方 $16/M chars 的近似）
		{"voxtral-mini-tts-2603", 1.5, 15.0},
		{"voxtral-mini-tts-latest", 1.5, 15.0},
	}
	const epsilon = 1e-6
	for _, tc := range cases {
		ratio, ok, _ := GetModelRatio(tc.model)
		if !ok {
			t.Errorf("%s: model ratio not found", tc.model)
			continue
		}
		gotIn := ratio * 2
		if math.Abs(gotIn-tc.wantInPerM) > epsilon {
			t.Errorf("%s: input price = $%v/M, want $%v/M", tc.model, gotIn, tc.wantInPerM)
		}
		if tc.wantOutPerM > 0 {
			cr := GetCompletionRatio(tc.model)
			gotOut := ratio * cr * 2
			if math.Abs(gotOut-tc.wantOutPerM) > 0.01 {
				t.Errorf("%s: output price = $%v/M (completionRatio=%v), want $%v/M", tc.model, gotOut, cr, tc.wantOutPerM)
			}
		}
	}
}

func TestMistralCacheRatio(t *testing.T) {
	InitRatioSettings()
	for _, m := range []string{
		"mistral-large-2512", "mistral-large-latest",
		"mistral-medium-3-5", "mistral-medium-latest",
		"mistral-small-2603", "mistral-small-latest",
		"ministral-14b-2512", "ministral-8b-2512", "ministral-3b-2512",
		"codestral-2508", "codestral-latest", "zai-glm-5-2",
	} {
		r, ok := GetCacheRatio(m)
		if !ok || r != 0.1 {
			t.Errorf("%s: cache ratio = %v, %v; want 0.1, true", m, r, ok)
		}
	}
}
