package service

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
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

func riskRequestGroup(relayInfo *relaycommon.RelayInfo) string {
	if relayInfo == nil {
		return ""
	}
	for _, group := range []string{relayInfo.UsingGroup, relayInfo.TokenGroup, relayInfo.UserGroup} {
		if group = strings.TrimSpace(group); group != "" {
			return group
		}
	}
	return ""
}

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

// probeBanTier 根据目标维度自身的违规次数返回封禁时长（分钟）与是否永久。
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

func writeProbeGuardUserLog(reason, clientIP string, userBase *model.UserBase, models string, durationMinutes int, isPermanent bool, unbanAt int64, offenseCount int, dryRun bool, now int64) {
	_ = model.CreateRiskBanLog(&model.RiskBanLog{
		Dimension:       model.RiskBanDimensionUser,
		TargetIP:        clientIP,
		UserId:          userBase.Id,
		Username:        userBase.Username,
		Source:          model.RiskBanSourceProbeGuard,
		Action:          risk_setting.TierActionDisableUser,
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

func buildProbeGuardInfo(setting risk_setting.ProbeGuardSetting, dimension, reason, clientIP string, userBase *model.UserBase, models string, durationMinutes int, isPermanent bool, unbanAt int64, offenseCount int, dryRun bool, now int64) RiskBanInfo {
	tierAction := probeTierAction(isPermanent)
	if dimension == model.RiskBanDimensionUser {
		tierAction = risk_setting.TierActionDisableUser
	}
	return RiskBanInfo{
		Source:          model.RiskBanSourceProbeGuard,
		Dimension:       dimension,
		TriggerIP:       clientIP,
		UserId:          userBase.Id,
		Username:        userBase.Username,
		Reason:          reason,
		IsPermanent:     isPermanent,
		DurationMinutes: durationMinutes,
		BannedAt:        now,
		UnbanAt:         unbanAt,
		OffenseCount:    offenseCount,
		TierAction:      tierAction,
		TriggeredModels: models,
		AppealHint:      setting.AppealHint,
		DryRun:          dryRun,
	}
}

type probeGuardWindow struct {
	dimension   string
	target      string
	windowKey   string
	cooldownKey string
	count       int64
}

func observeProbeGuardWindows(setting risk_setting.ProbeGuardSetting, clientIP, modelName string, userBase *model.UserBase) []probeGuardWindow {
	windows := make([]probeGuardWindow, 0, 2)
	if setting.BansIP() {
		windows = append(windows, probeGuardWindow{
			dimension:   model.RiskBanDimensionIP,
			target:      clientIP,
			windowKey:   riskIPKey("probe_guard:ip", clientIP),
			cooldownKey: riskIPKey("probe_guard:cooldown:ip", clientIP),
		})
	}
	if setting.BansUser() {
		userTarget := strconv.Itoa(userBase.Id)
		windows = append(windows, probeGuardWindow{
			dimension:   model.RiskBanDimensionUser,
			target:      userTarget,
			windowKey:   "probe_guard:user:" + userTarget,
			cooldownKey: "probe_guard:cooldown:user:" + userTarget,
		})
	}

	triggered := make([]probeGuardWindow, 0, len(windows))
	for i := range windows {
		window := &windows[i]
		window.count = riskWindowAddDistinct(window.windowKey, modelName, setting.WindowSeconds)
		contextValue := clientIP
		if window.dimension == model.RiskBanDimensionIP {
			contextValue = userBase.Username
		}
		recordRiskLiveProgress(riskLiveProgressRecord{
			RiskLiveTarget: RiskLiveTarget{
				Source:        RiskLiveSourceProbeGuard,
				RuleId:        RiskLiveProbeGuardRuleID,
				RuleName:      "Probe Guard",
				Dimension:     window.dimension,
				Target:        window.target,
				UserId:        userBase.Id,
				Username:      userBase.Username,
				Context:       contextValue,
				CurrentCount:  window.count,
				Threshold:     setting.DistinctModelCount,
				WindowSeconds: setting.WindowSeconds,
			},
			WindowKey:   window.windowKey,
			CooldownKey: window.cooldownKey,
		})
		if int(window.count) >= setting.DistinctModelCount && riskCooldownAcquire(window.cooldownKey, setting.OffenseDedupeSeconds) {
			triggered = append(triggered, *window)
		}
	}
	return triggered
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
	if setting.IsGroupWhitelisted(riskRequestGroup(relayInfo)) {
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

	triggered := observeProbeGuardWindows(setting, clientIP, modelName, userBase)
	if len(triggered) == 0 {
		return nil
	}

	now := common.GetTimestamp()
	ipPermanent := false
	userPermanent := false
	processed := false
	for _, window := range triggered {
		windowModels := riskWindowMembers(window.windowKey)
		triggeredModels := strings.Join(windowModels, ",")
		if window.dimension == model.RiskBanDimensionIP {
			state, stateErr := model.IncrementProbeIPAbuseOffense(clientIP, userBase.Id, windowModels)
			if stateErr != nil {
				common.SysError("probe guard increment IP offense failed: " + stateErr.Error())
				continue
			}
			processed = true
			durationMinutes, isPermanent := probeBanTier(state.OffenseCount, setting)
			unbanAt := probeGuardUnbanAt(now, durationMinutes, isPermanent)
			reason := probeGuardReason(state.OffenseCount, setting.DryRun)
			if setting.DryRun {
				writeProbeGuardIPLog(reason, clientIP, userBase, triggeredModels, durationMinutes, isPermanent, unbanAt, state.OffenseCount, true, now)
				if setting.NotifyAdminEnabled {
					NotifyAdminAutoBan(buildProbeGuardInfo(setting, model.RiskBanDimensionIP, reason, clientIP, userBase, triggeredModels, durationMinutes, isPermanent, unbanAt, state.OffenseCount, true, now))
				}
				continue
			}
			ipPermanent = isPermanent
			if err := model.UpsertProbeGuardIPBan(clientIP, reason, unbanAt); err != nil {
				common.SysError("probe guard apply ip ban failed: " + err.Error())
				continue
			}
			writeProbeGuardIPLog(reason, clientIP, userBase, triggeredModels, durationMinutes, isPermanent, unbanAt, state.OffenseCount, false, now)
			if setting.NotifyAdminEnabled {
				NotifyAdminAutoBan(buildProbeGuardInfo(setting, model.RiskBanDimensionIP, reason, clientIP, userBase, triggeredModels, durationMinutes, isPermanent, unbanAt, state.OffenseCount, false, now))
			}
			continue
		}

		state, stateErr := model.IncrementProbeUserAbuseOffense(userBase.Id, clientIP, windowModels)
		if stateErr != nil {
			common.SysError("probe guard increment user offense failed: " + stateErr.Error())
			continue
		}
		processed = true
		durationMinutes, isPermanent := probeBanTier(state.OffenseCount, setting)
		userPermanent = isPermanent
		if setting.DryRun {
			unbanAt := probeGuardUnbanAt(now, durationMinutes, isPermanent)
			writeProbeGuardUserLog(setting.UserBanReason, clientIP, userBase, triggeredModels, durationMinutes, isPermanent, unbanAt, state.OffenseCount, true, now)
			if setting.NotifyAdminEnabled {
				NotifyAdminAutoBan(buildProbeGuardInfo(setting, model.RiskBanDimensionUser, setting.UserBanReason, clientIP, userBase, triggeredModels, durationMinutes, isPermanent, unbanAt, state.OffenseCount, true, now))
			}
			continue
		}
		userInfo, applied := applyProbeGuardUserBan(setting, clientIP, userBase, triggeredModels, state.OffenseCount, now)
		if applied && setting.NotifyAdminEnabled {
			NotifyAdminAutoBan(userInfo)
		}
	}
	if !processed || setting.DryRun {
		return nil
	}

	statusCode := http.StatusTooManyRequests
	if ipPermanent || userPermanent {
		statusCode = http.StatusForbidden
	}
	return types.NewErrorWithStatusCode(
		errors.New("bulk model probing detected"),
		types.ErrorCodeBulkProbeDetected,
		statusCode,
		types.ErrOptionWithSkipRetry(),
	)
}

func probeGuardUnbanAt(now int64, durationMinutes int, isPermanent bool) int64 {
	if isPermanent {
		return 0
	}
	return now + int64(durationMinutes)*60
}

// applyProbeGuardUserBan 禁用触发探测的账号并记录、通知。
func applyProbeGuardUserBan(setting risk_setting.ProbeGuardSetting, clientIP string, userBase *model.UserBase, models string, offenseCount int, now int64) (RiskBanInfo, bool) {
	durationMinutes, isPermanent := probeBanTier(offenseCount, setting)
	unbanAt := probeGuardUnbanAt(now, durationMinutes, isPermanent)
	info := buildProbeGuardInfo(setting, model.RiskBanDimensionUser, setting.UserBanReason, clientIP, userBase, models, durationMinutes, isPermanent, unbanAt, offenseCount, false, now)
	disabled, err := model.DisableUserByRiskBan(userBase.Id, setting.UserBanReason, durationMinutes, now)
	if err != nil {
		common.SysError("probe guard disable user failed: " + err.Error())
		return info, false
	}
	if !disabled {
		return info, false
	}
	_ = model.InvalidateUserCache(userBase.Id)
	_ = model.InvalidateUserTokensCache(userBase.Id)
	writeProbeGuardUserLog(setting.UserBanReason, clientIP, userBase, models, durationMinutes, isPermanent, unbanAt, offenseCount, false, now)

	if setting.NotifyUserEnabled {
		user, uErr := model.GetUserById(userBase.Id, false)
		if uErr != nil {
			return info, true
		}
		info.Username = user.Username
		info.DisplayName = user.DisplayName
		if nErr := NotifyUserAutoBanned(user, info); nErr != nil {
			common.SysLog("failed to notify probe-guard banned user: " + nErr.Error())
		}
	}
	return info, true
}
