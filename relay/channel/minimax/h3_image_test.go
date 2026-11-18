package minimax

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	dto "github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
)

func TestGetRequestURLH3ImageModels(t *testing.T) {
	for _, c := range []struct {
		mode   int
		model  string
		suffix string
	}{
		{relayconstant.RelayModeImagesGenerations, "MiniMax-H3-Image", "/v1/images/generations"},
		{relayconstant.RelayModeImagesEdits, "MiniMax-H3-Image-Highspeed", "/v1/images/edits"},
		{relayconstant.RelayModeImagesGenerations, "image-01", "/v1/image_generation"},
	} {
		info := &relaycommon.RelayInfo{
			RelayMode:   c.mode,
			ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.minimax.chat", UpstreamModelName: c.model},
		}
		url, err := GetRequestURL(info)
		if err != nil {
			t.Fatalf("GetRequestURL(%s): %v", c.model, err)
		}
		if !strings.HasSuffix(url, c.suffix) {
			t.Errorf("model %s: url = %s, want suffix %s", c.model, url, c.suffix)
		}
	}
}

func TestOaiImage2H3ImageRequest(t *testing.T) {
	request := dto.ImageRequest{
		Model:  "MiniMax-H3-Image",
		Prompt: "an orange cat",
		Size:   "2720x1536",
		Extra: map[string]json.RawMessage{
			"aspect_ratio":            json.RawMessage(`"16:9"`),
			"area":                    json.RawMessage(`"2K"`),
			"disable_prompt_enhanced": json.RawMessage(`true`),
		},
	}

	converted := oaiImage2H3ImageRequest(request)
	if converted.Model != "MiniMax-H3-Image" || converted.Size != "2720x1536" {
		t.Fatalf("model/size = %s/%s", converted.Model, converted.Size)
	}
	if converted.AspectRatio != "16:9" || converted.Area != "2K" {
		t.Fatalf("aspect/area = %s/%s", converted.AspectRatio, converted.Area)
	}
	if converted.DisablePromptEnhanced == nil || !*converted.DisablePromptEnhanced {
		t.Fatalf("disable_prompt_enhanced = %v", converted.DisablePromptEnhanced)
	}

	// edits 入参归一:字符串数组 → images[].image_url
	images := h3ImagesFromImageInput(json.RawMessage(`["https://a.com/1.png","https://a.com/2.png"]`))
	var parsed []map[string]any
	if err := json.Unmarshal(images, &parsed); err != nil || len(parsed) != 2 {
		t.Fatalf("images normalize = %s", images)
	}
	if parsed[0]["image_url"].(map[string]any)["url"] != "https://a.com/1.png" {
		t.Fatalf("first image = %v", parsed[0])
	}

	// 已是原生形状则原样透传
	native := json.RawMessage(`[{"image_url":{"url":"https://a.com/x.png"}}]`)
	if string(h3ImagesFromImageInput(native)) != string(native) {
		t.Fatal("native images[] shape should pass through unchanged")
	}
}

func TestMiniMaxH3ImageHandlerExtractsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := `{"created":1792308695,"data":[{"url":"https://example.com/out.jpg","size":"2720x1536"}],"output_format":"jpeg","usage":{"generated_images":1,"total_tokens":13022,"input_tokens":4862,"output_tokens":8160,"input_tokens_details":{"text_tokens":4862,"image_tokens":0}},"request_id":"r"}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	usage, apiErr := miniMaxH3ImageHandler(c, resp, &relaycommon.RelayInfo{})
	if apiErr != nil {
		t.Fatalf("handler error: %v", apiErr)
	}
	if usage.PromptTokens != 4862 || usage.CompletionTokens != 8160 || usage.TotalTokens != 13022 {
		t.Fatalf("usage = %+v", usage)
	}
	if usage.PromptTokensDetails.TextTokens != 4862 {
		t.Fatalf("text tokens = %d", usage.PromptTokensDetails.TextTokens)
	}
	if !strings.Contains(recorder.Body.String(), "out.jpg") {
		t.Fatalf("body not passed through: %s", recorder.Body.String())
	}
}

func TestMiniMaxH3ImageStreamHandlerExtractsCompletedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	sse := "data: {\"type\":\"image_generation.partial_succeeded\",\"image_index\":0,\"url\":\"https://example.com/0.jpg\"}\n" +
		"data: {\"type\":\"image_generation.completed\",\"usage\":{\"generated_images\":1,\"total_tokens\":13022,\"input_tokens\":4862,\"output_tokens\":8160,\"input_tokens_details\":{\"text_tokens\":4862,\"image_tokens\":0}}}\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
	}

	usage, apiErr := miniMaxH3ImageStreamHandler(c, resp, &relaycommon.RelayInfo{})
	if apiErr != nil {
		t.Fatalf("stream handler error: %v", apiErr)
	}
	if usage.PromptTokens != 4862 || usage.CompletionTokens != 8160 {
		t.Fatalf("stream usage = %+v", usage)
	}
	if !strings.Contains(recorder.Body.String(), "partial_succeeded") || !strings.Contains(recorder.Body.String(), "completed") {
		t.Fatalf("stream events not forwarded: %s", recorder.Body.String())
	}
}
