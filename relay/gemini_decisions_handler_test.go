package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestHasBoxQuestion(t *testing.T) {
	cases := []struct {
		name      string
		questions string
		want      bool
	}{
		{"empty", ``, false},
		{"noul only", `{"ok":{"type":"noul","instructions":"hi"}}`, false},
		{"choice only", `{"route":{"type":"choice","instructions":"pick","criteria":{"a":"1"}}}`, false},
		{"box present", `{"where":{"type":"box","instructions":"find it"}}`, true},
		{"box mixed", `{"ok":{"type":"noul"},"where":{"type":"Box"}}`, true},
		{"invalid json", `{not json`, false},
	}
	for _, c := range cases {
		if got := hasBoxQuestion([]byte(c.questions)); got != c.want {
			t.Errorf("%s: hasBoxQuestion = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestBuildDecisionsInputFromState(t *testing.T) {
	// 纯字符串 state 无资产:原样透传
	input, err := buildDecisionsInputFromState([]byte(`"just a text state"`), nil)
	if err != nil {
		t.Fatalf("string state: %v", err)
	}
	if string(input) != `"just a text state"` {
		t.Fatalf("string state passthrough = %s", input)
	}

	// 整值引用:state 即引用串,转换为单图片内容块
	assets := []byte(`{"ref-photo":{"type":"image","format":"png","data_base64":"aGk="}}`)
	input, err = buildDecisionsInputFromState([]byte(`"ref-photo"`), assets)
	if err != nil {
		t.Fatalf("whole-ref state: %v", err)
	}
	var blocks []map[string]any
	if err := common.Unmarshal(input, &blocks); err != nil {
		t.Fatalf("whole-ref decode: %v", err)
	}
	if len(blocks) != 1 || blocks[0]["type"] != "image" || blocks[0]["mime_type"] != "image/png" || blocks[0]["data"] != "aGk=" {
		t.Fatalf("whole-ref image block = %v", blocks[0])
	}

	// 结构化 state:文本块 + 每资产[标签块,图片块],引用顺序稳定
	structured := []byte(`{"ticket":{"subject":"Duplicate charge"}}`)
	twoAssets := []byte(`{"b-second":{"type":"image","format":"jpg","data_base64":"Yg=="},"a-first":{"type":"image","format":"jpeg","data_base64":"YT09"}}`)
	input, err = buildDecisionsInputFromState(structured, twoAssets)
	if err != nil {
		t.Fatalf("structured state: %v", err)
	}
	if err := common.Unmarshal(input, &blocks); err != nil {
		t.Fatalf("structured decode: %v", err)
	}
	if len(blocks) != 5 {
		t.Fatalf("structured blocks = %d, want 5", len(blocks))
	}
	if blocks[0]["type"] != "text" || blocks[0]["text"] != string(structured) {
		t.Fatalf("state text block = %v", blocks[0])
	}
	if blocks[1]["text"] != "a-first:" || blocks[2]["mime_type"] != "image/jpeg" {
		t.Fatalf("first labeled image = %v %v", blocks[1], blocks[2])
	}
	if blocks[3]["text"] != "b-second:" || blocks[4]["mime_type"] != "image/jpeg" {
		t.Fatalf("second labeled image = %v %v", blocks[3], blocks[4])
	}

	// 不支持的图片格式与 multipart part 形式:明确报错
	if _, err := buildDecisionsInputFromState(structured, []byte(`{"x":{"type":"image","format":"webp","data_base64":"Zm9v"}}`)); err == nil {
		t.Fatal("webp format should be rejected")
	}
	if _, err := buildDecisionsInputFromState(structured, []byte(`{"x":{"type":"image","format":"png","part":"file-1"}}`)); err == nil {
		t.Fatal("multipart part asset should be rejected")
	}
}
