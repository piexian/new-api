package middleware

import (
	"net/http"
	"net/url"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type turnstileCheckResponse struct {
	Success bool `json:"success"`
}

func ValidateTurnstile(c *gin.Context) bool {
	return validateTurnstile(c, true)
}

func ValidateTurnstileNoSession(c *gin.Context) bool {
	return validateTurnstile(c, false)
}

func validateTurnstile(c *gin.Context, allowSessionReuse bool) bool {
	if !common.TurnstileCheckEnabled {
		return true
	}

	var session sessions.Session
	if allowSessionReuse {
		session = sessions.Default(c)
		turnstileChecked := session.Get("turnstile")
		if turnstileChecked != nil {
			return true
		}
	}

	response := c.Query("turnstile")
	if response == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Turnstile token 为空",
		})
		c.Abort()
		return false
	}

	rawRes, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
		"secret":   {common.TurnstileSecretKey},
		"response": {response},
		"remoteip": {c.ClientIP()},
	})
	if err != nil {
		common.SysLog(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		c.Abort()
		return false
	}
	defer rawRes.Body.Close()

	var res turnstileCheckResponse
	err = common.DecodeJson(rawRes.Body, &res)
	if err != nil {
		common.SysLog(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		c.Abort()
		return false
	}
	if !res.Success {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Turnstile 校验失败，请刷新重试！",
		})
		c.Abort()
		return false
	}

	if allowSessionReuse {
		session.Set("turnstile", true)
		err = session.Save()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"message": "无法保存会话信息，请重试",
				"success": false,
			})
			c.Abort()
			return false
		}
	}
	return true
}

func TurnstileCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ValidateTurnstile(c) {
			return
		}
		c.Next()
	}
}

func TurnstileCheckNoSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ValidateTurnstileNoSession(c) {
			return
		}
		c.Next()
	}
}
