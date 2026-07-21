package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

func IPBan() gin.HandlerFunc {
	return func(c *gin.Context) {
		ban, matched := model.MatchIPBan(c.ClientIP())
		if !matched {
			c.Next()
			return
		}
		c.Set(SkipAccessLogKey, true)

		// 永久封禁且标记 auto_ban_user 时，尝试根据请求中的令牌连坐封禁账号。
		// 该中间件位于鉴权之前，session 尚不可用，只能解析原始 Authorization 头。
		if ban.AutoBanUser && ban.ExpiresAt == 0 {
			authHeader := c.GetHeader("Authorization")
			clientIP := c.ClientIP()
			reason := ban.Reason
			gopool.Go(func() {
				handleIPBanAutoUserBan(authHeader, clientIP, reason)
			})
		}

		c.String(http.StatusForbidden, "该ip已被封禁，原因："+ban.Reason)
		c.Abort()
	}
}

// handleIPBanAutoUserBan 解析请求令牌并连坐封禁对应账号（仅普通用户）。
func handleIPBanAutoUserBan(authHeader, clientIP, reason string) {
	tokenKey := extractBearerTokenKey(authHeader)
	if tokenKey == "" {
		return
	}
	token, err := model.GetTokenByKey(tokenKey, false)
	if err != nil || token == nil || token.UserId == 0 {
		return
	}
	user, err := model.GetUserById(token.UserId, false)
	if err != nil || user == nil {
		return
	}
	// 管理员豁免连坐封禁。
	if user.Role >= common.RoleAdminUser {
		return
	}
	disabled, err := model.DisableUserByRiskBan(user.Id, reason)
	if err != nil {
		common.SysError("ip ban auto user ban failed: " + err.Error())
		return
	}
	if !disabled {
		// 已被禁用或不是普通用户，无需重复处理。
		return
	}
	_ = model.InvalidateUserCache(user.Id)
	_ = model.InvalidateUserTokensCache(user.Id)

	now := common.GetTimestamp()
	_ = model.CreateRiskBanLog(&model.RiskBanLog{
		Dimension:   model.RiskBanDimensionUser,
		TargetIP:    clientIP,
		UserId:      user.Id,
		Username:    user.Username,
		Source:      model.RiskBanSourceIPMiddleware,
		Action:      risk_setting.TierActionDisableUser,
		IsPermanent: true,
		Reason:      reason,
		CreatedAt:   now,
	})

	info := service.RiskBanInfo{
		Source:      model.RiskBanSourceIPMiddleware,
		Dimension:   model.RiskBanDimensionUser,
		TriggerIP:   clientIP,
		UserId:      user.Id,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Reason:      reason,
		IsPermanent: true,
		BannedAt:    now,
		TierAction:  risk_setting.TierActionDisableUser,
		AppealHint:  risk_setting.GetRiskCenterAppealHint(),
	}

	riskCenter := risk_setting.GetRiskCenterSetting()
	if riskCenter.NotifyIPCollateralUser {
		if err := service.NotifyUserAutoBanned(user, info); err != nil {
			common.SysLog("failed to notify collateral banned user: " + err.Error())
		}
	}
	if riskCenter.NotifyIPCollateralAdmin {
		service.NotifyAdminAutoBan(info)
	}
}

// extractBearerTokenKey 从 Authorization 头中提取令牌 key，逻辑与鉴权中间件保持一致。
func extractBearerTokenKey(authHeader string) string {
	key := strings.TrimSpace(authHeader)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	key = strings.TrimPrefix(key, "sk-")
	parts := strings.Split(key, "-")
	return parts[0]
}
