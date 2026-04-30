package minimax

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func TestGetRequestURLForImageGeneration(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.minimax.chat",
		},
	}

	got, err := GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	want := "https://api.minimax.chat/v1/image_generation"
	if got != want {
		t.Fatalf("GetRequestURL() = %q, want %q", got, want)
	}
}

func TestGetRequestURLForOfficialCompatibleEndpoints(t *testing.T) {
	t.Parallel()

	chatURL, err := GetRequestURL(&relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.minimaxi.com/v1",
		},
	})
	if err != nil {
		t.Fatalf("GetRequestURL chat returned error: %v", err)
	}
	if chatURL != "https://api.minimaxi.com/v1/chat/completions" {
		t.Fatalf("chatURL = %q", chatURL)
	}

	claudeURL, err := GetRequestURL(&relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://api.minimaxi.com/anthropic",
		},
	})
	if err != nil {
		t.Fatalf("GetRequestURL claude returned error: %v", err)
	}
	if claudeURL != "https://api.minimaxi.com/anthropic/v1/messages" {
		t.Fatalf("claudeURL = %q", claudeURL)
	}
}

func TestConvertImageRequest(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "image-01",
	}
	request := dto.ImageRequest{
		Model:          "image-01",
		Prompt:         "a red fox in snowfall",
		Size:           "1536x1024",
		ResponseFormat: "url",
		N:              uintPtr(2),
	}

	got, err := adaptor.ConvertImageRequest(gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()), info, request)
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}

	if payload["model"] != "image-01" {
		t.Fatalf("model = %#v, want %q", payload["model"], "image-01")
	}
	if payload["prompt"] != request.Prompt {
		t.Fatalf("prompt = %#v, want %q", payload["prompt"], request.Prompt)
	}
	if payload["n"] != float64(2) {
		t.Fatalf("n = %#v, want 2", payload["n"])
	}
	if payload["aspect_ratio"] != "3:2" {
		t.Fatalf("aspect_ratio = %#v, want %q", payload["aspect_ratio"], "3:2")
	}
	if payload["response_format"] != "url" {
		t.Fatalf("response_format = %#v, want %q", payload["response_format"], "url")
	}
}

func TestConvertImageRequestPreservesMiniMaxExtraFields(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "image-01",
	}
	request := dto.ImageRequest{
		Model:  "image-01",
		Prompt: "a portrait",
		ExtraFields: []byte(`{
			"width": 1024,
			"height": 768,
			"seed": 7,
			"prompt_optimizer": true,
			"subject_reference": [{"type": "character", "image_file": "https://example.com/ref.png"}]
		}`),
	}

	got, err := adaptor.ConvertImageRequest(gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()), info, request)
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if payload["width"] != float64(1024) || payload["height"] != float64(768) {
		t.Fatalf("payload size = %#v x %#v", payload["width"], payload["height"])
	}
	if payload["seed"] != float64(7) {
		t.Fatalf("seed = %#v", payload["seed"])
	}
	if payload["prompt_optimizer"] != true {
		t.Fatalf("prompt_optimizer = %#v", payload["prompt_optimizer"])
	}
	if _, ok := payload["subject_reference"].([]any); !ok {
		t.Fatalf("subject_reference = %#v", payload["subject_reference"])
	}
}

func TestConvertImageRequestPassesThroughNativeEndpoint(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		OriginModelName: "image-01",
	}
	body := `{"model":"image-01","prompt":"a portrait","width":1024,"height":768,"prompt_optimizer":false}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image_generation", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	got, err := adaptor.ConvertImageRequest(c, info, dto.ImageRequest{Model: "image-01", Prompt: "a portrait"})
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}
	buf, ok := got.(*bytes.Buffer)
	if !ok {
		t.Fatalf("ConvertImageRequest returned %T, want *bytes.Buffer", got)
	}
	if buf.String() != body {
		t.Fatalf("body = %s, want %s", buf.String(), body)
	}
}

func TestPath2RelayModeSupportsMiniMaxNativeImageEndpoint(t *testing.T) {
	t.Parallel()

	got := relayconstant.Path2RelayMode("/v1/image_generation")
	if got != relayconstant.RelayModeImagesGenerations {
		t.Fatalf("Path2RelayMode = %d, want %d", got, relayconstant.RelayModeImagesGenerations)
	}
}

func TestDoResponseForImageGeneration(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		StartTime: time.Unix(1700000000, 0),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       httptest.NewRecorder().Result().Body,
	}
	resp.Body = ioNopCloser(`{"data":{"image_urls":["https://example.com/minimax.png"]}}`)

	adaptor := &Adaptor{}
	usage, err := adaptor.DoResponse(c, resp, info)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}
	if usage == nil {
		t.Fatalf("DoResponse returned nil usage")
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"url":"https://example.com/minimax.png"`) {
		t.Fatalf("response body = %s, want OpenAI image response with image URL", body)
	}
	if strings.Contains(body, `"image_urls"`) {
		t.Fatalf("response body = %s, should not expose raw MiniMax image_urls payload", body)
	}
}

func TestDoResponseForNativeImageEndpointPassesMiniMaxBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/image_generation", nil)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		StartTime: time.Unix(1700000000, 0),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       ioNopCloser(`{"data":{"image_urls":["https://example.com/minimax.png"]},"base_resp":{"status_code":0,"status_msg":"success"}}`),
	}
	resp.Header.Set("Content-Type", "application/json")

	adaptor := &Adaptor{}
	usage, err := adaptor.DoResponse(c, resp, info)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}
	if usage == nil {
		t.Fatalf("DoResponse returned nil usage")
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `"image_urls":["https://example.com/minimax.png"]`) {
		t.Fatalf("response body = %s, want native MiniMax image_urls payload", body)
	}
	if strings.Contains(body, `"url":"https://example.com/minimax.png"`) {
		t.Fatalf("response body = %s, should not convert native MiniMax payload", body)
	}
}

func TestDoResponseForChatCompletionsMapsMiniMaxBaseRespError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		IsStream:    true,
		StartTime:   time.Unix(1700000000, 0),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")
	resp.Body = ioNopCloser(`{"base_resp":{"status_code":2056,"status_msg":"usage limit exceeded (2056)"}}`)

	adaptor := &Adaptor{}
	usage, err := adaptor.DoResponse(c, resp, info)
	if err == nil {
		t.Fatalf("DoResponse returned nil error")
	}
	if usage != nil {
		t.Fatalf("DoResponse returned unexpected usage: %#v", usage)
	}
	if err.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusTooManyRequests)
	}
	if err.ToOpenAIError().Message != "usage limit exceeded (2056)" {
		t.Fatalf("message = %q, want %q", err.ToOpenAIError().Message, "usage limit exceeded (2056)")
	}
}

func TestDoResponseForChatCompletionsRejectsEmptyBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: types.RelayFormatOpenAI,
		IsStream:    true,
		StartTime:   time.Unix(1700000000, 0),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")
	resp.Body = ioNopCloser(``)

	adaptor := &Adaptor{}
	usage, err := adaptor.DoResponse(c, resp, info)
	if err == nil {
		t.Fatalf("DoResponse returned nil error")
	}
	if usage != nil {
		t.Fatalf("DoResponse returned unexpected usage: %#v", usage)
	}
	if err.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusInternalServerError)
	}
	if err.GetErrorCode() != types.ErrorCodeEmptyResponse {
		t.Fatalf("ErrorCode = %q, want %q", err.GetErrorCode(), types.ErrorCodeEmptyResponse)
	}
}

func TestDoResponseForChatCompletionsPassesOpenAIJSONBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "minimax-m2.7",
		StartTime:       time.Unix(1700000000, 0),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "minimax-m2.7",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")
	resp.Body = ioNopCloser(`{"id":"chatcmpl-1","object":"chat.completion","created":1700000000,"model":"minimax-m2.7","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)

	adaptor := &Adaptor{}
	usage, err := adaptor.DoResponse(c, resp, info)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}
	if usage == nil {
		t.Fatalf("DoResponse returned nil usage")
	}
	if usage.(*dto.Usage).TotalTokens != 2 {
		t.Fatalf("TotalTokens = %d, want 2", usage.(*dto.Usage).TotalTokens)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"content":"hello"`) {
		t.Fatalf("response body = %s, want assistant content", body)
	}
}

type nopReadCloser struct {
	*strings.Reader
}

func (n nopReadCloser) Close() error {
	return nil
}

func ioNopCloser(body string) nopReadCloser {
	return nopReadCloser{Reader: strings.NewReader(body)}
}

func uintPtr(v uint) *uint {
	return &v
}
