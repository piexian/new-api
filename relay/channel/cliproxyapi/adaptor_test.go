package cliproxyapi

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func newTestInfo(baseURL, requestURLPath, upstreamModel string, supportStreamOptions bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RequestURLPath: requestURLPath,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:       baseURL,
			UpstreamModelName:    upstreamModel,
			SupportStreamOptions: supportStreamOptions,
		},
	}
}

func TestGetRequestURLPassthrough(t *testing.T) {
	a := &Adaptor{}
	paths := []string{
		"/v1/chat/completions",
		"/v1/messages",
		"/v1/responses",
		"/v1beta/interactions",
		"/v1/images/generations",
	}
	for _, p := range paths {
		info := newTestInfo("http://localhost:8317", p, "gpt-5.6", false)
		got, err := a.GetRequestURL(info)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", p, err)
		}
		if want := "http://localhost:8317" + p; got != want {
			t.Errorf("%s: got %q, want %q", p, got, want)
		}
	}
}

func TestGetRequestURLGeminiModelReplacement(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("http://localhost:8317", "/v1beta/models/gemini-old:streamGenerateContent?alt=sse", "gemini-new", false)
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "http://localhost:8317/v1beta/models/gemini-new:streamGenerateContent?alt=sse"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetRequestURLGeminiPlaceholderReplacement(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("http://localhost:8317", "/v1beta/models/{model}:generateContent", "gemini-3.1-pro", false)
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "http://localhost:8317/v1beta/models/gemini-3.1-pro:generateContent"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetRequestURLEmptyBaseError(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("", "/v1/chat/completions", "gpt-5.6", false)
	if _, err := a.GetRequestURL(info); err == nil {
		t.Fatal("expected error for empty base URL, got nil")
	}
}

func TestConvertOpenAIRequestStreamOptionsInjection(t *testing.T) {
	a := &Adaptor{}
	stream := true

	// 流式 + SupportStreamOptions + 无 StreamOptions:注入 IncludeUsage
	info := newTestInfo("http://localhost:8317", "/v1/chat/completions", "gpt-5.6", true)
	info.RelayFormat = types.RelayFormatOpenAI
	req := &dto.GeneralOpenAIRequest{Model: "gpt-5.6", Stream: &stream}
	out, err := a.ConvertOpenAIRequest(nil, info, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	converted, ok := out.(*dto.GeneralOpenAIRequest)
	if !ok {
		t.Fatalf("expected *dto.GeneralOpenAIRequest, got %T", out)
	}
	if converted != req {
		t.Error("expected the same request object back")
	}
	if converted.StreamOptions == nil || !converted.StreamOptions.IncludeUsage {
		t.Error("expected StreamOptions.IncludeUsage to be injected for streaming request")
	}

	// 显式 StreamOptions:保持原样
	explicit := &dto.StreamOptions{IncludeUsage: false}
	req2 := &dto.GeneralOpenAIRequest{Model: "gpt-5.6", Stream: &stream, StreamOptions: explicit}
	if _, err := a.ConvertOpenAIRequest(nil, info, req2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req2.StreamOptions != explicit || req2.StreamOptions.IncludeUsage {
		t.Error("expected explicit StreamOptions to be preserved")
	}

	// SupportStreamOptions=false:不注入
	info3 := newTestInfo("http://localhost:8317", "/v1/chat/completions", "gpt-5.6", false)
	info3.RelayFormat = types.RelayFormatOpenAI
	req3 := &dto.GeneralOpenAIRequest{Model: "gpt-5.6", Stream: &stream}
	if _, err := a.ConvertOpenAIRequest(nil, info3, req3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req3.StreamOptions != nil {
		t.Error("expected no StreamOptions injection when SupportStreamOptions is false")
	}

	// 非流式:保持原样
	info4 := newTestInfo("http://localhost:8317", "/v1/chat/completions", "gpt-5.6", true)
	info4.RelayFormat = types.RelayFormatOpenAI
	req4 := &dto.GeneralOpenAIRequest{Model: "gpt-5.6"}
	if _, err := a.ConvertOpenAIRequest(nil, info4, req4); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req4.StreamOptions != nil {
		t.Error("expected non-stream request to be returned unchanged")
	}
}

func TestConvertOpenAIRequestNil(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("http://localhost:8317", "/v1/chat/completions", "gpt-5.6", false)
	if _, err := a.ConvertOpenAIRequest(nil, info, nil); err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

func TestConvertPassthroughRequestsReturnSameObject(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("http://localhost:8317", "/v1/messages", "claude-fable-5", false)

	claudeReq := &dto.ClaudeRequest{Model: "claude-fable-5"}
	out, err := a.ConvertClaudeRequest(nil, info, claudeReq)
	if err != nil {
		t.Fatalf("ConvertClaudeRequest: %v", err)
	}
	if out != any(claudeReq) {
		t.Error("ConvertClaudeRequest should return the same object")
	}

	geminiReq := &dto.GeminiChatRequest{}
	out, err = a.ConvertGeminiRequest(nil, info, geminiReq)
	if err != nil {
		t.Fatalf("ConvertGeminiRequest: %v", err)
	}
	if out != any(geminiReq) {
		t.Error("ConvertGeminiRequest should return the same object")
	}

	responsesReq := dto.OpenAIResponsesRequest{Model: "gpt-5.6"}
	out, err = a.ConvertOpenAIResponsesRequest(nil, info, responsesReq)
	if err != nil {
		t.Fatalf("ConvertOpenAIResponsesRequest: %v", err)
	}
	if !reflect.DeepEqual(out, responsesReq) {
		t.Error("ConvertOpenAIResponsesRequest should return the same value")
	}

	imageReq := dto.ImageRequest{Model: "gpt-image-1", Prompt: "a cat"}
	out, err = a.ConvertImageRequest(nil, info, imageReq)
	if err != nil {
		t.Fatalf("ConvertImageRequest: %v", err)
	}
	if !reflect.DeepEqual(out, imageReq) {
		t.Error("ConvertImageRequest should return the same value")
	}
}

func TestConvertUnsupportedEndpointsError(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("http://localhost:8317", "/v1/embeddings", "text-embedding-3", false)

	if _, err := a.ConvertEmbeddingRequest(nil, info, dto.EmbeddingRequest{}); err == nil {
		t.Error("expected error for embeddings")
	}
	if _, err := a.ConvertAudioRequest(nil, info, dto.AudioRequest{}); err == nil {
		t.Error("expected error for audio")
	}
	if _, err := a.ConvertRerankRequest(nil, 0, dto.RerankRequest{}); err == nil {
		t.Error("expected error for rerank")
	}
}

func TestGetRequestURLGeminiStreamForcesSSE(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("http://localhost:8317", "/v1beta/models/gemini-3.1-pro:streamGenerateContent", "gemini-3.1-pro", false)
	info.RelayMode = relayconstant.RelayModeGemini
	info.IsStream = true
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "http://localhost:8317/v1beta/models/gemini-3.1-pro:streamGenerateContent?alt=sse"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !info.DisablePing {
		t.Error("expected DisablePing to be set for native gemini stream")
	}
}

func TestGetRequestURLGeminiStreamKeepsExistingAlt(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("http://localhost:8317", "/v1beta/models/gemini-3.1-pro:streamGenerateContent?alt=sse", "gemini-3.1-pro", false)
	info.RelayMode = relayconstant.RelayModeGemini
	info.IsStream = true
	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "http://localhost:8317/v1beta/models/gemini-3.1-pro:streamGenerateContent?alt=sse"
	if got != want {
		t.Errorf("got %q, want %q (alt should not be duplicated)", got, want)
	}
}

func TestConvertImageRequestEditsJSONPassthrough(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo("http://localhost:8317", "/v1/images/edits", "gpt-image-1", false)
	info.RelayMode = relayconstant.RelayModeImagesEdits
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	imageReq := dto.ImageRequest{Model: "gpt-image-1", Prompt: "make it brighter"}
	out, err := a.ConvertImageRequest(c, info, imageReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(out, imageReq) {
		t.Error("JSON edits request should pass through unchanged")
	}
}
