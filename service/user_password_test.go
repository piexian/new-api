package service

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func TestConcurrentFirstPasswordUpdate(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.User{}))
	user := model.User{Username: "concurrent-password", Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, model.DB.Create(&user).Error)
	t.Cleanup(func() { model.DB.Unscoped().Where("id = ?", user.Id).Delete(&model.User{}) })

	start := make(chan struct{})
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, candidate := range []string{"first-hash", "second-hash"} {
		wg.Add(1)
		go func(hash string) {
			defer wg.Done()
			<-start
			updated, err := model.UpdateUserPasswordIfUnchanged(user.Id, "", hash)
			results <- updated
			errs <- err
		}(candidate)
	}
	close(start)
	wg.Wait()
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.NotEqual(t, <-results, <-results, "并发的首次设置只能有一个成功")
}

func TestSetUserLoginPassword(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.User{}))
	user := model.User{Username: "password-user", DisplayName: "unchanged", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, model.DB.Create(&user).Error)
	t.Cleanup(func() { model.DB.Unscoped().Where("id = ?", user.Id).Delete(&model.User{}) })

	has, err := model.HasUserPassword(user.Id)
	require.NoError(t, err)
	require.False(t, has)
	// 弱密码被拒
	require.ErrorIs(t, SetUserLoginPassword(user.Id, "", "password123"), common.ErrPasswordPolicy)
	// 首次设置不需要原密码
	require.NoError(t, SetUserLoginPassword(user.Id, "", "Password123!"))
	has, err = model.HasUserPassword(user.Id)
	require.NoError(t, err)
	require.True(t, has)
	// 已有密码：必须提供正确原密码，且新密码不能相同
	require.ErrorIs(t, SetUserLoginPassword(user.Id, "", "Another123!"), ErrOriginalPassword)
	require.ErrorIs(t, SetUserLoginPassword(user.Id, "wrong", "Another123!"), ErrOriginalPassword)
	require.ErrorIs(t, SetUserLoginPassword(user.Id, "Password123!", "Password123!"), ErrPasswordUnchanged)
	require.NoError(t, SetUserLoginPassword(user.Id, "Password123!", "Another123!"))
	stored, err := model.GetUserById(user.Id, true)
	require.NoError(t, err)
	require.True(t, common.ValidatePasswordAndHash("Another123!", stored.Password))
	require.Equal(t, "unchanged", stored.DisplayName)
	// 基于过期空哈希的并发更新失败
	updated, err := model.UpdateUserPasswordIfUnchanged(user.Id, "", "obsolete")
	require.NoError(t, err)
	require.False(t, updated)
	// 历史弱密码仍是有效凭据，可用于换强密码
	weakHash, err := common.Password2Hash("weak")
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&user).Update("password", weakHash).Error)
	require.NoError(t, SetUserLoginPassword(user.Id, "weak", "Strong123!"))
}
