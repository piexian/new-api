package stepfun

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
)

func TestLookupWssEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path         string
		wantOK       bool
		wantOpenOnly bool
	}{
		{path: "/v1/realtime/audio", wantOK: true},
		{path: "/v1/audio/asr/stream", wantOK: true, wantOpenOnly: true},
		{path: "/v1/realtime", wantOK: true},
		{path: "/v1/realtime/audio/", wantOK: true}, // 尾部斜杠按与 HTTP 端点表一致的规则归一
		{path: "/v1/audio/speech", wantOK: false},
		{path: "/v1/chat/completions", wantOK: false},
	}
	for _, tt := range tests {
		endpoint, ok := LookupWssEndpoint(tt.path)
		if ok != tt.wantOK {
			t.Fatalf("LookupWssEndpoint(%s) ok = %v, want %v", tt.path, ok, tt.wantOK)
		}
		if !ok {
			continue
		}
		if endpoint.OpenPlatform != tt.wantOpenOnly {
			t.Fatalf("LookupWssEndpoint(%s) OpenPlatform = %v, want %v", tt.path, endpoint.OpenPlatform, tt.wantOpenOnly)
		}
	}
	if WssEndpointTableSize() != 3 {
		t.Fatalf("unexpected wss endpoint table size: %d", WssEndpointTableSize())
	}
}

func TestWssEndpointAvailableOnBase(t *testing.T) {
	t.Parallel()

	ttsWS, _ := LookupWssEndpoint("/v1/realtime/audio")
	asrWS, _ := LookupWssEndpoint("/v1/audio/asr/stream")

	if !ttsWS.AvailableOnBase("stepfun-step-plan") {
		t.Fatal("realtime/audio is available on Step Plan according to the live probe")
	}
	if asrWS.AvailableOnBase("stepfun-step-plan") {
		t.Fatal("audio/asr/stream is open-platform only (Step Plan returns 404)")
	}
	if !asrWS.AvailableOnBase("https://api.stepfun.com") {
		t.Fatal("audio/asr/stream must be available on the open platform")
	}
}

func TestIsRealtimeModel(t *testing.T) {
	t.Parallel()

	positive := []string{"stepaudio-2.5-realtime", "stepaudio-3-realtime-preview", "StepAudio-2.5-Realtime"}
	for _, model := range positive {
		if !IsRealtimeModel(model) {
			t.Fatalf("IsRealtimeModel(%q) = false, want true", model)
		}
	}
	negative := []string{"gpt-4o-realtime-preview", "stepaudio-2.5-tts", "step-3.7-flash", ""}
	for _, model := range negative {
		if IsRealtimeModel(model) {
			t.Fatalf("IsRealtimeModel(%q) = true, want false", model)
		}
	}
}

func TestWebsocketBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "https://api.stepfun.com", want: "wss://api.stepfun.com"},
		{in: "http://127.0.0.1:8080", want: "ws://127.0.0.1:8080"},
		{in: "wss://api.stepfun.com", want: "wss://api.stepfun.com"},
	}
	for _, tt := range tests {
		if got := WebsocketBaseURL(tt.in); got != tt.want {
			t.Fatalf("WebsocketBaseURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGetRequestURLForWebsocketEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		base        string
		requestPath string
		want        string
		wantErr     bool
	}{
		{
			name:        "tts websocket on open platform",
			base:        "stepfun",
			requestPath: "/v1/realtime/audio?model=stepaudio-3-tts",
			want:        "wss://api.stepfun.com/v1/realtime/audio?model=stepaudio-3-tts",
		},
		{
			name:        "tts websocket on step plan keeps prefix",
			base:        "stepfun-step-plan",
			requestPath: "/v1/realtime/audio?model=stepaudio-2.5-tts",
			want:        "wss://api.stepfun.com/step_plan/v1/realtime/audio?model=stepaudio-2.5-tts",
		},
		{
			name:        "asr stream on explicit url base",
			base:        "https://api.stepfun.com/v1",
			requestPath: "/v1/audio/asr/stream?model=stepaudio-2.5-asr",
			want:        "wss://api.stepfun.com/v1/audio/asr/stream?model=stepaudio-2.5-asr",
		},
		{
			name:        "realtime chat websocket",
			base:        "stepfun-intl-step-plan",
			requestPath: "/v1/realtime?model=stepaudio-2.5-realtime",
			want:        "wss://api.stepfun.ai/step_plan/v1/realtime?model=stepaudio-2.5-realtime",
		},
		{
			name:        "non whitelisted base is rejected",
			base:        "https://my-proxy.example.com/stepfun",
			requestPath: "/v1/realtime/audio?model=stepaudio-3-tts",
			wantErr:     true,
		},
		{
			name:        "http endpoint is rejected on the websocket format",
			base:        "stepfun",
			requestPath: "/v1/audio/speech",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := stepfunTestInfo(tt.base)
			info.RelayFormat = types.RelayFormatStepFunWss
			info.RequestURLPath = tt.requestPath

			got, err := (&Adaptor{}).GetRequestURL(info)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetRequestURL: %v", err)
			}
			if got != tt.want {
				t.Fatalf("GetRequestURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWebsocketEndpointsAreGetOnly 防止 WS 端点被误当成 HTTP 端点处理。
func TestWebsocketEndpointsAreGetOnly(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/v1/realtime/audio", "/v1/audio/asr/stream", "/v1/realtime"} {
		if _, ok := LookupNativeEndpoint(path, http.MethodPost); ok {
			t.Fatalf("%s must not be registered as an HTTP POST endpoint", path)
		}
		if !IsWssPath(path) {
			t.Fatalf("%s must be a websocket endpoint", path)
		}
	}
}
