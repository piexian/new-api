package service

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

func notificationEventCategory(event string) string {
	switch event {
	case EmailTemplateEventBalanceLow, EmailTemplateEventSubscriptionBalanceLow:
		return "quota"
	case EmailTemplateEventTopUpSucceeded:
		return "topup"
	case EmailTemplateEventSubscriptionSucceeded, EmailTemplateEventSubscriptionExpired, EmailTemplateEventSubscriptionResetQuota:
		return "subscription"
	case EmailTemplateEventUserDisabled, EmailTemplateEventAccountAutoBanned:
		return "security"
	case EmailTemplateEventChannelModelUpdates:
		return "upstream"
	default:
		return "system"
	}
}

func notificationCategory(data dto.Notify) string {
	if data.EmailTemplate != nil && data.EmailTemplate.Event != EmailTemplateEventGeneralNotification {
		return notificationEventCategory(data.EmailTemplate.Event)
	}
	if data.Type == dto.NotifyTypeQuotaExceed || strings.HasPrefix(data.Type, dto.NotifyTypeQuotaExceed+":") {
		return "quota"
	}
	if data.Type == RiskNotifyTypeUser {
		return "security"
	}
	return "system"
}
