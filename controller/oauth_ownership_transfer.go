package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	oauthOwnershipTransferSessionKey   = "oauth_ownership_transfer_id"
	oauthOwnershipTransferRequiredCode = "oauth_ownership_transfer_required"
	oauthOwnershipTransferClosedCode   = "oauth_ownership_transfer_closed"
)

type oauthOwnershipTransferView struct {
	Active            bool   `json:"active"`
	Provider          string `json:"provider,omitempty"`
	Email             string `json:"email,omitempty"`
	CodeSent          bool   `json:"code_sent"`
	ExpiresAt         int64  `json:"expires_at"`
	FailedAttempts    int    `json:"failed_attempts"`
	AttemptsRemaining int    `json:"attempts_remaining"`
	MaxAttempts       int    `json:"max_attempts"`
	Mode              string `json:"mode,omitempty"`
	Closed            bool   `json:"closed"`
}

type oauthOwnershipTransferConfirmRequest struct {
	Code string `json:"code"`
}

func oauthOwnershipProviderKey(routeProvider string, provider oauth.Provider) string {
	if genericProvider, ok := provider.(*oauth.GenericOAuthProvider); ok {
		return fmt.Sprintf("custom:%d", genericProvider.GetProviderId())
	}
	if key := strings.ToLower(strings.TrimSpace(routeProvider)); key != "" {
		return key
	}
	if provider != nil {
		return strings.ToLower(strings.TrimSpace(provider.GetName()))
	}
	return "oauth"
}

func oauthOwnershipBindingDescriptor(provider oauth.Provider, providerUserId string) (string, int, bool) {
	if genericProvider, ok := provider.(*oauth.GenericOAuthProvider); ok {
		return "", genericProvider.GetProviderId(), genericProvider.GetProviderId() > 0
	}
	probe := &model.User{}
	provider.SetProviderUserID(probe, providerUserId)
	switch {
	case probe.GitHubId == providerUserId:
		return "github_id", 0, true
	case probe.DiscordId == providerUserId:
		return "discord_id", 0, true
	case probe.OidcId == providerUserId:
		return "oidc_id", 0, true
	case probe.LinuxDOId == providerUserId:
		return "linux_do_id", 0, true
	case probe.WeChatId == providerUserId:
		return "wechat_id", 0, true
	case probe.TelegramId == providerUserId:
		return "telegram_id", 0, true
	case probe.QQId == providerUserId:
		return "qq_id", 0, true
	case probe.SteamId == providerUserId:
		return "steam_id", 0, true
	default:
		return "", 0, false
	}
}

func oauthOwnershipTransferExpiresAt(challenge *model.OAuthOwnershipTransfer) int64 {
	if challenge == nil || challenge.CodeSentAt <= 0 {
		return 0
	}
	return challenge.CodeSentAt + int64(common.VerificationValidMinutes*60)
}

func oauthOwnershipTransferViewFromModel(challenge *model.OAuthOwnershipTransfer) oauthOwnershipTransferView {
	remaining := model.OAuthOwnershipTransferMaxAttempts
	if challenge != nil {
		remaining -= challenge.FailedAttempts
	}
	if remaining < 0 {
		remaining = 0
	}
	if challenge == nil {
		return oauthOwnershipTransferView{
			MaxAttempts:       model.OAuthOwnershipTransferMaxAttempts,
			AttemptsRemaining: remaining,
		}
	}
	return oauthOwnershipTransferView{
		Active:            challenge.Status == model.OAuthOwnershipTransferStatusReady || challenge.Status == model.OAuthOwnershipTransferStatusCodeSent,
		Provider:          challenge.ProviderName,
		Email:             common.MaskEmail(challenge.Email),
		CodeSent:          challenge.Status == model.OAuthOwnershipTransferStatusCodeSent,
		ExpiresAt:         oauthOwnershipTransferExpiresAt(challenge),
		FailedAttempts:    challenge.FailedAttempts,
		AttemptsRemaining: remaining,
		MaxAttempts:       model.OAuthOwnershipTransferMaxAttempts,
		Mode:              challenge.Mode,
		Closed:            challenge.Status == model.OAuthOwnershipTransferStatusFailed || challenge.Status == model.OAuthOwnershipTransferStatusCompleted,
	}
}

func clearOAuthOwnershipTransferSession(session sessions.Session) {
	session.Delete(oauthOwnershipTransferSessionKey)
	if err := session.Save(); err != nil {
		common.SysLog("failed to clear OAuth ownership transfer session: " + err.Error())
	}
}

func getOAuthOwnershipTransferFromSession(c *gin.Context) (*model.OAuthOwnershipTransfer, sessions.Session, error) {
	session := sessions.Default(c)
	challengeId, ok := session.Get(oauthOwnershipTransferSessionKey).(int)
	if !ok || challengeId <= 0 {
		return nil, session, model.ErrOAuthOwnershipTransferState
	}
	challenge, err := model.GetOAuthOwnershipTransferById(challengeId)
	return challenge, session, err
}

func prepareOAuthOwnershipTransfer(
	c *gin.Context,
	routeProvider string,
	provider oauth.Provider,
	oauthUser *oauth.OAuthUser,
	targetUser *model.User,
	conflict *OAuthEmailAlreadyTakenError,
	mode string,
) bool {
	if targetUser == nil || targetUser.Id <= 0 || oauthUser == nil || conflict == nil {
		return false
	}
	bindingColumn, customProviderId, ok := oauthOwnershipBindingDescriptor(provider, oauthUser.ProviderUserID)
	if !ok {
		conflict.OpportunityUnavailable = true
		return false
	}
	owners, err := model.GetUsersByNormalizedEmailUnscoped(conflict.Email, targetUser.Id)
	if err != nil {
		common.SysLog(fmt.Sprintf("[OAuth] ownership transfer owner lookup failed: target_user_id=%d email=%q error=%q", targetUser.Id, conflict.Email, err.Error()))
		common.ApiError(c, err)
		return true
	}
	if len(owners) != 1 || owners[0].DeletedAt.Valid {
		conflict.OpportunityUnavailable = true
		return false
	}

	challenge, err := model.PrepareOAuthOwnershipTransfer(
		oauthOwnershipProviderKey(routeProvider, provider),
		conflict.Provider,
		oauthUser.ProviderUserID,
		conflict.Email,
		owners[0].Id,
		targetUser.Id,
		mode,
		bindingColumn,
		customProviderId,
		common.GetTimestamp(),
	)
	if err != nil {
		if errors.Is(err, model.ErrOAuthOwnershipTransferUnavailable) {
			conflict.OpportunityUnavailable = true
			return false
		}
		common.SysLog(fmt.Sprintf("[OAuth] ownership transfer preparation failed: provider=%q target_user_id=%d previous_user_id=%d email=%q error=%q", conflict.Provider, targetUser.Id, owners[0].Id, conflict.Email, err.Error()))
		common.ApiError(c, err)
		return true
	}

	session := sessions.Default(c)
	session.Set(oauthOwnershipTransferSessionKey, challenge.Id)
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return true
	}
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"code":    oauthOwnershipTransferRequiredCode,
		"message": i18n.T(c, i18n.MsgOAuthOwnershipTransferRequired, providerParams(conflict.Provider)),
		"data":    oauthOwnershipTransferViewFromModel(challenge),
	})
	return true
}

func GetOAuthOwnershipTransferStatus(c *gin.Context) {
	challenge, session, err := getOAuthOwnershipTransferFromSession(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    oauthOwnershipTransferViewFromModel(nil),
		})
		return
	}
	if challenge.Status == model.OAuthOwnershipTransferStatusCodeSent && common.GetTimestamp() >= oauthOwnershipTransferExpiresAt(challenge) {
		_ = model.ExpireOAuthOwnershipTransfer(challenge.Id, common.GetTimestamp())
		common.DeleteKey(challenge.PairKey, common.OAuthOwnershipTransferPurpose)
		clearOAuthOwnershipTransferSession(session)
		view := oauthOwnershipTransferViewFromModel(challenge)
		view.Active = false
		view.Closed = true
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"code":    oauthOwnershipTransferClosedCode,
			"message": i18n.T(c, i18n.MsgOAuthOwnershipCodeExpired),
			"data":    view,
		})
		return
	}
	if challenge.Status != model.OAuthOwnershipTransferStatusReady && challenge.Status != model.OAuthOwnershipTransferStatusCodeSent {
		clearOAuthOwnershipTransferSession(session)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    oauthOwnershipTransferViewFromModel(challenge),
	})
}

func SendOAuthOwnershipTransferVerification(c *gin.Context) {
	challenge, _, err := getOAuthOwnershipTransferFromSession(c)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgOAuthOwnershipTransferUnavailable, providerParams("OAuth"))
		return
	}
	if challenge.Status == model.OAuthOwnershipTransferStatusCodeSent {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": i18n.T(c, i18n.MsgOAuthOwnershipCodeSent),
			"data":    oauthOwnershipTransferViewFromModel(challenge),
		})
		return
	}
	if challenge.Status != model.OAuthOwnershipTransferStatusReady {
		common.ApiErrorI18n(c, i18n.MsgOAuthOwnershipTransferUnavailable, providerParams(challenge.ProviderName))
		return
	}
	if err := common.CheckEmailVerificationDailyLimit(challenge.Email); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	ok, release := common.TryAcquireEmailVerificationSendLock(challenge.Email)
	if !ok {
		common.ApiErrorI18n(c, i18n.MsgOAuthOwnershipCodeSendLimited)
		return
	}
	claimed, err := model.ClaimOAuthOwnershipTransferSend(challenge.Id, common.GetTimestamp())
	if err != nil {
		release()
		common.ApiErrorI18n(c, i18n.MsgOAuthOwnershipTransferUnavailable, providerParams(challenge.ProviderName))
		return
	}

	code := common.GenerateVerificationCode(6)
	common.RegisterVerificationCodeWithKey(claimed.PairKey, code, common.OAuthOwnershipTransferPurpose)
	err = service.SendTemplatedEmail(
		service.EmailTemplateEventVerification,
		i18n.GetLangFromContext(c),
		claimed.Email,
		map[string]string{
			"code":                 code,
			"valid_minutes":        fmt.Sprintf("%d", common.VerificationValidMinutes),
			"verification_purpose": "oauth_ownership_transfer",
		},
	)
	if err != nil {
		common.DeleteKey(claimed.PairKey, common.OAuthOwnershipTransferPurpose)
		_ = model.ResetOAuthOwnershipTransferSend(claimed.Id)
		release()
		common.ApiError(c, err)
		return
	}
	common.IncrEmailVerificationDailyCount(claimed.Email)
	if err := model.MarkOAuthOwnershipTransferCodeSent(claimed.Id, common.GetTimestamp()); err != nil {
		common.DeleteKey(claimed.PairKey, common.OAuthOwnershipTransferPurpose)
		_ = model.CloseOAuthOwnershipTransfer(claimed.Id, "internal_error", common.GetTimestamp())
		common.ApiError(c, err)
		return
	}
	challenge, _ = model.GetOAuthOwnershipTransferById(claimed.Id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgOAuthOwnershipCodeSent),
		"data":    oauthOwnershipTransferViewFromModel(challenge),
	})
}

func ConfirmOAuthOwnershipTransfer(c *gin.Context) {
	var request oauthOwnershipTransferConfirmRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	if request.Code == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	challenge, session, err := getOAuthOwnershipTransferFromSession(c)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgOAuthOwnershipTransferUnavailable, providerParams("OAuth"))
		return
	}
	now := common.GetTimestamp()
	if challenge.Status != model.OAuthOwnershipTransferStatusCodeSent {
		common.ApiErrorI18n(c, i18n.MsgOAuthOwnershipTransferUnavailable, providerParams(challenge.ProviderName))
		return
	}
	if now >= oauthOwnershipTransferExpiresAt(challenge) {
		_ = model.ExpireOAuthOwnershipTransfer(challenge.Id, now)
		common.DeleteKey(challenge.PairKey, common.OAuthOwnershipTransferPurpose)
		clearOAuthOwnershipTransferSession(session)
		view := oauthOwnershipTransferViewFromModel(challenge)
		view.Active = false
		view.Closed = true
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    oauthOwnershipTransferClosedCode,
			"message": i18n.T(c, i18n.MsgOAuthOwnershipCodeExpired),
			"data":    view,
		})
		return
	}
	claimed, err := model.ClaimOAuthOwnershipTransferAttempt(challenge.Id, now)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgOAuthOwnershipTransferUnavailable, providerParams(challenge.ProviderName))
		return
	}
	if !common.VerifyCodeWithKey(claimed.PairKey, request.Code, common.OAuthOwnershipTransferPurpose) {
		rejected, rejectErr := model.RejectOAuthOwnershipTransferAttempt(claimed.Id, now)
		if rejectErr != nil {
			common.ApiError(c, rejectErr)
			return
		}
		view := oauthOwnershipTransferViewFromModel(rejected)
		if rejected.Status == model.OAuthOwnershipTransferStatusFailed {
			common.DeleteKey(rejected.PairKey, common.OAuthOwnershipTransferPurpose)
			clearOAuthOwnershipTransferSession(session)
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"code":    oauthOwnershipTransferClosedCode,
				"message": i18n.T(c, i18n.MsgOAuthOwnershipTransferClosed),
				"data":    view,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "oauth_ownership_code_invalid",
			"message": i18n.T(c, i18n.MsgOAuthOwnershipCodeInvalid, map[string]any{"Remaining": view.AttemptsRemaining}),
			"data":    view,
		})
		return
	}

	result, err := model.CompleteOAuthOwnershipTransfer(claimed.Id, now)
	common.DeleteKey(claimed.PairKey, common.OAuthOwnershipTransferPurpose)
	clearOAuthOwnershipTransferSession(session)
	if err != nil {
		_ = model.CloseOAuthOwnershipTransfer(claimed.Id, "transfer_failed", now)
		common.ApiError(c, err)
		return
	}
	if err := model.InvalidateUserCache(result.PreviousUser.Id); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate previous OAuth ownership user cache for user %d: %s", result.PreviousUser.Id, err.Error()))
	}
	if err := model.InvalidateUserTokensCache(result.PreviousUser.Id); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate previous OAuth ownership token cache for user %d: %s", result.PreviousUser.Id, err.Error()))
	}
	if err := model.InvalidateUserCache(result.TargetUser.Id); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate OAuth ownership target cache for user %d: %s", result.TargetUser.Id, err.Error()))
	}
	common.SysLog(fmt.Sprintf("[OAuth] ownership transferred: provider=%q provider_user_id=%q email=%q previous_user_id=%d target_user_id=%d", result.Challenge.ProviderName, result.Challenge.ProviderUserId, result.Challenge.Email, result.PreviousUser.Id, result.TargetUser.Id))

	if result.Challenge.Mode == model.OAuthOwnershipTransferModeBind {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": i18n.T(c, i18n.MsgOAuthOwnershipTransferSuccess),
			"data": gin.H{
				"action": "bind",
			},
		})
		return
	}
	setupLoginSession(&result.TargetUser, c)
}
