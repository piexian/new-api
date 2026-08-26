package gmicloud

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newInfo(relayMode int, relayFormat types.RelayFormat) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode:   relayMode,
		RelayFormat: relayFormat,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: 71},
	}
}

func TestGetRequestURLChatCompletionsDefaultBase(t *testing.T) {
	info := newInfo(relayconstant.RelayModeChatCompletions, types.RelayFormatOpenAI)
	info.OriginModelName = "MiniMaxAI/MiniMax-M2.7"

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, defaultLLMBaseURL+"/v1/chat/completions", url)
}

func TestGetRequestURLChatCompletionsCustomBase(t *testing.T) {
	info := newInfo(relayconstant.RelayModeChatCompletions, types.RelayFormatOpenAI)
	info.OriginModelName = "MiniMaxAI/MiniMax-M2.7"
	info.ChannelBaseUrl = "https://proxy.example.com/"

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://proxy.example.com/v1/chat/completions", url)
}

func TestGetRequestURLResponsesConvertedToChat(t *testing.T) {
	info := newInfo(relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses)
	info.OriginModelName = "MiniMaxAI/MiniMax-M2.7"
	info.FinalRequestRelayFormat = types.RelayFormatOpenAI

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, defaultLLMBaseURL+"/v1/chat/completions", url)
}
func TestGetRequestURLResponsesWithoutConversion(t *testing.T) {
	info := newInfo(relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses)
	info.OriginModelName = "MiniMaxAI/MiniMax-M2.7"
	info.FinalRequestRelayFormat = types.RelayFormatOpenAIResponses

	_, err := (&Adaptor{}).GetRequestURL(info)
	require.ErrorContains(t, err, "chat-completions conversion")
}

func TestGetRequestURLClaudeMessages(t *testing.T) {
	info := newInfo(relayconstant.RelayModeChatCompletions, types.RelayFormatClaude)
	info.OriginModelName = "MiniMaxAI/MiniMax-M2.7"

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, defaultLLMBaseURL+"/v1/messages", url)
}

func TestGetRequestURLAudioUsesAudioHost(t *testing.T) {
	for _, model := range []string{
		"minimax-tts-speech-2.8-turbo",
		"minimax-tts-speech-2.8-hd",
		"minimax-audio-voice-clone-speech-2.8-hd",
	} {
		info := newInfo(relayconstant.RelayModeAudioSpeech, types.RelayFormatOpenAIAudio)
		info.OriginModelName = model
		info.ChannelBaseUrl = defaultLLMBaseURL

		url, err := (&Adaptor{}).GetRequestURL(info)
		require.NoError(t, err)
		require.Equal(t, defaultAudioBaseURL+submitRequestPath, url, "model %s", model)
	}
}

func TestGetRequestURLRejectsMusicOnAudioSpeechEndpoint(t *testing.T) {
	info := newInfo(relayconstant.RelayModeAudioSpeech, types.RelayFormatOpenAIAudio)
	info.OriginModelName = "minimax-music-3.0"

	_, err := (&Adaptor{}).GetRequestURL(info)
	require.ErrorContains(t, err, "/v1/music_generation")
}

func TestGetRequestURLMiniMaxMusicUsesRequestqueue(t *testing.T) {
	info := newInfo(relayconstant.RelayModeMiniMaxMusicGeneration, types.RelayFormatMiniMax)
	info.OriginModelName = "client-music-model"
	info.UpstreamModelName = "minimax-music-3.0"
	info.ChannelBaseUrl = defaultLLMBaseURL

	url, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, defaultAudioBaseURL+submitRequestPath, url)
}

func TestGetRequestURLRejectsResponsesInputTokens(t *testing.T) {
	info := newInfo(relayconstant.RelayModeResponsesInputTokens, types.RelayFormatOpenAIResponses)

	_, err := (&Adaptor{}).GetRequestURL(info)
	require.ErrorContains(t, err, "/v1/responses/input_tokens")
}

func TestSetupRequestHeaderOpenAIBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := newInfo(relayconstant.RelayModeChatCompletions, types.RelayFormatOpenAI)
	info.ApiKey = "test-key"

	header := http.Header{}
	err := (&Adaptor{}).SetupRequestHeader(c, &header, info)
	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", header.Get("Authorization"))
	require.Empty(t, header.Get("x-api-key"))
}

func TestDoRequestMediaBadRequestSkipsRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, submitRequestPath, r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid voice"}`))
	}))
	defer server.Close()

	info := audioInfo("minimax-tts-speech-2.8-turbo")
	info.ChannelBaseUrl = server.URL
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(`{"input":"hello"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	response, err := (&Adaptor{}).DoRequest(c, info, strings.NewReader(`{"model":"minimax-tts-speech-2.8-turbo","payload":{"text":"hello"}}`))
	require.Nil(t, response)
	apiErr, ok := err.(*types.NewAPIError)
	require.True(t, ok, "unexpected error type %T", err)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))
}

func TestSetupRequestHeaderClaudeXApiKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := newInfo(relayconstant.RelayModeChatCompletions, types.RelayFormatClaude)
	info.ApiKey = "test-key"

	header := http.Header{}
	err := (&Adaptor{}).SetupRequestHeader(c, &header, info)
	require.NoError(t, err)
	require.Equal(t, "test-key", header.Get("x-api-key"))
	require.Empty(t, header.Get("Authorization"))
	require.Equal(t, "2023-06-01", header.Get("anthropic-version"))
}

func TestConvertOpenAIResponsesRequestConvertsToChat(t *testing.T) {
	info := newInfo(relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses)

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model: "MiniMaxAI/MiniMax-M2.7",
		Input: json.RawMessage(`"Hello"`),
	})
	require.NoError(t, err)
	require.EqualValues(t, types.RelayFormatOpenAI, info.FinalRequestRelayFormat)

	chatRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok, "converted type %T", converted)
	require.Equal(t, "MiniMaxAI/MiniMax-M2.7", chatRequest.Model)
	require.NotEmpty(t, chatRequest.Messages)
}

func TestConvertOpenAIRequestPassthrough(t *testing.T) {
	info := newInfo(relayconstant.RelayModeChatCompletions, types.RelayFormatOpenAI)
	request := &dto.GeneralOpenAIRequest{Model: "MiniMaxAI/MiniMax-M2.7"}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	require.NoError(t, err)
	require.Same(t, request, converted)
}

func TestConvertOpenAIRequestToClaude(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := newInfo(relayconstant.RelayModeChatCompletions, types.RelayFormatOpenAI)
	info.RelayFormat = types.RelayFormatClaude

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, &dto.GeneralOpenAIRequest{
		Model: "MiniMaxAI/MiniMax-M2.7",
		Messages: []dto.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, types.RelayFormatClaude, info.FinalRequestRelayFormat)

	claudeRequest, ok := converted.(*dto.ClaudeRequest)
	require.True(t, ok, "converted type %T", converted)
	require.Equal(t, "MiniMaxAI/MiniMax-M2.7", claudeRequest.Model)
}

func TestModelClassification(t *testing.T) {
	require.True(t, isGMIMusicModel("minimax-music-3.0"))
	require.True(t, isGMIVoiceCloneModel("minimax-audio-voice-clone-speech-2.8-hd"))
	require.True(t, isGMITTSModel("minimax-tts-speech-2.8-hd"))
	require.False(t, isGMIAudioModel("MiniMaxAI/MiniMax-M2.7"))
	require.True(t, isGMIAudioModel("minimax-music-3.0"))
	require.True(t, IsSupportedSpeechModel("minimax-tts-speech-2.8-hd"))
	require.True(t, IsSupportedSpeechModel("minimax-audio-voice-clone-speech-2.6-hd"))
	require.False(t, IsSupportedSpeechModel("minimax-tts-speech-9.9"))
}

func TestConvertAudioRequestRejectsTextModel(t *testing.T) {
	info := newInfo(relayconstant.RelayModeAudioSpeech, types.RelayFormatOpenAIAudio)
	info.OriginModelName = "MiniMaxAI/MiniMax-M2.7"

	_, err := (&Adaptor{}).ConvertAudioRequest(nil, info, dto.AudioRequest{Model: info.OriginModelName, Input: "hello"})
	require.ErrorContains(t, err, "does not support /v1/audio/speech")
}

func TestDoResponseConvertsChatCompletionToResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := newInfo(relayconstant.RelayModeResponses, types.RelayFormatOpenAIResponses)
	info.OriginModelName = "MiniMaxAI/MiniMax-M2.7"
	info.UpstreamModelName = info.OriginModelName
	info.FinalRequestRelayFormat = types.RelayFormatOpenAI

	usage, apiErr := (&Adaptor{}).DoResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_1","object":"chat.completion","model":"MiniMaxAI/MiniMax-M2.7","created":123,"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`)),
	}, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "response", response["object"])
	require.Equal(t, "completed", response["status"])
	require.Equal(t, "MiniMaxAI/MiniMax-M2.7", response["model"])
	require.Len(t, response["output"], 1)
}
