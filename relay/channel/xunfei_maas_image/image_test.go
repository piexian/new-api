package xunfei_maas_image

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
)

func TestGetRequestURLForImageGeneration(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeXunfeiMaaSImage,
			ChannelBaseUrl: "https://maas-api.cn-huabei-1.xf-yun.com/v2",
			ApiKey:         "app-123|key-456|secret-789",
		},
	}

	got, err := adaptor.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}

	parsedURL, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}
	if parsedURL.Scheme != "https" || parsedURL.Host != "maas-api.cn-huabei-1.xf-yun.com" {
		t.Fatalf("unexpected host: %s", got)
	}
	if parsedURL.Path != "/v2.1/tti" {
		t.Fatalf("path = %q, want %q", parsedURL.Path, "/v2.1/tti")
	}
	if parsedURL.Query().Get("host") != "maas-api.cn-huabei-1.xf-yun.com" {
		t.Fatalf("host query = %q, want %q", parsedURL.Query().Get("host"), "maas-api.cn-huabei-1.xf-yun.com")
	}
	if parsedURL.Query().Get("date") == "" {
		t.Fatal("date query is empty")
	}
	if parsedURL.Query().Get("authorization") == "" {
		t.Fatal("authorization query is empty")
	}
}

func TestConvertImageRequest(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeXunfeiMaaSImage,
			ApiKey:      "app-123|key-456|secret-789",
		},
	}
	request := dto.ImageRequest{
		Model:          "kolors",
		Prompt:         "draw a mountain",
		Size:           "1024x768",
		ResponseFormat: "b64_json",
		User:           []byte(`"user-1"`),
		Extra: map[string]json.RawMessage{
			"seed":                []byte(`7`),
			"num_inference_steps": []byte(`28`),
			"guidance_scale":      []byte(`6.5`),
			"scheduler":           []byte(`"Euler"`),
			"negative_prompt":     []byte(`"low quality"`),
			"patch_id":            []byte(`["patch-a"]`),
		},
	}

	got, err := adaptor.ConvertImageRequest(gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()), info, request)
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}

	payload, ok := got.(*imageRequest)
	if !ok {
		t.Fatalf("converted payload type = %T, want *imageRequest", got)
	}
	if payload.Header.AppID != "app-123" {
		t.Fatalf("app_id = %q, want %q", payload.Header.AppID, "app-123")
	}
	if payload.Header.UID != "user-1" {
		t.Fatalf("uid = %q, want %q", payload.Header.UID, "user-1")
	}
	if len(payload.Header.PatchID) != 1 || payload.Header.PatchID[0] != "patch-a" {
		t.Fatalf("patch_id = %#v, want []string{\"patch-a\"}", payload.Header.PatchID)
	}
	if payload.Parameter.Chat.Domain != "kolors" {
		t.Fatalf("domain = %q, want %q", payload.Parameter.Chat.Domain, "kolors")
	}
	if payload.Parameter.Chat.Width != 1024 || payload.Parameter.Chat.Height != 768 {
		t.Fatalf("size = %dx%d, want 1024x768", payload.Parameter.Chat.Width, payload.Parameter.Chat.Height)
	}
	if payload.Parameter.Chat.Seed != 7 {
		t.Fatalf("seed = %d, want 7", payload.Parameter.Chat.Seed)
	}
	if payload.Parameter.Chat.NumInferenceSteps != 28 {
		t.Fatalf("num_inference_steps = %d, want 28", payload.Parameter.Chat.NumInferenceSteps)
	}
	if payload.Parameter.Chat.GuidanceScale != 6.5 {
		t.Fatalf("guidance_scale = %v, want 6.5", payload.Parameter.Chat.GuidanceScale)
	}
	if payload.Parameter.Chat.Scheduler != "Euler" {
		t.Fatalf("scheduler = %q, want %q", payload.Parameter.Chat.Scheduler, "Euler")
	}
	if len(payload.Payload.Message.Text) != 1 || payload.Payload.Message.Text[0].Content != "draw a mountain" {
		t.Fatalf("message text = %#v, want prompt", payload.Payload.Message.Text)
	}
	if payload.Payload.NegativePrompts == nil || payload.Payload.NegativePrompts.Text != "low quality" {
		t.Fatalf("negative_prompts = %#v, want low quality", payload.Payload.NegativePrompts)
	}
}

func TestConvertImageRequestUsesContextPatchID(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeXunfeiMaaSImage,
			ApiKey:      "app-123|key-456|secret-789",
		},
	}
	request := dto.ImageRequest{
		Model:  "kolors",
		Prompt: "draw a mountain",
	}
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Set("patch_id", "patch-from-channel")

	got, err := adaptor.ConvertImageRequest(c, info, request)
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}

	payload, ok := got.(*imageRequest)
	if !ok {
		t.Fatalf("converted payload type = %T, want *imageRequest", got)
	}
	if len(payload.Header.PatchID) != 1 || payload.Header.PatchID[0] != "patch-from-channel" {
		t.Fatalf("patch_id = %#v, want []string{\"patch-from-channel\"}", payload.Header.PatchID)
	}
}

func TestSetupRequestHeader(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeXunfeiMaaSImage,
			ApiKey:      "app-123|key-456|secret-789",
		},
	}
	header := make(http.Header)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	c := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	c.Request = req

	if err := adaptor.SetupRequestHeader(c, &header, info); err != nil {
		t.Fatalf("SetupRequestHeader returned error: %v", err)
	}
	if got := header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
	if got := header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
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
		Request: &dto.ImageRequest{
			Model:          "kolors",
			ResponseFormat: "b64_json",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeXunfeiMaaSImage,
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`{"header":{"code":0,"message":"Success","sid":"sid-1","status":2},"payload":{"choices":{"status":2,"seq":0,"text":[{"content":"ZmFrZS1pbWFnZQ==","index":0,"role":"assistant"}]}}}`,
		)),
	}

	adaptor := &Adaptor{}
	usage, err := adaptor.DoResponse(c, resp, info)
	if err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}
	if usage == nil {
		t.Fatal("DoResponse returned nil usage")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"b64_json":"ZmFrZS1pbWFnZQ=="`) {
		t.Fatalf("response body = %s, want OpenAI image response with b64_json", body)
	}
	if strings.Contains(body, `"payload"`) {
		t.Fatalf("response body = %s, should not expose raw xunfei payload", body)
	}
}

func TestDoResponseForImageGenerationURLModeUsesDataURL(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		StartTime: time.Unix(1700000000, 0),
		Request: &dto.ImageRequest{
			Model:          "kolors",
			ResponseFormat: "url",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeXunfeiMaaSImage,
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`{"header":{"code":0,"message":"Success","sid":"sid-2","status":2},"payload":{"choices":{"status":2,"seq":0,"text":[{"content":"ZmFrZS1pbWFnZQ==","index":0,"role":"assistant"}]}}}`,
		)),
	}

	adaptor := &Adaptor{}
	if _, err := adaptor.DoResponse(c, resp, info); err != nil {
		t.Fatalf("DoResponse returned error: %v", err)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"url":"data:image/png;base64,ZmFrZS1pbWFnZQ=="`) {
		t.Fatalf("response body = %s, want data URL in url field", body)
	}
}
