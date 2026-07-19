package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/qwentokenplan"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type qwenOAuthCompleteRequest struct {
	APIKey string `json:"api_key"`
}

func qwenOAuthSessionKey(channelID int, field string) string {
	return fmt.Sprintf("qwen_oauth_%s_%d", field, channelID)
}

func StartQwenOAuth(c *gin.Context) {
	startQwenOAuthWithChannelID(c, 0)
}

func StartQwenOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	startQwenOAuthWithChannelID(c, channelID)
}

func startQwenOAuthWithChannelID(c *gin.Context, channelID int) {
	proxyURL := ""
	if channelID > 0 {
		channel, err := getQwenTokenPlanChannel(channelID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		proxyURL = channel.GetSetting().Proxy
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	flow, err := service.CreateQwenOAuthAuthorizationFlow(ctx, proxyURL, uuid.NewString())
	if err != nil {
		common.SysError("failed to start qwen oauth: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "启动 QianWen 授权失败，请稍后重试"})
		return
	}

	session := sessions.Default(c)
	session.Set(qwenOAuthSessionKey(channelID, "client_id"), flow.ClientID)
	session.Set(qwenOAuthSessionKey(channelID, "token"), flow.Token)
	session.Set(qwenOAuthSessionKey(channelID, "verifier"), flow.Verifier)
	session.Set(qwenOAuthSessionKey(channelID, "expires_at"), time.Now().Add(time.Duration(flow.ExpiresIn)*time.Second).Unix())
	_ = session.Save()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"verification_url": flow.VerificationURL,
			"expires_in":       flow.ExpiresIn,
			"interval":         flow.Interval,
		},
	})
}

func CompleteQwenOAuth(c *gin.Context) {
	completeQwenOAuthWithChannelID(c, 0)
}

func CompleteQwenOAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}
	completeQwenOAuthWithChannelID(c, channelID)
}

func completeQwenOAuthWithChannelID(c *gin.Context, channelID int) {
	request := qwenOAuthCompleteRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}

	proxyURL := ""
	apiKey := strings.TrimSpace(request.APIKey)
	var channel *model.Channel
	if channelID > 0 {
		var err error
		channel, err = getQwenTokenPlanChannel(channelID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
		proxyURL = channel.GetSetting().Proxy
		credential, err := qwentokenplan.ParseCredential(channel.Key)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "existing channel credential is invalid"})
			return
		}
		apiKey = credential.APIKey
	}
	if !strings.HasPrefix(apiKey, "sk-sp-") {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "a valid sk-sp- API key is required before OAuth authorization"})
		return
	}

	session := sessions.Default(c)
	clientID, _ := session.Get(qwenOAuthSessionKey(channelID, "client_id")).(string)
	token, _ := session.Get(qwenOAuthSessionKey(channelID, "token")).(string)
	verifier, _ := session.Get(qwenOAuthSessionKey(channelID, "verifier")).(string)
	expiresAt, _ := session.Get(qwenOAuthSessionKey(channelID, "expires_at")).(int64)
	if clientID == "" || token == "" || verifier == "" || (expiresAt > 0 && time.Now().Unix() >= expiresAt) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "qwen oauth flow not started or expired"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := service.PollQwenOAuthAuthorization(ctx, proxyURL, clientID, token, verifier)
	if err != nil {
		common.SysError("failed to poll qwen oauth: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "查询 QianWen 授权状态失败，请稍后重试"})
		return
	}
	if result.Status != "complete" {
		if result.Status == "expired_token" || result.Status == "access_denied" {
			clearQwenOAuthSession(session, channelID)
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    gin.H{"status": result.Status},
		})
		return
	}
	if result.Credentials == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "qwen oauth completed without credentials"})
		return
	}

	credential := qwentokenplan.Credential{
		Type:        "qwen_token_plan",
		APIKey:      apiKey,
		AccessToken: result.Credentials.AccessToken,
		ExpiresAt:   result.Credentials.ExpiresAt,
		User: qwentokenplan.CredentialUser{
			ID:       result.Credentials.User.ID,
			Email:    result.Credentials.User.Email,
			AliyunID: result.Credentials.User.AliyunID,
		},
	}
	encoded, err := qwentokenplan.EncodeCredential(credential)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	clearQwenOAuthSession(session, channelID)

	data := gin.H{
		"status":     "complete",
		"email":      credential.User.Email,
		"aliyun_id":  credential.User.AliyunID,
		"expires_at": credential.ExpiresAt,
	}
	if channelID > 0 {
		if err := model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("key", encoded).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		model.InitChannelCache()
		service.ResetProxyClientCache()
		data["channel_id"] = channelID
	} else {
		data["key"] = encoded
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

func getQwenTokenPlanChannel(channelID int) (*model.Channel, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("channel not found")
	}
	if channel.Type != constant.ChannelTypeQwenTokenPlan {
		return nil, fmt.Errorf("channel type is not Qwen Token Plan")
	}
	return channel, nil
}

func clearQwenOAuthSession(session sessions.Session, channelID int) {
	for _, field := range []string{"client_id", "token", "verifier", "expires_at"} {
		session.Delete(qwenOAuthSessionKey(channelID, field))
	}
	_ = session.Save()
}
