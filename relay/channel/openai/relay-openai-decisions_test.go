package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

const openaiDecisionsResponseBody = `{"model":"gpt-6-luna","answers":[{"type":"predicate","name":"damaged","probability":0.95}],"usage":{"input_tokens":42,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0},"output_tokens":0,"output_tokens_details":{"reasoning_tokens":0},"total_tokens":42}}`

func newOpenAIDecisionsTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/decisions", nil)
	return c, recorder
}

func TestOpenAIDecisionsHandlerPassesThroughAndExtractsUsage(t *testing.T) {
	c, recorder := newOpenAIDecisionsTestContext(t)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(openaiDecisionsResponseBody)),
	}

	usage, apiErr := OpenAIDecisionsHandler(c, &relaycommon.RelayInfo{}, resp)
	if apiErr != nil {
		t.Fatalf("handler error: %v", apiErr)
	}

	billingUsage, ok := usage.(*kitdto.Usage)
	if !ok || billingUsage == nil {
		t.Fatalf("usage type = %T", usage)
	}
	if billingUsage.PromptTokens != 42 || billingUsage.TotalTokens != 42 {
		t.Fatalf("usage = %+v, want prompt/total 42", billingUsage)
	}
	if !strings.Contains(recorder.Body.String(), `"damaged"`) {
		t.Fatalf("response body not passed through: %s", recorder.Body.String())
	}
}

func TestOpenAIDecisionsSystemOneHandlerReshapesResponse(t *testing.T) {
	c, recorder := newOpenAIDecisionsTestContext(t)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-6-luna"}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(openaiDecisionsResponseBody)),
	}

	usage, apiErr := OpenAIDecisionsSystemOneHandler(c, info, resp)
	if apiErr != nil {
		t.Fatalf("handler error: %v", apiErr)
	}

	billingUsage, ok := usage.(*kitdto.Usage)
	if !ok || billingUsage == nil {
		t.Fatalf("usage type = %T", usage)
	}
	if billingUsage.PromptTokens != 42 {
		t.Fatalf("usage prompt = %d, want 42", billingUsage.PromptTokens)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`"model":"gpt-6-luna"`,
		`"damaged"`,
		`"type":"noul"`,
		`"noul":0.95`,
		`"input_tokens":42`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("systemone body missing %s: %s", want, body)
		}
	}
}
