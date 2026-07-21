package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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
func (info RiskBanInfo) banDurationText() string {
	if info.IsPermanent {
		return "永久封禁"
	}
	if info.DurationMinutes <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d 分钟", info.DurationMinutes)
}

// toEmailVariables 构造邮件模板变量。
func (info RiskBanInfo) toEmailVariables() map[string]string {
	isPermanent := "no"
	if info.IsPermanent {
		isPermanent = "yes"
	}
	return map[string]string{
		"user_id":          strconv.Itoa(info.UserId),
		"username":         info.Username,
		"display_name":     info.DisplayName,
		"ban_source":       info.Source,
		"ban_reason":       info.Reason,
		"is_permanent":     isPermanent,
		"ban_duration":     info.banDurationText(),
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
		info.banDurationText(), info.OffenseCount, formatRiskTime(info.BannedAt))
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
	variables := info.toEmailVariables()
	subject := fmt.Sprintf("[%s] 您的账号已被自动封禁", common.SystemName)
	content := fmt.Sprintf("您的账号 %s 因风控规则被封禁，原因：%s。%s", info.Username, info.Reason, info.AppealHint)
	notification := dto.NewNotify(RiskNotifyTypeUser, subject, content, nil).
		WithEmailTemplate(EmailTemplateEventAccountAutoBanned, userSetting.Language, variables)
	return NotifyUser(user.Id, user.Email, userSetting, notification)
}

// NotifyAdminAutoBan 向管理员（root）发送风控封禁通知。
func NotifyAdminAutoBan(info RiskBanInfo) {
	subject := fmt.Sprintf("[%s] 风控自动封禁：%s", common.SystemName, info.describeTarget())
	notifyRootUser(dto.NewNotify(RiskNotifyTypeAdmin, subject, info.adminContent(), nil))
}
