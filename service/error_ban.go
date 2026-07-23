package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/risk_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// errorBanDedupeSeconds 同一 RequestId+RuleId 的触发去重窗口。
const errorBanDedupeSeconds = 300

// ErrorBanSnapshot 是进入异步处理前对请求上下文的不可变快照。
// 绝不能在 goroutine 中持有 *gin.Context（gin 会回收复用）。
type ErrorBanSnapshot struct {
	ClientIP   string
	UserId     int
	Username   string
	ModelName  string
	ErrorText  string
	ErrorCode  string
	StatusCode int
	RequestId  string
	Group      string
}

// truncateErrorBanString 按 rune 截断字符串。
func truncateErrorBanString(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// CheckErrorBan 在 relay 终态错误处同步抽取快照，随后异步执行错误封禁检测。
// 与错误日志开关相互独立，每次请求只计数一次。
func CheckErrorBan(c *gin.Context, relayInfo *relaycommon.RelayInfo, finalErr *types.NewAPIError) {
	if finalErr == nil || relayInfo == nil {
		return
	}
	// 快速路径：未启用时直接跳过，避免快照开销。
	if !risk_setting.GetErrorBanSetting().Enabled {
		return
	}
	snapshot := ErrorBanSnapshot{
		ClientIP:   c.ClientIP(),
		UserId:     relayInfo.UserId,
		Username:   c.GetString("username"),
		ModelName:  relayInfo.OriginModelName,
		ErrorText:  truncateErrorBanString(finalErr.MaskSensitiveErrorWithStatusCode(), 2048),
		ErrorCode:  string(finalErr.GetErrorCode()),
		StatusCode: finalErr.StatusCode,
		RequestId:  c.GetString(common.RequestIdKey),
		Group:      riskRequestGroup(relayInfo),
	}
	gopool.Go(func() {
		processErrorBan(snapshot)
	})
}

// processErrorBan 异步执行规则匹配与处罚。
func processErrorBan(snap ErrorBanSnapshot) {
	setting := risk_setting.GetErrorBanSetting()
	if !setting.Enabled {
		return
	}
	if setting.IsStatusCodeExcluded(snap.StatusCode) {
		return
	}

	// 管理员豁免与用户名补齐。
	if snap.UserId > 0 {
		if userBase, err := model.GetUserCache(snap.UserId); err == nil && userBase != nil {
			if userBase.Role >= common.RoleAdminUser {
				return
			}
			if snap.Username == "" {
				snap.Username = userBase.Username
			}
		}
	}
	if setting.IsUserWhitelisted(snap.UserId) {
		return
	}
	if setting.IsGroupWhitelisted(snap.Group) {
		return
	}

	rules := risk_setting.GetCompiledRules()
	if len(rules) == 0 {
		// 缓存可能尚未构建（如启动加载顺序问题），按需重建一次。
		_ = risk_setting.RebuildRegexCache()
		rules = risk_setting.GetCompiledRules()
	}
	if len(rules) == 0 {
		return
	}

	for _, cr := range rules {
		if cr.Matches(snap.ErrorText, snap.ErrorCode) {
			processErrorBanRuleMatch(setting, snap, cr.Rule)
		}
	}
}

// processErrorBanRuleMatch 处理单条规则命中：窗口计数、阈值判定、阶梯处罚。
func processErrorBanRuleMatch(setting risk_setting.ErrorBanSetting, snap ErrorBanSnapshot, rule risk_setting.ErrorBanRule) {
	dimension := setting.ResolveDimension(rule.Dimension)
	if dimension == risk_setting.DimensionUser && snap.UserId <= 0 {
		dimension = risk_setting.DimensionIP
	}

	var windowKey string
	var target string
	contextValue := snap.ClientIP
	if dimension == risk_setting.DimensionUser {
		windowKey = fmt.Sprintf("error_ban:user:%d:%s", snap.UserId, rule.Id)
		target = strconv.Itoa(snap.UserId)
	} else {
		ip, ok := normalizeProbeClientIP(snap.ClientIP)
		if !ok {
			return
		}
		dimension = risk_setting.DimensionIP
		windowKey = riskIPKey("error_ban:ip:"+rule.Id, ip)
		target = ip
		contextValue = snap.Username
		if contextValue == "" && snap.UserId > 0 {
			contextValue = strconv.Itoa(snap.UserId)
		}
	}

	count := riskWindowAddEvent(windowKey, setting.WindowSeconds)
	recordRiskLiveProgress(riskLiveProgressRecord{
		RiskLiveTarget: RiskLiveTarget{
			Source:        RiskLiveSourceErrorBan,
			RuleId:        rule.Id,
			RuleName:      rule.Name,
			Dimension:     dimension,
			Target:        target,
			UserId:        snap.UserId,
			Username:      snap.Username,
			Context:       contextValue,
			CurrentCount:  count,
			Threshold:     rule.Threshold,
			WindowSeconds: setting.WindowSeconds,
		},
		WindowKey:   windowKey,
		CooldownKey: windowKey + ":offense",
	})
	if int(count) < rule.Threshold {
		return
	}

	// 同一 RequestId+RuleId 只触发一次。
	dedupeKey := fmt.Sprintf("error_ban:dedupe:%s:%s", snap.RequestId, rule.Id)
	if !riskCooldownAcquire(dedupeKey, errorBanDedupeSeconds) {
		return
	}
	// 每个窗口内同一目标+规则最多记一次违规，避免阶梯失控与通知轰炸。
	if !riskCooldownAcquire(windowKey+":offense", setting.WindowSeconds) {
		return
	}

	offenseCount, err := incrementErrorBanOffense(setting, snap, rule, dimension, int(count))
	if err != nil {
		common.SysError("error ban increment offense failed: " + err.Error())
		return
	}

	tier, ok := rule.MatchTier(offenseCount)
	if !ok {
		return
	}
	reason := buildErrorBanReason(setting, rule, tier)
	now := common.GetTimestamp()

	if setting.DryRun {
		info := buildErrorBanInfo(setting, snap, rule, tier, reason, dimension, offenseCount, now)
		info.DryRun = true
		writeErrorBanLog(info, snap.ErrorText, true)
		if setting.NotifyAdminEnabled {
			NotifyAdminAutoBan(info)
		}
		return
	}

	applyErrorBanPenalty(setting, snap, rule, tier, reason, dimension, offenseCount, now)
}

// incrementErrorBanOffense 按维度累加违规计数并返回最新违规次数。
func incrementErrorBanOffense(setting risk_setting.ErrorBanSetting, snap ErrorBanSnapshot, rule risk_setting.ErrorBanRule, dimension string, count int) (int, error) {
	windowStart := common.GetTimestamp() - int64(setting.WindowSeconds)
	if dimension == risk_setting.DimensionUser && snap.UserId > 0 {
		state, err := model.IncrementErrorBanUserState(snap.UserId, rule.Id, count, windowStart, snap.ErrorText)
		if err != nil {
			return 0, err
		}
		return state.OffenseCount, nil
	}
	ip, ok := normalizeProbeClientIP(snap.ClientIP)
	if !ok {
		return 0, fmt.Errorf("invalid client ip: %s", snap.ClientIP)
	}
	state, err := model.IncrementErrorBanIPState(ip, rule.Id, count, windowStart, snap.ErrorText)
	if err != nil {
		return 0, err
	}
	return state.OffenseCount, nil
}

// renderErrorBanTemplate 渲染封禁原因模板中的占位符。
func renderErrorBanTemplate(tpl string, rule risk_setting.ErrorBanRule, tier risk_setting.ErrorBanTier) string {
	replacer := strings.NewReplacer(
		"{rule_id}", rule.Id,
		"{rule_name}", rule.Name,
		"{pattern}", rule.Pattern,
		"{offense_count}", strconv.Itoa(tier.OffenseCount),
		"{action}", tier.Action,
	)
	return replacer.Replace(tpl)
}

// buildErrorBanReason 依据规则/全局模板构造封禁原因，并截断到 255 字符。
func buildErrorBanReason(setting risk_setting.ErrorBanSetting, rule risk_setting.ErrorBanRule, tier risk_setting.ErrorBanTier) string {
	var reason string
	switch {
	case strings.TrimSpace(rule.ReasonTemplate) != "":
		reason = renderErrorBanTemplate(rule.ReasonTemplate, rule, tier)
	case strings.TrimSpace(setting.DefaultReasonTemplate) != "":
		reason = renderErrorBanTemplate(setting.DefaultReasonTemplate, rule, tier)
	default:
		reason = fmt.Sprintf("触发自动封禁规则 %s 被封禁", rule.Id)
	}
	if strings.TrimSpace(tier.ReasonSuffix) != "" {
		reason = fmt.Sprintf("%s（%s）", reason, tier.ReasonSuffix)
	}
	return truncateErrorBanString(reason, 255)
}

func buildErrorBanInfo(setting risk_setting.ErrorBanSetting, snap ErrorBanSnapshot, rule risk_setting.ErrorBanRule, tier risk_setting.ErrorBanTier, reason, dimension string, offenseCount int, now int64) RiskBanInfo {
	return RiskBanInfo{
		Source:          model.RiskBanSourceErrorBan,
		Dimension:       dimension,
		TriggerIP:       snap.ClientIP,
		UserId:          snap.UserId,
		Username:        snap.Username,
		Reason:          reason,
		IsPermanent:     tier.Action == risk_setting.TierActionPermIPBan || tier.Action == risk_setting.TierActionBoth,
		DurationMinutes: tier.DurationMinutes,
		BannedAt:        now,
		OffenseCount:    offenseCount,
		TierLevel:       tier.OffenseCount,
		TierAction:      tier.Action,
		RuleId:          rule.Id,
		RuleName:        rule.Name,
		ErrorSample:     snap.ErrorText,
		AppealHint:      setting.AppealHint,
	}
}

func writeErrorBanLog(info RiskBanInfo, errorSample string, dryRun bool) {
	_ = model.CreateRiskBanLog(&model.RiskBanLog{
		Dimension:       info.Dimension,
		TargetIP:        info.TriggerIP,
		UserId:          info.UserId,
		Username:        info.Username,
		Source:          model.RiskBanSourceErrorBan,
		RuleId:          info.RuleId,
		RuleName:        info.RuleName,
		Action:          info.TierAction,
		DurationMinutes: info.DurationMinutes,
		IsPermanent:     info.IsPermanent,
		UnbanAt:         info.UnbanAt,
		OffenseCount:    info.OffenseCount,
		Reason:          info.Reason,
		ErrorSample:     errorSample,
		DryRun:          dryRun,
		CreatedAt:       info.BannedAt,
	})
}

// applyErrorBanPenalty 按阶梯动作执行处罚（IP 封禁和/或账号禁用）。
func applyErrorBanPenalty(setting risk_setting.ErrorBanSetting, snap ErrorBanSnapshot, rule risk_setting.ErrorBanRule, tier risk_setting.ErrorBanTier, reason, dimension string, offenseCount int, now int64) {
	baseInfo := buildErrorBanInfo(setting, snap, rule, tier, reason, dimension, offenseCount, now)

	switch tier.Action {
	case risk_setting.TierActionTempIPBan:
		applyErrorBanIPBan(snap, baseInfo, false)
	case risk_setting.TierActionPermIPBan:
		applyErrorBanIPBan(snap, baseInfo, true)
	case risk_setting.TierActionDisableUser:
		applyErrorBanUserBan(setting, snap, baseInfo)
	case risk_setting.TierActionBoth:
		applyErrorBanIPBan(snap, baseInfo, true)
		applyErrorBanUserBan(setting, snap, baseInfo)
	}

	if setting.NotifyAdminEnabled {
		NotifyAdminAutoBan(baseInfo)
	}
}

func applyErrorBanIPBan(snap ErrorBanSnapshot, info RiskBanInfo, permanent bool) {
	ip, ok := normalizeProbeClientIP(snap.ClientIP)
	if !ok {
		return
	}
	var unbanAt int64
	if !permanent {
		unbanAt = info.BannedAt + int64(info.DurationMinutes)*60
	}
	if err := model.UpsertProbeGuardIPBan(ip, info.Reason, unbanAt); err != nil {
		common.SysError("error ban apply ip ban failed: " + err.Error())
		return
	}
	info.Dimension = model.RiskBanDimensionIP
	info.TriggerIP = ip
	info.IsPermanent = permanent
	info.UnbanAt = unbanAt
	info.TierAction = risk_setting.TierActionTempIPBan
	if permanent {
		info.DurationMinutes = 0
		info.TierAction = risk_setting.TierActionPermIPBan
	}
	writeErrorBanLog(info, snap.ErrorText, false)
}

func applyErrorBanUserBan(setting risk_setting.ErrorBanSetting, snap ErrorBanSnapshot, info RiskBanInfo) {
	if snap.UserId <= 0 {
		return
	}
	durationMinutes := info.DurationMinutes
	isPermanent := durationMinutes == 0
	unbanAt := int64(0)
	if !isPermanent {
		unbanAt = info.BannedAt + int64(durationMinutes)*60
	}
	disabled, err := model.DisableUserByRiskBan(snap.UserId, info.Reason, durationMinutes, info.BannedAt)
	if err != nil {
		common.SysError("error ban disable user failed: " + err.Error())
		return
	}
	if !disabled {
		return
	}
	_ = model.InvalidateUserCache(snap.UserId)
	_ = model.InvalidateUserTokensCache(snap.UserId)

	info.Dimension = model.RiskBanDimensionUser
	info.IsPermanent = isPermanent
	info.UnbanAt = unbanAt
	info.TierAction = risk_setting.TierActionDisableUser
	writeErrorBanLog(info, snap.ErrorText, false)

	if setting.NotifyUserEnabled {
		if user, uErr := model.GetUserById(snap.UserId, false); uErr == nil {
			info.Username = user.Username
			info.DisplayName = user.DisplayName
			if nErr := NotifyUserAutoBanned(user, info); nErr != nil {
				common.SysLog("failed to notify error-ban user: " + nErr.Error())
			}
		}
	}
}
