package controller

// MiniMax Code 渠道授权：OAuth 设备码登录（PKCE），成功后保存 JSON 凭证。

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/minimaxcode"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const minimaxCodeAuthFlowTTL = 15 * time.Minute

type minimaxCodeAuthStartRequest struct {
	Region string `json:"region"`
}

func minimaxCodeAuthSessionKey(channelID int, field string) string {
	return "minimaxcode_auth_" + field + "_" + strconv.Itoa(channelID)
}

func loadMiniMaxCodeAuthChannel(c *gin.Context) (*model.Channel, bool) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelIdFormatError)
		return nil, false
	}
	ch, err := model.GetChannelById(channelID, false)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.T(c, i18n.MsgChannelNotExists)})
		return nil, false
	}
	if ch.Type != constant.ChannelTypeMiniMaxCode {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.T(c, i18n.MsgChannelTypeNotMatched)})
		return nil, false
	}
	if ch.ChannelInfo.IsMultiKey {
		common.ApiErrorI18n(c, i18n.MsgChannelMultiKeyUnsupported)
		return nil, false
	}
	return ch, true
}

func startMiniMaxCodeAuth(c *gin.Context, channelID int) {
	request := minimaxCodeAuthStartRequest{}
	_ = common.DecodeJson(c.Request.Body, &request)
	region := minimaxcode.NormalizeRegion(request.Region)

	proxy := ""
	if channelID > 0 {
		ch, ok := loadMiniMaxCodeAuthChannel(c)
		if !ok {
			return
		}
		proxy = ch.GetSetting().Proxy
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	flow, result, err := service.StartMiniMaxCodeDeviceFlow(ctx, proxy, region)
	if err != nil {
		common.SysError("failed to start minimax code device flow: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "发起 MiniMax 授权失败，请稍后重试"})
		return
	}

	session := sessions.Default(c)
	prefix := channelID
	session.Set(minimaxCodeAuthSessionKey(prefix, "device_code"), flow.DeviceCode)
	session.Set(minimaxCodeAuthSessionKey(prefix, "user_code"), flow.UserCode)
	session.Set(minimaxCodeAuthSessionKey(prefix, "verifier"), flow.Verifier)
	session.Set(minimaxCodeAuthSessionKey(prefix, "region"), flow.Region)
	session.Set(minimaxCodeAuthSessionKey(prefix, "polling_user_code"), flow.UserCodePolling)
	session.Set(minimaxCodeAuthSessionKey(prefix, "expires_at"), flow.ExpiresAt.Unix())
	session.Set(minimaxCodeAuthSessionKey(prefix, "created_at"), time.Now().Unix())
	_ = session.Save()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"region":                    result.Region,
			"user_code":                 result.UserCode,
			"verification_url":          result.VerificationURL,
			"verification_url_complete": result.VerificationURLComplete,
			"interval_ms":               result.IntervalMs,
			"expires_in":                result.ExpiresInSec,
		},
	})
}

func pollMiniMaxCodeAuth(c *gin.Context, channelID int) {
	proxy := ""
	if channelID > 0 {
		ch, ok := loadMiniMaxCodeAuthChannel(c)
		if !ok {
			return
		}
		proxy = ch.GetSetting().Proxy
	}

	session := sessions.Default(c)
	get := func(field string) string {
		v, _ := session.Get(minimaxCodeAuthSessionKey(channelID, field)).(string)
		return v
	}
	deviceCode := get("device_code")
	userCode := get("user_code")
	verifier := get("verifier")
	region := minimaxcode.NormalizeRegion(get("region"))
	pollingUserCode, _ := session.Get(minimaxCodeAuthSessionKey(channelID, "polling_user_code")).(bool)
	createdAt, _ := session.Get(minimaxCodeAuthSessionKey(channelID, "created_at")).(int64)

	if deviceCode == "" || verifier == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": i18n.T(c, i18n.MsgOAuthFlowNotStarted)})
		return
	}
	if createdAt > 0 && time.Since(time.Unix(createdAt, 0)) > minimaxCodeAuthFlowTTL {
		clearMiniMaxCodeAuthSession(c, channelID)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "expired"}})
		return
	}

	flow := &service.MiniMaxCodeDeviceFlow{
		Region:          region,
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		Verifier:        verifier,
		UserCodePolling: pollingUserCode,
		ExpiresAt:       time.Now().Add(minimaxCodeAuthFlowTTL),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := service.PollMiniMaxCodeDeviceFlow(ctx, proxy, region, flow)
	if err != nil {
		common.SysError("failed to poll minimax code device flow: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "查询授权状态失败，请重试"})
		return
	}

	switch result.Status {
	case "pending":
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "pending"}})
	case "denied":
		clearMiniMaxCodeAuthSession(c, channelID)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "denied"}})
	case "expired":
		clearMiniMaxCodeAuthSession(c, channelID)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": "expired"}})
	case "ready":
		oauthKey := buildMiniMaxCodeOAuthKey(region, result.Grant)
		// 尽力补齐身份与套餐分组；失败不阻断保存。
		if identity := fetchMiniMaxCodeIdentityQuiet(ctx, proxy, region, result.Grant.AccessToken, ""); identity != nil {
			oauthKey.UserID = identity.RealUserID
			oauthKey.UserName = identity.UserName
			oauthKey.Email = identity.Email
			oauthKey.GroupID = identity.GroupID
		}
		encoded, err := oauthKey.Marshal()
		if err != nil {
			common.ApiError(c, err)
			return
		}
		clearMiniMaxCodeAuthSession(c, channelID)

		data := gin.H{
			"status":     "ready",
			"region":     region,
			"user_code":  userCode,
			"expires_at": oauthKey.Expired,
			"group_id":   oauthKey.GroupID,
		}

		if channelID > 0 {
			updated, err := model.UpdateSingleChannelKey(channelID, encoded)
			if err != nil {
				common.ApiError(c, err)
				return
			}
			if !updated {
				common.ApiErrorI18n(c, i18n.MsgChannelMultiKeyUnsupported)
				return
			}
			model.InitChannelCache()
			service.ResetProxyClientCache()
			data["channel_id"] = channelID
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "saved", "data": data})
			return
		}

		data["key"] = encoded
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "generated", "data": data})
	default:
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "未知授权状态: " + result.Status})
	}
}

func buildMiniMaxCodeOAuthKey(region string, grant *service.MiniMaxCodeTokenGrant) *service.MiniMaxCodeOAuthKey {
	return &service.MiniMaxCodeOAuthKey{
		AccessToken:  grant.AccessToken,
		RefreshToken: grant.RefreshToken,
		TokenType:    "Bearer",
		Scope:        grant.Scope,
		Subject:      grant.Subject,
		AccountID:    grant.AccountID,
		Region:       region,
		LastRefresh:  time.Now().Format(time.RFC3339),
		Expired:      grant.ExpiresAt.Format(time.RFC3339),
		Type:         "minimax_code",
	}
}

func fetchMiniMaxCodeIdentityQuiet(ctx context.Context, proxy, region, accessToken, userID string) *service.MiniMaxCodeIdentity {
	identityCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client, err := service.NewMiniMaxCodeAccountClient(proxy, region, accessToken, userID)
	if err != nil {
		return nil
	}
	identity, err := client.FetchMiniMaxCodeIdentity(identityCtx)
	if err != nil {
		return nil
	}
	return identity
}

func clearMiniMaxCodeAuthSession(c *gin.Context, channelID int) {
	session := sessions.Default(c)
	for _, field := range []string{"device_code", "user_code", "verifier", "region", "polling_user_code", "expires_at", "created_at"} {
		session.Delete(minimaxCodeAuthSessionKey(channelID, field))
	}
	_ = session.Save()
}

// ---- HTTP handlers ----

func StartMiniMaxCodeAuth(c *gin.Context) { startMiniMaxCodeAuth(c, 0) }
func StartMiniMaxCodeAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelIdFormatError)
		return
	}
	startMiniMaxCodeAuth(c, channelID)
}

func PollMiniMaxCodeAuth(c *gin.Context) { pollMiniMaxCodeAuth(c, 0) }

func PollMiniMaxCodeAuthForChannel(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelIdFormatError)
		return
	}
	pollMiniMaxCodeAuth(c, channelID)
}

// RefreshMiniMaxCodeChannelCredential 手动刷新渠道令牌。
func RefreshMiniMaxCodeChannelCredential(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgChannelIdFormatError)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	oauthKey, ch, err := service.RefreshMiniMaxCodeChannelCredential(ctx, channelID, service.MiniMaxCodeCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		common.SysError("failed to refresh minimax code credential: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "刷新凭证失败，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"channel_id":   ch.Id,
			"expires_at":   oauthKey.Expired,
			"last_refresh": oauthKey.LastRefresh,
			"group_id":     oauthKey.GroupID,
		},
	})
}
