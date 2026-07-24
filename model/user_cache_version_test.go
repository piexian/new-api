package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserCacheVersionTest(t *testing.T) *miniredis.Miniredis {
	t.Helper()

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	originalRedisClient := common.RDB
	originalRedisEnabled := common.RedisEnabled
	originalSyncFrequency := common.SyncFrequency
	originalDB := DB

	common.RDB = redisClient
	common.RedisEnabled = true
	common.SyncFrequency = 60

	dsn := fmt.Sprintf("file:user_cache_version_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}))
	DB = db

	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
		require.NoError(t, redisClient.Close())
		DB = originalDB
		common.RDB = originalRedisClient
		common.RedisEnabled = originalRedisEnabled
		common.SyncFrequency = originalSyncFrequency
	})

	return redisServer
}

func requireUserCacheEventually(t *testing.T, userId int, expectedRole int, expectedGroup string) {
	t.Helper()
	require.Eventually(t, func() bool {
		cached, hasRole, err := cacheGetUserBase(userId)
		return err == nil && hasRole && cached.Role == expectedRole && cached.Group == expectedGroup
	}, time.Second, 10*time.Millisecond)
}

func TestUserEditInvalidatesCachedGroup(t *testing.T) {
	redisServer := setupUserCacheVersionTest(t)
	user := User{
		Username: "group-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "vip",
	}
	require.NoError(t, DB.Create(&user).Error)

	written, err := fillUserCacheIfVersion(user, 0)
	require.NoError(t, err)
	require.True(t, written)
	require.True(t, redisServer.Exists(getUserCacheKey(user.Id)))

	updatedUser := user
	updatedUser.Group = "default"
	require.NoError(t, updatedUser.Edit(false))
	require.False(t, redisServer.Exists(getUserCacheKey(user.Id)))

	version, err := common.RedisGetVersion(getUserCacheVersionKey(user.Id))
	require.NoError(t, err)
	require.EqualValues(t, 1, version)

	cached, err := GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, "default", cached.Group)
	requireUserCacheEventually(t, user.Id, common.RoleCommonUser, "default")
}

func TestStaleUserCacheFillRejectedAfterRoleDowngrade(t *testing.T) {
	redisServer := setupUserCacheVersionTest(t)
	oldUser := User{
		Username: "role-user",
		Password: "password123",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Group:    "vip",
	}
	require.NoError(t, DB.Create(&oldUser).Error)

	versionBeforeDowngrade, err := common.RedisGetVersion(getUserCacheVersionKey(oldUser.Id))
	require.NoError(t, err)

	downgradedUser := oldUser
	downgradedUser.Role = common.RoleCommonUser
	require.NoError(t, downgradedUser.Update(false))

	written, err := fillUserCacheIfVersion(oldUser, versionBeforeDowngrade)
	require.NoError(t, err)
	require.False(t, written)
	require.False(t, redisServer.Exists(getUserCacheKey(oldUser.Id)))

	cached, err := GetUserCache(oldUser.Id)
	require.NoError(t, err)
	require.Equal(t, common.RoleCommonUser, cached.Role)
	requireUserCacheEventually(t, oldUser.Id, common.RoleCommonUser, "vip")
}

func TestUpdateUserGroupCacheRejectsOlderSnapshot(t *testing.T) {
	redisServer := setupUserCacheVersionTest(t)
	oldUser := User{
		Username: "subscription-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "vip",
	}
	require.NoError(t, DB.Create(&oldUser).Error)

	versionBeforeUpdate, err := common.RedisGetVersion(getUserCacheVersionKey(oldUser.Id))
	require.NoError(t, err)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", oldUser.Id).Update("group", "default").Error)
	require.NoError(t, UpdateUserGroupCache(oldUser.Id, "default"))

	written, err := fillUserCacheIfVersion(oldUser, versionBeforeUpdate)
	require.NoError(t, err)
	require.False(t, written)
	require.False(t, redisServer.Exists(getUserCacheKey(oldUser.Id)))

	cached, err := GetUserCache(oldUser.Id)
	require.NoError(t, err)
	require.Equal(t, "default", cached.Group)
	requireUserCacheEventually(t, oldUser.Id, common.RoleCommonUser, "default")
}

func TestGetUserCacheRefreshesSnapshotWithoutGitHubIdField(t *testing.T) {
	setupUserCacheVersionTest(t)
	user := User{
		Username: "github-cache-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		GitHubId: "123456",
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, common.RDB.HSet(context.Background(), getUserCacheKey(user.Id), map[string]interface{}{
		"Id":       user.Id,
		"Role":     user.Role,
		"Group":    user.Group,
		"Email":    user.Email,
		"Quota":    user.Quota,
		"Status":   user.Status,
		"Username": user.Username,
		"Setting":  user.Setting,
	}).Err())

	cached, err := GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, user.GitHubId, cached.GitHubId)
	require.Eventually(t, func() bool {
		refreshed, hasRequiredFields, cacheErr := cacheGetUserBase(user.Id)
		return cacheErr == nil && hasRequiredFields && refreshed.GitHubId == user.GitHubId
	}, time.Second, 10*time.Millisecond)
}
