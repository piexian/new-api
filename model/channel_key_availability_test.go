package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestChannelHasAvailableKey(t *testing.T) {
	tests := []struct {
		name    string
		channel *Channel
		want    bool
	}{
		{
			name:    "single key channel with empty key is unavailable",
			channel: &Channel{},
			want:    false,
		},
		{
			name: "single key channel with key is available",
			channel: &Channel{
				Key: "sk-test",
			},
			want: true,
		},
		{
			name: "multi key channel with only blank keys is unavailable",
			channel: &Channel{
				Key: " \n\t",
				ChannelInfo: ChannelInfo{
					IsMultiKey: true,
				},
			},
			want: false,
		},
		{
			name: "multi key channel skips disabled key and keeps enabled key available",
			channel: &Channel{
				Key: "disabled-key\nenabled-key",
				ChannelInfo: ChannelInfo{
					IsMultiKey: true,
					MultiKeyStatusList: map[int]int{
						0: common.ChannelStatusAutoDisabled,
					},
				},
			},
			want: true,
		},
		{
			name: "multi key channel with all keys disabled is unavailable",
			channel: &Channel{
				Key: "disabled-key-1\ndisabled-key-2",
				ChannelInfo: ChannelInfo{
					IsMultiKey: true,
					MultiKeyStatusList: map[int]int{
						0: common.ChannelStatusAutoDisabled,
						1: common.ChannelStatusAutoDisabled,
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.channel.HasAvailableKey(); got != tt.want {
				t.Fatalf("HasAvailableKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNextEnabledKeySkipsBlankMultiKeys(t *testing.T) {
	channel := &Channel{
		Key: " \nvalid-key",
		ChannelInfo: ChannelInfo{
			IsMultiKey:   true,
			MultiKeyMode: constant.MultiKeyModeRandom,
		},
	}

	key, index, err := channel.GetNextEnabledKey()
	if err != nil {
		t.Fatalf("GetNextEnabledKey() returned error: %v", err)
	}
	if key != "valid-key" || index != 1 {
		t.Fatalf("GetNextEnabledKey() = (%q, %d), want (%q, %d)", key, index, "valid-key", 1)
	}
}

func TestChannelModelCooldown(t *testing.T) {
	truncateTables(t)
	initCol()

	until := common.GetTimestamp() + 3600
	channel := &Channel{
		Id:     9001,
		Name:   "minimax",
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Group:  "default",
		Models: "MiniMax-M2.7,MiniMax-Hailuo-02",
	}
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))

	require.True(t, UpdateChannelModelStatusUntil(channel.Id, "MiniMax-M2.7", "token plan limit", until))
	require.False(t, UpdateChannelModelStatusUntil(channel.Id, "MiniMax-M2.7", "token plan limit", until))

	var reloaded Channel
	require.NoError(t, DB.First(&reloaded, channel.Id).Error)
	require.True(t, reloaded.IsModelCoolingDown("MiniMax-M2.7", common.GetTimestamp()))
	require.False(t, reloaded.IsModelCoolingDown("MiniMax-Hailuo-02", common.GetTimestamp()))

	selected, err := GetChannel("default", "MiniMax-M2.7", 0)
	require.NoError(t, err)
	require.Nil(t, selected)

	selected, err = GetChannel("default", "MiniMax-Hailuo-02", 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, channel.Id, selected.Id)

	releasedChannels, releasedKeys, releasedModels, err := ReleaseExpiredPlanQuotaCooldowns(until+1, 500)
	require.NoError(t, err)
	require.Equal(t, 0, releasedChannels)
	require.Equal(t, 0, releasedKeys)
	require.Equal(t, 1, releasedModels)

	require.NoError(t, DB.First(&reloaded, channel.Id).Error)
	require.False(t, reloaded.IsModelCoolingDown("MiniMax-M2.7", until+1))
}

func TestRateLimitedChannelCooldown(t *testing.T) {
	truncateTables(t)
	initCol()

	until := common.GetTimestamp() + 3600
	channel := &Channel{
		Id:     9002,
		Name:   "coding-plan",
		Key:    "test-key",
		Status: common.ChannelStatusRateLimited,
		Group:  "default",
		Models: "glm-4.5",
	}
	channel.SetOtherInfo(map[string]interface{}{
		"status_until": until,
	})
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.UpdateAbilities(nil))

	selected, err := GetChannel("default", "glm-4.5", 0)
	require.NoError(t, err)
	require.Nil(t, selected)

	releasedChannels, releasedKeys, releasedModels, err := ReleaseExpiredPlanQuotaCooldowns(until+1, 500)
	require.NoError(t, err)
	require.Equal(t, 1, releasedChannels)
	require.Equal(t, 0, releasedKeys)
	require.Equal(t, 0, releasedModels)

	var reloaded Channel
	require.NoError(t, DB.First(&reloaded, channel.Id).Error)
	require.Equal(t, common.ChannelStatusEnabled, reloaded.Status)
	require.Equal(t, int64(0), reloaded.GetStatusUntil())
}
