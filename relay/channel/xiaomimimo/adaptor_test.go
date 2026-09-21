package xiaomimimo

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLByRelayMode(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	base := "https://api.xiaomimimo.com"

	chatInfo := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}
	chatInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelBaseUrl: base}
	url, err := adaptor.GetRequestURL(chatInfo)
	require.NoError(t, err)
	require.Equal(t, base+"/v1/chat/completions", url)

	responsesInfo := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeResponses}
	responsesInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelBaseUrl: base}
	url, err = adaptor.GetRequestURL(responsesInfo)
	require.NoError(t, err)
	require.Equal(t, base+"/v1/responses", url)

	claudeInfo := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatClaude}
	claudeInfo.ChannelMeta = &relaycommon.ChannelMeta{ChannelBaseUrl: base}
	url, err = adaptor.GetRequestURL(claudeInfo)
	require.NoError(t, err)
	require.Equal(t, base+"/anthropic/v1/messages", url)
}

func TestConvertOpenAIResponsesRequestSanitizes(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:              "mimo-v2.6-pro-ultraspeed",
		Input:              json.RawMessage(`"hi"`),
		PreviousResponseID: "resp_123",
		ContextManagement:  json.RawMessage(`{"b":{"t":"x"}}`),
		Reasoning:          &dto.Reasoning{Effort: "minimal", Summary: "auto"},
	})
	require.NoError(t, err)
	req, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.Empty(t, req.PreviousResponseID, "previous_response_id 上游不支持应清洗")
	require.Nil(t, req.ContextManagement, "context_management 上游不支持应清洗")
	require.NotNil(t, req.Reasoning)
	require.Equal(t, "low", req.Reasoning.Effort, "minimal 应归一为 low")
	require.Empty(t, req.Reasoning.Summary, "summary 上游未定义应剥离")
	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.FinalRequestRelayFormat)

	// xhigh -> high, 已合法档位原样放行
	for effort, want := range map[string]string{"xhigh": "high", "none": "none", "": ""} {
		converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
			Reasoning: &dto.Reasoning{Effort: effort},
		})
		require.NoError(t, err)
		require.Equal(t, want, converted.(dto.OpenAIResponsesRequest).Reasoning.Effort)
	}
}

func buildASRTestContext(t *testing.T, filename, contentType, language string, content []byte) *gin.Context {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if language != "" {
		_ = writer.WriteField("language", language)
	}
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c
}

func TestConvertOpenAISTTToMiMoWAV(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	audio := []byte("fake-wav-bytes")
	c := buildASRTestContext(t, "test.wav", "", "zh", audio)

	reader, err := convertOpenAISTTToMiMo(c, dto.AudioRequest{Model: "mimo-v2.5-asr"}, "mimo-v2.5-asr")
	require.NoError(t, err)
	require.Equal(t, "application/json", c.Request.Header.Get("Content-Type"))

	var payload map[string]any
	require.NoError(t, json.NewDecoder(reader).Decode(&payload))
	require.Equal(t, "mimo-v2.5-asr", payload["model"])
	messages := payload["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)[0].(map[string]any)
	require.Equal(t, "input_audio", content["type"])
	inputAudio := content["input_audio"].(map[string]any)
	require.Equal(t, "data:audio/wav;base64,"+base64StdEncode(audio), inputAudio["data"])
	require.Equal(t, "wav", inputAudio["format"])
	require.Equal(t, map[string]any{"language": "zh"}, payload["asr_options"])
}

func TestConvertOpenAISTTToMiMoMP3AndDefaults(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	c := buildASRTestContext(t, "audio.mp3", "", "", []byte("fake-mp3"))
	reader, err := convertOpenAISTTToMiMo(c, dto.AudioRequest{Model: "mimo-v2.5-asr"}, "")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(reader).Decode(&payload))
	require.Equal(t, "mimo-v2.5-asr", payload["model"], "上游模型名缺省应回退请求模型")
	inputAudio := payload["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["input_audio"].(map[string]any)
	require.Equal(t, "mp3", inputAudio["format"])
	require.True(t, strings.HasPrefix(inputAudio["data"].(string), "data:audio/mpeg;base64,"))
	require.Equal(t, map[string]any{"language": "auto"}, payload["asr_options"], "无效/缺省 language 应回退 auto")
}

func TestConvertOpenAISTTToMiMoRejectsUnsupportedFormat(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	c := buildASRTestContext(t, "audio.ogg", "", "en", []byte("ogg"))
	_, err := convertOpenAISTTToMiMo(c, dto.AudioRequest{Model: "mimo-v2.5-asr"}, "mimo-v2.5-asr")
	require.Error(t, err)
	require.Contains(t, err.Error(), "only accepts mp3/wav")
}

func TestConvertOpenAISTTToMiMoRequiresFile(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "mimo-v2.5-asr")
	require.NoError(t, writer.Close())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := convertOpenAISTTToMiMo(c, dto.AudioRequest{Model: "mimo-v2.5-asr"}, "mimo-v2.5-asr")
	require.Error(t, err)
	require.Contains(t, err.Error(), "file is required")
}

func TestHandleASRResponse(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	upstreamBody := `{"id":"x","choices":[{"message":{"content":"Good morning.","role":"assistant"}}],"usage":{"prompt_tokens":46,"completion_tokens":20,"total_tokens":66,"seconds":4}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}

	usageAny, apiErr := handleASRResponse(c, resp, &relaycommon.RelayInfo{})
	require.Nil(t, apiErr)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 46, usage.PromptTokens)
	require.Equal(t, 20, usage.CompletionTokens)
	require.Equal(t, 66, usage.TotalTokens)
	require.JSONEq(t, `{"text":"Good morning."}`, recorder.Body.String())
}

func base64StdEncode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
