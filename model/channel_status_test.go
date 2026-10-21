package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelStatusTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldMainDatabaseType := common.MainDatabaseType()
	oldLogDatabaseType := common.LogDatabaseType()

	channelSyncLock.Lock()
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	oldAdvancedCustomConfig := channel2advancedCustomConfig
	group2model2channels = nil
	channelsIDM = nil
	channel2advancedCustomConfig = nil
	channelSyncLock.Unlock()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	common.MemoryCacheEnabled = true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.SetDatabaseTypes(oldMainDatabaseType, oldLogDatabaseType)
		initCol()
		channelSyncLock.Lock()
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
		channel2advancedCustomConfig = oldAdvancedCustomConfig
		channelSyncLock.Unlock()
		InvalidatePricingCache()
	})

	return db
}

func createChannelStatusFixture(t *testing.T, db *gorm.DB, name string, tag string, channelInfo ChannelInfo) Channel {
	t.Helper()

	priority := int64(10)
	weight := uint(100)
	channel := Channel{
		Type:        constant.ChannelTypeOpenAI,
		Key:         "key-a\nkey-b",
		Status:      common.ChannelStatusEnabled,
		Name:        name,
		Models:      "gpt-4o",
		Group:       "default",
		Priority:    &priority,
		Weight:      &weight,
		OtherInfo:   "{}",
		ChannelInfo: channelInfo,
	}
	channel.SetTag(tag)
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&Ability{
		Group:     channel.Group,
		Model:     channel.Models,
		ChannelId: channel.Id,
		Enabled:   true,
		Priority:  channel.Priority,
		Weight:    *channel.Weight,
		Tag:       channel.Tag,
	}).Error)
	return channel
}

func requireChannelStatusState(t *testing.T, db *gorm.DB, channelId int, status int, abilityEnabled bool, routed bool) {
	t.Helper()

	var stored Channel
	require.NoError(t, db.First(&stored, "id = ?", channelId).Error)
	assert.Equal(t, status, stored.Status)

	var ability Ability
	require.NoError(t, db.First(&ability, "channel_id = ?", channelId).Error)
	assert.Equal(t, abilityEnabled, ability.Enabled)

	cached, err := CacheGetChannel(channelId)
	require.NoError(t, err)
	assert.Equal(t, status, cached.Status)
	assert.Equal(t, routed, IsChannelEnabledForGroupModel("default", "gpt-4o", channelId))
}

func TestUpdateChannelStatusKeepsDatabaseAbilitiesAndCacheInSync(t *testing.T) {
	db := setupChannelStatusTestDB(t)
	channel := createChannelStatusFixture(t, db, "status-sync", "status-sync", ChannelInfo{})
	InitChannelCache()

	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)

	changed, err := UpdateChannelStatusWithError(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual operation")
	require.NoError(t, err)
	assert.True(t, changed)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusManuallyDisabled, false, false)

	changed, err = UpdateChannelStatusWithError(channel.Id, "", common.ChannelStatusEnabled, "manual operation")
	require.NoError(t, err)
	assert.True(t, changed)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)

	changed, err = UpdateChannelStatusWithError(channel.Id, "", common.ChannelStatusEnabled, "manual operation")
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestUpdateChannelStatusSyncsMultiKeyStateAndRouting(t *testing.T) {
	db := setupChannelStatusTestDB(t)
	channel := createChannelStatusFixture(t, db, "multi-key-status", "multi-key-status", ChannelInfo{
		IsMultiKey:   true,
		MultiKeySize: 2,
		MultiKeyMode: constant.MultiKeyModePolling,
	})
	InitChannelCache()

	cached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	cached.ChannelInfo.MultiKeyPollingIndex = 1

	changed, err := UpdateChannelStatusWithError(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "first key failed")
	require.NoError(t, err)
	assert.True(t, changed)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)

	cached, err = CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, cached.ChannelInfo.MultiKeyStatusList[0])
	assert.Equal(t, 1, cached.ChannelInfo.MultiKeyPollingIndex)

	changed, err = UpdateChannelStatusWithError(channel.Id, "key-b", common.ChannelStatusAutoDisabled, "second key failed")
	require.NoError(t, err)
	assert.True(t, changed)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusAutoDisabled, false, false)

	cached, err = CacheGetChannel(channel.Id)
	require.NoError(t, err)
	_, _, apiErr := cached.GetNextEnabledKey()
	require.NotNil(t, apiErr)
	testKey, testIdx, testErr := cached.GetNextKeyForTest()
	require.Nil(t, testErr)
	assert.Equal(t, "key-b", testKey)
	assert.Equal(t, 1, testIdx)

	changed, err = UpdateChannelStatusWithError(channel.Id, testKey, common.ChannelStatusEnabled, "")
	require.NoError(t, err)
	assert.True(t, changed)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)

	cached, err = CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.True(t, cached.HasAvailableKey())
	// Only the probed key-b is restored; key-a stays auto-disabled until verified.
	assert.Equal(t, map[int]int{0: common.ChannelStatusAutoDisabled}, cached.ChannelInfo.MultiKeyStatusList)
	assert.NotContains(t, cached.GetOtherInfo(), "status_reason")
}

func TestChannelLevelEnableRestoresAutoDisabledMultiKeys(t *testing.T) {
	db := setupChannelStatusTestDB(t)
	channel := createChannelStatusFixture(t, db, "multi-key-manual-enable", "multi-key-tag", ChannelInfo{
		IsMultiKey:   true,
		MultiKeySize: 2,
		MultiKeyMode: constant.MultiKeyModeRandom,
	})
	InitChannelCache()

	_, err := UpdateChannelStatusWithError(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "key-a failed")
	require.NoError(t, err)
	_, err = UpdateChannelStatusWithError(channel.Id, "key-b", common.ChannelStatusAutoDisabled, "key-b failed")
	require.NoError(t, err)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusAutoDisabled, false, false)

	changed, err := UpdateChannelStatusWithError(channel.Id, "", common.ChannelStatusEnabled, "manual operation")
	require.NoError(t, err)
	assert.True(t, changed)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)

	cached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.True(t, cached.HasAvailableKey())
	assert.Empty(t, cached.ChannelInfo.MultiKeyStatusList)

	selected, err := GetRandomSatisfiedChannel("default", "gpt-4o", 0, "/v1/chat/completions")
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, channel.Id, selected.Id)
}

func TestUpdateChannelStatusDoesNotMutateCacheWhenDatabaseWriteFails(t *testing.T) {
	db := setupChannelStatusTestDB(t)
	channel := createChannelStatusFixture(t, db, "failed-status", "failed-status", ChannelInfo{})
	InitChannelCache()

	require.NoError(t, db.Exec(`CREATE TRIGGER fail_channel_status_update
		BEFORE UPDATE ON channels
		BEGIN
			SELECT RAISE(FAIL, 'forced channel update failure');
		END`).Error)

	changed, err := UpdateChannelStatusWithError(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual operation")
	require.Error(t, err)
	assert.False(t, changed)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)
}

func TestUpdateChannelStatusByTagRollsBackOnAbilityFailure(t *testing.T) {
	db := setupChannelStatusTestDB(t)
	channel := createChannelStatusFixture(t, db, "tag-rollback", "rollback-tag", ChannelInfo{})
	InitChannelCache()

	require.NoError(t, db.Exec(`CREATE TRIGGER fail_ability_status_update
		BEFORE UPDATE ON abilities
		BEGIN
			SELECT RAISE(FAIL, 'forced ability update failure');
		END`).Error)

	require.Error(t, DisableChannelByTag("rollback-tag"))
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)
}

func TestMultiKeyChannelUpdateRestoresStatusOnlyOnKeyEdit(t *testing.T) {
	db := setupChannelStatusTestDB(t)
	channel := createChannelStatusFixture(t, db, "multi-key-update", "", ChannelInfo{
		IsMultiKey:   true,
		MultiKeySize: 2,
		MultiKeyMode: constant.MultiKeyModeRandom,
	})
	setAutoDisabled := func(until int64) {
		info := map[string]interface{}{"status_reason": "upstream down", "keep": "me"}
		if until > 0 {
			info[channelStatusUntilKey] = until
		}
		otherInfo, err := common.Marshal(info)
		require.NoError(t, err)
		require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).
			Updates(map[string]interface{}{"status": common.ChannelStatusAutoDisabled, "other_info": string(otherInfo)}).Error)
	}
	load := func() *Channel {
		stored, err := GetChannelById(channel.Id, true)
		require.NoError(t, err)
		return stored
	}

	// Partial update without keys or other_info: status and metadata stay untouched.
	setAutoDisabled(0)
	partial := &Channel{Id: channel.Id, Name: "renamed", ChannelInfo: load().ChannelInfo}
	require.NoError(t, partial.Update())
	stored := load()
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.Status)
	assert.Equal(t, "upstream down", stored.GetOtherInfo()["status_reason"])
	assert.Equal(t, "me", stored.GetOtherInfo()["keep"])

	// Key edit while the cooldown is still running keeps the channel disabled.
	setAutoDisabled(common.GetTimestamp() + 3600)
	edit := &Channel{Id: channel.Id, Key: "key-c\nkey-d", Status: common.ChannelStatusAutoDisabled, ChannelInfo: load().ChannelInfo}
	require.NoError(t, edit.Update())
	assert.Equal(t, common.ChannelStatusAutoDisabled, load().Status)

	// Key edit after the cooldown re-enables and only clears status metadata from the stored other_info.
	setAutoDisabled(0)
	edit = &Channel{Id: channel.Id, Key: "key-c\nkey-d", ChannelInfo: load().ChannelInfo}
	require.NoError(t, edit.Update())
	stored = load()
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	assert.NotContains(t, stored.GetOtherInfo(), "status_reason")
	assert.Equal(t, "me", stored.GetOtherInfo()["keep"])
}

func TestChannelLevelEnableKeepsManuallyDisabledKeys(t *testing.T) {
	db := setupChannelStatusTestDB(t)
	channel := createChannelStatusFixture(t, db, "multi-key-manual-keep", "", ChannelInfo{
		IsMultiKey:   true,
		MultiKeySize: 2,
		MultiKeyMode: constant.MultiKeyModeRandom,
		MultiKeyStatusList: map[int]int{
			0: common.ChannelStatusManuallyDisabled,
			1: common.ChannelStatusAutoDisabled,
		},
	})
	require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Update("status", common.ChannelStatusAutoDisabled).Error)
	InitChannelCache()

	changed, err := UpdateChannelStatusWithError(channel.Id, "", common.ChannelStatusEnabled, "manual operation")
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	// The revoked key stays disabled; only the auto-disabled one is restored.
	assert.Equal(t, map[int]int{0: common.ChannelStatusManuallyDisabled}, stored.ChannelInfo.MultiKeyStatusList)
}
