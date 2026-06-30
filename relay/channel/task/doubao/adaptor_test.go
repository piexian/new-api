package doubao

import (
	"net/http/httptest"
	"testing"

	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoubaoTaskOpenAIBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "default volcengine root",
			baseURL: "",
			want:    "https://ark.cn-beijing.volces.com/api/v3",
		},
		{
			name:    "official v3 base",
			baseURL: "https://ark.cn-beijing.volces.com/api/v3",
			want:    "https://ark.cn-beijing.volces.com/api/v3",
		},
		{
			name:    "agent plan alias",
			baseURL: "doubao-agent-plan",
			want:    "https://ark.cn-beijing.volces.com/api/plan/v3",
		},
		{
			name:    "official agent plan base",
			baseURL: "https://ark.cn-beijing.volces.com/api/plan/v3",
			want:    "https://ark.cn-beijing.volces.com/api/plan/v3",
		},
		{
			name:    "coding plan alias",
			baseURL: "doubao-coding-plan",
			want:    "https://ark.cn-beijing.volces.com/api/coding/v3",
		},
		{
			name:    "official coding plan base",
			baseURL: "https://ark.cn-beijing.volces.com/api/coding/v3",
			want:    "https://ark.cn-beijing.volces.com/api/coding/v3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, doubaoTaskOpenAIBaseURL(tt.baseURL))
		})
	}
}

func TestBuildRequestURLUsesPlanAwareBase(t *testing.T) {
	t.Parallel()

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    channelconstant.ChannelTypeVolcEngine,
			ChannelBaseUrl: "doubao-agent-plan",
		},
	})

	got, err := adaptor.BuildRequestURL(&relaycommon.RelayInfo{})

	require.NoError(t, err)
	require.Equal(t, "https://ark.cn-beijing.volces.com/api/plan/v3/contents/generations/tasks", got)
}

func TestResponseTaskGeneratedContentURLFallbacks(t *testing.T) {
	t.Parallel()

	var result responseTask
	require.Empty(t, result.generatedContentURL())

	result.Content.ImageURL = "https://example.com/preview.png"
	require.Equal(t, "https://example.com/preview.png", result.generatedContentURL())

	result.Content.FileURL = "https://example.com/model.glb"
	require.Equal(t, "https://example.com/model.glb", result.generatedContentURL())

	result.Content.VideoURL = "https://example.com/video.mp4"
	require.Equal(t, "https://example.com/video.mp4", result.generatedContentURL())
}

func TestParseTaskResultUsesGeneratedContentURLFallback(t *testing.T) {
	t.Parallel()

	taskInfo, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"id":"task_123",
		"status":"succeeded",
		"content":{"file_url":"https://example.com/model.glb"}
	}`))

	require.NoError(t, err)
	require.Equal(t, "SUCCESS", taskInfo.Status)
	require.Equal(t, "https://example.com/model.glb", taskInfo.Url)
}

func TestDoubaoTaskImageURLsIncludesOpenAIVideoAliases(t *testing.T) {
	t.Parallel()

	req := &relaycommon.TaskSubmitReq{
		Image:          "https://example.com/image.png",
		ImageURL:       "https://example.com/image-url.png",
		InputReference: "https://example.com/reference.png",
		Images: []string{
			"https://example.com/image.png",
			"https://example.com/second.png",
		},
	}

	require.Equal(t, []string{
		"https://example.com/image.png",
		"https://example.com/second.png",
		"https://example.com/image-url.png",
		"https://example.com/reference.png",
	}, doubaoTaskImageURLs(req))
}

func TestConvertToRequestPayloadPassesSeedance2Fields(t *testing.T) {
	t.Parallel()

	req := &relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "generate a city flythrough",
		Metadata: map[string]interface{}{
			"safety_identifier": "user-123",
			"priority":          0,
		},
	}

	body, err := (&TaskAdaptor{}).convertToRequestPayload(req)

	require.NoError(t, err)
	require.Equal(t, "user-123", body.SafetyIdentifier)
	require.NotNil(t, body.Priority)
	require.Equal(t, 0, int(*body.Priority))
}

func TestGetVideoInputRatioResolutionAware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		model      string
		resolution string
		hasVideo   bool
		want       float64
		wantOK     bool
	}{
		{
			name:   "unknown model",
			model:  "unknown",
			want:   1.0,
			wantOK: false,
		},
		{
			name:     "pro default text to video uses base price",
			model:    "doubao-seedance-2-0-260128",
			want:     1.0,
			wantOK:   true,
			hasVideo: false,
		},
		{
			name:     "pro default video input",
			model:    "doubao-seedance-2-0-260128",
			hasVideo: true,
			want:     28.0 / 46.0,
			wantOK:   true,
		},
		{
			name:       "pro 1080p text to video",
			model:      "doubao-seedance-2-0-260128",
			resolution: "1080p",
			want:       51.0 / 46.0,
			wantOK:     true,
		},
		{
			name:       "pro 1080p video input",
			model:      "doubao-seedance-2-0-260128",
			resolution: "1080p",
			hasVideo:   true,
			want:       31.0 / 46.0,
			wantOK:     true,
		},
		{
			name:       "pro 4k video input trims and lowercases",
			model:      "doubao-seedance-2-0-260128",
			resolution: " 4K ",
			hasVideo:   true,
			want:       16.0 / 46.0,
			wantOK:     true,
		},
		{
			name:     "fast default video input",
			model:    "doubao-seedance-2-0-fast-260128",
			hasVideo: true,
			want:     22.0 / 37.0,
			wantOK:   true,
		},
		{
			name:       "fast missing 1080p combo falls back to base billing",
			model:      "doubao-seedance-2-0-fast-260128",
			resolution: "1080p",
			want:       1.0,
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := GetVideoInputRatio(tt.model, tt.resolution, tt.hasVideo)

			require.Equal(t, tt.wantOK, ok)
			require.InDelta(t, tt.want, got, 0.000001)
		})
	}
}

func TestEstimateBillingUsesSeedance2ResolutionAndVideoInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata map[string]interface{}
		wantNil  bool
		want     float64
	}{
		{
			name:    "base request has no adjustment",
			wantNil: true,
		},
		{
			name: "video input applies discount",
			metadata: map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{"type": "video_url", "video_url": map[string]interface{}{"url": "https://example.com/input.mp4"}},
				},
			},
			want: 28.0 / 46.0,
		},
		{
			name: "1080p without video applies resolution price",
			metadata: map[string]interface{}{
				"resolution": "1080p",
			},
			want: 51.0 / 46.0,
		},
		{
			name: "4k video input applies resolution and video price",
			metadata: map[string]interface{}{
				"resolution": "4k",
				"content": []interface{}{
					map[string]interface{}{"video_url": map[string]interface{}{"url": "https://example.com/input.mp4"}},
				},
			},
			want: 16.0 / 46.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("task_request", relaycommon.TaskSubmitReq{Metadata: tt.metadata})

			got := (&TaskAdaptor{}).EstimateBilling(c, &relaycommon.RelayInfo{OriginModelName: "doubao-seedance-2-0-260128"})

			if tt.wantNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.InDelta(t, tt.want, got["video_input"], 0.000001)
		})
	}
}
