package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultEmailTemplatesCoverEveryEventAndLocale(t *testing.T) {
	catalog := GetEmailTemplateCatalog()
	require.Len(t, catalog.Events, 17)
	require.ElementsMatch(t, []string{i18n.LangZhCN, i18n.LangZhTW, i18n.LangEn}, catalog.Locales)
	requiredEventPlaceholders := map[string][]string{
		EmailTemplateEventSubscriptionResetQuota: {"quota_status", "reset_period", "reset_at", "reset_in"},
		EmailTemplateEventSubscriptionSucceeded:  {"subscription_id", "payment_provider", "subscription_source"},
		EmailTemplateEventSubscriptionExpired:    {"subscription_id", "expired_at", "allow_wallet_overflow"},
		EmailTemplateEventTopUpSucceeded:         {"order_no", "quota_added", "completed_at"},
		EmailTemplateEventUserDisabled:           {"disable_reason", "disabled_at"},
	}

	for _, event := range catalog.Events {
		for _, placeholder := range emailTemplateBasePlaceholders {
			assert.Contains(t, event.Placeholders, placeholder, "%s should expose %s", event.Event, placeholder)
		}
		for _, placeholder := range requiredEventPlaceholders[event.Event] {
			assert.Contains(t, event.Placeholders, placeholder, "%s should expose %s", event.Event, placeholder)
		}
		sampleVariables := SampleEmailTemplateVariables(event.Event)
		for _, placeholder := range event.Placeholders {
			assert.Contains(t, sampleVariables, placeholder, "%s should provide a preview value for %s", event.Event, placeholder)
		}
		for _, locale := range catalog.Locales {
			template, err := GetEmailTemplate(event.Event, locale)
			require.NoError(t, err, "%s/%s", event.Event, locale)
			assert.NotEmpty(t, template.Subject)
			assert.Contains(t, template.HTML, "<!doctype html>")
			assert.Contains(t, template.HTML, "\n  <head>")
			assert.False(t, template.IsCustom)
			require.NoError(t, ValidateEmailTemplate(event.Event, template.Subject, template.HTML))
			_, err = RenderEmailTemplate(template, sampleVariables)
			require.NoError(t, err, "%s/%s", event.Event, locale)
		}
	}
}

func TestNormalizeEmailTemplateLocaleUsesPreferenceAndConfiguredFallback(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	wasNil := common.OptionMap == nil
	if wasNil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[common.EmailDefaultLanguageOptionKey]
	common.OptionMap[common.EmailDefaultLanguageOptionKey] = i18n.LangZhTW
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if wasNil {
			common.OptionMap = nil
			return
		}
		if existed {
			common.OptionMap[common.EmailDefaultLanguageOptionKey] = previous
			return
		}
		delete(common.OptionMap, common.EmailDefaultLanguageOptionKey)
	})

	testCases := map[string]string{
		"en-US":   i18n.LangEn,
		"zhCN":    i18n.LangZhCN,
		"zh-Hans": i18n.LangZhCN,
		"zhTW":    i18n.LangZhTW,
		"zh-Hant": i18n.LangZhTW,
		"zh-HK":   i18n.LangZhTW,
		"fr":      i18n.LangZhTW,
		"":        i18n.LangZhTW,
	}
	for locale, expected := range testCases {
		assert.Equal(t, expected, NormalizeEmailTemplateLocale(locale), locale)
	}
}

func TestNormalizeEmailTemplateLocaleFallsBackToDefaultForInvalidDefault(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	wasNil := common.OptionMap == nil
	if wasNil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[common.EmailDefaultLanguageOptionKey]
	common.OptionMap[common.EmailDefaultLanguageOptionKey] = "invalid"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if wasNil {
			common.OptionMap = nil
			return
		}
		if existed {
			common.OptionMap[common.EmailDefaultLanguageOptionKey] = previous
			return
		}
		delete(common.OptionMap, common.EmailDefaultLanguageOptionKey)
	})

	assert.Equal(t, i18n.DefaultLang, NormalizeEmailTemplateLocale("vi"))
}

func TestRenderEmailTemplateEscapesHTMLAndSanitizesSubject(t *testing.T) {
	template := EmailTemplate{
		Event:   EmailTemplateEventSystemTest,
		Subject: "Test {{ site_name }}\r\nInjected",
		HTML:    `<p>{{ site_name }}</p><p>{{ provider }}</p>`,
	}
	rendered, err := RenderEmailTemplate(template, map[string]string{
		"site_name": `<strong>Example</strong>`,
		"provider":  `SMTP & more`,
	})
	require.NoError(t, err)
	assert.NotContains(t, rendered.Subject, "\r")
	assert.NotContains(t, rendered.Subject, "\n")
	assert.Contains(t, rendered.HTML, "&lt;strong&gt;Example&lt;/strong&gt;")
	assert.Contains(t, rendered.HTML, "SMTP &amp; more")
}

func TestValidateEmailTemplateRejectsUnknownPlaceholder(t *testing.T) {
	err := ValidateEmailTemplate(
		EmailTemplateEventVerification,
		"Code {{ unknown }}",
		"<p>{{ code }}</p>",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported placeholder")
}

func TestRenderEmailTemplateRejectsUnsafeActionURL(t *testing.T) {
	template, err := GetEmailTemplate(EmailTemplateEventPasswordReset, i18n.LangEn)
	require.NoError(t, err)
	_, err = RenderEmailTemplate(template, map[string]string{
		"site_name":     "New API",
		"reset_url":     "javascript:alert(1)",
		"valid_minutes": "10",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid reset_url")
}

func TestBalanceLowTemplateIncludesRechargeButton(t *testing.T) {
	template, err := GetEmailTemplate(EmailTemplateEventBalanceLow, i18n.LangZhCN)
	require.NoError(t, err)
	rendered, err := RenderEmailTemplate(template, map[string]string{
		"site_name":       "New API",
		"logo_url":        "",
		"current_balance": "1.00",
		"threshold":       "10.00",
		"recharge_url":    "https://billing.example.com/wallet",
		"quota_status":    "偏低",
	})
	require.NoError(t, err)
	assert.Contains(t, rendered.HTML, `href="https://billing.example.com/wallet"`)
	assert.Contains(t, rendered.HTML, "立即充值")
	assert.NotContains(t, rendered.HTML, emailOptionalURLAttribute)
}

func TestBalanceLowTemplateOmitsRechargeButtonWithoutServerAddress(t *testing.T) {
	template, err := GetEmailTemplate(EmailTemplateEventBalanceLow, i18n.LangEn)
	require.NoError(t, err)
	rendered, err := RenderEmailTemplate(template, map[string]string{
		"site_name":       "New API",
		"logo_url":        "",
		"current_balance": "1.00",
		"threshold":       "10.00",
		"recharge_url":    "",
		"quota_status":    "running low",
	})
	require.NoError(t, err)
	assert.NotContains(t, rendered.HTML, "Recharge now")
	assert.NotContains(t, rendered.HTML, `href=""`)
	assert.NotContains(t, rendered.HTML, "<img")
}

func TestSubscriptionResetTemplateIncludesRecoveryTime(t *testing.T) {
	template, err := GetEmailTemplate(EmailTemplateEventSubscriptionResetQuota, i18n.LangZhCN)
	require.NoError(t, err)
	rendered, err := RenderEmailTemplate(template, map[string]string{
		"subscription_name": "每日套餐",
		"subscription_id":   "108",
		"current_balance":   "1.00",
		"threshold":         "10.00",
		"quota_status":      "偏低",
		"reset_period":      "daily",
		"reset_at":          "2026-07-22 00:00:00 CST",
		"reset_in":          "17 小时",
	})
	require.NoError(t, err)
	assert.Contains(t, rendered.HTML, "2026-07-22 00:00:00 CST")
	assert.Contains(t, rendered.HTML, "17 小时")
}

func TestUserDisabledTemplateIncludesReason(t *testing.T) {
	template, err := GetEmailTemplate(EmailTemplateEventUserDisabled, i18n.LangZhCN)
	require.NoError(t, err)
	rendered, err := RenderEmailTemplate(template, map[string]string{
		"user_id":        "42",
		"username":       "example",
		"display_name":   "Example",
		"disable_reason": "恶意请求导致上游风险",
		"disabled_at":    "2026-07-21 07:00:00 CST",
	})
	require.NoError(t, err)
	assert.Contains(t, rendered.HTML, "封禁理由")
	assert.Contains(t, rendered.HTML, "恶意请求导致上游风险")
}

func TestAccountAutoBannedTemplateIsNoticeOnly(t *testing.T) {
	catalog := GetEmailTemplateCatalog()
	var placeholders []string
	for _, event := range catalog.Events {
		if event.Event == EmailTemplateEventAccountAutoBanned {
			placeholders = event.Placeholders
			break
		}
	}
	require.NotEmpty(t, placeholders)
	for _, placeholder := range []string{"ban_type", "ban_duration", "unban_at", "appeal_hint"} {
		assert.Contains(t, placeholders, placeholder)
	}
	for _, placeholder := range []string{
		"user_id",
		"username",
		"display_name",
		"ban_source",
		"ban_reason",
		"is_permanent",
		"banned_at",
		"offense_count",
		"tier_level",
		"tier_action",
		"rule_id",
		"rule_name",
		"error_sample",
		"triggered_models",
		"trigger_ip",
	} {
		assert.NotContains(t, placeholders, placeholder)
	}

	for _, locale := range []string{i18n.LangEn, i18n.LangZhCN, i18n.LangZhTW} {
		template, err := GetEmailTemplate(EmailTemplateEventAccountAutoBanned, locale)
		require.NoError(t, err)
		for _, token := range []string{
			"{{ user_id }}",
			"{{ username }}",
			"{{ ban_source }}",
			"{{ ban_reason }}",
			"{{ offense_count }}",
			"{{ rule_id }}",
			"{{ error_sample }}",
			"{{ trigger_ip }}",
		} {
			assert.NotContains(t, template.Subject+template.HTML, token)
		}
	}
}

func TestAccountAutoBannedTemplateIgnoresUnsafeLegacyOverride(t *testing.T) {
	key := emailTemplateOptionKey(EmailTemplateEventAccountAutoBanned, i18n.LangEn)
	common.OptionMapRWMutex.Lock()
	wasNil := common.OptionMap == nil
	if wasNil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[key]
	common.OptionMap[key] = `{"subject":"Legacy ban report","html":"<p>{{ error_sample }}</p>","updated_at":1}`
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if wasNil {
			common.OptionMap = nil
			return
		}
		if existed {
			common.OptionMap[key] = previous
			return
		}
		delete(common.OptionMap, key)
	})

	template, err := GetEmailTemplate(EmailTemplateEventAccountAutoBanned, i18n.LangEn)
	require.NoError(t, err)
	assert.False(t, template.IsCustom)
	assert.NotContains(t, template.HTML, "error_sample")
}

func TestBalanceLowRechargeURLUsesServerAddress(t *testing.T) {
	previous := system_setting.ServerAddress
	t.Cleanup(func() { system_setting.ServerAddress = previous })

	system_setting.ServerAddress = ""
	assert.Empty(t, GetBalanceLowRechargeURL())

	system_setting.ServerAddress = "https://billing.example.com/"
	assert.Equal(
		t,
		"https://billing.example.com"+common.ThemeAwarePath("/console/topup"),
		GetBalanceLowRechargeURL(),
	)
}

func TestEmailTemplateUsesConfiguredLogo(t *testing.T) {
	previousLogo := common.Logo
	previousServerAddress := system_setting.ServerAddress
	t.Cleanup(func() {
		common.Logo = previousLogo
		system_setting.ServerAddress = previousServerAddress
	})

	common.Logo = "/assets/logo.png"
	system_setting.ServerAddress = "https://billing.example.com"
	variables := SampleEmailTemplateVariables(EmailTemplateEventSystemTest)
	assert.Equal(t, "https://billing.example.com/assets/logo.png", variables["logo_url"])

	template, err := GetEmailTemplate(EmailTemplateEventSystemTest, i18n.LangEn)
	require.NoError(t, err)
	rendered, err := RenderEmailTemplate(template, variables)
	require.NoError(t, err)
	assert.Contains(t, rendered.HTML, `src="https://billing.example.com/assets/logo.png"`)
	assert.NotContains(t, rendered.HTML, emailOptionalURLAttribute)
}

func TestNotificationTemplatesIncludeOptOutHint(t *testing.T) {
	transactional := map[string]bool{
		EmailTemplateEventVerification:  true,
		EmailTemplateEventPasswordReset: true,
		EmailTemplateEventSystemTest:    true,
	}
	for _, event := range GetEmailTemplateCatalog().Events {
		for _, locale := range []string{i18n.LangEn, i18n.LangZhCN, i18n.LangZhTW} {
			template, err := GetEmailTemplate(event.Event, locale)
			require.NoError(t, err, "%s/%s", event.Event, locale)
			if transactional[event.Event] {
				assert.NotContains(t, template.HTML, "notification_settings_url", "%s/%s", event.Event, locale)
				continue
			}
			assert.Contains(t, template.HTML, `data-email-optional-url="{{ notification_settings_url }}"`, "%s/%s", event.Event, locale)
			assert.Contains(t, template.HTML, `href="{{ notification_settings_url }}"`, "%s/%s", event.Event, locale)
		}
	}
	assert.NotContains(t, emailOptOutHint(i18n.LangZhCN), "notification settings")
	assert.Contains(t, emailOptOutHint(i18n.LangZhCN), "通知设置")
	assert.Contains(t, emailOptOutHint(i18n.LangZhTW), "通知設定")
	assert.Contains(t, emailOptOutHint(i18n.LangEn), "notification settings")
}

func TestNotificationOptOutHintRendersThemeAwareURL(t *testing.T) {
	previousAddress := system_setting.ServerAddress
	previousTheme := common.GetTheme()
	t.Cleanup(func() {
		system_setting.ServerAddress = previousAddress
		common.SetTheme(previousTheme)
	})
	system_setting.ServerAddress = "https://api.example.com/"

	common.SetTheme(common.FrontendThemeDefault)
	assert.Equal(t, "https://api.example.com/profile", GetNotificationSettingsURL())
	common.SetTheme(common.FrontendThemeClassic)
	assert.Equal(t, "https://api.example.com/console/personal", GetNotificationSettingsURL())

	system_setting.ServerAddress = ""
	assert.Empty(t, GetNotificationSettingsURL())

	template, err := GetEmailTemplate(EmailTemplateEventBalanceLow, i18n.LangZhCN)
	require.NoError(t, err)
	rendered, err := RenderEmailTemplate(template, map[string]string{
		"site_name":                 "New API",
		"logo_url":                  "",
		"current_balance":           "1.00",
		"threshold":                 "10.00",
		"recharge_url":              "",
		"quota_status":              "偏低",
		"notification_settings_url": "https://api.example.com/profile",
	})
	require.NoError(t, err)
	assert.Contains(t, rendered.HTML, `href="https://api.example.com/profile"`)
	assert.Contains(t, rendered.HTML, "通知设置")
	assert.NotContains(t, rendered.HTML, emailOptionalURLAttribute)
}

func TestNotificationOptOutHintRemovedWithoutServerAddress(t *testing.T) {
	template, err := GetEmailTemplate(EmailTemplateEventGeneralNotification, i18n.LangEn)
	require.NoError(t, err)
	rendered, err := RenderEmailTemplate(template, map[string]string{
		"site_name":                 "New API",
		"logo_url":                  "",
		"notification_type":         "system.notice",
		"notification_title":        "System notification",
		"notification_content":      "A system event requires your attention.",
		"notification_settings_url": "",
	})
	require.NoError(t, err)
	assert.NotContains(t, rendered.HTML, "notification settings")
	assert.NotContains(t, rendered.HTML, `href=""`)
}

func TestLocalizeEmailVariableValues(t *testing.T) {
	values := map[string]string{
		"reset_period":          "never",
		"subscription_source":   "wallet",
		"payment_method":        "wallet",
		"payment_provider":      "balance",
		"allow_wallet_overflow": "true",
		"subscription_name":     "日卡（#4）",
	}
	localizeEmailVariableValues(values, i18n.LangZhCN)
	assert.Equal(t, "不重置", values["reset_period"])
	assert.Equal(t, "钱包余额购买", values["subscription_source"])
	assert.Equal(t, "钱包余额", values["payment_method"])
	assert.Equal(t, "余额支付", values["payment_provider"])
	assert.Equal(t, "允许", values["allow_wallet_overflow"])
	assert.Equal(t, "日卡（#4）", values["subscription_name"])

	valuesEn := map[string]string{
		"reset_period":        "monthly",
		"subscription_source": "redemption",
		"payment_method":      "alipay",
	}
	localizeEmailVariableValues(valuesEn, i18n.LangEn)
	assert.Equal(t, "Monthly", valuesEn["reset_period"])
	assert.Equal(t, "Redemption code", valuesEn["subscription_source"])
	assert.Equal(t, "Alipay", valuesEn["payment_method"])

	// 未知值原样保留
	valuesUnknown := map[string]string{"payment_method": "custom_gateway"}
	localizeEmailVariableValues(valuesUnknown, i18n.LangZhCN)
	assert.Equal(t, "custom_gateway", valuesUnknown["payment_method"])
}

func TestSubscriptionTemplatesRenderLocalizedEnumValues(t *testing.T) {
	rendered, err := renderTemplatedEmail(EmailTemplateEventSubscriptionExpired, i18n.LangZhCN, "user@example.com", map[string]string{
		"site_name":             "New API",
		"subscription_name":     "日卡",
		"plan_id":               "4",
		"subscription_id":       "9779",
		"expired_at":            "2026/09/20 16:06:09",
		"subscription_source":   "wallet",
		"allow_wallet_overflow": "true",
	})
	require.NoError(t, err)
	assert.Contains(t, rendered.HTML, "钱包余额购买")
	assert.Contains(t, rendered.HTML, "允许")
	assert.NotContains(t, rendered.HTML, ">wallet<")
}
