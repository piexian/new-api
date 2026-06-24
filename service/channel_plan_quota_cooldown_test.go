package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestParsePlanQuotaResetUntil(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 6, 6, 2, 30, 28, 0, location)

	tests := []struct {
		name    string
		message string
		want    time.Time
		wantOK  bool
	}{
		{
			name:    "token plan rfc3339 reset",
			message: "usage limit exceeded, 5-hour usage limit reached for Token Plan Plus (6596000/6596000 used), resets at 2026-06-06T05:00:00+08:00 (2056)",
			want:    time.Date(2026, 6, 6, 5, 0, 0, 0, location),
			wantOK:  true,
		},
		{
			name:    "weekly quota cst reset",
			message: "You have exceeded the weekly usage quota. It will reset at 2026-06-15 00:00:00 +0800 CST. We recommend upgrading your plan.",
			want:    time.Date(2026, 6, 15, 0, 0, 0, 0, location),
			wantOK:  true,
		},
		{
			name:    "duration reset after",
			message: "You have exhausted your capacity on this model. Your quota will reset after 1h40m15s.",
			want:    now.Add(time.Hour + 40*time.Minute + 15*time.Second),
			wantOK:  true,
		},
		{
			name:    "duration resets in",
			message: "Individual quota reached. Contact your administrator to enable overages. Resets in 146h54m51s.",
			want:    now.Add(146*time.Hour + 54*time.Minute + 51*time.Second),
			wantOK:  true,
		},
		{
			name:    "chinese quota reset time",
			message: "status_code=429, 您已达到每周/每月使用上限，您的限额将在 2026-06-24 01:20:15 重置。",
			want:    time.Date(2026, 6, 24, 1, 20, 15, 0, location),
			wantOK:  true,
		},
		{
			name:    "no reset time",
			message: "token plan limit exhausted",
			wantOK:  false,
		},
		{
			name:    "past reset time",
			message: "quota will reset at 2026-06-06T01:00:00+08:00",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParsePlanQuotaResetUntil(tt.message, now)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got != tt.want.Unix() {
				t.Fatalf("until = %d (%s), want %d (%s)", got, time.Unix(got, 0).In(location), tt.want.Unix(), tt.want)
			}
		})
	}
}

func TestDisableChannelUntilRecordsManageLogWhenCooldownAlreadyActive(t *testing.T) {
	truncate(t)

	until := common.GetTimestamp() + 3600
	reason := "status_code=429, 您已达到每周/每月使用上限，您的限额将在 2026-06-24 01:20:15 重置。"
	channel := &model.Channel{
		Id:     782,
		Name:   "智谱coding",
		Key:    "test-key",
		Status: common.ChannelStatusRateLimited,
	}
	channel.SetOtherInfo(map[string]interface{}{
		"status_until": until,
	})
	require.NoError(t, model.DB.Create(channel).Error)

	channelError := types.ChannelError{
		ChannelId:   channel.Id,
		ChannelName: channel.Name,
	}
	DisableChannelUntil(channelError, reason, until)

	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("channel_id = ? AND type = ?", channel.Id, model.LogTypeManage).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Contains(t, logs[0].Content, "已处于套餐限额冷却")

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "channel_plan_quota_cooldown", adminInfo["event"])
	require.Equal(t, "already_active", adminInfo["state"])
	require.Equal(t, false, adminInfo["status_changed"])

	DisableChannelUntil(channelError, reason, until)

	var count int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("channel_id = ? AND type = ?", channel.Id, model.LogTypeManage).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestDisableChannelUntilUsesRateLimitedChannelStatus(t *testing.T) {
	truncate(t)

	until := common.GetTimestamp() + 3600
	channel := &model.Channel{
		Id:     784,
		Name:   "coding-plan",
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	DisableChannelUntil(
		types.ChannelError{ChannelId: channel.Id, ChannelName: channel.Name},
		"status_code=429, quota will reset at 2026-06-24T01:20:15+08:00",
		until,
	)

	var reloaded model.Channel
	require.NoError(t, model.DB.First(&reloaded, channel.Id).Error)
	require.Equal(t, common.ChannelStatusRateLimited, reloaded.Status)
	require.Equal(t, until, reloaded.GetStatusUntil())
}

func TestDisableChannelUntilRateLimitsMultiKeyChannelScope(t *testing.T) {
	truncate(t)

	until := common.GetTimestamp() + 3600
	channel := &model.Channel{
		Id:     785,
		Name:   "multi-key-plan",
		Key:    "key-1\nkey-2",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:         true,
			MultiKeySize:       2,
			MultiKeyMode:       constant.MultiKeyModeRandom,
			MultiKeyStatusList: map[int]int{},
		},
	}
	require.NoError(t, model.DB.Create(channel).Error)

	DisableChannelUntil(
		types.ChannelError{ChannelId: channel.Id, ChannelName: channel.Name, IsMultiKey: true, UsingKey: "key-1"},
		"status_code=429, quota will reset at 2026-06-24T01:20:15+08:00",
		until,
	)

	var reloaded model.Channel
	require.NoError(t, model.DB.First(&reloaded, channel.Id).Error)
	require.Equal(t, common.ChannelStatusRateLimited, reloaded.Status)
	require.Equal(t, until, reloaded.GetStatusUntil())
	require.Empty(t, reloaded.ChannelInfo.MultiKeyStatusList)
}

func TestDisableChannelModelUntilRecordsManageLog(t *testing.T) {
	truncate(t)

	until := common.GetTimestamp() + 3600
	channel := &model.Channel{
		Id:     783,
		Name:   "MiniMax",
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	channelError := types.ChannelError{
		ChannelId:   channel.Id,
		ChannelType: constant.ChannelTypeMiniMax,
		ChannelName: channel.Name,
	}
	DisableChannelModelUntil(channelError, "MiniMax-M2.7", "token plan limit", until)

	var log model.Log
	require.NoError(t, model.LOG_DB.Where("channel_id = ? AND type = ?", channel.Id, model.LogTypeManage).First(&log).Error)
	require.Contains(t, log.Content, "模型「MiniMax-M2.7」进入套餐限额冷却")
	require.Contains(t, log.Content, "禁用至")
	require.NotContains(t, log.Content, "限流至")

	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "model", adminInfo["scope"])
	require.Equal(t, "MiniMax-M2.7", adminInfo["model_name"])
	require.Equal(t, "entered", adminInfo["state"])
	require.Equal(t, true, adminInfo["status_changed"])
}
