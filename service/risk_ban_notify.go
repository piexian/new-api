package service

import (
	"fmt"
	"strings"
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
			return "永久封禁"
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
	case i18n.LangEn:
		return fmt.Sprintf("%d minutes", info.DurationMinutes)
	default:
		return fmt.Sprintf("%d 分钟", info.DurationMinutes)
	}
}

func (info RiskBanInfo) banTypeText(language string) string {
	banType := "临时封禁"
	if info.IsPermanent {
		banType = "永久封禁"
	}
	switch language {
	case i18n.LangZhTW:
		banType = "暫時停用"
		if info.IsPermanent {
			banType = "永久停用"
		}
	case i18n.LangEn:
		banType = "Temporary"
		if info.IsPermanent {
			banType = "Permanent"
		}
	}
	return banType
}

// toUserEmailVariables only exposes fields users need to understand the
// restriction. Detailed risk context remains available to administrators.
func (info RiskBanInfo) toUserEmailVariables(language string) map[string]string {
	return map[string]string{
		"ban_type":     info.banTypeText(language),
		"ban_duration": info.banDurationText(language),
		"unban_at":     formatRiskTime(info.UnbanAt),
		"appeal_hint":  strings.TrimSpace(info.AppealHint),
	}
}

func (info RiskBanInfo) userNotification(language string) (string, string) {
	var subject, content string
	switch language {
	case i18n.LangZhCN:
		subject = fmt.Sprintf("[%s] 账号访问受限通知", common.SystemName)
		content = fmt.Sprintf("您的账号访问已受到限制。限制类型：%s；限制时长：%s。", info.banTypeText(language), info.banDurationText(language))
		if info.UnbanAt > 0 {
			content += fmt.Sprintf("预计恢复时间：%s。", formatRiskTime(info.UnbanAt))
		}
	case i18n.LangZhTW:
		subject = fmt.Sprintf("[%s] 帳號存取受限通知", common.SystemName)
		content = fmt.Sprintf("您的帳號存取已受到限制。限制類型：%s；限制時長：%s。", info.banTypeText(language), info.banDurationText(language))
		if info.UnbanAt > 0 {
			content += fmt.Sprintf("預計恢復時間：%s。", formatRiskTime(info.UnbanAt))
		}
	case i18n.LangEn:
		subject = fmt.Sprintf("[%s] Account access restriction notice", common.SystemName)
		content = fmt.Sprintf("Access to your account has been restricted. Restriction type: %s. Duration: %s.", info.banTypeText(language), info.banDurationText(language))
		if info.UnbanAt > 0 {
			content += fmt.Sprintf(" Expected restoration time: %s.", formatRiskTime(info.UnbanAt))
		}
	default:
		subject = fmt.Sprintf("[%s] 账号访问受限通知", common.SystemName)
		content = fmt.Sprintf("您的账号访问已受到限制。限制类型：%s；限制时长：%s。", info.banTypeText(language), info.banDurationText(language))
		if info.UnbanAt > 0 {
			content += fmt.Sprintf("预计恢复时间：%s。", formatRiskTime(info.UnbanAt))
		}
	}
	if appealHint := strings.TrimSpace(info.AppealHint); appealHint != "" {
		content += " " + appealHint
	}
	return subject, content
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
	subject, content := info.userNotification(userSetting.Language)
	notification := dto.NewNotify(RiskNotifyTypeUser, subject, content, nil).
		WithEmailTemplate(EmailTemplateEventAccountAutoBanned, userSetting.Language, info.toUserEmailVariables(userSetting.Language))
	return NotifyUser(user.Id, user.Email, userSetting, notification)
}

// NotifyAdminAutoBan 向管理员（root）发送风控封禁通知。
func NotifyAdminAutoBan(info RiskBanInfo) {
	subject := fmt.Sprintf("[%s] 风控自动封禁：%s", common.SystemName, info.describeTarget())
	notifyRootUser(dto.NewNotify(RiskNotifyTypeAdmin, subject, info.adminContent(), nil))
}
