package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var (
	ErrOriginalPassword  = errors.New("user.original_password_incorrect")
	ErrPasswordUnchanged = errors.New("user.password_unchanged")
	ErrPasswordConflict  = errors.New("user.password_changed_retry")
)

// SetUserLoginPassword 设置或修改登录密码。已有密码的用户必须提供正确的
// 原密码且新密码不能与原密码相同；未设置过密码的用户（如第三方注册）直接设置。
func SetUserLoginPassword(userID int, originalPassword, password string) error {
	if err := common.ValidateLoginPassword(password); err != nil {
		return err
	}
	user, err := model.GetUserById(userID, true)
	if err != nil {
		return err
	}
	if user.Password != "" {
		if !common.ValidatePasswordAndHash(originalPassword, user.Password) {
			return ErrOriginalPassword
		}
		if common.ValidatePasswordAndHash(password, user.Password) {
			return ErrPasswordUnchanged
		}
	}
	hash, err := common.Password2Hash(password)
	if err != nil {
		return err
	}
	updated, err := model.UpdateUserPasswordIfUnchanged(userID, user.Password, hash)
	if err != nil {
		return err
	}
	if !updated {
		return ErrPasswordConflict
	}
	return nil
}
