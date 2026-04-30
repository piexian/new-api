package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestMiniMaxTokenPlanRequestURLs(t *testing.T) {
	t.Run("default order uses minimaxi first", func(t *testing.T) {
		channel := &model.Channel{}
		urls := miniMaxTokenPlanRequestURLs(channel)

		require.Equal(t, "https://www.minimaxi.com/v1/token_plan/remains", urls[0])
		require.Contains(t, urls, "https://www.minimax.io/v1/token_plan/remains")
		require.Contains(t, urls, "https://www.minimax.com/v1/token_plan/remains")
		require.Contains(t, urls, "https://www.minimaxi.com/v1/api/openplatform/coding_plan/remains")
	})

	t.Run("io base url prioritizes io host", func(t *testing.T) {
		baseURL := "https://api.minimax.io"
		channel := &model.Channel{BaseURL: &baseURL}
		urls := miniMaxTokenPlanRequestURLs(channel)

		require.Equal(t, "https://www.minimax.io/v1/token_plan/remains", urls[0])
		require.Len(t, urls, 6)
	})

	t.Run("duplicates are removed", func(t *testing.T) {
		baseURL := "https://www.minimaxi.com"
		channel := &model.Channel{BaseURL: &baseURL}
		urls := miniMaxTokenPlanRequestURLs(channel)

		require.Equal(t, "https://www.minimaxi.com/v1/token_plan/remains", urls[0])
		require.Len(t, urls, 6)
	})
}
