package deepseek

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

func TestDeepSeekToolsUpstreamPath(t *testing.T) {
	if path, err := DeepSeekToolsUpstreamPath("/v1/tools/search", "deepseek-search"); err != nil || path != "/v1/tools/search" {
		t.Fatalf("search: %v %v", path, err)
	}
	if path, err := DeepSeekToolsUpstreamPath("/v1/tools/fetch", "deepseek-fetch"); err != nil || path != "/v1/tools/fetch" {
		t.Fatalf("fetch: %v %v", path, err)
	}
	if _, err := DeepSeekToolsUpstreamPath("/v1/tools/search", "deepseek-fetch"); err == nil {
		t.Fatal("search with fetch model should be rejected")
	}
	if _, err := DeepSeekToolsUpstreamPath("/v1/tools/fetch", "deepseek-search"); err == nil {
		t.Fatal("fetch with search model should be rejected")
	}
}

func TestGetRequestURLToolsAndAudio(t *testing.T) {
	cases := []struct {
		mode   int
		model  string
		path   string
		suffix string
	}{
		{relayconstant.RelayModeDeepSeekTools, "deepseek-search", "/v1/tools/search", "/v1/tools/search"},
		{relayconstant.RelayModeDeepSeekTools, "deepseek-fetch", "/v1/tools/fetch", "/v1/tools/fetch"},
		{relayconstant.RelayModeAudioSpeech, "deepseek-tts", "", "/v1/tools/tts"},
		{relayconstant.RelayModeAudioTranscription, "deepseek-asr", "", "/v1/tools/asr"},
	}
	for _, c := range cases {
		info := &relaycommon.RelayInfo{
			RelayMode:      c.mode,
			RequestURLPath: c.path,
			ChannelMeta:    &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.deepseek.com", UpstreamModelName: c.model},
		}
		url, err := (&Adaptor{}).GetRequestURL(info)
		if err != nil {
			t.Fatalf("GetRequestURL(%s): %v", c.model, err)
		}
		if !strings.HasSuffix(url, c.suffix) {
			t.Errorf("model %s: url = %s, want suffix %s", c.model, url, c.suffix)
		}
	}
}

func TestConvertDeepSeekAudioRequestTTS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-tts"},
	}

	reader, err := convertDeepSeekAudioRequest(c, info, dto.AudioRequest{Input: "你好世界", Voice: "tide", ResponseFormat: "pcm"})
	if err != nil {
		t.Fatalf("convert tts: %v", err)
	}
	body, _ := io.ReadAll(reader)
	var ttsReq deepSeekTTSRequest
	if err := common.Unmarshal(body, &ttsReq); err != nil {
		t.Fatalf("parse tts body: %v", err)
	}
	if ttsReq.Text != "你好世界" || ttsReq.VoiceID != "tide" || ttsReq.Format != "pcm" {
		t.Fatalf("tts request = %+v", ttsReq)
	}

	// mp3 等不支持格式回落 wav;voice 缺省 stella
	reader, _ = convertDeepSeekAudioRequest(c, info, dto.AudioRequest{Input: "hi", ResponseFormat: "mp3"})
	body, _ = io.ReadAll(reader)
	var fallbackReq deepSeekTTSRequest
	_ = common.Unmarshal(body, &fallbackReq)
	if fallbackReq.Format != "wav" || fallbackReq.VoiceID != deepSeekTTSDefaultVoice {
		t.Fatalf("fallback request = %+v", fallbackReq)
	}

	// 非 tts 模型拒绝
	badInfo := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeAudioSpeech, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-chat"}}
	if _, err := convertDeepSeekAudioRequest(c, badInfo, dto.AudioRequest{Input: "hi"}); err == nil {
		t.Fatal("non-tts model should be rejected")
	}
}

func TestDoDeepSeekASRResponseBillsBySeconds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeAudioTranscription,
		StartTime:   time.Unix(1700000000, 0),
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-asr"},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"r","text":"hello world","duration_ms":3200}`)),
	}

	usage, apiErr := doDeepSeekASRResponse(c, resp, info)
	if apiErr != nil {
		t.Fatalf("asr handler error: %v", apiErr)
	}
	// 3.2 秒向上取整为 4 个计费 token(1 秒=1 token,$0.116/小时)
	if usage.PromptTokens != 4 || usage.TotalTokens != 4 {
		t.Fatalf("asr usage = %+v, want 4 seconds", usage)
	}
	if !strings.Contains(recorder.Body.String(), "hello world") {
		t.Fatalf("asr body not passed through: %s", recorder.Body.String())
	}
}
