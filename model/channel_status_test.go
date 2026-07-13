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

	changed, err = UpdateChannelStatusWithError(channel.Id, "key-a", common.ChannelStatusEnabled, "")
	require.NoError(t, err)
	assert.True(t, changed)
	requireChannelStatusState(t, db, channel.Id, common.ChannelStatusEnabled, true, true)
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
