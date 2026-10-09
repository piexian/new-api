package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// PasswordSetupForSession 用于 OAuth 回调等公开路由：登录流程没有会话直接放行；
// 已有会话时属于账号绑定操作，同样要求先设置登录密码。
func PasswordSetupForSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, ok := sessions.Default(c).Get("id").(int); ok {
			if !requireUserPassword(c, id) {
				return
			}
		}
		c.Next()
	}
}

// requireUserPassword 拦截未设置登录密码的已认证用户，只放行查询自身信息与
// 设置密码两个接口，其余一律 403 + PASSWORD_SETUP_REQUIRED。
func requireUserPassword(c *gin.Context, userID int) bool {
	hasPassword, err := model.HasUserPassword(userID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "message": i18n.T(c, i18n.MsgDatabaseError)})
		return false
	}
	c.Set("has_password", hasPassword)
	if hasPassword || (c.Request.Method == http.MethodGet && c.FullPath() == "/api/user/self") ||
		(c.Request.Method == http.MethodPut && c.FullPath() == "/api/user/self/password") {
		return true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"success": false,
		"code":    "PASSWORD_SETUP_REQUIRED",
		"message": i18n.T(c, i18n.MsgUserPasswordSetupRequired),
	})
	return false
}
