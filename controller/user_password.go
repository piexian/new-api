package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func respondPasswordError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrPasswordPolicy), errors.Is(err, service.ErrOriginalPassword),
		errors.Is(err, service.ErrPasswordUnchanged), errors.Is(err, service.ErrPasswordConflict):
		common.ApiErrorI18n(c, err.Error())
	default:
		common.ApiError(c, err)
	}
}

// UpdateSelfPassword 是唯一的自助设置/修改登录密码入口，仅接受会话认证。
func UpdateSelfPassword(c *gin.Context) {
	if c.GetBool("use_access_token") || sessions.Default(c).Get("id") != c.GetInt("id") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": i18n.T(c, i18n.MsgUnauthorized)})
		return
	}
	var request struct {
		Password         string `json:"password"`
		OriginalPassword string `json:"original_password"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userID := c.GetInt("id")
	hasBefore, err := model.HasUserPassword(userID)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if err := service.SetUserLoginPassword(userID, request.OriginalPassword, request.Password); err != nil {
		respondPasswordError(c, err)
		return
	}
	// 保留密码变更审计日志（不出现密码内容）
	content := "用户自助修改密码"
	if !hasBefore {
		content = "用户自助设置密码"
	}
	model.RecordLog(userID, model.LogTypeSystem, content)
	common.ApiSuccess(c, gin.H{"has_password": true})
}
