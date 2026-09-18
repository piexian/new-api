package stepfun

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
)

func newStepFunGinContext(t *testing.T, method, path, contentType, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	return c, recorder
}

func TestConvertAudioRequestKeepsStepFunTTSFields(t *testing.T) {
	t.Parallel()

	body := `{"model":"stepaudio-2.5-tts","input":"你好","voice":"cixingnansheng",` +
		`"response_format":"mp3","speed":1.2,"volume":0.8,"text_normalization":"enhanced",` +
		`"voice_label":{"emotion":"高兴"},"instructions":"冷淡一点","sample_rate":16000,` +
		`"pronunciation_map":{"tone":["绯闻/fei1闻"]},"markdown_filter":true,` +
		`"return_url":true,"timestamp":true,"metadata":{"user":"x"},"task_type":"tts","language":"zh"}`

	c, _ := newStepFunGinContext(t, http.MethodPost, "/v1/audio/speech", "application/json", body)
	info := stepfunTestInfo("stepfun")
	info.RelayMode = relayconstant.RelayModeAudioSpeech

	request := dto.AudioRequest{
		Model:          "stepaudio-2.5-tts",
		Input:          "你好",
		Voice:          "cixingnansheng",
		ResponseFormat: "mp3",
		StreamFormat:   "audio",
	}
	reader, err := (&Adaptor{}).ConvertAudioRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertAudioRequest returned error: %v", err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read payload: %v", err)
	}

	var payload map[string]any
	if err := common.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	allowed := map[string]struct{}{}
	for _, key := range stepFunTTSAllowedFields {
		allowed[key] = struct{}{}
	}
	for key := range payload {
		if _, ok := allowed[key]; !ok {
			t.Fatalf("unexpected field forwarded to StepFun: %q (payload=%v)", key, payload)
		}
	}

	if payload["volume"] != 0.8 {
		t.Fatalf("volume not restored: %v", payload["volume"])
	}
	if payload["text_normalization"] != "enhanced" {
		t.Fatalf("text_normalization not restored: %v", payload["text_normalization"])
	}
	if payload["sample_rate"] != float64(16000) {
		t.Fatalf("sample_rate not restored: %v", payload["sample_rate"])
	}
	if payload["markdown_filter"] != true {
		t.Fatalf("markdown_filter not restored: %v", payload["markdown_filter"])
	}
	if payload["return_url"] != true || payload["timestamp"] != true {
		t.Fatalf("return_url/timestamp not restored: %v / %v", payload["return_url"], payload["timestamp"])
	}
	// OpenAI 的 instructions 映射为 StepFun 的 instruction
	if payload["instruction"] != "冷淡一点" {
		t.Fatalf("instructions should map to instruction, got %v", payload["instruction"])
	}
	if _, ok := payload["instructions"]; ok {
		t.Fatalf("plural instructions must not be forwarded")
	}
	if label, ok := payload["voice_label"].(map[string]any); !ok || label["emotion"] != "高兴" {
		t.Fatalf("voice_label not restored: %v", payload["voice_label"])
	}
	if pm, ok := payload["pronunciation_map"].(map[string]any); !ok || pm["tone"] == nil {
		t.Fatalf("pronunciation_map not restored: %v", payload["pronunciation_map"])
	}
	if payload["speed"] != 1.2 || payload["response_format"] != "mp3" || payload["input"] != "你好" {
		t.Fatalf("base fields changed: %v", payload)
	}
}

func TestConvertAudioRequestWithoutRawBody(t *testing.T) {
	t.Parallel()

	c, _ := newStepFunGinContext(t, http.MethodPost, "/v1/audio/speech", "", "")
	info := stepfunTestInfo("stepfun")
	info.RelayMode = relayconstant.RelayModeAudioSpeech

	request := dto.AudioRequest{Model: "stepaudio-2.5-tts", Input: "hi", Voice: "v", ResponseFormat: "mp3"}
	reader, err := (&Adaptor{}).ConvertAudioRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertAudioRequest returned error: %v", err)
	}
	raw, _ := io.ReadAll(reader)
	var payload map[string]any
	if err := common.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["model"] != "stepaudio-2.5-tts" || payload["input"] != "hi" {
		t.Fatalf("base payload missing: %v", payload)
	}
}

func TestDoResponsePassesThroughTTSJSON(t *testing.T) {
	t.Parallel()

	c, recorder := newStepFunGinContext(t, http.MethodPost, "/v1/audio/speech", "application/json", "{}")
	info := stepfunTestInfo("stepfun")
	info.RelayMode = relayconstant.RelayModeAudioSpeech

	upstreamBody := `{"created":1782963404,"data":{"url":"https://example.com/a.mp3","subtitles":[{"text":"你好"}]}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}

	usageAny, apiErr := (&Adaptor{}).DoResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("DoResponse returned error: %v", apiErr)
	}
	usage, ok := usageAny.(*dto.Usage)
	if !ok || usage == nil {
		t.Fatalf("expected usage, got %T", usageAny)
	}
	if usage.TotalTokens != usage.PromptTokens {
		t.Fatalf("usage should mirror prompt estimate, got %+v", usage)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, "a.mp3") || !strings.Contains(body, "subtitles") {
		t.Fatalf("json response must be forwarded verbatim, got %s", body)
	}
	if ct := recorder.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content type should stay json, got %q", ct)
	}
}

func TestConvertAudioRequestDelegatesTranscription(t *testing.T) {
	t.Parallel()

	c, _ := newStepFunGinContext(t, http.MethodPost, "/v1/audio/transcriptions", "multipart/form-data; boundary=x", "")
	info := stepfunTestInfo("stepfun")
	info.RelayMode = relayconstant.RelayModeAudioTranscription

	request := dto.AudioRequest{Model: "stepaudio-2.5-asr", ResponseFormat: "json"}
	adaptor := &Adaptor{}
	// 通用实现会先记录 response_format（供 STT 响应解析使用），再解析 multipart；
	// 这里的空表单必然解析失败，用于锁定「转写仍走通用实现」这一契约。
	_, _ = adaptor.ConvertAudioRequest(c, info, request)
	if adaptor.ResponseFormat != "json" {
		t.Fatalf("expected delegated transcription conversion to keep response_format, got %q", adaptor.ResponseFormat)
	}
}
