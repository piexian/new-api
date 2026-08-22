package agnes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"github.com/QuantumNous/new-api/service"
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
