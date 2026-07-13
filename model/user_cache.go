package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
)

// UserBase struct remains the same as it represents the cached data structure
type UserBase struct {
	Id            int    `json:"id"`
	Role          int    `json:"role"`
	Group         string `json:"group"`
	Email         string `json:"email"`
	Quota         int    `json:"quota"`
	Status        int    `json:"status"`
	Username      string `json:"username"`
	Setting       string `json:"setting"`
	DisableReason string `json:"disable_reason"`
}

func (user *UserBase) WriteContext(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
	common.SetContextKey(c, constant.ContextKeyUserQuota, user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserStatus, user.Status)
	common.SetContextKey(c, constant.ContextKeyUserEmail, user.Email)
	common.SetContextKey(c, constant.ContextKeyUserName, user.Username)
	common.SetContextKey(c, constant.ContextKeyUserSetting, user.GetSetting())
}

func (user *UserBase) GetSetting() dto.UserSetting {
	setting := dto.UserSetting{}
	if user.Setting != "" {
		err := common.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog("failed to unmarshal setting: " + err.Error())
		}
	}
	return setting
}

// getUserCacheKey returns the key for user cache
func getUserCacheKey(userId int) string {
	return fmt.Sprintf("user:%d", userId)
}

func getUserCacheVersionKey(userId int) string {
	return fmt.Sprintf("user:%d:version", userId)
}

// invalidateUserCache clears user cache
func invalidateUserCache(userId int) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisBumpVersionAndDelete(
		getUserCacheVersionKey(userId),
		getUserCacheKey(userId),
	)
}

// InvalidateUserCache is the exported version of invalidateUserCache.
// 供 controller 等上层包在用户状态变更（如禁用、删除、角色变更）后主动清理缓存。
func InvalidateUserCache(userId int) error {
	return invalidateUserCache(userId)
}

// fillUserCacheIfVersion 仅在数据库读取期间未发生失效时回填缓存。
func fillUserCacheIfVersion(user User, version int64) (bool, error) {
	if !common.RedisEnabled {
		return false, nil
	}

	return common.RedisHSetObjIfVersion(
		getUserCacheKey(user.Id),
		getUserCacheVersionKey(user.Id),
		version,
		user.ToBaseUser(),
		time.Duration(common.RedisKeyCacheSeconds())*time.Second,
	)
}

// updateUserCache 失效全量快照，避免并发回填用旧用户数据覆盖新字段。
func updateUserCache(user User) error {
	return invalidateUserCache(user.Id)
}

// GetUserCache gets complete user cache from hash
func GetUserCache(userId int) (*UserBase, error) {
	// Try getting from Redis first
	userCache, hasRoleField, err := cacheGetUserBase(userId)
	if err == nil {
		// 老版本缓存没有 Role 字段，会反序列化成 0。鉴权必须拿到最新角色，
		// 否则提升为管理员后仍可能被旧缓存当作游客处理。
		if hasRoleField {
			return userCache, nil
		}
	}

	var cacheVersion int64
	canFillCache := false
	if common.RedisEnabled {
		cacheVersion, err = common.RedisGetVersion(getUserCacheVersionKey(userId))
		canFillCache = err == nil
	}

	// If Redis fails, get from DB
	user, err := GetUserById(userId, false)
	if err != nil {
		return nil, err // Return nil and error if DB lookup fails
	}

	if canFillCache {
		userSnapshot := *user
		gopool.Go(func() {
			if _, err := fillUserCacheIfVersion(userSnapshot, cacheVersion); err != nil {
				common.SysLog("failed to fill user cache: " + err.Error())
			}
		})
	}

	return user.ToBaseUser(), nil
}

func cacheGetUserBase(userId int) (*UserBase, bool, error) {
	if !common.RedisEnabled {
		return nil, false, fmt.Errorf("redis is not enabled")
	}
	var userCache UserBase
	// Try getting from Redis first
	cacheKey := getUserCacheKey(userId)
	fields, err := common.RedisHGetObjFields(cacheKey, &userCache)
	if err != nil {
		return nil, false, err
	}
	_, hasRoleField := fields["Role"]
	return &userCache, hasRoleField, nil
}

// Add atomic quota operations using hash fields
func cacheIncrUserQuota(userId int, delta int64) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHIncrBy(getUserCacheKey(userId), "Quota", delta)
}

func cacheDecrUserQuota(userId int, delta int64) error {
	return cacheIncrUserQuota(userId, -delta)
}

// Helper functions to get individual fields if needed
func getUserGroupCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Group, nil
}

func getUserQuotaCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Quota, nil
}

func getUserStatusCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Status, nil
}

func getUserNameCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Username, nil
}

func getUserSettingCache(userId int) (dto.UserSetting, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return dto.UserSetting{}, err
	}
	return cache.GetSetting(), nil
}

// New functions for individual field updates
func updateUserStatusCache(userId int, status bool) error {
	if !common.RedisEnabled {
		return nil
	}
	statusInt := common.UserStatusEnabled
	if !status {
		statusInt = common.UserStatusDisabled
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Status", fmt.Sprintf("%d", statusInt))
}

func updateUserQuotaCache(userId int, quota int) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Quota", fmt.Sprintf("%d", quota))
}

// UpdateUserGroupCache 通过失效全量快照刷新分组，避免旧回填覆盖新分组。
func UpdateUserGroupCache(userId int, _ string) error {
	return invalidateUserCache(userId)
}

func updateUserEmailCache(userId int, email string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Email", email)
}

func updateUserNameCache(userId int, username string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Username", username)
}

func updateUserSettingCache(userId int, setting string) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisHSetField(getUserCacheKey(userId), "Setting", setting)
}

// GetUserLanguage returns the user's language preference from cache
// Uses the existing GetUserCache mechanism for efficiency
func GetUserLanguage(userId int) string {
	userCache, err := GetUserCache(userId)
	if err != nil {
		return ""
	}
	return userCache.GetSetting().Language
}
