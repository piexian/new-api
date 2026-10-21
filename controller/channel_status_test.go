package controller

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelStatusControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldRedisEnabled := common.RedisEnabled
	oldMainDatabaseType := common.MainDatabaseType()
	oldLogDatabaseType := common.LogDatabaseType()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Log{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.RedisEnabled = oldRedisEnabled
		common.SetDatabaseTypes(oldMainDatabaseType, oldLogDatabaseType)
	})

	return db
}

func createChannelStatusControllerFixture(t *testing.T, db *gorm.DB) model.Channel {
	t.Helper()

	priority := int64(10)
	weight := uint(100)
	channel := model.Channel{
		Type:      constant.ChannelTypeOpenAI,
		Key:       "test-key",
		Status:    common.ChannelStatusEnabled,
		Name:      "legacy-status",
		Models:    "gpt-4o",
		Group:     "default",
		Priority:  &priority,
		Weight:    &weight,
		OtherInfo: "{}",
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     channel.Group,
		Model:     channel.Models,
		ChannelId: channel.Id,
		Enabled:   true,
		Priority:  channel.Priority,
		Weight:    *channel.Weight,
	}).Error)
	return channel
}

func TestUpdateChannelSupportsLegacyStatusOnlyRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupChannelStatusControllerTestDB(t)
	channel := createChannelStatusControllerFixture(t, db)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/channel/",
		bytes.NewBufferString(fmt.Sprintf(`{"id":%d,"status":%d}`, channel.Id, common.ChannelStatusManuallyDisabled)),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			ID     int `json:"id"`
			Status int `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, channel.Id, response.Data.ID)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, response.Data.Status)

	var stored model.Channel
	require.NoError(t, db.First(&stored, "id = ?", channel.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, stored.Status)
	var ability model.Ability
	require.NoError(t, db.First(&ability, "channel_id = ?", channel.Id).Error)
	assert.False(t, ability.Enabled)
}

func TestMultiKeyChannelRecoveryAcrossManageAndUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupChannelStatusControllerTestDB(t)

	t.Run("enable_key and enable_all_keys restore auto-disabled channel status", func(t *testing.T) {
		channel := createChannelStatusControllerFixture(t, db)
		converted, err := model.ConvertChannelToMultiKey(channel.Id)
		require.NoError(t, err)
		require.NoError(t, converted.SetKeys([]string{"key-1", "key-2"}))
		converted.Status = common.ChannelStatusAutoDisabled
		converted.SetOtherInfo(map[string]any{"status_reason": "All keys are disabled"})
		converted.ChannelInfo.MultiKeySize = 2
		converted.ChannelInfo.MultiKeyStatusList = map[int]int{
			0: common.ChannelStatusAutoDisabled,
			1: common.ChannelStatusAutoDisabled,
		}
		require.NoError(t, db.Save(converted).Error)
		require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", converted.Id).Update("enabled", false).Error)

		body := []byte(fmt.Sprintf(`{"channel_id":%d,"action":"enable_key","key_index":0}`, converted.Id))
		ctx, w := conversionTestContext(http.MethodPost, "/api/channel/multi_key/manage", body)
		ManageMultiKeys(ctx)
		require.Contains(t, w.Body.String(), `"success":true`)

		updated, err := model.GetChannelById(converted.Id, true)
		require.NoError(t, err)
		assert.Equal(t, common.ChannelStatusEnabled, updated.Status)
		assert.True(t, updated.HasAvailableKey())
		assert.NotContains(t, updated.GetOtherInfo(), "status_reason")

		var ability model.Ability
		require.NoError(t, db.First(&ability, "channel_id = ?", converted.Id).Error)
		assert.True(t, ability.Enabled)
	})

	t.Run("replace keys clears stale disabled state and restores auto-disabled channel", func(t *testing.T) {
		channel := createChannelStatusControllerFixture(t, db)
		converted, err := model.ConvertChannelToMultiKey(channel.Id)
		require.NoError(t, err)
		require.NoError(t, converted.SetKeys([]string{"bad-1", "bad-2"}))
		converted.Status = common.ChannelStatusAutoDisabled
		converted.ChannelInfo.MultiKeySize = 2
		converted.ChannelInfo.MultiKeyStatusList = map[int]int{
			0: common.ChannelStatusAutoDisabled,
			1: common.ChannelStatusAutoDisabled,
		}
		require.NoError(t, db.Save(converted).Error)
		require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", converted.Id).Update("enabled", false).Error)

		patchBody, err := common.Marshal(map[string]any{
			"id":       converted.Id,
			"type":     converted.Type,
			"key":      "fresh-1\nfresh-2",
			"key_mode": "replace",
		})
		require.NoError(t, err)
		ctx, w := conversionTestContext(http.MethodPut, "/api/channel/", patchBody)
		UpdateChannel(ctx)
		require.Contains(t, w.Body.String(), `"success":true`)

		updated, err := model.GetChannelById(converted.Id, true)
		require.NoError(t, err)
		assert.Equal(t, common.ChannelStatusEnabled, updated.Status)
		assert.Empty(t, updated.ChannelInfo.MultiKeyStatusList)
		key, _, apiErr := updated.GetNextEnabledKey()
		require.Nil(t, apiErr)
		assert.Contains(t, []string{"fresh-1", "fresh-2"}, key)
	})

	t.Run("automatic channel test probes and restores auto-disabled multi-key channel", func(t *testing.T) {
		oldAutoEnable := common.AutomaticEnableChannelEnabled
		common.AutomaticEnableChannelEnabled = true
		t.Cleanup(func() {
			common.AutomaticEnableChannelEnabled = oldAutoEnable
		})

		require.NoError(t, db.Save(&model.User{
			Id:       1,
			Username: "test-root",
			Role:     common.RoleRootUser,
			Status:   common.UserStatusEnabled,
			Group:    "default",
			Quota:    1000000,
			Setting:  `{"accept_unset_model_ratio_model":true}`,
		}).Error)

		var gotAuthHeader string
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuthHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		}))
		defer upstream.Close()

		channel := createChannelStatusControllerFixture(t, db)
		converted, err := model.ConvertChannelToMultiKey(channel.Id)
		require.NoError(t, err)
		require.NoError(t, converted.SetKeys([]string{"probe-key-1", "probe-key-2"}))
		baseURL := upstream.URL
		converted.BaseURL = &baseURL
		converted.Status = common.ChannelStatusAutoDisabled
		converted.ChannelInfo.MultiKeySize = 2
		converted.ChannelInfo.MultiKeyMode = constant.MultiKeyModePolling
		converted.ChannelInfo.MultiKeyStatusList = map[int]int{
			0: common.ChannelStatusAutoDisabled,
			1: common.ChannelStatusAutoDisabled,
		}
		require.NoError(t, db.Save(converted).Error)
		require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", converted.Id).Update("enabled", false).Error)

		summary := performChannelTests(context.Background(), []*model.Channel{converted}, 1, true, nil)
		assert.Equal(t, 1, summary.Tested)
		assert.Equal(t, 1, summary.Succeeded)
		assert.Equal(t, 1, summary.Enabled)
		assert.Equal(t, "Bearer probe-key-1", gotAuthHeader)

		updated, err := model.GetChannelById(converted.Id, true)
		require.NoError(t, err)
		assert.Equal(t, common.ChannelStatusEnabled, updated.Status)
		// Only the probed key is restored; the unverified one stays auto-disabled.
		assert.Equal(t, map[int]int{1: common.ChannelStatusAutoDisabled}, updated.ChannelInfo.MultiKeyStatusList)
		assert.True(t, updated.HasAvailableKey())

		var ability model.Ability
		require.NoError(t, db.First(&ability, "channel_id = ?", converted.Id).Error)
		assert.True(t, ability.Enabled)
	})
}
