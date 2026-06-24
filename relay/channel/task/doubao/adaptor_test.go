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
