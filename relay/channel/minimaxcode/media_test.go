package minimaxcode

import (
	"encoding/json"
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

func TestGetRequestURLMediaModes(t *testing.T) {
	cases := []struct {
		mode   int
		model  string
		suffix string
	}{
		{relayconstant.RelayModeImagesGenerations, "MiniMax-H3-Image", MCPImagePath},
		{relayconstant.RelayModeAudioSpeech, "music-3.0", MCPMusicPath},
		{relayconstant.RelayModeAudioSpeech, "speech-2.8-hd", MCPSpeechPath},
	}
	for _, c := range cases {
		info := &relaycommon.RelayInfo{
			RelayMode:   c.mode,
			ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: DefaultBaseURL, UpstreamModelName: c.model},
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

func TestConvertMCPAudioRequest(t *testing.T) {
	// TTS:voice 透传 + speed
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "speech-2.8-hd"}}
	speed := 1.5
	reader, err := convertMCPAudioRequest(info, dto.AudioRequest{Input: "你好", Voice: "female-shaonv", Speed: &speed})
	if err != nil {
		t.Fatalf("convert speech: %v", err)
	}
	body, _ := io.ReadAll(reader)
	var speechReq mcpSpeechRequest
	if err := common.Unmarshal(body, &speechReq); err != nil {
		t.Fatalf("parse speech body: %v", err)
	}
	if speechReq.Text != "你好" || speechReq.VoiceID != "female-shaonv" || speechReq.Speed == nil || *speechReq.Speed != 1.5 {
		t.Fatalf("speech request = %+v", speechReq)
	}

	// 缺省音色
	reader, _ = convertMCPAudioRequest(info, dto.AudioRequest{Input: "hi"})
	body, _ = io.ReadAll(reader)
	var defaultReq mcpSpeechRequest
	_ = common.Unmarshal(body, &defaultReq)
	if defaultReq.VoiceID != mcpDefaultVoiceID {
		t.Fatalf("default voice = %q", defaultReq.VoiceID)
	}

	// 音乐:instructions→lyrics,metadata 覆盖 format
	musicInfo := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "music-3.0"}}
	reader, err = convertMCPAudioRequest(musicInfo, dto.AudioRequest{
		Input:        "轻快的钢琴曲",
		Instructions: "风吹过山谷",
		Metadata:     json.RawMessage(`{"requests":[{"format":"wav","sample_rate":44100}]}`),
	})
	if err != nil {
		t.Fatalf("convert music: %v", err)
	}
	body, _ = io.ReadAll(reader)
	var musicEnvelope mcpMusicEnvelope
	if err := common.Unmarshal(body, &musicEnvelope); err != nil || len(musicEnvelope.Requests) != 1 {
		t.Fatalf("music envelope: %s", body)
	}
	musicReq := musicEnvelope.Requests[0]
	if musicReq.Prompt != "轻快的钢琴曲" || musicReq.Lyrics != "风吹过山谷" || musicReq.Format != "wav" || musicReq.SampleRate != 44100 {
		t.Fatalf("music request = %+v", musicReq)
	}
}

func TestConvertMCPImageRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "MiniMax-H3-Image"},
	}
	request := dto.ImageRequest{
		Model:  "MiniMax-H3-Image",
		Prompt: "a cat",
		Extra: map[string]json.RawMessage{
			"aspect_ratio": json.RawMessage(`"16:9"`),
			"area":         json.RawMessage(`"2K"`),
		},
	}

	converted, err := convertMCPImageRequest(c, info, request)
	if err != nil {
		t.Fatalf("convert image: %v", err)
	}
	body, _ := common.Marshal(converted)
	var envelope mcpImageEnvelope
	if err := common.Unmarshal(body, &envelope); err != nil || len(envelope.Requests) != 1 {
		t.Fatalf("image envelope: %s", body)
	}
	if envelope.Requests[0].Prompt != "a cat" || envelope.Requests[0].AspectRatio != "16:9" || envelope.Requests[0].Resolution != "2K" {
		t.Fatalf("image request = %+v", envelope.Requests[0])
	}
	if c.GetString(mcpImageResolutionKey) != "2K" {
		t.Fatalf("resolution not stashed: %q", c.GetString(mcpImageResolutionKey))
	}

	// edits 不支持
	editsInfo := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}
	if _, err := convertMCPImageRequest(c, editsInfo, request); err == nil {
		t.Fatal("image edits should be rejected")
	}
}

func newMediaTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeAudioSpeech,
		StartTime:   time.Unix(1700000000, 0),
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "speech-2.8-hd"},
	}
	return c, recorder, info
}

func mcpResponseRecorder(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestDoMCPSpeechResponseDownloadsAudio(t *testing.T) {
	// 模拟 MCP 响应里的公开 CDN 产物地址
	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FAKE_MP3_BYTES"))
	}))
	defer fileServer.Close()

	c, recorder, info := newMediaTestContext(t)
	respBody := `{"code":0,"message":"ok","output_url":"` + fileServer.URL + `/tts.mp3"}`
	usage, apiErr := doMCPSpeechResponse(c, mcpResponseRecorder(respBody), info)
	if apiErr != nil {
		t.Fatalf("speech handler error: %v", apiErr)
	}
	if usage.PromptTokens <= 0 {
		t.Fatalf("speech usage = %+v", usage)
	}
	if !strings.Contains(recorder.Body.String(), "FAKE_MP3_BYTES") {
		t.Fatalf("audio not returned: %s", recorder.Body.String())
	}
}

func TestDoMCPMusicResponseDownloadsAudio(t *testing.T) {
	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FAKE_MUSIC_BYTES"))
	}))
	defer fileServer.Close()

	c, recorder, info := newMediaTestContext(t)
	respBody := `{"code":0,"message":"ok","success_items":[{"is_success":true,"output_url":"` + fileServer.URL + `/track.mp3"}]}`
	usage, apiErr := doMCPMusicResponse(c, mcpResponseRecorder(respBody), info)
	if apiErr != nil {
		t.Fatalf("music handler error: %v", apiErr)
	}
	if usage.PromptTokens != 1 {
		t.Fatalf("music usage = %+v", usage)
	}
	if !strings.Contains(recorder.Body.String(), "FAKE_MUSIC_BYTES") {
		t.Fatalf("music not returned: %s", recorder.Body.String())
	}
}

func TestDoMCPImageResponseBuildsOpenAIShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(mcpImageResolutionKey, "1K")
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeImagesGenerations,
		StartTime:   time.Unix(1700000000, 0),
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "MiniMax-H3-Image"},
	}
	respBody := `{"code":0,"message":"ok","success_items":[{"is_success":true,"output_url":"https://cdn.example.com/out.png"}]}`

	usage, apiErr := doMCPImageResponse(c, mcpResponseRecorder(respBody), info)
	if apiErr != nil {
		t.Fatalf("image handler error: %v", apiErr)
	}
	billingUsage := usage
	if billingUsage.PromptTokens != 9531 || billingUsage.TotalTokens != 9531 {
		t.Fatalf("image usage = %+v, want 1K token equivalent 9531", billingUsage)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"url":"https://cdn.example.com/out.png"`) {
		t.Fatalf("image response = %s", body)
	}

	// 全部失败 → 502
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, apiErr := doMCPImageResponse(c2, mcpResponseRecorder(`{"code":1008,"message":"insufficient balance","failed_items":[{}]}`), info); apiErr == nil {
		t.Fatal("failed generation should return error")
	}
}

func TestMcpImageCallTokens(t *testing.T) {
	if mcpImageCallTokens("1K") != 9531 || mcpImageCallTokens("2k") != 10206 || mcpImageCallTokens("") != 30145 {
		t.Fatalf("token equivalents: 1K=%d 2K=%d default=%d", mcpImageCallTokens("1K"), mcpImageCallTokens("2k"), mcpImageCallTokens(""))
	}
}
