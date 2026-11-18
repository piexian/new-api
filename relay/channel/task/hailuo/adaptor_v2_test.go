package hailuo

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestConvertToRequestPayloadV2(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt:   "a cat surfing",
		Duration: 8,
		Size:     "2K",
		Images:   []string{"https://example.com/first.png", "https://example.com/last.png"},
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "MiniMax-H3"}}

	videoRequest, err := (&TaskAdaptor{}).convertToRequestPayloadV2(&req, info)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if videoRequest.Resolution != Resolution2K || videoRequest.Duration != 8 {
		t.Fatalf("resolution/duration = %s/%d", videoRequest.Resolution, videoRequest.Duration)
	}
	if len(videoRequest.Content) != 3 {
		t.Fatalf("content = %d items, want text + 2 frames", len(videoRequest.Content))
	}
	if videoRequest.Content[0].Type != "text" || videoRequest.Content[0].Text != "a cat surfing" {
		t.Fatalf("text item = %+v", videoRequest.Content[0])
	}
	if videoRequest.Content[1].Role != "first_frame" || videoRequest.Content[2].Role != "last_frame" {
		t.Fatalf("frame roles = %s, %s", videoRequest.Content[1].Role, videoRequest.Content[2].Role)
	}
	if videoRequest.Ratio != "" {
		t.Fatalf("i2v ratio should stay empty, got %q", videoRequest.Ratio)
	}

	// 纯文生视频缺省 16:9
	t2 := relaycommon.TaskSubmitReq{Prompt: "a cat"}
	videoRequest2, err := (&TaskAdaptor{}).convertToRequestPayloadV2(&t2, info)
	if err != nil {
		t.Fatalf("convert t2v: %v", err)
	}
	if videoRequest2.Ratio != "16:9" {
		t.Fatalf("t2v default ratio = %q, want 16:9", videoRequest2.Ratio)
	}

	// reference 模式映射 reference_image
	t3 := relaycommon.TaskSubmitReq{Prompt: "make it move", Mode: "subject-reference", Images: []string{"https://example.com/ref.png"}}
	videoRequest3, err := (&TaskAdaptor{}).convertToRequestPayloadV2(&t3, info)
	if err != nil {
		t.Fatalf("convert ref: %v", err)
	}
	if videoRequest3.Content[1].Role != "reference_image" {
		t.Fatalf("reference role = %q", videoRequest3.Content[1].Role)
	}
}

func TestParseVideoTaskV2FromResponse(t *testing.T) {
	// task 包裹形态
	taskBody := []byte(`{"task":{"id":"42","model":"MiniMax-H3","status":"succeeded","content":{"url":"https://cdn.example.com/v.mp4"},"resolution":"2K","usage":{"total_seconds":5,"input_seconds":0,"output_seconds":5,"input_image_count":1}}}`)
	v2Task, ok := parseVideoTaskV2FromResponse(taskBody)
	if !ok || v2Task.ID != "42" || v2Task.Status != "succeeded" || v2Task.Usage.OutputSeconds != 5 {
		t.Fatalf("task envelope parse: ok=%v task=%+v", ok, v2Task)
	}

	// flat 形态
	flatBody := []byte(`{"id":"43","model":"MiniMax-H3-Lite","status":"failed","error":{"code":"1026","message":"sensitive"}}`)
	flatTask, ok := parseVideoTaskV2FromResponse(flatBody)
	if !ok || flatTask.ID != "43" || flatTask.Error.Message != "sensitive" {
		t.Fatalf("flat parse: ok=%v task=%+v", ok, flatTask)
	}

	// V1 响应恒带 base_resp,不得误判为 V2
	v1Body := []byte(`{"task_id":"t","status":"Success","file_id":7,"base_resp":{"status_code":0,"status_msg":""}}`)
	if _, ok := parseVideoTaskV2FromResponse(v1Body); ok {
		t.Fatal("v1 response misdetected as v2")
	}

	// V2 状态映射
	info := parseVideoTaskV2Info(v2Task)
	if info.Status == "" || info.Url != "https://cdn.example.com/v.mp4" || info.Progress != "100%" {
		t.Fatalf("v2 task info = %+v", info)
	}
}

func TestMiniMaxH3VideoRatio(t *testing.T) {
	cases := []struct {
		model        string
		resolution   string
		out, in, img int
		want         float64
	}{
		// H3 768P:5 秒输出 + 1 张免额内图片 → 5.0
		{"MiniMax-H3", Resolution768P, 5, 0, 1, 5},
		// H3 2K:输出 0.80/0.50 = 1.6 × 5 秒 → 8.0
		{"MiniMax-H3", Resolution2K, 5, 0, 0, 8},
		// Highspeed 480P:0.26/0.40 = 0.65 × 10 秒 → 6.5
		{"MiniMax-H3-Highspeed", Resolution480P, 10, 0, 0, 6.5},
		// Lite 768P 6 秒 + 超免额 2 张图片:6 + 2×0.5 → 7
		{"MiniMax-H3-Lite", Resolution768P, 6, 0, 7, 7},
		// Max 768P 5 秒输出 + 3 秒输入视频(0.97/0.50=1.94) + 1 张免额内:5 + 5.82 → 10.82
		{"MiniMax-H3-Max", Resolution768P, 5, 3, 1, 10.82},
		// Max 480P 4 秒输出(0.66) + 3 秒输入(0.74) → 4.86
		{"MiniMax-H3-Max", Resolution480P, 4, 3, 0, 4.86},
	}
	for _, c := range cases {
		got := miniMaxH3VideoRatio(c.model, c.resolution, c.out, c.in, c.img)
		if diff := got - c.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s %s: ratio = %v, want %v", c.model, c.resolution, got, c.want)
		}
	}
	if miniMaxH3VideoRatio("MiniMax-Hailuo-2.3", Resolution768P, 6, 0, 0) != 0 {
		t.Error("non-H3 model should return 0")
	}
}

func TestEstimateH3BillingReturnsDurationResolution(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Prompt: "a cat", Duration: 5}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "MiniMax-H3"}}

	estimated := (&TaskAdaptor{}).estimateH3Billing(&req, info)
	ratio, ok := estimated["duration_resolution"]
	if !ok || ratio != 5 {
		t.Fatalf("estimate = %v, want duration_resolution 5", estimated)
	}
}
