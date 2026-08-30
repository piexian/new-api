package agnes

import (
	"github.com/QuantumNous/new-api/service"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchTaskPrefersAgnesapiWhenVideoIDPresent(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"completed","progress":100,"url":"https://cos.example.com/v.mp4"}`))
	}))
	defer srv.Close()

	service.InitHttpClient()

	a := &TaskAdaptor{}
	resp, err := a.FetchTask(srv.URL, "sk-test", map[string]any{
		"task_id":  "task_abc",
		"video_id": "video_xyz",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if !strings.Contains(gotURI, "/agnesapi") || !strings.Contains(gotURI, "video_id=video_xyz") {
		t.Fatalf("expected /agnesapi?video_id=..., got %q", gotURI)
	}

	resp, err = a.FetchTask(srv.URL, "sk-test", map[string]any{"task_id": "task_abc"}, "")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if !strings.Contains(gotURI, "/v1/videos/task_abc") {
		t.Fatalf("expected legacy path fallback, got %q", gotURI)
	}
}

func TestFetchTaskAppendsModelNameForAgnesapi(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"completed","progress":100,"url":"https://cos.example.com/v.mp4"}`))
	}))
	defer srv.Close()

	service.InitHttpClient()

	a := &TaskAdaptor{}
	// 有 model_name 时按文档推荐同时携带 video_id 与 model_name
	resp, err := a.FetchTask(srv.URL, "sk-test", map[string]any{
		"task_id":    "task_abc",
		"video_id":   "video_xyz",
		"model_name": "agnes-video-2.5-flash",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if !strings.Contains(gotURI, "video_id=video_xyz") ||
		!strings.Contains(gotURI, "model_name=agnes-video-2.5-flash") {
		t.Fatalf("expected video_id and model_name query params, got %q", gotURI)
	}

	// 缺 model_name 的旧任务保持 video_id 单参查询
	resp, err = a.FetchTask(srv.URL, "sk-test", map[string]any{
		"task_id":  "task_abc",
		"video_id": "video_xyz",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if strings.Contains(gotURI, "model_name=") {
		t.Fatalf("did not expect model_name query param, got %q", gotURI)
	}
}
