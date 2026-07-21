package service

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// probeGuardCGNATPrefix 运营商级 NAT 地址段，视为内网，避免误封代理网关。
var probeGuardCGNATPrefix = netip.MustParsePrefix("100.64.0.0/10")

// normalizeProbeClientIP 规范化客户端 IP，拒绝非公网地址，避免误封内网网关。
func normalizeProbeClientIP(clientIP string) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(clientIP))
	if err != nil {
		return "", false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return "", false
	}
	if probeGuardCGNATPrefix.Contains(addr) {
		return "", false
	}
	return addr.String(), true
}

// probeBanTier 根据违规次数返回 IP 封禁时长（分钟）与是否永久。
func probeBanTier(offenseCount int, setting risk_setting.ProbeGuardSetting) (durationMinutes int, isPermanent bool) {
	if offenseCount >= setting.PermanentOffenseCount {
		return 0, true
	}
	if offenseCount <= 1 {
		return setting.FirstIPBanMinutes, false
	}
	return setting.SecondIPBanMinutes, false
}

func probeTierAction(isPermanent bool) string {
	if isPermanent {
		return risk_setting.TierActionPermIPBan
	}
	return risk_setting.TierActionTempIPBan
}

func probeGuardReason(offenseCount int, dryRun bool) string {
	if dryRun {
		return "批量模型探测自动封禁（演练模式）"
	}
	return fmt.Sprintf("批量模型探测自动封禁（第%d次违规）", offenseCount)
}

func writeProbeGuardIPLog(reason, clientIP string, userBase *model.UserBase, models string, durationMinutes int, isPermanent bool, unbanAt int64, offenseCount int, dryRun bool, now int64) {
	_ = model.CreateRiskBanLog(&model.RiskBanLog{
		Dimension:       model.RiskBanDimensionIP,
		TargetIP:        clientIP,
		UserId:          userBase.Id,
		Username:        userBase.Username,
		Source:          model.RiskBanSourceProbeGuard,
		Action:          probeTierAction(isPermanent),
		DurationMinutes: durationMinutes,
		IsPermanent:     isPermanent,
		UnbanAt:         unbanAt,
		OffenseCount:    offenseCount,
		Reason:          reason,
		Models:          models,
		DryRun:          dryRun,
		CreatedAt:       now,
	})
}

func writeProbeGuardUserLog(reason, clientIP string, userBase *model.UserBase, models string, offenseCount int, now int64) {
	_ = model.CreateRiskBanLog(&model.RiskBanLog{
		Dimension:    model.RiskBanDimensionUser,
		TargetIP:     clientIP,
		UserId:       userBase.Id,
		Username:     userBase.Username,
		Source:       model.RiskBanSourceProbeGuard,
		Action:       risk_setting.TierActionDisableUser,
		IsPermanent:  true,
		OffenseCount: offenseCount,
		Reason:       reason,
		Models:       models,
		CreatedAt:    now,
	})
}

func buildProbeGuardInfo(reason, clientIP string, userBase *model.UserBase, models string, durationMinutes int, isPermanent bool, unbanAt int64, offenseCount int, dryRun bool, now int64) RiskBanInfo {
	return RiskBanInfo{
		Source:          model.RiskBanSourceProbeGuard,
		Dimension:       model.RiskBanDimensionIP,
		TriggerIP:       clientIP,
		UserId:          userBase.Id,
		Username:        userBase.Username,
		Reason:          reason,
		IsPermanent:     isPermanent,
		DurationMinutes: durationMinutes,
		BannedAt:        now,
		UnbanAt:         unbanAt,
		OffenseCount:    offenseCount,
		TierAction:      probeTierAction(isPermanent),
		TriggeredModels: models,
		AppealHint:      risk_setting.GetProbeGuardSetting().AppealHint,
		DryRun:          dryRun,
	}
}

// CheckProbeGuard 检测批量模型探测行为。触发时返回中断请求的错误，否则返回 nil。
// 该函数在重试循环之前调用，保证每次用户请求只计数一次。
func CheckProbeGuard(c *gin.Context, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	setting := risk_setting.GetProbeGuardSetting()
	if !setting.Enabled || relayInfo == nil || relayInfo.UserId <= 0 {
		return nil
	}

	userBase, err := model.GetUserCache(relayInfo.UserId)
	if err != nil || userBase == nil {
		return nil // fail-open：无法确认用户身份时不拦截
	}
	if userBase.Role >= common.RoleAdminUser {
		return nil
	}
	if setting.IsUserWhitelisted(userBase.Id) {
		return nil
	}

	clientIP, ok := normalizeProbeClientIP(c.ClientIP())
	if !ok {
		return nil
	}

	modelName := strings.TrimSpace(relayInfo.OriginModelName)
	if modelName == "" {
		return nil
	}

	windowKey := riskIPKey("probe_guard:ip", clientIP)
	distinct := riskWindowAddDistinct(windowKey, modelName, setting.WindowSeconds)
	if int(distinct) < setting.DistinctModelCount {
		return nil
	}

	// 冷却去重：冷却窗口内只计一次违规。
	cooldownKey := riskIPKey("probe_guard:cooldown", clientIP)
	if !riskCooldownAcquire(cooldownKey, setting.OffenseDedupeSeconds) {
		return nil
	}

	triggeredModels := strings.Join(riskWindowMembers(windowKey), ",")
	now := common.GetTimestamp()

	// 演练模式：仅记录与通知，不实际封禁。
	if setting.DryRun {
		reason := probeGuardReason(0, true)
		writeProbeGuardIPLog(reason, clientIP, userBase, triggeredModels, 0, false, 0, 0, true, now)
		if setting.NotifyAdminEnabled {
			NotifyAdminAutoBan(buildProbeGuardInfo(reason, clientIP, userBase, triggeredModels, 0, false, 0, 0, true, now))
		}
		return nil
	}

	state, err := model.IncrementProbeIPAbuseOffense(clientIP, userBase.Id, riskWindowMembers(windowKey))
	if err != nil {
		common.SysError("probe guard increment offense failed: " + err.Error())
		return nil
	}

	durationMinutes, isPermanent := probeBanTier(state.OffenseCount, setting)
	reason := probeGuardReason(state.OffenseCount, false)
	var unbanAt int64
	if !isPermanent {
		unbanAt = now + int64(durationMinutes)*60
	}
	if err := model.UpsertProbeGuardIPBan(clientIP, reason, unbanAt); err != nil {
		common.SysError("probe guard apply ip ban failed: " + err.Error())
		return nil
	}

	writeProbeGuardIPLog(reason, clientIP, userBase, triggeredModels, durationMinutes, isPermanent, unbanAt, state.OffenseCount, false, now)
	info := buildProbeGuardInfo(reason, clientIP, userBase, triggeredModels, durationMinutes, isPermanent, unbanAt, state.OffenseCount, false, now)

	// 连坐禁用账号：达到阈值时封禁发起请求的账号。
	if setting.UserBanEnabled && state.OffenseCount >= setting.UserBanThreshold {
		applyProbeGuardUserBan(setting, clientIP, userBase, triggeredModels, state.OffenseCount, info, now)
	}

	if setting.NotifyAdminEnabled {
		NotifyAdminAutoBan(info)
	}

	statusCode := http.StatusTooManyRequests
	if isPermanent {
		statusCode = http.StatusForbidden
	}
	return types.NewErrorWithStatusCode(
		errors.New("bulk model probing detected"),
		types.ErrorCodeBulkProbeDetected,
		statusCode,
		types.ErrOptionWithSkipRetry(),
	)
}

// applyProbeGuardUserBan 禁用触发探测的账号并记录、通知。
func applyProbeGuardUserBan(setting risk_setting.ProbeGuardSetting, clientIP string, userBase *model.UserBase, models string, offenseCount int, info RiskBanInfo, now int64) {
	disabled, err := model.DisableUserByRiskBan(userBase.Id, setting.UserBanReason)
	if err != nil {
		common.SysError("probe guard disable user failed: " + err.Error())
		return
	}
	if !disabled {
		return
	}
	_ = model.InvalidateUserCache(userBase.Id)
	_ = model.InvalidateUserTokensCache(userBase.Id)
	_, _ = model.IncrementProbeUserAbuseOffense(userBase.Id, clientIP, riskWindowMembers(riskIPKey("probe_guard:ip", clientIP)))
	writeProbeGuardUserLog(setting.UserBanReason, clientIP, userBase, models, offenseCount, now)

	if setting.NotifyUserEnabled {
		user, uErr := model.GetUserById(userBase.Id, false)
		if uErr != nil {
			return
		}
		userInfo := info
		userInfo.Dimension = model.RiskBanDimensionUser
		userInfo.Reason = setting.UserBanReason
		userInfo.IsPermanent = true
		userInfo.TierAction = risk_setting.TierActionDisableUser
		if nErr := NotifyUserAutoBanned(user, userInfo); nErr != nil {
			common.SysLog("failed to notify probe-guard banned user: " + nErr.Error())
		}
	}
}
