package gemini

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

const decisionsResponseBody = `{"answers":{"needs_billing_refund":{"type":"noul","confidence":0.97,"noul":0.97}},"usage":{"input_tokens_by_modality":[{"modality":"text","tokens":152}],"total_input_tokens":152,"total_output_tokens":0,"total_tokens":152}}`

func newDecisionsTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1beta/decisions", nil)
	return c, recorder
}

func TestGeminiDecisionsHandlerPassesThroughAndExtractsUsage(t *testing.T) {
	c, recorder := newDecisionsTestContext(t)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(decisionsResponseBody)),
	}

	usage, apiErr := GeminiDecisionsHandler(c, &relaycommon.RelayInfo{}, resp)
	if apiErr != nil {
		t.Fatalf("handler error: %v", apiErr)
	}

	billingUsage, ok := usage.(*kitdto.Usage)
	if !ok || billingUsage == nil {
		t.Fatalf("usage type = %T", usage)
	}
	if billingUsage.PromptTokens != 152 || billingUsage.TotalTokens != 152 {
		t.Fatalf("usage = %+v, want prompt/total 152", billingUsage)
	}
	if !strings.Contains(recorder.Body.String(), `"needs_billing_refund"`) {
		t.Fatalf("response body not passed through: %s", recorder.Body.String())
	}
}

func TestGeminiDecisionsSystemOneHandlerReshapesResponse(t *testing.T) {
	c, recorder := newDecisionsTestContext(t)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gemini-decision-1"}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(decisionsResponseBody)),
	}

	usage, apiErr := GeminiDecisionsSystemOneHandler(c, info, resp)
	if apiErr != nil {
		t.Fatalf("handler error: %v", apiErr)
	}

	billingUsage, ok := usage.(*kitdto.Usage)
	if !ok || billingUsage == nil {
		t.Fatalf("usage type = %T", usage)
	}
	if billingUsage.PromptTokens != 152 {
		t.Fatalf("usage prompt = %d, want 152", billingUsage.PromptTokens)
	}

	body := recorder.Body.String()
	for _, want := range []string{
		`"model":"gemini-decision-1"`,
		`"needs_billing_refund"`,
		`"input_tokens":152`,
		`"output_tokens":0`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("systemone body missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "total_input_tokens") {
		t.Fatalf("systemone body should not carry decisions usage keys: %s", body)
	}
}
