package gmicloud

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func audioInfo(model string) *relaycommon.RelayInfo {
	info := newInfo(relayconstant.RelayModeAudioSpeech, types.RelayFormatOpenAIAudio)
	info.OriginModelName = model
	info.ApiKey = "test-key"
	return info
}

func readRequestJSON(t *testing.T, reader io.Reader) map[string]any {
	t.Helper()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, common.Unmarshal(data, &out))
	return out
}

func TestBuildAudioRequestBodyTTS(t *testing.T) {
	info := audioInfo("minimax-tts-speech-2.8-turbo")
	speed := 1.2
	reader, err := buildAudioRequestBody(nil, info, &dto.AudioRequest{
		Model:          "minimax-tts-speech-2.8-turbo",
		Input:          "你好世界",
		Voice:          "English_expressive_narrator",
		Speed:          &speed,
		ResponseFormat: "wav",
	})
	require.NoError(t, err)

	body := readRequestJSON(t, reader)
	require.Equal(t, "minimax-tts-speech-2.8-turbo", body["model"])
	payload := body["payload"].(map[string]any)
	require.Equal(t, "你好世界", payload["text"])
	require.Equal(t, "English_expressive_narrator", payload["voice_id"])
	require.Equal(t, "1.2", payload["speed"])
	require.Equal(t, "wav", payload["format"])
}

func TestBuildAudioRequestBodyRejectsMusicModels(t *testing.T) {
	info := audioInfo("minimax-music-3.0")
	_, err := buildAudioRequestBody(nil, info, &dto.AudioRequest{
		Model: "minimax-music-3.0",
		Input: "[verse]\nHello world",
	})
	require.ErrorContains(t, err, "/v1/music_generation")
}
func TestBuildAudioRequestBodyRejectsUnsupportedSpeechModels(t *testing.T) {
	info := audioInfo("minimax-tts-speech-9.9")
	_, err := buildAudioRequestBody(nil, info, &dto.AudioRequest{
		Model: "minimax-tts-speech-9.9",
		Input: "Hello world",
	})
	require.ErrorContains(t, err, "does not support /v1/audio/speech")
}

func TestBuildAudioRequestBodyVoiceCloneRequiresSourceAudio(t *testing.T) {
	info := audioInfo("minimax-audio-voice-clone-speech-2.8-hd")
	_, err := buildAudioRequestBody(nil, info, &dto.AudioRequest{
		Model: "minimax-audio-voice-clone-speech-2.8-hd",
		Input: "要合成的文本",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "source_audio")
}

func TestBuildAudioRequestBodyVoiceCloneRejectsInvalidSourceAudio(t *testing.T) {
	info := audioInfo("minimax-audio-voice-clone-speech-2.8-hd")
	for _, metadata := range []string{`{"source_audio":""}`, `{"source_audio":"file:///tmp/source.wav"}`, `{"source_audio":123}`} {
		_, err := buildAudioRequestBody(nil, info, &dto.AudioRequest{Input: "text", Metadata: []byte(metadata)})
		require.ErrorContains(t, err, "HTTP(S) URL")
	}
}

func TestBuildAudioRequestBodyVoiceCloneWithSourceAudio(t *testing.T) {
	info := audioInfo("minimax-audio-voice-clone-speech-2.8-hd")
	reader, err := buildAudioRequestBody(nil, info, &dto.AudioRequest{
		Model:    "minimax-audio-voice-clone-speech-2.8-hd",
		Input:    "要合成的文本",
		Metadata: []byte(`{"source_audio":"https://example.com/source.mp3","need_noise_reduction":true}`),
	})
	require.NoError(t, err)

	body := readRequestJSON(t, reader)
	payload := body["payload"].(map[string]any)
	require.Equal(t, "要合成的文本", payload["text"])
	require.Equal(t, "https://example.com/source.mp3", payload["source_audio"])
	require.Equal(t, true, payload["need_noise_reduction"])
}

func TestBuildAudioRequestBodyMetadataPrecedence(t *testing.T) {
	info := audioInfo("minimax-tts-speech-2.8-hd")
	reader, err := buildAudioRequestBody(nil, info, &dto.AudioRequest{
		Model:    "minimax-tts-speech-2.8-hd",
		Input:    "explicit wins",
		Voice:    "explicit_voice",
		Metadata: []byte(`{"text":"metadata text","voice_id":"metadata_voice"}`),
	})
	require.NoError(t, err)

	body := readRequestJSON(t, reader)
	payload := body["payload"].(map[string]any)
	require.Equal(t, "explicit wins", payload["text"])
	require.Equal(t, "explicit_voice", payload["voice_id"])
}

func TestExtractAudioURL(t *testing.T) {
	require.Equal(t, "", extractAudioURL(nil))
	require.Equal(t, "https://a.mp3", extractAudioURL(&gmiOutcome{AudioURL: "https://a.mp3"}))
	require.Equal(t, "https://b.mp3", extractAudioURL(&gmiOutcome{MediaURLs: []gmiMedia{{ID: "1", URL: "https://b.mp3"}}}))
	require.Equal(t, "https://c.mp3", extractAudioURL(&gmiOutcome{Medias: []gmiMedia{{ID: "2", URL: "https://c.mp3"}}}))
	require.Equal(t, "", extractAudioURL(&gmiOutcome{}))
}

func TestNormalizeAudioFormat(t *testing.T) {
	require.Equal(t, "mp3", normalizeAudioFormat(""))
	require.Equal(t, "mp3", normalizeAudioFormat("opus"))
	require.Equal(t, "wav", normalizeAudioFormat("WAV"))
	require.Equal(t, "flac", normalizeAudioFormat(" flac "))
}

// newGMIMockServer simulates the GMI requestqueue: submit -> polling -> media download.
func newGMIMockServer(t *testing.T, submitStatus string, statusCalls *int) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/ie/requestqueue/apikey/requests" && r.Method == http.MethodPost:
			require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"request_id":"req-1","model":"minimax-tts-speech-2.8-turbo","status":"` + submitStatus + `"}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/ie/requestqueue/apikey/requests/") && r.Method == http.MethodGet:
			require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			*statusCalls++
			w.Header().Set("Content-Type", "application/json")
			body := strings.ReplaceAll(`{
				"request_id":"req-1",
				"model":"minimax-tts-speech-2.8-turbo",
				"status":"success",
				"outcome":{"media_urls":[{"id":"m1","url":"SERVER_URL/audio.mp3"}]}
			}`, "SERVER_URL", server.URL)
			_, _ = w.Write([]byte(body))
		case r.URL.Path == "/audio.mp3":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("fake-audio-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newAudioGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
	return c, w
}

func TestHandleAudioResponseSuccessWithPolling(t *testing.T) {
	allowPrivateDownloadFetch(t)
	statusCalls := 0
	server := newGMIMockServer(t, "processing", &statusCalls)

	info := audioInfo("minimax-tts-speech-2.8-turbo")
	info.ChannelBaseUrl = server.URL
	info.SetEstimatePromptTokens(42)

	c, w := newAudioGinContext()
	submitResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"req-1","status":"processing"}`)),
	}

	usage, apiErr := handleAudioResponse(c, submitResp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 42, usage.PromptTokens)
	require.GreaterOrEqual(t, statusCalls, 1)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	require.Equal(t, "fake-audio-bytes", w.Body.String())
}

func TestHandleAudioResponseImmediateSuccess(t *testing.T) {
	allowPrivateDownloadFetch(t)
	statusCalls := 0
	server := newGMIMockServer(t, "success", &statusCalls)

	info := audioInfo("minimax-tts-speech-2.8-turbo")
	info.ChannelBaseUrl = server.URL

	c, w := newAudioGinContext()
	submitResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"req-1","status":"success"}`)),
	}

	usage, apiErr := handleAudioResponse(c, submitResp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.GreaterOrEqual(t, statusCalls, 1)
	require.Equal(t, "fake-audio-bytes", w.Body.String())
}

func TestHandleAudioResponseFailedStatus(t *testing.T) {
	info := audioInfo("minimax-tts-speech-2.8-turbo")
	c, _ := newAudioGinContext()

	submitResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"req-2","status":"failed","message":"quota exceeded"}`)),
	}

	usage, apiErr := handleAudioResponse(c, submitResp, info)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "quota exceeded")
}

func TestHandleAudioResponseEmptyRequestID(t *testing.T) {
	info := audioInfo("minimax-tts-speech-2.8-turbo")
	c, _ := newAudioGinContext()

	submitResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"status":"processing"}`)),
	}

	usage, apiErr := handleAudioResponse(c, submitResp, info)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "request_id")
}

func TestHandleAudioResponseMalformedSubmit(t *testing.T) {
	info := audioInfo("minimax-tts-speech-2.8-turbo")
	c, _ := newAudioGinContext()

	submitResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`not-json`)),
	}

	usage, apiErr := handleAudioResponse(c, submitResp, info)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "decode submit response")
}
