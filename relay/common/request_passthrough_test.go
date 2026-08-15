package common

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newPassthroughTestContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func mappedInfo(mapped bool, upstreamModel string) *RelayInfo {
	return &RelayInfo{
		ChannelMeta: &ChannelMeta{
			IsModelMapped:     mapped,
			UpstreamModelName: upstreamModel,
		},
	}
}

func readAll(t *testing.T, r io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}

func TestPassThroughRequestBodyUnmappedKeepsRawBody(t *testing.T) {
	raw := `{"model":"alias-model","messages":[{"role":"user","content":"hi"}]}`
	reader, size, err := PassThroughRequestBody(newPassthroughTestContext(raw), mappedInfo(false, "alias-model"))
	if err != nil {
		t.Fatalf("pass through: %v", err)
	}
	if got := readAll(t, reader); got != raw {
		t.Fatalf("body changed: %s", got)
	}
	if size != int64(len(raw)) {
		t.Fatalf("size = %d, want %d", size, len(raw))
	}
}

func TestPassThroughRequestBodyRewritesMappedModel(t *testing.T) {
	raw := `{"model":"alias-model","messages":[{"role":"user","content":"hi"}]}`
	reader, _, err := PassThroughRequestBody(newPassthroughTestContext(raw), mappedInfo(true, "real-upstream-model"))
	if err != nil {
		t.Fatalf("pass through: %v", err)
	}
	got := readAll(t, reader)
	if !strings.Contains(got, `"model":"real-upstream-model"`) {
		t.Fatalf("model not rewritten: %s", got)
	}
	if strings.Contains(got, "alias-model") {
		t.Fatalf("original model leaked: %s", got)
	}
	if !strings.Contains(got, `"messages"`) {
		t.Fatalf("other fields lost: %s", got)
	}
}

func TestPassThroughRequestBodyNonJSONPassthrough(t *testing.T) {
	raw := `not-json-body`
	reader, _, err := PassThroughRequestBody(newPassthroughTestContext(raw), mappedInfo(true, "real-upstream-model"))
	if err != nil {
		t.Fatalf("pass through: %v", err)
	}
	if got := readAll(t, reader); got != raw {
		t.Fatalf("non-json body changed: %s", got)
	}
}

func TestPassThroughRequestBodyNoModelField(t *testing.T) {
	raw := `{"input":"hello"}`
	reader, _, err := PassThroughRequestBody(newPassthroughTestContext(raw), mappedInfo(true, "real-upstream-model"))
	if err != nil {
		t.Fatalf("pass through: %v", err)
	}
	if got := readAll(t, reader); got != raw {
		t.Fatalf("body without model changed: %s", got)
	}
}
