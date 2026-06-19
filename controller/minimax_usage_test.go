package controller

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
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

func TestIsMiniMaxTokenPlanLimitError(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("已达到 Token Plan 用量上限：请升级 Token Plan 套餐或购买积分补充用量。 (2056)"),
		types.ErrorCodeBadResponseStatusCode,
		429,
		types.ErrOptionWithSkipSensitiveMask(),
	)

	channelError := types.ChannelError{ChannelType: constant.ChannelTypeMiniMax}
	require.True(t, isMiniMaxTokenPlanLimitError(channelError, err))

	channelError.ChannelType = constant.ChannelTypeOpenAI
	require.False(t, isMiniMaxTokenPlanLimitError(channelError, err))
}

func TestMiniMaxTokenPlanCooldownUntilUsesExhaustedGeneralEndTime(t *testing.T) {
	now := time.Unix(1781869349, 0)
	body := []byte(`{
	  "base_resp": {
	    "status_code": 0,
	    "status_msg": "success"
	  },
	  "model_remains": [
	    {
	      "current_interval_remaining_percent": 0,
	      "current_interval_status": 2,
	      "current_interval_total_count": 0,
	      "current_interval_usage_count": 0,
	      "current_weekly_remaining_percent": 100,
	      "current_weekly_status": 3,
	      "current_weekly_total_count": 0,
	      "current_weekly_usage_count": 0,
	      "end_time": 1781870400000,
	      "model_name": "general",
	      "remains_time": 1050334,
	      "start_time": 1781852400000,
	      "weekly_end_time": 1782057600000,
	      "weekly_remains_time": 188250334,
	      "weekly_start_time": 1781452800000
	    },
	    {
	      "current_interval_remaining_percent": 100,
	      "current_interval_status": 1,
	      "current_interval_total_count": 3,
	      "current_interval_usage_count": 0,
	      "current_weekly_remaining_percent": 100,
	      "current_weekly_status": 1,
	      "current_weekly_total_count": 21,
	      "current_weekly_usage_count": 0,
	      "end_time": 1781884800000,
	      "model_name": "video",
	      "remains_time": 15450334,
	      "weekly_end_time": 1782057600000,
	      "weekly_remains_time": 188250334,
	      "weekly_start_time": 1781452800000
	    }
	  ]
	}`)

	until, detail, ok, err := miniMaxTokenPlanCooldownUntil(body, now, "MiniMax-M2.7")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(1781870400), until)
	require.Contains(t, detail, "model=general")
	require.Contains(t, detail, "window=interval")
}

func TestMiniMaxTokenPlanCooldownUntilIgnoresNonExhaustedPreferredBucket(t *testing.T) {
	now := time.Unix(1781869349, 0)
	body := []byte(`{
	  "base_resp": {"status_code": 0, "status_msg": "success"},
	  "model_remains": [
	    {
	      "current_interval_remaining_percent": 0,
	      "current_interval_status": 2,
	      "end_time": 1781870400000,
	      "model_name": "general"
	    },
	    {
	      "current_interval_remaining_percent": 100,
	      "current_interval_status": 1,
	      "end_time": 1781884800000,
	      "model_name": "video"
	    }
	  ]
	}`)

	_, _, ok, err := miniMaxTokenPlanCooldownUntil(body, now, "MiniMax-Hailuo-02")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMiniMaxTokenPlanCooldownUntilUsesWeeklyResetWhenWeeklyExhausted(t *testing.T) {
	now := time.Unix(1781869349, 0)
	body := []byte(`{
	  "base_resp": {"status_code": 0, "status_msg": "success"},
	  "model_remains": [
	    {
	      "current_interval_remaining_percent": 100,
	      "current_interval_status": 1,
	      "current_weekly_remaining_percent": 0,
	      "current_weekly_status": 2,
	      "weekly_end_time": 1782057600000,
	      "model_name": "general"
	    }
	  ]
	}`)

	until, detail, ok, err := miniMaxTokenPlanCooldownUntil(body, now, "MiniMax-M2.7")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(1782057600), until)
	require.Contains(t, detail, "window=weekly")
}
