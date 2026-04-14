package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
)

func TestGetAndValidateTextRequestRejectsCompactModelOnChatCompletions(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-5-openai-compact","messages":[{"role":"user","content":"hi"}]}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	_, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
	if err == nil {
		t.Fatal("expected compact model to be rejected on chat completions")
	}
	if !strings.Contains(err.Error(), "/v1/responses/compact") {
		t.Fatalf("expected compact endpoint hint, got %v", err)
	}
}

func TestGetAndValidateResponsesRequestRejectsCompactModelOnResponses(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1/responses",
		strings.NewReader(`{"model":"gpt-5-openai-compact","input":"hi"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	_, err := GetAndValidateResponsesRequest(c)
	if err == nil {
		t.Fatal("expected compact model to be rejected on responses")
	}
	if !strings.Contains(err.Error(), "/v1/responses/compact") {
		t.Fatalf("expected compact endpoint hint, got %v", err)
	}
}
