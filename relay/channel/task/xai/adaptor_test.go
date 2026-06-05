package xai

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestURLSelectsXAIVideoEndpoint(t *testing.T) {
	t.Parallel()

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.x.ai/"},
	})

	for _, test := range []struct {
		name   string
		action string
		want   string
	}{
		{name: "generate", action: actionVideoGenerate, want: "https://api.x.ai/v1/videos/generations"},
		{name: "edit", action: actionVideoEdit, want: "https://api.x.ai/v1/videos/edits"},
		{name: "extend", action: actionVideoExtend, want: "https://api.x.ai/v1/videos/extensions"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := adaptor.BuildRequestURL(newRelayInfoWithModel(test.action, "grok-imagine-video"))

			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestBuildRequestURLRejectsNonXAIVideoModel(t *testing.T) {
	t.Parallel()

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.x.ai/"},
	})

	_, err := adaptor.BuildRequestURL(newRelayInfoWithModel(actionVideoGenerate, "grok-imagine-image"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a video model")
}

func TestResolveActionUsesNativeXAIVideoPaths(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		path string
		want string
	}{
		{path: "/v1/videos/generations", want: actionVideoGenerate},
		{path: "/v1/videos/edits", want: actionVideoEdit},
		{path: "/v1/videos/extensions", want: actionVideoExtend},
	} {
		test := test
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, test.path, nil)

			require.Equal(t, test.want, resolveAction(c, relaycommon.TaskSubmitReq{}))
		})
	}
}

func TestConvertToRequestPayloadPreservesNativeGenerationFields(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the waterfall move",
		Size:   "1280x720",
		Image:  "https://example.com/still.png",
		Metadata: map[string]interface{}{
			"resolution": "720p",
			"seed":       float64(7),
		},
	}

	body, err := convertToRequestPayload(req, newRelayInfo(actionVideoGenerate))

	require.NoError(t, err)
	require.Equal(t, "grok-imagine-video", body["model"])
	require.Equal(t, "make the waterfall move", body["prompt"])
	require.Equal(t, "16:9", body["aspect_ratio"])
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, float64(7), body["seed"])
	image, ok := body["image"].(map[string]any)
	require.True(t, ok, "image = %T", body["image"])
	require.Equal(t, "https://example.com/still.png", image["url"])
}

func TestConvertToRequestPayloadRequiresImageForPreviewModel(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the waterfall move",
	}

	_, err := convertToRequestPayload(req, newRelayInfoWithModel(actionVideoGenerate, "grok-imagine-video-1.5-preview"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "image is required")
}

func TestConvertToRequestPayloadAllowsPreviewModelWithImage(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the waterfall move",
		Image:  "https://example.com/still.png",
	}

	body, err := convertToRequestPayload(req, newRelayInfoWithModel(actionVideoGenerate, "grok-imagine-video-1.5-preview"))

	require.NoError(t, err)
	image, ok := body["image"].(map[string]any)
	require.True(t, ok, "image = %T", body["image"])
	require.Equal(t, "https://example.com/still.png", image["url"])
}

func TestConvertToRequestPayloadNormalizesPreviewImageURLField(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the waterfall move",
		Metadata: map[string]interface{}{
			"image_url": "https://example.com/still.png",
		},
	}

	body, err := convertToRequestPayload(req, newRelayInfoWithModel(actionVideoGenerate, "grok-imagine-video-1.5-preview"))

	require.NoError(t, err)
	require.NotContains(t, body, "image_url")
	image, ok := body["image"].(map[string]any)
	require.True(t, ok, "image = %T", body["image"])
	require.Equal(t, "https://example.com/still.png", image["url"])
}

func TestConvertToRequestPayloadAllowsPreviewAliasWithImageObject(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the waterfall move",
		Metadata: map[string]interface{}{
			"image": map[string]interface{}{"url": "https://example.com/still.png"},
		},
	}

	body, err := convertToRequestPayload(req, newRelayInfoWithModel(actionVideoGenerate, "grok-imagine-video-1.5-2026-05-30"))

	require.NoError(t, err)
	image, ok := body["image"].(map[string]any)
	require.True(t, ok, "image = %T", body["image"])
	require.Equal(t, "https://example.com/still.png", image["url"])
}

func TestConvertToRequestPayloadPassesThroughReferenceImages(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the runway scene match",
		Metadata: map[string]interface{}{
			"reference_images": []interface{}{
				map[string]interface{}{"url": "https://example.com/ref-1.png"},
				map[string]interface{}{"url": "https://example.com/ref-2.png"},
			},
		},
	}

	body, err := convertToRequestPayload(req, newRelayInfo(actionVideoGenerate))

	require.NoError(t, err)
	refs, ok := body["reference_images"].([]map[string]any)
	require.True(t, ok, "reference_images = %T", body["reference_images"])
	require.Len(t, refs, 2)
	require.Equal(t, "https://example.com/ref-1.png", refs[0]["url"])
	require.Equal(t, "https://example.com/ref-2.png", refs[1]["url"])
	require.NotContains(t, body, "image")
	require.NotContains(t, body, "image_url")
}

func TestConvertToRequestPayloadConvertsImagesToReferenceImages(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the runway scene match",
		Images: []string{
			"https://example.com/ref-1.png",
			"https://example.com/ref-2.png",
		},
	}

	body, err := convertToRequestPayload(req, newRelayInfo(actionVideoGenerate))

	require.NoError(t, err)
	refs, ok := body["reference_images"].([]map[string]any)
	require.True(t, ok, "reference_images = %T", body["reference_images"])
	require.Len(t, refs, 2)
	require.Equal(t, "https://example.com/ref-1.png", refs[0]["url"])
	require.Equal(t, "https://example.com/ref-2.png", refs[1]["url"])
	require.NotContains(t, body, "image")
}

func TestConvertToRequestPayloadAcceptsReferenceImageURLsAlias(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the runway scene match",
		Metadata: map[string]interface{}{
			"reference_image_urls": []interface{}{
				"https://example.com/ref-1.png",
				"https://example.com/ref-2.png",
			},
		},
	}

	body, err := convertToRequestPayload(req, newRelayInfo(actionVideoGenerate))

	require.NoError(t, err)
	refs, ok := body["reference_images"].([]map[string]any)
	require.True(t, ok, "reference_images = %T", body["reference_images"])
	require.Len(t, refs, 2)
	require.Equal(t, "https://example.com/ref-1.png", refs[0]["url"])
	require.Equal(t, "https://example.com/ref-2.png", refs[1]["url"])
	require.NotContains(t, body, "reference_image_urls")
}

func TestConvertToRequestPayloadRejectsTooManyReferenceImages(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "make the runway scene match",
		Images: []string{
			"https://example.com/ref-1.png",
			"https://example.com/ref-2.png",
			"https://example.com/ref-3.png",
			"https://example.com/ref-4.png",
			"https://example.com/ref-5.png",
			"https://example.com/ref-6.png",
			"https://example.com/ref-7.png",
			"https://example.com/ref-8.png",
		},
	}

	_, err := convertToRequestPayload(req, newRelayInfo(actionVideoGenerate))

	require.Error(t, err)
	require.Contains(t, err.Error(), "reference_images cannot contain more than 7 images")
}

func TestConvertToRequestPayloadNormalizesVideoStringForEdit(t *testing.T) {
	t.Parallel()

	req := relaycommon.TaskSubmitReq{
		Prompt: "add snow",
		Metadata: map[string]interface{}{
			"video":      "https://example.com/source.mp4",
			"duration":   float64(6),
			"resolution": "720p",
		},
	}

	body, err := convertToRequestPayload(req, newRelayInfo(actionVideoEdit))

	require.NoError(t, err)
	video, ok := body["video"].(map[string]any)
	require.True(t, ok, "video = %T", body["video"])
	require.Equal(t, "https://example.com/source.mp4", video["url"])
	require.Equal(t, float64(6), body["duration"])
	require.Equal(t, "720p", body["resolution"])
}

func TestValidateRequestAndSetActionStoresReferenceImages(t *testing.T) {
	t.Parallel()

	body := `{
		"model": "grok-imagine-video",
		"prompt": "match the runway scene",
		"reference_images": [
			{"url": "https://example.com/ref-1.png"},
			{"url": "https://example.com/ref-2.png"}
		],
		"duration": 10
	}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	info := newRelayInfo("")

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)

	require.Nil(t, taskErr)
	require.Equal(t, actionVideoGenerate, info.Action)
	stored, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	refs, ok := stored.Metadata["reference_images"].([]interface{})
	require.True(t, ok, "reference_images = %T", stored.Metadata["reference_images"])
	require.Len(t, refs, 2)
}

func TestParseTaskResultUsesXAIProgressAndResultURL(t *testing.T) {
	t.Parallel()

	adaptor := &TaskAdaptor{}

	inProgress, err := adaptor.ParseTaskResult([]byte(`{"status":"processing","progress":72}`))
	require.NoError(t, err)
	require.Equal(t, model.TaskStatusInProgress, inProgress.Status)
	require.Equal(t, "72%", inProgress.Progress)

	done, err := adaptor.ParseTaskResult([]byte(`{
		"status": "done",
		"progress": 100,
		"video": {"url": "https://example.com/out.mp4"}
	}`))
	require.NoError(t, err)
	require.Equal(t, model.TaskStatusSuccess, done.Status)
	require.Equal(t, "100%", done.Progress)
	require.Equal(t, "https://example.com/out.mp4", done.Url)
}

func TestParseTaskResultAcceptsStringError(t *testing.T) {
	t.Parallel()

	adaptor := &TaskAdaptor{}

	result, err := adaptor.ParseTaskResult([]byte(`{
		"status": "failed",
		"progress": 100,
		"error": "Cannot read properties of undefined (reading 'message')"
	}`))

	require.NoError(t, err)
	require.Equal(t, model.TaskStatusFailure, result.Status)
	require.Equal(t, "100%", result.Progress)
	require.Equal(t, "Cannot read properties of undefined (reading 'message')", result.Reason)
}

func TestValidateRequestAndSetActionStoresRawNativeVideoRequest(t *testing.T) {
	t.Parallel()

	body := `{
		"model": "grok-imagine-video",
		"prompt": "continue the shot",
		"duration": 6,
		"video": {"url": "https://example.com/source.mp4"},
		"resolution": "720p"
	}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/extensions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	info := newRelayInfo("")

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)

	require.Nil(t, taskErr)
	require.Equal(t, actionVideoExtend, info.Action)
	stored, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	require.Equal(t, "grok-imagine-video", stored.Model)
	require.Equal(t, "continue the shot", stored.Prompt)
	require.Equal(t, 6, stored.Duration)
	video, ok := stored.Metadata["video"].(map[string]interface{})
	require.True(t, ok, "video = %T", stored.Metadata["video"])
	require.Equal(t, "https://example.com/source.mp4", video["url"])
	require.Equal(t, "720p", stored.Metadata["resolution"])
}

func TestBuildRequestBodyStoresGenerationParams(t *testing.T) {
	t.Parallel()

	body := `{
		"model": "grok-imagine-video",
		"prompt": "continue the shot",
		"duration": 6,
		"video": {"url": "data:video/mp4;base64,abcdef"},
		"resolution": "720p"
	}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/extensions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	info := newRelayInfoWithModel(actionVideoExtend, "grok-imagine-video")

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	requestBody, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	require.NotNil(t, requestBody)

	rawDetails, exists := c.Get(service.GenerationParamsContextKey)
	require.True(t, exists)
	details, ok := rawDetails.(map[string]interface{})
	require.True(t, ok, "details = %T", rawDetails)
	require.Equal(t, "xai", details["provider"])
	require.Equal(t, "video", details["type"])
	require.Equal(t, "grok-imagine-video", details["model"])
	require.Equal(t, "continue the shot", details["prompt"])
	require.Equal(t, 6, details["duration"])
	require.Equal(t, "720p", details["resolution"])
	video, ok := details["video"].(map[string]any)
	require.True(t, ok, "video = %T", details["video"])
	require.Equal(t, "data:video/mp4;base64,<omitted>", video["url"])
}

func TestValidateRequestAndSetActionRejectsPreviewGenerationWithoutImage(t *testing.T) {
	t.Parallel()

	body := `{
		"model": "grok-imagine-video-1.5-preview",
		"prompt": "make the waterfall move"
	}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	info := newRelayInfoWithModel("", "grok-imagine-video-1.5-preview")

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)

	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_request", taskErr.Code)
	require.Contains(t, taskErr.Message, "image is required")
}

func TestValidateRequestAndSetActionAllowsPreviewGenerationWithImageURL(t *testing.T) {
	t.Parallel()

	body := `{
		"model": "grok-imagine-video-1.5-preview",
		"prompt": "make the waterfall move",
		"image_url": "https://example.com/still.png"
	}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	info := newRelayInfoWithModel("", "grok-imagine-video-1.5-preview")

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)

	require.Nil(t, taskErr)
	require.Equal(t, actionVideoGenerate, info.Action)
	stored, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/still.png", stored.Metadata["image_url"])
}

func TestValidateRequestAndSetActionAllowsPreviewMultipartImageUpload(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "grok-imagine-video-1.5-preview"))
	require.NoError(t, writer.WriteField("prompt", "make the waterfall move"))
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="image"; filename="still.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	info := newRelayInfoWithModel("", "grok-imagine-video-1.5-preview")

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)

	require.Nil(t, taskErr)
	require.Equal(t, actionVideoGenerate, info.Action)
	stored, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(stored.Image, "data:image/png;base64,"))
	require.Len(t, stored.Images, 1)
	require.Equal(t, stored.Image, stored.Images[0])

	requestBody, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(requestBody)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	image, ok := payload["image"].(map[string]any)
	require.True(t, ok, "image = %T", payload["image"])
	require.True(t, strings.HasPrefix(image["url"].(string), "data:image/png;base64,"))
}

func newRelayInfo(action string) *relaycommon.RelayInfo {
	return newRelayInfoWithModel(action, "grok-imagine-video")
}

func newRelayInfoWithModel(action, modelName string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.x.ai",
			UpstreamModelName: modelName,
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: action},
	}
}
