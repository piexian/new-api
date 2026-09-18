package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/relay/channel/stepfun"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func newStepFunNativeTestContext(t *testing.T, method, path, contentType, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	return c, recorder
}

func TestIsStepFunStreamResponse(t *testing.T) {
	t.Parallel()

	jsonResp := &http.Response{Header: http.Header{"Content-Type": []string{"application/json"}}}
	if isStepFunStreamResponse(jsonResp) {
		t.Fatal("json response must not be treated as a stream")
	}
	sseResp := &http.Response{Header: http.Header{"Content-Type": []string{"text/event-stream; charset=utf-8"}}}
	if !isStepFunStreamResponse(sseResp) {
		t.Fatal("text/event-stream must be treated as a stream")
	}
	if isStepFunStreamResponse(nil) {
		t.Fatal("nil response must not be a stream")
	}
}

func TestCopyStepFunStreamResponse(t *testing.T) {
	t.Parallel()

	c, recorder := newStepFunNativeTestContext(t, http.MethodPost, "/v1/audio/asr/sse", "application/json", "{}")
	stream := "data: {\"type\":\"transcript.text.delta\",\"delta\":\"你\"}\n\n" +
		"data: {\"type\":\"transcript.text.done\",\"text\":\"你好\"}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}

	if err := copyStepFunStreamResponse(c, resp); err != nil {
		t.Fatalf("copyStepFunStreamResponse: %v", err)
	}
	if got := recorder.Body.String(); got != stream {
		t.Fatalf("SSE body must be forwarded verbatim, got %q", got)
	}
	if ct := recorder.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content type = %q", ct)
	}
}

func TestStepFunNativeRequestBodyStripsRoutingModel(t *testing.T) {
	t.Parallel()

	endpoint, ok := stepfun.LookupNativeEndpoint("/v1/audio/music/submit", http.MethodPost)
	if !ok {
		t.Fatal("music submit endpoint should exist")
	}

	body := `{"model":"stepaudio-3-music-preview","model_id":"stepaudio-3-music-preview","caption":"city pop"}`
	c, _ := newStepFunNativeTestContext(t, http.MethodPost, "/v1/audio/music/submit", "application/json", body)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	reader, closer, err := stepFunNativeRequestBody(c, info, endpoint)
	if err != nil {
		t.Fatalf("stepFunNativeRequestBody: %v", err)
	}
	if closer != nil {
		defer closer.Close()
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(raw), `"model"`) {
		t.Fatalf("routing model must be stripped before forwarding: %s", raw)
	}
	if !strings.Contains(string(raw), `"model_id":"stepaudio-3-music-preview"`) {
		t.Fatalf("model_id must survive: %s", raw)
	}
	if info.UpstreamRequestBodySize != int64(len(raw)) {
		t.Fatalf("upstream body size = %d, want %d", info.UpstreamRequestBodySize, len(raw))
	}
}

func TestStepFunNativeRequestBodyPassthroughWhenNoRewrite(t *testing.T) {
	t.Parallel()

	endpoint, ok := stepfun.LookupNativeEndpoint("/v1/audio/generate", http.MethodPost)
	if !ok {
		t.Fatal("audio generate endpoint should exist")
	}

	body := `{"model":"stepaudio-3-gen-preview","task":"text_to_audio"}`
	c, _ := newStepFunNativeTestContext(t, http.MethodPost, "/v1/audio/generate", "application/json", body)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	reader, closer, err := stepFunNativeRequestBody(c, info, endpoint)
	if err != nil {
		t.Fatalf("stepFunNativeRequestBody: %v", err)
	}
	if closer != nil {
		defer closer.Close()
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(raw) != body {
		t.Fatalf("body must be forwarded untouched, got %s", raw)
	}
}

func TestStepFunNativeUsageFallsBackToMinimalTokens(t *testing.T) {
	t.Parallel()

	usage := stepFunNativeUsage(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	if usage.PromptTokens <= 0 || usage.TotalTokens != usage.PromptTokens {
		t.Fatalf("unexpected usage fallback: %+v", usage)
	}
}
