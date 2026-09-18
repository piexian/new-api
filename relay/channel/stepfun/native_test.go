package stepfun

import (
	"net/http"
	"net/url"
	"testing"
)

func TestLookupNativeEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		method     string
		wantOK     bool
		wantBill   bool
		wantPath   string
		wantOpenPl bool
	}{
		{name: "audio generate", path: "/v1/audio/generate", method: http.MethodPost, wantOK: true, wantBill: true, wantOpenPl: true},
		{name: "music submit", path: "/v1/audio/music/submit", method: http.MethodPost, wantOK: true, wantBill: true, wantOpenPl: true},
		{name: "music query not billable", path: "/v1/audio/music/query", method: http.MethodPost, wantOK: true, wantBill: false, wantOpenPl: true},
		{name: "asr sse available on step plan", path: "/v1/audio/asr/sse", method: http.MethodPost, wantOK: true, wantBill: true, wantOpenPl: false},
		{name: "voices create", path: "/v1/audio/voices", method: http.MethodPost, wantOK: true, wantBill: true},
		{name: "voices list", path: "/v1/audio/voices", method: http.MethodGet, wantOK: true, wantBill: false},
		{name: "system voices", path: "/v1/audio/system_voices", method: http.MethodGet, wantOK: true, wantBill: false, wantOpenPl: true},
		{name: "files upload", path: "/v1/files", method: http.MethodPost, wantOK: true, wantBill: true, wantOpenPl: true},
		{name: "file by id", path: "/v1/files/file-abc", method: http.MethodGet, wantOK: true, wantOpenPl: true},
		{name: "file content", path: "/v1/files/file-abc/content", method: http.MethodGet, wantOK: true, wantOpenPl: true},
		{name: "file delete", path: "/v1/files/file-abc", method: http.MethodDelete, wantOK: true, wantOpenPl: true},
		{name: "trailing slash tolerated", path: "/v1/audio/generate/", method: http.MethodPost, wantOK: true, wantBill: true, wantOpenPl: true},
		{name: "method mismatch", path: "/v1/audio/system_voices", method: http.MethodPost, wantOK: false},
		{name: "unknown native path", path: "/v1/audio/speech", method: http.MethodPost, wantOK: false},
		{name: "chat path is not native", path: "/v1/chat/completions", method: http.MethodPost, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			endpoint, ok := LookupNativeEndpoint(tt.path, tt.method)
			if ok != tt.wantOK {
				t.Fatalf("LookupNativeEndpoint(%s %s) ok = %v, want %v", tt.method, tt.path, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if endpoint.Billable != tt.wantBill {
				t.Fatalf("billable = %v, want %v", endpoint.Billable, tt.wantBill)
			}
			if endpoint.OpenPlatform != tt.wantOpenPl {
				t.Fatalf("openPlatformOnly = %v, want %v", endpoint.OpenPlatform, tt.wantOpenPl)
			}
		})
	}

	if NativeEndpointTableSize() < 15 {
		t.Fatalf("native endpoint table unexpectedly small: %d", NativeEndpointTableSize())
	}
}

// TestNativeEndpointTableHasNoDuplicates 防止 gin 路由重复注册导致启动 panic。
func TestNativeEndpointTableHasNoDuplicates(t *testing.T) {
	t.Parallel()

	seen := map[string]struct{}{}
	for _, endpoint := range stepFunNativeEndpoints {
		key := endpoint.Method + " " + endpoint.Path
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate native endpoint registration: %s", key)
		}
		seen[key] = struct{}{}
	}
}

func TestNativeRouteModel(t *testing.T) {
	t.Parallel()

	musicSubmit, _ := LookupNativeEndpoint("/v1/audio/music/submit", http.MethodPost)
	musicQuery, _ := LookupNativeEndpoint("/v1/audio/music/query", http.MethodPost)
	asrFileSubmit, _ := LookupNativeEndpoint("/v1/audio/asr/file/submit", http.MethodPost)
	asrSSE, _ := LookupNativeEndpoint("/v1/audio/asr/sse", http.MethodPost)
	systemVoices, _ := LookupNativeEndpoint("/v1/audio/system_voices", http.MethodGet)

	tests := []struct {
		name     string
		endpoint NativeEndpoint
		body     string
		query    string
		want     string
	}{
		{name: "music submit uses model_id", endpoint: musicSubmit, body: `{"model_id":"stepaudio-3-music-preview","caption":"x"}`, want: "stepaudio-3-music-preview"},
		{name: "music submit prefers explicit model", endpoint: musicSubmit, body: `{"model":"routed-model","model_id":"stepaudio-3-music-preview"}`, want: "routed-model"},
		{name: "asr file submit reads nested model_name", endpoint: asrFileSubmit, body: `{"request":{"model_name":"stepaudio-2.5-asr"},"audio":{"format":"wav"}}`, want: "stepaudio-2.5-asr"},
		{name: "asr sse reads nested transcription model", endpoint: asrSSE, body: `{"audio":{"input":{"transcription":{"model":"stepaudio-3-asr-max"}}}}`, want: "stepaudio-3-asr-max"},
		{name: "query param fallback", endpoint: musicQuery, body: `{"task_id":"t1"}`, query: "model=stepaudio-3-music-preview", want: "stepaudio-3-music-preview"},
		{name: "system voices uses query param", endpoint: systemVoices, query: "model=step-tts-2", want: "step-tts-2"},
		{name: "empty when nothing matches", endpoint: musicQuery, body: `{"task_id":"t1"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			values, _ := url.ParseQuery(tt.query)
			got := NativeRouteModel([]byte(tt.body), values, tt.endpoint)
			if got != tt.want {
				t.Fatalf("NativeRouteModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrepareNativeRequestBodyStripsRoutingModel(t *testing.T) {
	t.Parallel()

	musicSubmit, _ := LookupNativeEndpoint("/v1/audio/music/submit", http.MethodPost)
	generate, _ := LookupNativeEndpoint("/v1/audio/generate", http.MethodPost)

	body := []byte(`{"model":"stepaudio-3-music-preview","model_id":"stepaudio-3-music-preview","caption":"c"}`)
	prepared, err := PrepareNativeRequestBody(body, musicSubmit)
	if err != nil {
		t.Fatalf("PrepareNativeRequestBody: %v", err)
	}
	if string(prepared) == string(body) {
		t.Fatalf("routing model should be stripped: %s", prepared)
	}
	if !containsAll(string(prepared), []string{`"model_id":"stepaudio-3-music-preview"`, `"caption":"c"`}) {
		t.Fatalf("payload broken after stripping: %s", prepared)
	}
	if containsAll(string(prepared), []string{`"model"`}) {
		t.Fatalf("model key should be gone: %s", prepared)
	}

	// generate 的 model 是上游真实字段，必须保留
	generateBody := []byte(`{"model":"stepaudio-3-gen-preview","task":"text_to_audio"}`)
	unchanged, err := PrepareNativeRequestBody(generateBody, generate)
	if err != nil {
		t.Fatalf("PrepareNativeRequestBody: %v", err)
	}
	if string(unchanged) != string(generateBody) {
		t.Fatalf("generate body should stay untouched: %s", unchanged)
	}
}

func TestNativeUpstreamQuery(t *testing.T) {
	t.Parallel()

	systemVoices, _ := LookupNativeEndpoint("/v1/audio/system_voices", http.MethodGet)
	voicesList, _ := LookupNativeEndpoint("/v1/audio/voices", http.MethodGet)

	if got := NativeUpstreamQuery("model=step-tts-2", systemVoices); got != "model=step-tts-2" {
		t.Fatalf("system_voices must keep model param, got %q", got)
	}
	if got := NativeUpstreamQuery("model=stepaudio-2.5-tts&limit=20&order=desc", voicesList); got != "limit=20&order=desc" {
		t.Fatalf("voices list must drop routing model, got %q", got)
	}
	if got := NativeUpstreamQuery("", voicesList); got != "" {
		t.Fatalf("empty query should stay empty, got %q", got)
	}
}

func TestNativeForwardPath(t *testing.T) {
	t.Parallel()

	if got := NativeForwardPath("/v1/audio/voices?model=stepaudio-2.5-tts&limit=5"); got != "/v1/audio/voices?limit=5" {
		t.Fatalf("voices forward path = %q", got)
	}
	if got := NativeForwardPath("/v1/audio/system_voices?model=step-tts-2"); got != "/v1/audio/system_voices?model=step-tts-2" {
		t.Fatalf("system_voices forward path = %q", got)
	}
	if got := NativeForwardPath("/v1/audio/music/query"); got != "/v1/audio/music/query" {
		t.Fatalf("music query forward path = %q", got)
	}
	// 非原生路径原样返回
	if got := NativeForwardPath("/v1/chat/completions"); got != "/v1/chat/completions" {
		t.Fatalf("non-native path = %q", got)
	}
}

func TestNativeEndpointAvailableOnBase(t *testing.T) {
	t.Parallel()

	generate, _ := LookupNativeEndpoint("/v1/audio/generate", http.MethodPost)
	asrSSE, _ := LookupNativeEndpoint("/v1/audio/asr/sse", http.MethodPost)
	filesUpload, _ := LookupNativeEndpoint("/v1/files", http.MethodPost)

	if generate.AvailableOnBase("stepfun-step-plan") {
		t.Fatal("audio/generate must be rejected on Step Plan channels")
	}
	if !generate.AvailableOnBase("stepfun") {
		t.Fatal("audio/generate must be available on the open platform")
	}
	if !asrSSE.AvailableOnBase("https://api.stepfun.com/step_plan/v1") {
		t.Fatal("asr/sse is available on Step Plan according to the live probe")
	}
	if filesUpload.AvailableOnBase("stepfun-step-plan") {
		t.Fatal("files endpoints are open-platform only (Step Plan returns 404 upstream)")
	}
	if !filesUpload.AvailableOnBase("stepfun") {
		t.Fatal("files endpoints must be available on the open platform")
	}
}

func containsAll(haystack string, needles []string) bool {
	for _, needle := range needles {
		found := false
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
