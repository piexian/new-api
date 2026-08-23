package mistral

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertSpeechRequest(t *testing.T) {
	reader, err := convertSpeechRequest(&dto.AudioRequest{
		Model:          "voxtral-mini-tts-2603",
		Input:          "hello world",
		Voice:          "voice-abc",
		ResponseFormat: "wav",
	})
	require.NoError(t, err)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, common.Unmarshal(data, &got))
	require.Equal(t, "voxtral-mini-tts-2603", got["model"])
	require.Equal(t, "hello world", got["input"])
	require.Equal(t, "voice-abc", got["voice_id"])
	require.Equal(t, "wav", got["response_format"])
	// 上游要求 stream 恒为 false，不能省略
	require.Equal(t, false, got["stream"])
}

func TestConvertSpeechRequestDropsUnsupportedFormat(t *testing.T) {
	reader, err := convertSpeechRequest(&dto.AudioRequest{
		Model:          "voxtral-mini-tts-2603",
		Input:          "hello",
		ResponseFormat: "aac",
	})
	require.NoError(t, err)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	// 上游不支持的格式直接省略，走默认 mp3
	require.NotContains(t, string(data), "response_format")
}

func newMultipartAudioContext(t *testing.T) *gin.Context {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "origin-model"))
	require.NoError(t, writer.WriteField("diarize", "true"))
	require.NoError(t, writer.WriteField("timestamp_granularities", "segment"))
	require.NoError(t, writer.WriteField("timestamp_granularities", "word"))
	part, err := writer.CreateFormFile("file", "audio.mp3")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake audio bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &buf)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c
}

func TestConvertTranscriptionRequestPassThrough(t *testing.T) {
	c := newMultipartAudioContext(t)
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeAudioTranscription}

	// request.Model 已是模型映射后的上游模型名
	reader, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{Model: "voxtral-mini-latest"})
	require.NoError(t, err)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	contentType := c.Request.Header.Get("Content-Type")
	require.True(t, strings.HasPrefix(contentType, "multipart/form-data; boundary="))
	boundary := strings.TrimPrefix(contentType, "multipart/form-data; boundary=")
	form, err := multipart.NewReader(bytes.NewReader(data), boundary).ReadForm(1 << 20)
	require.NoError(t, err)

	// model 被重写为映射后的模型，其余字段原样透传
	require.Equal(t, []string{"voxtral-mini-latest"}, form.Value["model"])
	require.Equal(t, []string{"true"}, form.Value["diarize"])
	require.Equal(t, []string{"segment", "word"}, form.Value["timestamp_granularities"])

	files := form.File["file"]
	require.Len(t, files, 1)
	require.Equal(t, "audio.mp3", files[0].Filename)
	file, err := files[0].Open()
	require.NoError(t, err)
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, []byte("fake audio bytes"), fileBytes)
}

func TestConvertTranscriptionRequestRequiresFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "voxtral-mini-latest"))
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &buf)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := convertTranscriptionRequest(c, &dto.AudioRequest{Model: "voxtral-mini-latest"})
	require.Error(t, err)
}

func newTTSResponse(t *testing.T, audioBytes []byte) (*gin.Context, *httptest.ResponseRecorder, *http.Response) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	body := `{"audio_data":"` + base64.StdEncoding.EncodeToString(audioBytes) + `"}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	return c, recorder, resp
}

func TestMistralTTSHandlerPCM(t *testing.T) {
	// 60s 的 24kHz/16bit/单声道 PCM，按时长法应计 1000 tokens
	pcmBytes := make([]byte, 24000*2*60)
	c, recorder, resp := newTTSResponse(t, pcmBytes)

	info := &relaycommon.RelayInfo{Request: &dto.AudioRequest{ResponseFormat: "pcm"}}
	usage, apiErr := MistralTTSHandler(c, resp, info)
	require.Nil(t, apiErr)
	require.Equal(t, pcmBytes, recorder.Body.Bytes())
	require.Equal(t, "audio/L16", recorder.Header().Get("Content-Type"))
	require.Equal(t, 1000, usage.CompletionTokens)
	require.Equal(t, 1000, usage.TotalTokens)
	require.Equal(t, 1000, usage.CompletionTokenDetails.AudioTokens)
}

func TestMistralTTSHandlerRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"unexpected":true}`)),
	}
	info := &relaycommon.RelayInfo{Request: &dto.AudioRequest{ResponseFormat: "mp3"}}
	_, apiErr := MistralTTSHandler(c, resp, info)
	require.NotNil(t, apiErr)
}
