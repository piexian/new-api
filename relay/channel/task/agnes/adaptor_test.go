package agnes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAgnesVideoContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func buildAgnesPayload(t *testing.T, body string, info *relaycommon.RelayInfo) map[string]any {
	t.Helper()

	c, _ := newAgnesVideoContext(body)
	adaptor := &TaskAdaptor{}
	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	return payload
}

func TestBuildRequestBodyPreservesTextVideoFieldsAndMapsModel(t *testing.T) {
	payload := buildAgnesPayload(t, `{
		"model": "agnes-video-v2.0",
		"prompt": "cinematic beach cat",
		"height": 768,
		"width": 1152,
		"num_frames": 121,
		"frame_rate": 24,
		"seed": 0,
		"group": "should-not-forward"
	}`, &relaycommon.RelayInfo{
		OriginModelName: ModelVideoV20,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "upstream-agnes-video"},
	})

	require.Equal(t, "upstream-agnes-video", payload["model"])
	require.Equal(t, "cinematic beach cat", payload["prompt"])
	require.Equal(t, float64(768), payload["height"])
	require.Equal(t, float64(1152), payload["width"])
	require.Equal(t, float64(121), payload["num_frames"])
	require.Equal(t, float64(24), payload["frame_rate"])
	require.Equal(t, float64(0), payload["seed"])
	require.NotContains(t, payload, "group")
}

func TestBuildRequestBodyPreservesTopLevelImage(t *testing.T) {
	payload := buildAgnesPayload(t, `{
		"model": "agnes-video-v2.0",
		"prompt": "animate the portrait",
		"image": "https://example.com/input.png",
		"num_frames": 121,
		"frame_rate": 24
	}`, &relaycommon.RelayInfo{
		OriginModelName: ModelVideoV20,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: ModelVideoV20},
	})

	require.Equal(t, "https://example.com/input.png", payload["image"])
}

func TestBuildRequestBodyPreservesExtraBodyForKeyframes(t *testing.T) {
	payload := buildAgnesPayload(t, `{
		"model": "agnes-video-v2.0",
		"prompt": "smooth keyframe transition",
		"extra_body": {
			"image": [
				"https://example.com/keyframe1.png",
				"https://example.com/keyframe2.png"
			],
			"mode": "keyframes"
		},
		"num_frames": 121,
		"frame_rate": 24
	}`, &relaycommon.RelayInfo{
		OriginModelName: ModelVideoV20,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: ModelVideoV20},
	})

	extraBody, ok := payload["extra_body"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "keyframes", extraBody["mode"])

	images, ok := extraBody["image"].([]any)
	require.True(t, ok)
	require.Len(t, images, 2)
	require.Equal(t, "https://example.com/keyframe1.png", images[0])
	require.Equal(t, "https://example.com/keyframe2.png", images[1])
}

func TestDoResponseRewritesUpstreamTaskIDToPublicOpenAIVideo(t *testing.T) {
	c, w := newAgnesVideoContext(`{}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: ModelVideoV20,
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public",
		},
	}
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{
			"id": "upstream_task",
			"task_id": "upstream_task",
			"object": "video",
			"model": "agnes-video-v2.0",
			"status": "queued",
			"progress": 0,
			"created_at": 1780457477,
			"seconds": "5.0",
			"size": "1152x768"
		}`)),
	}

	upstreamTaskID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, info)

	require.Nil(t, taskErr)
	require.Equal(t, "upstream_task", upstreamTaskID)
	require.Contains(t, string(taskData), "upstream_task")

	var got map[string]any
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "task_public", got["id"])
	require.Equal(t, "task_public", got["task_id"])
	require.Equal(t, "video", got["object"])
	require.Equal(t, ModelVideoV20, got["model"])
	require.Equal(t, "queued", got["status"])
	require.Equal(t, float64(0), got["progress"])
	require.Equal(t, "5.0", got["seconds"])
	require.Equal(t, "1152x768", got["size"])
}

func TestParseTaskResultMapsStatusesAndURLs(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus model.TaskStatus
		wantURL    string
		wantReason string
	}{
		{
			name:       "queued",
			body:       `{"status":"queued","progress":0}`,
			wantStatus: model.TaskStatusQueued,
		},
		{
			name:       "in progress",
			body:       `{"status":"in_progress","progress":42}`,
			wantStatus: model.TaskStatusInProgress,
		},
		{
			name:       "completed video url",
			body:       `{"status":"completed","progress":100,"video_url":"https://example.com/video.mp4"}`,
			wantStatus: model.TaskStatusSuccess,
			wantURL:    "https://example.com/video.mp4",
		},
		{
			name:       "completed top-level url (agnesapi)",
			body:       `{"status":"completed","progress":100,"url":"https://cos.example.com/video.mp4","metadata":null}`,
			wantStatus: model.TaskStatusSuccess,
			wantURL:    "https://cos.example.com/video.mp4",
		},
		{
			name:       "completed metadata url (docs)",
			body:       `{"status":"completed","progress":100,"metadata":{"url":"https://example.com/meta.mp4"}}`,
			wantStatus: model.TaskStatusSuccess,
			wantURL:    "https://example.com/meta.mp4",
		},
		{
			name:       "completed remixed url",
			body:       `{"status":"completed","progress":100,"remixed_from_video_id":"https://example.com/remixed.mp4"}`,
			wantStatus: model.TaskStatusSuccess,
			wantURL:    "https://example.com/remixed.mp4",
		},
		{
			name:       "completed remixed id is not a result url",
			body:       `{"status":"completed","progress":100,"remixed_from_video_id":"vid_abc123"}`,
			wantStatus: model.TaskStatusSuccess,
		},
		{
			name:       "failed",
			body:       `{"status":"failed","error":{"message":"safety filter","code":"content_policy"}}`,
			wantStatus: model.TaskStatusFailure,
			wantReason: "safety filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskInfo, err := (&TaskAdaptor{}).ParseTaskResult([]byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, string(tt.wantStatus), taskInfo.Status)
			require.Equal(t, tt.wantURL, taskInfo.Url)
			require.Equal(t, tt.wantReason, taskInfo.Reason)
		})
	}
}

func TestConvertToOpenAIVideoReturnsStandardDTO(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		CreatedAt:  1780457477,
		UpdatedAt:  1780457488,
		FinishTime: 1780457499,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		Properties: model.Properties{
			OriginModelName: ModelVideoV20,
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://example.com/result.mp4",
		},
		Data: []byte(`{
			"model": "agnes-video-v2.0",
			"status": "completed",
			"seconds": "5.0",
			"size": "1152x768",
			"video_url": "https://example.com/upstream.mp4"
		}`),
	}

	data, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, common.Unmarshal(data, &got))
	require.Equal(t, "task_public", got["id"])
	require.Equal(t, "task_public", got["task_id"])
	require.Equal(t, "video", got["object"])
	require.Equal(t, ModelVideoV20, got["model"])
	require.Equal(t, "completed", got["status"])
	require.Equal(t, float64(100), got["progress"])
	require.Equal(t, "5.0", got["seconds"])
	require.Equal(t, "1152x768", got["size"])

	metadata, ok := got["metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https://example.com/result.mp4", metadata["url"])
}

func TestEstimateAndAdjustBillingUseSeconds(t *testing.T) {
	c, _ := newAgnesVideoContext(`{
		"model": "agnes-video-v2.0",
		"prompt": "cinematic beach cat",
		"num_frames": 241,
		"frame_rate": 24
	}`)
	info := &relaycommon.RelayInfo{}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	ratios := (&TaskAdaptor{}).EstimateBilling(c, info)
	require.InDelta(t, 10.041666, ratios["seconds"], 0.000001)

	adjusted := (&TaskAdaptor{}).AdjustBillingOnSubmit(info, []byte(`{"id":"upstream","status":"queued","seconds":"10.0"}`))
	require.Equal(t, 10.0, adjusted["seconds"])
}

func validateAgnesRequest(t *testing.T, body string) *dto.TaskError {
	t.Helper()

	c, _ := newAgnesVideoContext(body)
	adaptor := &TaskAdaptor{}
	return adaptor.ValidateRequestAndSetAction(c, &relaycommon.RelayInfo{})
}

func TestBuildRequestBodyPreservesVideo25Fields(t *testing.T) {
	payload := buildAgnesPayload(t, `{
		"model": "agnes-video-2.5-flash",
		"prompt": "dance with <Picture 1>",
		"mode": "reference",
		"seconds": "8",
		"size": "720P",
		"aspect_ratio": "9:16",
		"seed": 7,
		"n": 1,
		"images": ["https://example.com/ref1.png", "https://example.com/ref2.png"]
	}`, &relaycommon.RelayInfo{
		OriginModelName: ModelVideo25Flash,
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: ModelVideo25Flash},
	})

	require.Equal(t, "reference", payload["mode"])
	require.Equal(t, "8", payload["seconds"])
	require.Equal(t, "720P", payload["size"])
	require.Equal(t, "9:16", payload["aspect_ratio"])
	require.Equal(t, float64(7), payload["seed"])
	require.Equal(t, float64(1), payload["n"])
	images, ok := payload["images"].([]any)
	require.True(t, ok)
	require.Len(t, images, 2)
}

func TestValidateVideo25ModeRules(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "mode is required",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat"}`,
			wantErr: "mode is required",
		},
		{
			name:    "invalid mode",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"remix"}`,
			wantErr: "mode must be text, keyframe or reference",
		},
		{
			name:    "text mode forbids media",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"text","images":["https://example.com/a.png"]}`,
			wantErr: "text mode must not contain media fields",
		},
		{
			name:    "keyframe mode requires a frame",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"keyframe"}`,
			wantErr: "keyframe mode requires first_frame or last_frame",
		},
		{
			name:    "keyframe mode forbids images",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"keyframe","first_frame":"https://example.com/f.png","images":["https://example.com/a.png"]}`,
			wantErr: "keyframe mode must not contain images, audios or videos",
		},
		{
			name:    "reference mode requires images or audios",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"reference"}`,
			wantErr: "reference mode requires images or audios",
		},
		{
			name:    "reference mode forbids frames",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"reference","images":["https://example.com/a.png"],"last_frame":"https://example.com/f.png"}`,
			wantErr: "reference mode must not contain first_frame, last_frame or videos",
		},
		{
			name:    "seconds out of range",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"text","seconds":"13"}`,
			wantErr: "seconds must be between 4 and 12",
		},
		{
			name:    "valid text request passes",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"text","seconds":"5","size":"720P"}`,
			wantErr: "",
		},
		{
			name:    "valid reference request passes",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"reference","seconds":"8","images":["https://example.com/a.png"],"audios":["https://example.com/a.mp3"]}`,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskErr := validateAgnesRequest(t, tt.body)
			if tt.wantErr == "" {
				require.Nil(t, taskErr)
				return
			}
			require.NotNil(t, taskErr)
			require.Contains(t, taskErr.Error.Error(), tt.wantErr)
		})
	}
}

func TestValidateVideo25FlashRestrictions(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "size must be 720P",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"text","size":"1080P"}`,
			wantErr: "size must be 720P",
		},
		{
			name:    "images capped at 5",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"reference","images":["https://e.com/1.png","https://e.com/2.png","https://e.com/3.png","https://e.com/4.png","https://e.com/5.png","https://e.com/6.png"]}`,
			wantErr: "images length must not exceed 5",
		},
		{
			name:    "videos not supported",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"reference","images":["https://e.com/1.png"],"videos":[{"url":"https://e.com/in.mp4"}]}`,
			wantErr: "videos is not supported",
		},
		{
			name:    "n must be 1",
			body:    `{"model":"agnes-video-2.5-flash","prompt":"cat","mode":"text","n":2}`,
			wantErr: "n must be 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskErr := validateAgnesRequest(t, tt.body)
			require.NotNil(t, taskErr)
			require.Contains(t, taskErr.Error.Error(), tt.wantErr)
		})
	}
}

func TestVideo20RequestSkipsVideo25Validation(t *testing.T) {
	// v2.0 无 mode、seconds 越界均不触发 2.5 系列校验
	taskErr := validateAgnesRequest(t, `{
		"model": "agnes-video-v2.0",
		"prompt": "cat",
		"seconds": "30",
		"num_frames": 121,
		"frame_rate": 24
	}`)
	require.Nil(t, taskErr)
}

func TestValidateVideo25SetsGenerateActionForReference(t *testing.T) {
	c, _ := newAgnesVideoContext(`{
		"model": "agnes-video-2.5-flash",
		"prompt": "dance",
		"mode": "reference",
		"images": ["https://example.com/ref.png"]
	}`)
	info := &relaycommon.RelayInfo{}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, constant.TaskActionGenerate, info.Action)

	c2, _ := newAgnesVideoContext(`{
		"model": "agnes-video-2.5-flash",
		"prompt": "dance",
		"mode": "text"
	}`)
	info2 := &relaycommon.RelayInfo{}
	taskErr = (&TaskAdaptor{}).ValidateRequestAndSetAction(c2, info2)
	require.Nil(t, taskErr)
	require.Equal(t, constant.TaskActionTextGenerate, info2.Action)
}
