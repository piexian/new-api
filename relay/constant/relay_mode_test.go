package constant

import "testing"

func TestPath2RelayModeVideoEndpoints(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/v1/video/generations", want: RelayModeVideoSubmit},
		{path: "/v1/videos/generations", want: RelayModeVideoSubmit},
		{path: "/v1/videos/edits", want: RelayModeVideoSubmit},
		{path: "/v1/videos/extensions", want: RelayModeVideoSubmit},
		{path: "/v1/videos", want: RelayModeVideoSubmit},
		{path: "/v1/videos/video_123/remix", want: RelayModeVideoSubmit},
		{path: "/v1/video/generations/task_123", want: RelayModeVideoFetchByID},
		{path: "/v1/videos/task_123", want: RelayModeVideoFetchByID},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := Path2RelayMode(tt.path); got != tt.want {
				t.Fatalf("Path2RelayMode(%q) = %d, want %d", tt.path, got, tt.want)
			}
		})
	}
}

func TestPath2RelayModeGeminiInteractions(t *testing.T) {
	for _, path := range []string{
		"/v1beta/interactions",
		"/v1beta2/interactions",
		"/v1/interactions",
		"/v1beta/interactions/v1_abc",
		"/v1beta/interactions/v1_abc/cancel",
	} {
		if mode := Path2RelayMode(path); mode != RelayModeGeminiInteractions {
			t.Fatalf("path %s: got mode %d, want RelayModeGeminiInteractions", path, mode)
		}
	}
	// 回归: models 路径不受影响
	if mode := Path2RelayMode("/v1beta/models/gemini-2.0-flash:generateContent"); mode != RelayModeGemini {
		t.Fatalf("models path mode = %d", mode)
	}
	if mode := Path2RelayMode("/v1/models/gemini-2.0-flash:generateContent"); mode != RelayModeGemini {
		t.Fatalf("v1 models path mode = %d", mode)
	}
	// 回归: interactions 前缀不被其他分支抢占
	if mode := Path2RelayMode("/v1/videos/abc"); mode != RelayModeVideoFetchByID {
		t.Fatalf("videos path mode = %d", mode)
	}
}
