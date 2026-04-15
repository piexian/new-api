package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestZhipuCodingPlanAPIBase(t *testing.T) {
	testCases := []struct {
		name    string
		baseURL string
		want    string
		ok      bool
	}{
		{
			name:    "domestic special base",
			baseURL: zhipuCodingPlanBaseURL,
			want:    "https://open.bigmodel.cn",
			ok:      true,
		},
		{
			name:    "international special base",
			baseURL: zhipuCodingPlanInternationalBaseURL,
			want:    "https://api.z.ai",
			ok:      true,
		},
		{
			name:    "domestic direct api url",
			baseURL: "https://open.bigmodel.cn/api/coding/paas/v4",
			want:    "https://open.bigmodel.cn",
			ok:      true,
		},
		{
			name:    "international direct api url",
			baseURL: "https://api.z.ai/api/coding/paas/v4",
			want:    "https://api.z.ai",
			ok:      true,
		},
		{
			name:    "unsupported base",
			baseURL: "https://example.com",
			want:    "",
			ok:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := zhipuCodingPlanAPIBase(testCase.baseURL)
			require.Equal(t, testCase.ok, ok)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestZhipuCodingPlanRequestURL(t *testing.T) {
	t.Run("special base switches to domestic query host", func(t *testing.T) {
		baseURL := zhipuCodingPlanBaseURL
		channel := &model.Channel{BaseURL: &baseURL}

		url, err := zhipuCodingPlanRequestURL(channel)
		require.NoError(t, err)
		require.Equal(t, "https://open.bigmodel.cn/api/monitor/usage/quota/limit", url)
	})

	t.Run("special international base switches to international query host", func(t *testing.T) {
		baseURL := zhipuCodingPlanInternationalBaseURL
		channel := &model.Channel{BaseURL: &baseURL}

		url, err := zhipuCodingPlanRequestURL(channel)
		require.NoError(t, err)
		require.Equal(t, "https://api.z.ai/api/monitor/usage/quota/limit", url)
	})
}
