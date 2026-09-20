package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDistributorTestContext(method, path, body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	return c
}

// TestGetModelRequestForStepFunNativeAudioEndpoints 回归：/v1/audio/* 的 StepFun 原生端点
// 曾被后面的 "/v1/audio" 分支覆盖 relay_mode 为 AudioSpeech，导致派发到 AudioHelper
// 返回 "invalid request type"。原生端点必须保持 RelayModeStepFunNative 与解析出的模型。
func TestGetModelRequestForStepFunNativeAudioEndpoints(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		wantModel     string
		wantRelayMode int
	}{
		{
			name:          "audio generate keeps body model",
			method:        http.MethodPost,
			path:          "/v1/audio/generate",
			body:          `{"model":"stepaudio-3-gen-preview","task":"text_to_audio"}`,
			wantModel:     "stepaudio-3-gen-preview",
			wantRelayMode: relayconstant.RelayModeStepFunNative,
		},
		{
			name:          "music submit resolves model_id",
			method:        http.MethodPost,
			path:          "/v1/audio/music/submit",
			body:          `{"model_id":"stepaudio-3-music-preview","caption":"city pop"}`,
			wantModel:     "stepaudio-3-music-preview",
			wantRelayMode: relayconstant.RelayModeStepFunNative,
		},
		{
			name:          "music query falls back to pseudo model",
			method:        http.MethodPost,
			path:          "/v1/audio/music/query",
			body:          `{"task_id":"t1"}`,
			wantModel:     "stepfun-native",
			wantRelayMode: relayconstant.RelayModeStepFunNative,
		},
		{
			name:          "voices list uses query model",
			method:        http.MethodGet,
			path:          "/v1/audio/voices?model=stepaudio-2.5-tts",
			wantModel:     "stepaudio-2.5-tts",
			wantRelayMode: relayconstant.RelayModeStepFunNative,
		},
		{
			name:          "openai speech keeps audio speech mode",
			method:        http.MethodPost,
			path:          "/v1/audio/speech",
			body:          `{"model":"stepaudio-3-tts","input":"hi","voice":"cixingnansheng"}`,
			wantModel:     "stepaudio-3-tts",
			wantRelayMode: relayconstant.RelayModeAudioSpeech,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newDistributorTestContext(tt.method, tt.path, tt.body)
			modelRequest, _, err := getModelRequest(c)
			require.NoError(t, err)
			require.NotNil(t, modelRequest)
			assert.Equal(t, tt.wantModel, modelRequest.Model)
			assert.Equal(t, tt.wantRelayMode, c.GetInt("relay_mode"))
		})
	}
}

// TestGetModelRequestLeavesRealtimeToRouterDispatch 回归：/v1/realtime 与 OpenAI Realtime 共用路径，
// isStepFunNativeRoute 必须显式排除它。若纳入，无 model 查询串时 modelRequest.Model 会退到
// stepfun-native（不满足 IsRealtimeModel），被 router 送进 OpenAI Realtime 渠道后上游报错。
func TestGetModelRequestLeavesRealtimeToRouterDispatch(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		wantModel     string
		body          string
		wantRelayMode int
	}{
		{
			name:          "stepfun realtime model leaves relay mode unset",
			method:        http.MethodGet,
			path:          "/v1/realtime?model=stepaudio-3-realtime-preview",
			wantModel:     "stepaudio-3-realtime-preview",
			wantRelayMode: relayconstant.RelayModeUnknown,
		},
		{
			name:          "realtime without model stays empty for router",
			method:        http.MethodGet,
			path:          "/v1/realtime",
			wantModel:     "",
			wantRelayMode: relayconstant.RelayModeUnknown,
		},
		{
			name:          "openai realtime model untouched",
			method:        http.MethodGet,
			path:          "/v1/realtime?model=gpt-4o-realtime-preview",
			wantModel:     "gpt-4o-realtime-preview",
			wantRelayMode: relayconstant.RelayModeUnknown,
		},
		{
			name:          "realtime client secrets stays with xai native",
			method:        http.MethodPost,
			path:          "/v1/realtime/client_secrets",
			body:          `{"session":{"model":"gpt-4o-realtime-preview"}}`,
			wantModel:     "grok-voice-latest",
			wantRelayMode: relayconstant.RelayModeXAINative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newDistributorTestContext(tt.method, tt.path, tt.body)
			modelRequest, _, err := getModelRequest(c)
			require.NoError(t, err)
			require.NotNil(t, modelRequest)
			assert.Equal(t, tt.wantModel, modelRequest.Model)
			assert.Equal(t, tt.wantRelayMode, c.GetInt("relay_mode"))
		})
	}
}

// TestIsStepFunNativeRouteCoversWssTable 回归：WSS 原生端点表里 /v1/realtime/audio 与 /v1/audio/asr/stream
// 必须被识别，而 /v1/realtime 必须被排除（由 router 按模型名分派）。
func TestIsStepFunNativeRouteCoversWssTable(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{name: "realtime audio is native", method: http.MethodGet, path: "/v1/realtime/audio", want: true},
		{name: "asr stream is native", method: http.MethodGet, path: "/v1/audio/asr/stream", want: true},
		{name: "realtime is not native", method: http.MethodGet, path: "/v1/realtime", want: false},
		{name: "chat completions is not native", method: http.MethodPost, path: "/v1/chat/completions", want: false},
		{name: "messages is not native", method: http.MethodPost, path: "/v1/messages", want: false},
		{name: "openai speech is not native", method: http.MethodPost, path: "/v1/audio/speech", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isStepFunNativeRoute(tt.method, tt.path))
		})
	}
}
