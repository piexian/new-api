package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

func init() {
	model.SetTopUpCompletedHook(NotifyTopUpCompleted)
	model.SetSubscriptionCompletedHook(NotifySubscriptionCompleted)
}

func queueTransactionalEmail(userId int, event string, variables map[string]string) {
	if userId <= 0 {
		return
	}
	gopool.Go(func() {
		user, err := model.GetUserById(userId, false)
		if err != nil || user == nil {
			common.SysLog(fmt.Sprintf("failed to load user %d for email event %s: %v", userId, event, err))
			return
		}
		setting := user.GetSetting()
		receiver := strings.TrimSpace(setting.NotificationEmail)
		if receiver == "" {
			receiver = strings.TrimSpace(user.Email)
		}
		if receiver == "" {
			return
		}
		if err := SendTemplatedEmail(event, setting.Language, receiver, variables); err != nil && !errors.Is(err, common.ErrDuplicateEmailSuppressed) {
			common.SysLog(fmt.Sprintf("failed to send email event %s to user %d: %v", event, userId, err))
		}
	})
}

func NotifyTopUpCompleted(event model.TopUpCompletedEvent) {
	if event.UserId <= 0 || event.QuotaAdded <= 0 {
		return
	}
	common.ResetQuotaNotificationSendLocks(event.UserId, BillingSourceWallet, 0)
	completedAt := event.CompletedAt
	if completedAt <= 0 {
		completedAt = time.Now().Unix()
	}
	queueTransactionalEmail(event.UserId, EmailTemplateEventTopUpSucceeded, map[string]string{
		"user_id":          fmt.Sprintf("%d", event.UserId),
		"order_no":         event.OrderNo,
		"quota_added":      logger.FormatQuota(int(event.QuotaAdded)),
		"payment_amount":   fmt.Sprintf("%.2f", event.PaymentAmount),
		"payment_method":   event.PaymentMethod,
		"payment_provider": event.PaymentProvider,
		"completed_at":     formatEmailTimestamp(completedAt),
	})
}

func NotifySubscriptionCompleted(event model.SubscriptionCompletedEvent) {
	if !shouldNotifySubscriptionCompleted(event) {
		return
	}
	nextResetAt := "-"
	if event.NextResetTime > 0 {
		nextResetAt = formatEmailTimestamp(event.NextResetTime)
	}
	queueTransactionalEmail(event.UserId, EmailTemplateEventSubscriptionSucceeded, map[string]string{
		"user_id":             fmt.Sprintf("%d", event.UserId),
		"subscription_id":     fmt.Sprintf("%d", event.SubscriptionId),
		"plan_id":             fmt.Sprintf("%d", event.PlanId),
		"subscription_name":   event.PlanTitle,
		"amount_total":        logger.FormatQuota(int(event.AmountTotal)),
		"start_at":            formatEmailTimestamp(event.StartTime),
		"end_at":              formatEmailTimestamp(event.EndTime),
		"next_reset_at":       nextResetAt,
		"reset_period":        event.ResetPeriod,
		"payment_amount":      fmt.Sprintf("%.2f", event.PaymentAmount),
		"payment_method":      event.PaymentMethod,
		"payment_provider":    event.PaymentProvider,
		"subscription_source": event.SubscriptionSource,
	})
}

func shouldNotifySubscriptionCompleted(event model.SubscriptionCompletedEvent) bool {
	return event.UserId > 0 && event.SubscriptionId > 0 && event.SubscriptionSource != "auto"
}

func NotifySubscriptionExpired(event model.ExpiredSubscriptionInfo) {
	if event.UserId <= 0 || event.SubscriptionId <= 0 {
		return
	}
	queueTransactionalEmail(event.UserId, EmailTemplateEventSubscriptionExpired, map[string]string{
		"user_id":               fmt.Sprintf("%d", event.UserId),
		"subscription_id":       fmt.Sprintf("%d", event.SubscriptionId),
		"plan_id":               fmt.Sprintf("%d", event.PlanId),
		"subscription_name":     event.PlanTitle,
		"expired_at":            formatEmailTimestamp(event.ExpiredAt),
		"subscription_source":   event.SubscriptionSource,
		"allow_wallet_overflow": fmt.Sprintf("%t", event.AllowWalletOverflow),
	})
}

func NotifyAccountDisabled(user model.User) {
	if user.Id <= 0 {
		return
	}
	reason := strings.TrimSpace(user.DisableReason)
	if reason == "" {
		reason = "Account disabled by an administrator"
	}
	displayName := strings.TrimSpace(user.DisplayName)
	if displayName == "" {
		displayName = user.Username
	}
	language := user.GetSetting().Language
	banType := "Permanent"
	banDuration := "Permanent"
	unbanAt := "-"
	if user.DisabledUntil > 0 {
		minutes := (user.DisabledUntil - time.Now().Unix() + 59) / 60
		if minutes < 1 {
			minutes = 1
		}
		banType = "Temporary"
		banDuration = fmt.Sprintf("%d minutes", minutes)
		unbanAt = formatEmailTimestamp(user.DisabledUntil)
	}
	switch language {
	case i18n.LangZhCN:
		if user.DisabledUntil > 0 {
			banType = "临时封禁"
			banDuration = strings.TrimSuffix(banDuration, " minutes") + " 分钟"
		} else {
			banType = "永久封禁"
			banDuration = "永久"
		}
	case i18n.LangZhTW:
		if user.DisabledUntil > 0 {
			banType = "暫時停用"
			banDuration = strings.TrimSuffix(banDuration, " minutes") + " 分鐘"
		} else {
			banType = "永久停用"
			banDuration = "永久"
		}
	}
	queueTransactionalEmail(user.Id, EmailTemplateEventUserDisabled, map[string]string{
		"user_id":        fmt.Sprintf("%d", user.Id),
		"username":       user.Username,
		"display_name":   displayName,
		"disable_reason": reason,
		"disabled_at":    formatEmailTimestamp(time.Now().Unix()),
		"ban_type":       banType,
		"ban_duration":   banDuration,
		"unban_at":       unbanAt,
	})
}
