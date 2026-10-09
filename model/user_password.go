package model

// HasUserPassword 直接读数据库，不走用户缓存（GetUserById 会省略密码），
// 同时覆盖已存在的旧会话。
func HasUserPassword(userID int) (bool, error) {
	var state struct {
		Password string
	}
	err := DB.Model(&User{}).Select("password").Where("id = ?", userID).First(&state).Error
	return state.Password != "", err
}

// UpdateUserPasswordIfUnchanged 用旧哈希做条件更新，防止两个首次设置密码的
// 并发请求互相覆盖，也拒绝基于过期密码哈希的变更。
func UpdateUserPasswordIfUnchanged(userID int, previousHash, nextHash string) (bool, error) {
	result := DB.Model(&User{}).Where("id = ? AND password = ?", userID, previousHash).Update("password", nextHash)
	return result.RowsAffected == 1, result.Error
}
