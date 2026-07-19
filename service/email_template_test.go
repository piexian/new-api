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
	require.Len(t, catalog.Events, 11)
	require.ElementsMatch(t, []string{i18n.LangZhCN, i18n.LangZhTW, i18n.LangEn}, catalog.Locales)

	for _, event := range catalog.Events {
		for _, placeholder := range emailTemplateBasePlaceholders {
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

func TestNormalizeEmailTemplateLocaleFallsBackToEnglishForInvalidDefault(t *testing.T) {
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

	assert.Equal(t, i18n.LangEn, NormalizeEmailTemplateLocale("vi"))
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
	})
	require.NoError(t, err)
	assert.NotContains(t, rendered.HTML, "Recharge now")
	assert.NotContains(t, rendered.HTML, `href=""`)
	assert.NotContains(t, rendered.HTML, "<img")
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
