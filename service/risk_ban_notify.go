package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
)

// 风控通知类型，作为独立的通知限流 key，避免封禁风暴期间挤占普通通知配额。
const (
	RiskNotifyTypeUser  = "risk_auto_ban_user"
	RiskNotifyTypeAdmin = "risk_auto_ban_admin"
)

// RiskBanInfo 描述一次风控封禁事件的完整上下文，供通知与审计日志复用。
type RiskBanInfo struct {
	Source          string // probe_guard | error_ban | ip_middleware
	Dimension       string // ip | user
	TriggerIP       string
	UserId          int
	Username        string
	DisplayName     string
	Reason          string
	IsPermanent     bool
	DurationMinutes int
	BannedAt        int64
	UnbanAt         int64
	OffenseCount    int
	TierLevel       int
	TierAction      string
	RuleId          string
	RuleName        string
	ErrorSample     string
	TriggeredModels string
	AppealHint      string
	DryRun          bool
}

// formatRiskTime 将 Unix 时间戳格式化为可读时间。
func formatRiskTime(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

// banDurationText 返回封禁时长的可读描述。
func (info RiskBanInfo) banDurationText(language string) string {
	if info.IsPermanent {
		switch language {
		case i18n.LangZhCN:
			return "永久封禁"
		case i18n.LangZhTW:
			return "永久停用"
		default:
			return "Permanent"
		}
	}
	if info.DurationMinutes <= 0 {
		return "-"
	}
	switch language {
	case i18n.LangZhCN:
		return fmt.Sprintf("%d 分钟", info.DurationMinutes)
	case i18n.LangZhTW:
		return fmt.Sprintf("%d 分鐘", info.DurationMinutes)
	default:
		return fmt.Sprintf("%d minutes", info.DurationMinutes)
	}
}

// toEmailVariables 构造邮件模板变量。
func (info RiskBanInfo) toEmailVariables(language string) map[string]string {
	isPermanent := "no"
	banType := "Temporary"
	if info.IsPermanent {
		isPermanent = "yes"
		banType = "Permanent"
	}
	if language == i18n.LangZhCN {
		banType = "临时封禁"
		if info.IsPermanent {
			banType = "永久封禁"
		}
	} else if language == i18n.LangZhTW {
		banType = "暫時停用"
		if info.IsPermanent {
			banType = "永久停用"
		}
	}
	return map[string]string{
		"user_id":          strconv.Itoa(info.UserId),
		"username":         info.Username,
		"display_name":     info.DisplayName,
		"ban_source":       info.Source,
		"ban_reason":       info.Reason,
		"is_permanent":     isPermanent,
		"ban_type":         banType,
		"ban_duration":     info.banDurationText(language),
		"banned_at":        formatRiskTime(info.BannedAt),
		"unban_at":         formatRiskTime(info.UnbanAt),
		"offense_count":    strconv.Itoa(info.OffenseCount),
		"tier_level":       strconv.Itoa(info.TierLevel),
		"tier_action":      info.TierAction,
		"rule_id":          info.RuleId,
		"rule_name":        info.RuleName,
		"error_sample":     info.ErrorSample,
		"triggered_models": info.TriggeredModels,
		"trigger_ip":       info.TriggerIP,
		"appeal_hint":      info.AppealHint,
	}
}

// describeTarget 返回封禁对象的简短描述，用于管理员通知标题。
func (info RiskBanInfo) describeTarget() string {
	if info.Dimension == model.RiskBanDimensionUser && info.UserId > 0 {
		if info.Username != "" {
			return fmt.Sprintf("用户 %s（#%d）", info.Username, info.UserId)
		}
		return fmt.Sprintf("用户 #%d", info.UserId)
	}
	if info.TriggerIP != "" {
		return fmt.Sprintf("IP %s", info.TriggerIP)
	}
	return "未知对象"
}

// adminContent 构造管理员通知正文（纯文本，用于 webhook/bark/gotify 与邮件通用模板）。
func (info RiskBanInfo) adminContent() string {
	permanent := "否"
	if info.IsPermanent {
		permanent = "是"
	}
	content := fmt.Sprintf("来源：%s\n对象：%s\n动作：%s\n原因：%s\n永久封禁：%s\n封禁时长：%s\n违规次数：%d\n时间：%s",
		info.Source, info.describeTarget(), info.TierAction, info.Reason, permanent,
		info.banDurationText(i18n.LangZhCN), info.OffenseCount, formatRiskTime(info.BannedAt))
	if info.RuleId != "" {
		content += fmt.Sprintf("\n规则：%s（%s）", info.RuleName, info.RuleId)
	}
	if info.TriggeredModels != "" {
		content += fmt.Sprintf("\n触发模型：%s", info.TriggeredModels)
	}
	if info.ErrorSample != "" {
		content += fmt.Sprintf("\n错误样本：%s", info.ErrorSample)
	}
	if info.DryRun {
		content += "\n（演练模式，未实际执行封禁）"
	}
	return content
}

// NotifyUserAutoBanned 向被封禁用户发送通知（含邮件模板）。
func NotifyUserAutoBanned(user *model.User, info RiskBanInfo) error {
	if user == nil {
		return nil
	}
	userSetting := user.GetSetting()
	variables := info.toEmailVariables(userSetting.Language)
	var subject, content string
	switch userSetting.Language {
	case i18n.LangZhCN:
		subject = fmt.Sprintf("[%s] 您的账号已被自动封禁", common.SystemName)
		content = fmt.Sprintf("您的账号 %s 因风控规则被封禁，原因：%s。封禁时长：%s。", info.Username, info.Reason, info.banDurationText(userSetting.Language))
		if info.UnbanAt > 0 {
			content += fmt.Sprintf("自动解封时间：%s。", formatRiskTime(info.UnbanAt))
		}
	case i18n.LangZhTW:
		subject = fmt.Sprintf("[%s] 您的帳號已被自動停用", common.SystemName)
		content = fmt.Sprintf("您的帳號 %s 因風控規則被停用，原因：%s。停用時長：%s。", info.Username, info.Reason, info.banDurationText(userSetting.Language))
		if info.UnbanAt > 0 {
			content += fmt.Sprintf("自動恢復時間：%s。", formatRiskTime(info.UnbanAt))
		}
	default:
		subject = fmt.Sprintf("[%s] Your account was automatically banned", common.SystemName)
		content = fmt.Sprintf("Your account %s was banned by automated risk control. Reason: %s. Duration: %s. ", info.Username, info.Reason, info.banDurationText(userSetting.Language))
		if info.UnbanAt > 0 {
			content += fmt.Sprintf("It will be restored automatically at %s. ", formatRiskTime(info.UnbanAt))
		}
	}
	content += info.AppealHint
	notification := dto.NewNotify(RiskNotifyTypeUser, subject, content, nil).
		WithEmailTemplate(EmailTemplateEventAccountAutoBanned, userSetting.Language, variables)
	return NotifyUser(user.Id, user.Email, userSetting, notification)
}

// NotifyAdminAutoBan 向管理员（root）发送风控封禁通知。
func NotifyAdminAutoBan(info RiskBanInfo) {
	subject := fmt.Sprintf("[%s] 风控自动封禁：%s", common.SystemName, info.describeTarget())
	notifyRootUser(dto.NewNotify(RiskNotifyTypeAdmin, subject, info.adminContent(), nil))
}
