package doubao

import (
	"testing"

	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

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
