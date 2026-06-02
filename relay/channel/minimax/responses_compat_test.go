package minimax

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestConvertOpenAIResponsesRequestPassesThroughOfficialResponsesAPI(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	maxOutputTokens := uint(64)
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		RelayFormat: types.RelayFormatOpenAIResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "MiniMax-M3",
		},
	}
	request := dto.OpenAIResponsesRequest{
		Model:           "MiniMax-M3",
		Input:           []byte(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_video","video_url":{"url":"mm_file://123","detail":"low"}}]}]`),
		MaxOutputTokens: &maxOutputTokens,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest returned error: %v", err)
	}

	got, ok := converted.(*dto.OpenAIResponsesRequest)
	if !ok {
		t.Fatalf("ConvertOpenAIResponsesRequest returned %T, want *dto.OpenAIResponsesRequest", converted)
	}
	if info.FinalRequestRelayFormat != types.RelayFormatOpenAIResponses {
		t.Fatalf("FinalRequestRelayFormat = %q, want %q", info.FinalRequestRelayFormat, types.RelayFormatOpenAIResponses)
	}
	if got.MaxOutputTokens == nil || *got.MaxOutputTokens != maxOutputTokens {
		t.Fatalf("MaxOutputTokens = %#v, want %d", got.MaxOutputTokens, maxOutputTokens)
	}
	if string(got.Input) != string(request.Input) {
		t.Fatalf("Input = %s, want %s", got.Input, request.Input)
	}
}

func TestMiniMaxResponsesInputTokensHandlerCopiesBodyAndReturnsUsage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c := gin.CreateTestContextOnly(recorder, gin.New())
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"object":"response.input_tokens","input_tokens":588}`)),
		Header:     make(http.Header),
	}

	usage, apiErr := miniMaxResponsesInputTokensHandler(c, resp)
	if apiErr != nil {
		t.Fatalf("miniMaxResponsesInputTokensHandler returned error: %v", apiErr)
	}
	if usage == nil || usage.InputTokens != 588 || usage.TotalTokens != 588 {
		t.Fatalf("usage = %#v, want input/total 588", usage)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"input_tokens":588`) {
		t.Fatalf("response body = %s, want original input_tokens payload", body)
	}
}

func TestMiniMaxClaudeCountTokensHandlerCopiesBodyAndReturnsUsage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c := gin.CreateTestContextOnly(recorder, gin.New())
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"usage":{"input_tokens":1209,"cache_creation_input_tokens":7,"cache_read_input_tokens":156}}`)),
		Header:     make(http.Header),
	}

	usage, apiErr := miniMaxClaudeCountTokensHandler(c, resp)
	if apiErr != nil {
		t.Fatalf("miniMaxClaudeCountTokensHandler returned error: %v", apiErr)
	}
	if usage == nil || usage.InputTokens != 1209 || usage.PromptTokens != 1209 || usage.TotalTokens != 1209 {
		t.Fatalf("usage = %#v, want prompt/input/total 1209", usage)
	}
	if usage.UsageSemantic != "anthropic" {
		t.Fatalf("UsageSemantic = %q, want anthropic", usage.UsageSemantic)
	}
	if usage.PromptTokensDetails.CachedTokens != 156 || usage.PromptTokensDetails.CachedCreationTokens != 7 {
		t.Fatalf("cache usage = %#v, want read 156 create 7", usage.PromptTokensDetails)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"input_tokens":1209`) {
		t.Fatalf("response body = %s, want original usage payload", body)
	}
}
