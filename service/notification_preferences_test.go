package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

func TestNotificationPreferences(t *testing.T) {
	for _, tc := range []struct{ event, category string }{
		{EmailTemplateEventBalanceLow, "quota"},
		{EmailTemplateEventSubscriptionBalanceLow, "quota"},
		{EmailTemplateEventTopUpSucceeded, "topup"},
		{EmailTemplateEventSubscriptionSucceeded, "subscription"},
		{EmailTemplateEventSubscriptionExpired, "subscription"},
		{EmailTemplateEventUserDisabled, "security"},
		{EmailTemplateEventAccountAutoBanned, "security"},
		{EmailTemplateEventChannelAutoDisabled, "system"},
		{EmailTemplateEventChannelModelUpdates, "upstream"},
	} {
		t.Run(tc.event, func(t *testing.T) {
			data := dto.NewNotify(dto.NotifyTypeChannelUpdate, "", "", nil).WithEmailTemplate(tc.event, "en", nil)
			category := notificationCategory(data)
			if category != tc.category {
				t.Fatalf("got %s, want %s", category, tc.category)
			}
			setting := dto.UserSetting{}
			if !setting.AllowsNotification(category) {
				t.Fatal("legacy settings must retain notifications")
			}
			setting.NotificationCategories = map[string]bool{category: false}
			if setting.AllowsNotification(category) {
				t.Fatal("disabled category allowed")
			}
		})
	}
	data := dto.NewNotify("quota_exceed:subscription:42:exhausted", "", "", nil)
	if notificationCategory(data) != "quota" {
		t.Fatal("dynamic quota type must use quota preference")
	}
}

func TestDisabledNotificationsSkipDeliveryAndLimit(t *testing.T) {
	disabled := false
	for _, setting := range []dto.UserSetting{
		{NotificationsEnabled: &disabled},
		{NotificationCategories: map[string]bool{"system": false}},
	} {
		setting.NotifyType = dto.NotifyTypeWebhook
		setting.WebhookUrl = ":invalid-url"
		data := dto.NewNotify("preference-test", "", "", nil)
		if err := NotifyUser(983241, "", setting, data); err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("983241:%s:%s", data.Type, time.Now().Format("2006010215"))
		if _, exists := notifyLimitStore.Load(key); exists {
			t.Fatal("disabled notification consumed rate limit")
		}
	}
}
