package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestCooldownRecoveryPreservesChannelAfterCacheReload(t *testing.T) {
	for _, scope := range []string{"channel", "rate-limit", "key", "model"} {
		t.Run(scope, func(t *testing.T) {
			db := setupChannelStatusTestDB(t)
			info := ChannelInfo{}
			if scope == "key" {
				info = ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling}
			}
			channel := createChannelStatusFixture(t, db, "recovery-"+scope, "test-tag", info)
			channel.BaseURL = common.GetPointer("https://example.invalid")
			channel.Setting = common.GetPointer(`{"system_prompt":"preserve me","plan_quota_cooldown_enabled":true}`)
			channel.AutoBan = common.GetPointer(1)
			channel.UsedQuota = 123
			require.NoError(t, db.Save(&channel).Error)
			until := common.GetTimestamp() + 60
			switch scope {
			case "channel":
				require.True(t, UpdateChannelStatusUntil(channel.Id, "", common.ChannelStatusAutoDisabled, "temporary", until))
			case "rate-limit":
				require.True(t, UpdateChannelStatusUntil(channel.Id, "", common.ChannelStatusRateLimited, "temporary", until))
			case "key":
				require.True(t, UpdateChannelStatusUntil(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "temporary", until))
				require.True(t, UpdateChannelStatusUntil(channel.Id, "key-b", common.ChannelStatusAutoDisabled, "temporary", until))
			case "model":
				require.True(t, UpdateChannelModelStatusUntil(channel.Id, "gpt-4o", "temporary", until))
			}
			// A new cache must recover using persisted deadlines, as after a restart.
			channelSyncLock.Lock()
			channelsIDM = nil
			channelSyncLock.Unlock()
			InitChannelCache()
			_, _, _, err := ReleaseExpiredPlanQuotaCooldowns(until+1, 1)
			require.NoError(t, err)
			var recovered Channel
			require.NoError(t, db.First(&recovered, channel.Id).Error)
			require.Equal(t, channel.Type, recovered.Type)
			require.Equal(t, channel.Models, recovered.Models)
			require.Equal(t, channel.Group, recovered.Group)
			require.Equal(t, channel.Key, recovered.Key)
			require.Equal(t, channel.BaseURL, recovered.BaseURL)
			require.Equal(t, channel.Setting, recovered.Setting)
			require.True(t, recovered.GetSetting().PlanQuotaCooldownEnabled)
			require.Equal(t, channel.AutoBan, recovered.AutoBan)
			require.Equal(t, channel.UsedQuota, recovered.UsedQuota)
			require.Equal(t, common.ChannelStatusEnabled, recovered.Status)
			InitChannelCache()
			requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)
			cached, err := CacheGetChannel(channel.Id)
			require.NoError(t, err)
			require.True(t, cached.GetSetting().PlanQuotaCooldownEnabled)
		})
	}
}

func TestCooldownRecoveryKeepsManualAndUntimedDisables(t *testing.T) {
	db := setupChannelStatusTestDB(t)
	now := common.GetTimestamp()
	for _, tc := range []struct {
		name   string
		status int
		until  int64
	}{
		{"manual", common.ChannelStatusManuallyDisabled, now - 1},
		{"untimed", common.ChannelStatusAutoDisabled, 0},
		{"future", common.ChannelStatusAutoDisabled, now + 60},
	} {
		channel := createChannelStatusFixture(t, db, tc.name, "", ChannelInfo{})
		channel.Status = tc.status
		channel.SetOtherInfo(map[string]interface{}{"status_until": tc.until})
		require.NoError(t, db.Save(&channel).Error)
	}
	channels, keys, models, err := ReleaseExpiredPlanQuotaCooldowns(now, 1)
	require.NoError(t, err)
	require.Zero(t, channels)
	require.Zero(t, keys)
	require.Zero(t, models)
	var stored []Channel
	require.NoError(t, db.Order("id").Find(&stored).Error)
	require.Equal(t, common.ChannelStatusManuallyDisabled, stored[0].Status)
	require.Equal(t, common.ChannelStatusAutoDisabled, stored[1].Status)
	require.Equal(t, common.ChannelStatusAutoDisabled, stored[2].Status)
}
