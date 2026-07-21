package service

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	xhtml "golang.org/x/net/html"
)

const (
	EmailTemplateEventVerification           = "auth.verify_code"
	EmailTemplateEventPasswordReset          = "auth.password_reset"
	EmailTemplateEventBalanceLow             = "balance.low"
	EmailTemplateEventTopUpSucceeded         = "balance.topup_succeeded"
	EmailTemplateEventSubscriptionBalanceLow = "subscription.balance_low"
	EmailTemplateEventSubscriptionResetQuota = "subscription.reset_quota"
	EmailTemplateEventSubscriptionSucceeded  = "subscription.succeeded"
	EmailTemplateEventSubscriptionExpired    = "subscription.expired"
	EmailTemplateEventUserDisabled           = "account.disabled"
	EmailTemplateEventAccountAutoBanned      = "account.auto_banned"
	EmailTemplateEventChannelAutoDisabled    = "channel.auto_disabled"
	EmailTemplateEventChannelAutoEnabled     = "channel.auto_enabled"
	EmailTemplateEventChannelQuotaCooldown   = "channel.quota_cooldown"
	EmailTemplateEventChannelTestResult      = "channel.test_result"
	EmailTemplateEventChannelModelUpdates    = "channel.model_updates"
	EmailTemplateEventGeneralNotification    = "notification.general"
	EmailTemplateEventSystemTest             = "system.test"

	emailTemplateSubjectMaxLength = 200
	emailTemplateHTMLMaxLength    = 50000
	emailOptionalURLAttribute     = "data-email-optional-url"
)

type EmailTemplateEventInfo struct {
	Event        string   `json:"event"`
	Placeholders []string `json:"placeholders"`
}

type EmailTemplateCatalog struct {
	Events  []EmailTemplateEventInfo `json:"events"`
	Locales []string                 `json:"locales"`
}

type EmailTemplate struct {
	Event        string   `json:"event"`
	Locale       string   `json:"locale"`
	Subject      string   `json:"subject"`
	HTML         string   `json:"html"`
	IsCustom     bool     `json:"is_custom"`
	UpdatedAt    int64    `json:"updated_at,omitempty"`
	Placeholders []string `json:"placeholders"`
}

type RenderedEmailTemplate struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type storedEmailTemplate struct {
	Subject   string `json:"subject"`
	HTML      string `json:"html"`
	UpdatedAt int64  `json:"updated_at"`
}

type defaultEmailTemplate struct {
	Subject string
	HTML    string
}

var (
	emailTemplateWriteMutex       sync.Mutex
	emailTemplateTokenRegex       = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
	emailTemplateBasePlaceholders = []string{"site_name", "site_url", "logo_url", "recipient_email", "provider", "sent_at", "current_year"}
	emailTemplateEvents           = []EmailTemplateEventInfo{
		{Event: EmailTemplateEventVerification, Placeholders: emailTemplatePlaceholders("code", "valid_minutes", "verification_purpose")},
		{Event: EmailTemplateEventPasswordReset, Placeholders: emailTemplatePlaceholders("reset_url", "valid_minutes")},
		{Event: EmailTemplateEventBalanceLow, Placeholders: emailTemplatePlaceholders("user_id", "current_balance", "threshold", "recharge_url", "quota_status")},
		{Event: EmailTemplateEventTopUpSucceeded, Placeholders: emailTemplatePlaceholders("user_id", "order_no", "quota_added", "payment_amount", "payment_method", "payment_provider", "completed_at")},
		{Event: EmailTemplateEventSubscriptionBalanceLow, Placeholders: emailTemplatePlaceholders("user_id", "subscription_id", "subscription_name", "current_balance", "threshold", "recharge_url", "quota_status")},
		{Event: EmailTemplateEventSubscriptionResetQuota, Placeholders: emailTemplatePlaceholders("user_id", "subscription_id", "subscription_name", "current_balance", "threshold", "quota_status", "reset_period", "reset_at", "reset_in")},
		{Event: EmailTemplateEventSubscriptionSucceeded, Placeholders: emailTemplatePlaceholders("user_id", "subscription_id", "plan_id", "subscription_name", "amount_total", "start_at", "end_at", "next_reset_at", "reset_period", "payment_amount", "payment_method", "payment_provider", "subscription_source")},
		{Event: EmailTemplateEventSubscriptionExpired, Placeholders: emailTemplatePlaceholders("user_id", "subscription_id", "plan_id", "subscription_name", "expired_at", "subscription_source", "allow_wallet_overflow")},
		{Event: EmailTemplateEventUserDisabled, Placeholders: emailTemplatePlaceholders("user_id", "username", "display_name", "disable_reason", "disabled_at")},
		{Event: EmailTemplateEventAccountAutoBanned, Placeholders: emailTemplatePlaceholders("user_id", "username", "display_name", "ban_source", "ban_reason", "is_permanent", "ban_duration", "banned_at", "unban_at", "offense_count", "tier_level", "tier_action", "rule_id", "rule_name", "error_sample", "triggered_models", "trigger_ip", "appeal_hint")},
		{Event: EmailTemplateEventChannelAutoDisabled, Placeholders: emailTemplatePlaceholders("channel_id", "channel_name", "channel_type", "reason")},
		{Event: EmailTemplateEventChannelAutoEnabled, Placeholders: emailTemplatePlaceholders("channel_id", "channel_name", "channel_type")},
		{Event: EmailTemplateEventChannelQuotaCooldown, Placeholders: emailTemplatePlaceholders("channel_id", "channel_name", "channel_type", "reason", "cooldown_until")},
		{Event: EmailTemplateEventChannelTestResult, Placeholders: emailTemplatePlaceholders("test_mode", "tested_channels", "succeeded_channels", "failed_channels", "disabled_channels", "enabled_channels")},
		{Event: EmailTemplateEventChannelModelUpdates, Placeholders: emailTemplatePlaceholders("checked_channels", "changed_channels", "detected_add_models", "detected_remove_models", "auto_added_models", "failed_channels", "changed_channel_details", "added_model_samples", "removed_model_samples", "failed_channel_ids")},
		{Event: EmailTemplateEventGeneralNotification, Placeholders: emailTemplatePlaceholders("notification_type", "notification_title", "notification_content")},
		{Event: EmailTemplateEventSystemTest, Placeholders: emailTemplatePlaceholders()},
	}
	defaultEmailTemplates = buildDefaultEmailTemplates()
)

func emailTemplatePlaceholders(eventPlaceholders ...string) []string {
	placeholders := make([]string, 0, len(emailTemplateBasePlaceholders)+len(eventPlaceholders))
	placeholders = append(placeholders, emailTemplateBasePlaceholders...)
	return append(placeholders, eventPlaceholders...)
}

func GetEmailTemplateCatalog() EmailTemplateCatalog {
	events := make([]EmailTemplateEventInfo, len(emailTemplateEvents))
	for index, event := range emailTemplateEvents {
		events[index] = EmailTemplateEventInfo{
			Event:        event.Event,
			Placeholders: slices.Clone(event.Placeholders),
		}
	}
	return EmailTemplateCatalog{
		Events:  events,
		Locales: []string{i18n.LangZhCN, i18n.LangZhTW, i18n.LangEn},
	}
}

func NormalizeEmailTemplateLocale(locale string) string {
	normalized, err := resolveEmailTemplateLocale(locale)
	if err == nil {
		return normalized
	}
	normalized, err = resolveEmailTemplateLocale(common.GetDefaultEmailLanguage())
	if err == nil {
		return normalized
	}
	return i18n.DefaultLang
}

func resolveEmailTemplateLocale(locale string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(locale))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	compact := strings.ReplaceAll(normalized, "-", "")
	switch {
	case strings.HasPrefix(normalized, "en"):
		return i18n.LangEn, nil
	case strings.HasPrefix(compact, "zhtw"),
		strings.HasPrefix(compact, "zhhk"),
		strings.HasPrefix(compact, "zhmo"),
		strings.HasPrefix(compact, "zhhant"):
		return i18n.LangZhTW, nil
	case strings.HasPrefix(normalized, "zh"):
		return i18n.LangZhCN, nil
	default:
		return "", fmt.Errorf("unsupported email template locale: %s", locale)
	}
}

func getEmailTemplateEvent(event string) (EmailTemplateEventInfo, bool) {
	for _, item := range emailTemplateEvents {
		if item.Event == event {
			return item, true
		}
	}
	return EmailTemplateEventInfo{}, false
}

func emailTemplateOptionKey(event, locale string) string {
	return common.EmailNotificationTemplateOptionPrefix + event + "." + locale
}

func readEmailTemplateOverride(event, locale string) (storedEmailTemplate, bool) {
	common.OptionMapRWMutex.RLock()
	raw := strings.TrimSpace(common.OptionMap[emailTemplateOptionKey(event, locale)])
	common.OptionMapRWMutex.RUnlock()
	if raw == "" {
		return storedEmailTemplate{}, false
	}
	var stored storedEmailTemplate
	if err := common.UnmarshalJsonStr(raw, &stored); err != nil || stored.Subject == "" || stored.HTML == "" {
		common.SysLog(fmt.Sprintf("ignored invalid email template override for %s/%s", event, locale))
		return storedEmailTemplate{}, false
	}
	return stored, true
}

func GetEmailTemplate(event, locale string) (EmailTemplate, error) {
	eventInfo, ok := getEmailTemplateEvent(event)
	if !ok {
		return EmailTemplate{}, fmt.Errorf("unsupported email template event: %s", event)
	}
	normalizedLocale, err := resolveEmailTemplateLocale(locale)
	if err != nil {
		return EmailTemplate{}, err
	}
	defaultTemplate, ok := defaultEmailTemplates[event][normalizedLocale]
	if !ok {
		return EmailTemplate{}, errors.New("default email template not found")
	}
	result := EmailTemplate{
		Event:        event,
		Locale:       normalizedLocale,
		Subject:      defaultTemplate.Subject,
		HTML:         defaultTemplate.HTML,
		Placeholders: slices.Clone(eventInfo.Placeholders),
	}
	if stored, exists := readEmailTemplateOverride(event, normalizedLocale); exists {
		result.Subject = stored.Subject
		result.HTML = stored.HTML
		result.IsCustom = true
		result.UpdatedAt = stored.UpdatedAt
	}
	return result, nil
}

func SaveEmailTemplate(event, locale, subject, htmlContent string) (EmailTemplate, error) {
	normalizedLocale, err := resolveEmailTemplateLocale(locale)
	if err != nil {
		return EmailTemplate{}, err
	}
	if err := ValidateEmailTemplate(event, subject, htmlContent); err != nil {
		return EmailTemplate{}, err
	}
	stored := storedEmailTemplate{
		Subject:   strings.TrimSpace(subject),
		HTML:      strings.TrimSpace(htmlContent),
		UpdatedAt: time.Now().Unix(),
	}
	raw, err := common.Marshal(stored)
	if err != nil {
		return EmailTemplate{}, err
	}
	emailTemplateWriteMutex.Lock()
	defer emailTemplateWriteMutex.Unlock()
	if err := model.UpdateOption(emailTemplateOptionKey(event, normalizedLocale), string(raw)); err != nil {
		return EmailTemplate{}, err
	}
	return GetEmailTemplate(event, normalizedLocale)
}

func RestoreDefaultEmailTemplate(event, locale string) (EmailTemplate, error) {
	if _, ok := getEmailTemplateEvent(event); !ok {
		return EmailTemplate{}, fmt.Errorf("unsupported email template event: %s", event)
	}
	normalizedLocale, err := resolveEmailTemplateLocale(locale)
	if err != nil {
		return EmailTemplate{}, err
	}
	emailTemplateWriteMutex.Lock()
	defer emailTemplateWriteMutex.Unlock()
	if err := model.UpdateOption(emailTemplateOptionKey(event, normalizedLocale), ""); err != nil {
		return EmailTemplate{}, err
	}
	return GetEmailTemplate(event, normalizedLocale)
}

func ValidateEmailTemplate(event, subject, htmlContent string) error {
	eventInfo, ok := getEmailTemplateEvent(event)
	if !ok {
		return fmt.Errorf("unsupported email template event: %s", event)
	}
	if strings.TrimSpace(subject) == "" {
		return errors.New("email subject cannot be empty")
	}
	if utf8.RuneCountInString(subject) > emailTemplateSubjectMaxLength {
		return fmt.Errorf("email subject cannot exceed %d characters", emailTemplateSubjectMaxLength)
	}
	if strings.TrimSpace(htmlContent) == "" {
		return errors.New("email HTML cannot be empty")
	}
	if utf8.RuneCountInString(htmlContent) > emailTemplateHTMLMaxLength {
		return fmt.Errorf("email HTML cannot exceed %d characters", emailTemplateHTMLMaxLength)
	}
	allowed := make(map[string]struct{}, len(eventInfo.Placeholders))
	for _, placeholder := range eventInfo.Placeholders {
		allowed[placeholder] = struct{}{}
	}
	for _, source := range []string{subject, htmlContent} {
		for _, match := range emailTemplateTokenRegex.FindAllStringSubmatch(source, -1) {
			placeholder := strings.TrimSpace(match[1])
			if _, exists := allowed[placeholder]; !exists {
				return fmt.Errorf("unsupported placeholder: %s", placeholder)
			}
		}
	}
	return nil
}

func RenderEmailTemplate(template EmailTemplate, variables map[string]string) (RenderedEmailTemplate, error) {
	if err := ValidateEmailTemplate(template.Event, template.Subject, template.HTML); err != nil {
		return RenderedEmailTemplate{}, err
	}
	for key, value := range variables {
		if strings.HasSuffix(key, "_url") && value != "" {
			if err := validateRenderedEmailActionURL(value); err != nil {
				return RenderedEmailTemplate{}, fmt.Errorf("invalid %s: %w", key, err)
			}
		}
	}
	subject := renderEmailTemplateText(template.Subject, variables, false)
	subject = strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(subject))
	preparedHTML, err := prepareEmailTemplateHTML(template.HTML, variables)
	if err != nil {
		return RenderedEmailTemplate{}, err
	}
	return RenderedEmailTemplate{
		Subject: subject,
		HTML:    renderEmailTemplateText(preparedHTML, variables, true),
	}, nil
}

func renderEmailTemplateText(source string, variables map[string]string, escapeHTML bool) string {
	return emailTemplateTokenRegex.ReplaceAllStringFunc(source, func(token string) string {
		match := emailTemplateTokenRegex.FindStringSubmatch(token)
		if len(match) != 2 {
			return token
		}
		value := variables[strings.TrimSpace(match[1])]
		if escapeHTML {
			return html.EscapeString(value)
		}
		return value
	})
}

func prepareEmailTemplateHTML(source string, variables map[string]string) (string, error) {
	if !strings.Contains(source, emailOptionalURLAttribute) && !emailHTMLUsesUnavailableURL(source, variables) {
		return source, nil
	}
	document, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		return "", fmt.Errorf("parse email HTML: %w", err)
	}
	sanitizeEmailTemplateChildren(document, variables)
	var output strings.Builder
	if err := xhtml.Render(&output, document); err != nil {
		return "", fmt.Errorf("render email HTML: %w", err)
	}
	return output.String(), nil
}

func emailHTMLUsesUnavailableURL(source string, variables map[string]string) bool {
	for _, match := range emailTemplateTokenRegex.FindAllStringSubmatch(source, -1) {
		key := strings.TrimSpace(match[1])
		if strings.HasSuffix(key, "_url") && strings.TrimSpace(variables[key]) == "" {
			return true
		}
	}
	return false
}

func sanitizeEmailTemplateChildren(parent *xhtml.Node, variables map[string]string) {
	for child := parent.FirstChild; child != nil; {
		next := child.NextSibling
		if sanitizeEmailTemplateNode(child, variables) {
			parent.RemoveChild(child)
		} else {
			sanitizeEmailTemplateChildren(child, variables)
		}
		child = next
	}
}

func sanitizeEmailTemplateNode(node *xhtml.Node, variables map[string]string) bool {
	if node.Type != xhtml.ElementNode {
		return false
	}

	removeOptionalNode := false
	attributes := node.Attr[:0]
	for _, attribute := range node.Attr {
		if attribute.Key == emailOptionalURLAttribute {
			removeOptionalNode = emailHTMLUsesUnavailableURL(attribute.Val, variables)
			continue
		}
		attributes = append(attributes, attribute)
	}
	node.Attr = attributes
	if removeOptionalNode {
		return true
	}

	missingHref := false
	missingSource := false
	for _, attribute := range node.Attr {
		switch attribute.Key {
		case "href":
			missingHref = emailHTMLUsesUnavailableURL(attribute.Val, variables)
		case "src":
			missingSource = emailHTMLUsesUnavailableURL(attribute.Val, variables)
		}
	}
	if node.Data == "img" && missingSource {
		return true
	}
	if node.Data == "a" && missingHref {
		filtered := node.Attr[:0]
		for _, attribute := range node.Attr {
			if attribute.Key != "href" && attribute.Key != "target" && attribute.Key != "rel" {
				filtered = append(filtered, attribute)
			}
		}
		node.Attr = filtered
	}
	return false
}

func SendTemplatedEmail(event, locale, receiver string, variables map[string]string) error {
	template, err := GetEmailTemplate(event, NormalizeEmailTemplateLocale(locale))
	if err != nil {
		return err
	}
	values := make(map[string]string, len(variables)+len(emailTemplateBasePlaceholders))
	for key, value := range variables {
		values[key] = value
	}
	if strings.TrimSpace(values["site_name"]) == "" {
		values["site_name"] = currentEmailSiteName()
	}
	if strings.TrimSpace(values["site_url"]) == "" {
		values["site_url"] = currentEmailSiteURL()
	}
	if strings.TrimSpace(values["recipient_email"]) == "" {
		values["recipient_email"] = strings.TrimSpace(receiver)
	}
	if strings.TrimSpace(values["provider"]) == "" {
		values["provider"] = strings.ToUpper(currentEmailProvider())
	}
	if strings.TrimSpace(values["sent_at"]) == "" {
		values["sent_at"] = time.Now().Format(time.RFC3339)
	}
	if strings.TrimSpace(values["current_year"]) == "" {
		values["current_year"] = fmt.Sprintf("%d", time.Now().Year())
	}
	if strings.TrimSpace(values["logo_url"]) == "" {
		values["logo_url"] = currentEmailLogoURL()
	}
	rendered, err := RenderEmailTemplate(template, values)
	if err != nil {
		return err
	}
	return common.SendEmail(rendered.Subject, receiver, rendered.HTML)
}

func PreviewEmailTemplate(event, locale, subject, htmlContent string) (RenderedEmailTemplate, error) {
	template, err := GetEmailTemplate(event, locale)
	if err != nil {
		return RenderedEmailTemplate{}, err
	}
	template.Subject = subject
	template.HTML = htmlContent
	return RenderEmailTemplate(template, SampleEmailTemplateVariables(event))
}

func SampleEmailTemplateVariables(event string) map[string]string {
	values := map[string]string{
		"site_name":               currentEmailSiteName(),
		"site_url":                currentEmailSiteURL(),
		"logo_url":                currentEmailLogoURL(),
		"recipient_email":         "user@example.com",
		"provider":                strings.ToUpper(currentEmailProvider()),
		"sent_at":                 time.Now().Format(time.RFC1123),
		"current_year":            fmt.Sprintf("%d", time.Now().Year()),
		"code":                    "482915",
		"valid_minutes":           fmt.Sprintf("%d", common.VerificationValidMinutes),
		"verification_purpose":    "register",
		"reset_url":               "https://example.com/user/reset?token=preview",
		"user_id":                 "42",
		"current_balance":         "$4.20",
		"threshold":               "$10.00",
		"recharge_url":            GetBalanceLowRechargeURL(),
		"quota_status":            "running low",
		"subscription_id":         "108",
		"subscription_name":       "Pro",
		"plan_id":                 "12",
		"amount_total":            "$100.00",
		"start_at":                time.Now().Format(time.RFC1123),
		"end_at":                  time.Now().AddDate(0, 1, 0).Format(time.RFC1123),
		"next_reset_at":           time.Now().Add(24 * time.Hour).Format(time.RFC1123),
		"reset_period":            "daily",
		"reset_at":                time.Now().Add(2 * time.Hour).Format(time.RFC1123),
		"reset_in":                "2 hours",
		"subscription_source":     "order",
		"expired_at":              time.Now().Format(time.RFC1123),
		"allow_wallet_overflow":   "yes",
		"order_no":                "ORDER-20260721-001",
		"quota_added":             "$25.00",
		"payment_amount":          "25.00",
		"payment_method":          "card",
		"payment_provider":        "stripe",
		"completed_at":            time.Now().Format(time.RFC1123),
		"username":                "example-user",
		"display_name":            "Example User",
		"disable_reason":          "Terms of service violation",
		"disabled_at":             time.Now().Format(time.RFC1123),
		"channel_id":              "12",
		"channel_name":            "Primary OpenAI",
		"channel_type":            "1",
		"reason":                  "Upstream returned HTTP 429",
		"cooldown_until":          time.Now().Add(time.Hour).Format(time.RFC3339),
		"test_mode":               "all",
		"tested_channels":         "24",
		"succeeded_channels":      "21",
		"failed_channels":         "3",
		"disabled_channels":       "2",
		"enabled_channels":        "1",
		"checked_channels":        "48",
		"changed_channels":        "4",
		"detected_add_models":     "7",
		"detected_remove_models":  "2",
		"auto_added_models":       "5",
		"changed_channel_details": "Primary OpenAI (+2 / -1)\nBackup provider (+5 / -1)",
		"added_model_samples":     "gpt-5, gpt-5-mini, o3",
		"removed_model_samples":   "gpt-4-0314, text-davinci-003",
		"failed_channel_ids":      "18, 31, 44",
		"notification_type":       "system.notice",
		"notification_title":      "System notification",
		"notification_content":    "A system event requires your attention.",
		"ban_source":              "error_ban",
		"ban_reason":              "Repeated upstream authentication failures",
		"is_permanent":            "no",
		"ban_duration":            "240 分钟",
		"banned_at":               time.Now().Format(time.RFC1123),
		"unban_at":                time.Now().Add(4 * time.Hour).Format(time.RFC1123),
		"offense_count":           "2",
		"tier_level":              "2",
		"tier_action":             "temp_ip_ban",
		"rule_id":                 "invalid_api_key",
		"rule_name":               "Invalid API key",
		"error_sample":            "status_code=401, invalid_api_key: incorrect api key provided",
		"triggered_models":        "gpt-5, claude-3-5-sonnet",
		"trigger_ip":              "203.0.113.42",
		"appeal_hint":             "如认为误封，请联系管理员。",
	}
	return values
}

func ValidateEmailActionURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("URL must use http or https")
	}
	return nil
}

func validateRenderedEmailActionURL(rawURL string) error {
	if strings.HasPrefix(strings.TrimSpace(rawURL), "/") {
		return nil
	}
	return ValidateEmailActionURL(rawURL)
}

func IsBalanceLowNotificationEnabled() bool {
	common.OptionMapRWMutex.RLock()
	value, exists := common.OptionMap[common.BalanceLowNotifyEnabledOptionKey]
	common.OptionMapRWMutex.RUnlock()
	return !exists || strings.TrimSpace(value) != "false"
}

func GetBalanceLowRechargeURL() string {
	if strings.TrimSpace(system_setting.ServerAddress) == "" {
		return ""
	}
	rechargeURL := PaymentReturnURL("/console/topup")
	if ValidateEmailActionURL(rechargeURL) != nil {
		return ""
	}
	return rechargeURL
}

func currentEmailSiteName() string {
	if siteName := strings.TrimSpace(common.SystemName); siteName != "" {
		return siteName
	}
	return "New API"
}

func currentEmailSiteURL() string {
	configured := strings.TrimSpace(system_setting.ServerAddress)
	if configured == "" || ValidateEmailActionURL(configured) != nil {
		return ""
	}
	return strings.TrimRight(configured, "/")
}

func currentEmailProvider() string {
	if provider := strings.TrimSpace(common.EmailProvider); provider != "" {
		return provider
	}
	return "smtp"
}

func currentEmailLogoURL() string {
	configured := strings.TrimSpace(common.Logo)
	if configured == "" {
		return ""
	}
	if ValidateEmailActionURL(configured) == nil {
		return configured
	}
	parsed, err := url.Parse(configured)
	siteURL := currentEmailSiteURL()
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || siteURL == "" || strings.HasPrefix(configured, "//") {
		return ""
	}
	logoURL := siteURL + "/" + strings.TrimLeft(configured, "/")
	if ValidateEmailActionURL(logoURL) != nil {
		return ""
	}
	return logoURL
}

func buildDefaultEmailTemplates() map[string]map[string]defaultEmailTemplate {
	return map[string]map[string]defaultEmailTemplate{
		EmailTemplateEventVerification: {
			i18n.LangEn: {
				Subject: "[{{ site_name }}] Verify your email",
				HTML: emailHTMLLayout("Verify your email", "Use the verification code below to continue with {{ site_name }}.", `<div style="margin:24px 0;padding:18px 20px;border:1px solid #dce3ea;border-radius:8px;background:#f6f8fa;text-align:center;font-size:30px;font-weight:700;letter-spacing:6px;color:#111827;">{{ code }}</div>
<p style="margin:0;color:#667085;font-size:14px;line-height:22px;">This code expires in {{ valid_minutes }} minutes. If you did not request it, you can safely ignore this email.</p>`),
			},
			i18n.LangZhCN: {
				Subject: "[{{ site_name }}] 邮箱验证码",
				HTML: emailHTMLLayout("验证您的邮箱", "请使用以下验证码继续访问 {{ site_name }}。", `<div style="margin:24px 0;padding:18px 20px;border:1px solid #dce3ea;border-radius:8px;background:#f6f8fa;text-align:center;font-size:30px;font-weight:700;letter-spacing:6px;color:#111827;">{{ code }}</div>
<p style="margin:0;color:#667085;font-size:14px;line-height:22px;">验证码将在 {{ valid_minutes }} 分钟后失效。如非本人操作，请忽略本邮件。</p>`),
			},
			i18n.LangZhTW: {
				Subject: "[{{ site_name }}] 電子郵件驗證碼",
				HTML: emailHTMLLayout("驗證您的電子郵件", "請使用以下驗證碼繼續存取 {{ site_name }}。", `<div style="margin:24px 0;padding:18px 20px;border:1px solid #dce3ea;border-radius:8px;background:#f6f8fa;text-align:center;font-size:30px;font-weight:700;letter-spacing:6px;color:#111827;">{{ code }}</div>
<p style="margin:0;color:#667085;font-size:14px;line-height:22px;">驗證碼將在 {{ valid_minutes }} 分鐘後失效。如非本人操作，請忽略本郵件。</p>`),
			},
		},
		EmailTemplateEventPasswordReset: {
			i18n.LangEn: {
				Subject: "[{{ site_name }}] Reset your password",
				HTML:    emailHTMLLayout("Reset your password", "A password reset was requested for your {{ site_name }} account.", emailActionButton("Reset password", "{{ reset_url }}")+"\n"+`<p style="margin:20px 0 0;color:#667085;font-size:14px;line-height:22px;">This link expires in {{ valid_minutes }} minutes. If you did not request a reset, you can ignore this email.</p>`),
			},
			i18n.LangZhCN: {
				Subject: "[{{ site_name }}] 重置密码",
				HTML:    emailHTMLLayout("重置您的密码", "我们收到了您的 {{ site_name }} 账号密码重置请求。", emailActionButton("重置密码", "{{ reset_url }}")+"\n"+`<p style="margin:20px 0 0;color:#667085;font-size:14px;line-height:22px;">链接将在 {{ valid_minutes }} 分钟后失效。如非本人操作，请忽略本邮件。</p>`),
			},
			i18n.LangZhTW: {
				Subject: "[{{ site_name }}] 重設密碼",
				HTML:    emailHTMLLayout("重設您的密碼", "我們收到了您的 {{ site_name }} 帳號密碼重設請求。", emailActionButton("重設密碼", "{{ reset_url }}")+"\n"+`<p style="margin:20px 0 0;color:#667085;font-size:14px;line-height:22px;">連結將在 {{ valid_minutes }} 分鐘後失效。如非本人操作，請忽略本郵件。</p>`),
			},
		},
		EmailTemplateEventBalanceLow:             buildBalanceLowTemplates(false),
		EmailTemplateEventTopUpSucceeded:         buildTopUpSucceededTemplates(),
		EmailTemplateEventSubscriptionBalanceLow: buildBalanceLowTemplates(true),
		EmailTemplateEventSubscriptionResetQuota: buildSubscriptionResetQuotaTemplates(),
		EmailTemplateEventSubscriptionSucceeded:  buildSubscriptionSucceededTemplates(),
		EmailTemplateEventSubscriptionExpired:    buildSubscriptionExpiredTemplates(),
		EmailTemplateEventUserDisabled:           buildUserDisabledTemplates(),
		EmailTemplateEventAccountAutoBanned:      buildAccountAutoBannedTemplates(),
		EmailTemplateEventChannelAutoDisabled:    buildChannelAutoDisabledTemplates(),
		EmailTemplateEventChannelAutoEnabled:     buildChannelAutoEnabledTemplates(),
		EmailTemplateEventChannelQuotaCooldown:   buildChannelQuotaCooldownTemplates(),
		EmailTemplateEventChannelTestResult:      buildChannelTestResultTemplates(),
		EmailTemplateEventChannelModelUpdates:    buildChannelModelUpdateTemplates(),
		EmailTemplateEventGeneralNotification:    buildGeneralNotificationTemplates(),
		EmailTemplateEventSystemTest: {
			i18n.LangEn: {
				Subject: "[{{ site_name }}] Test email",
				HTML:    emailHTMLLayout("Email delivery is working", "{{ site_name }} successfully sent this message through {{ provider }}.", `<div style="margin-top:24px;padding:16px;border-left:3px solid #16a34a;background:#f0fdf4;color:#166534;font-size:14px;line-height:22px;">Sent at {{ sent_at }}</div>`),
			},
			i18n.LangZhCN: {
				Subject: "[{{ site_name }}] 测试邮件",
				HTML:    emailHTMLLayout("邮件发送正常", "{{ site_name }} 已成功通过 {{ provider }} 发送此邮件。", `<div style="margin-top:24px;padding:16px;border-left:3px solid #16a34a;background:#f0fdf4;color:#166534;font-size:14px;line-height:22px;">发送时间：{{ sent_at }}</div>`),
			},
			i18n.LangZhTW: {
				Subject: "[{{ site_name }}] 測試郵件",
				HTML:    emailHTMLLayout("郵件傳送正常", "{{ site_name }} 已成功透過 {{ provider }} 傳送此郵件。", `<div style="margin-top:24px;padding:16px;border-left:3px solid #16a34a;background:#f0fdf4;color:#166534;font-size:14px;line-height:22px;">傳送時間：{{ sent_at }}</div>`),
			},
		},
	}
}

func buildChannelAutoDisabledTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Channel {{ channel_name }} (#{{ channel_id }}) was disabled",
			HTML: emailHTMLLayout("Channel automatically disabled", "Automated monitoring disabled a channel after detecting an error.", emailDetailTable(
				"Channel", "{{ channel_name }} (#{{ channel_id }})",
				"Channel type", "{{ channel_type }}",
				"Reason", "{{ reason }}",
				"Time", "{{ sent_at }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 通道 {{ channel_name }}（#{{ channel_id }}）已自动禁用",
			HTML: emailHTMLLayout("通道已自动禁用", "自动监控检测到错误后禁用了一个通道。", emailDetailTable(
				"通道", "{{ channel_name }}（#{{ channel_id }}）",
				"通道类型", "{{ channel_type }}",
				"原因", "{{ reason }}",
				"时间", "{{ sent_at }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 通道 {{ channel_name }}（#{{ channel_id }}）已自動停用",
			HTML: emailHTMLLayout("通道已自動停用", "自動監控偵測到錯誤後停用了一個通道。", emailDetailTable(
				"通道", "{{ channel_name }}（#{{ channel_id }}）",
				"通道類型", "{{ channel_type }}",
				"原因", "{{ reason }}",
				"時間", "{{ sent_at }}",
			)),
		},
	}
}

func buildChannelAutoEnabledTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Channel {{ channel_name }} (#{{ channel_id }}) recovered",
			HTML: emailHTMLLayout("Channel automatically restored", "A previously disabled channel passed its health check and was enabled again.", emailDetailTable(
				"Channel", "{{ channel_name }} (#{{ channel_id }})",
				"Channel type", "{{ channel_type }}",
				"Time", "{{ sent_at }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 通道 {{ channel_name }}（#{{ channel_id }}）已自动恢复",
			HTML: emailHTMLLayout("通道已自动恢复", "此前禁用的通道已通过健康检查并重新启用。", emailDetailTable(
				"通道", "{{ channel_name }}（#{{ channel_id }}）",
				"通道类型", "{{ channel_type }}",
				"时间", "{{ sent_at }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 通道 {{ channel_name }}（#{{ channel_id }}）已自動恢復",
			HTML: emailHTMLLayout("通道已自動恢復", "先前停用的通道已通過健康檢查並重新啟用。", emailDetailTable(
				"通道", "{{ channel_name }}（#{{ channel_id }}）",
				"通道類型", "{{ channel_type }}",
				"時間", "{{ sent_at }}",
			)),
		},
	}
}

func buildChannelQuotaCooldownTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Channel {{ channel_name }} (#{{ channel_id }}) entered quota cooldown",
			HTML: emailHTMLLayout("Channel quota cooldown", "A channel was temporarily rate-limited because its upstream plan quota was exhausted.", emailDetailTable(
				"Channel", "{{ channel_name }} (#{{ channel_id }})",
				"Channel type", "{{ channel_type }}",
				"Available again at", "{{ cooldown_until }}",
				"Reason", "{{ reason }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 通道 {{ channel_name }}（#{{ channel_id }}）进入额度冷却",
			HTML: emailHTMLLayout("通道进入额度冷却", "上游套餐额度耗尽，该通道已被临时限流。", emailDetailTable(
				"通道", "{{ channel_name }}（#{{ channel_id }}）",
				"通道类型", "{{ channel_type }}",
				"预计恢复时间", "{{ cooldown_until }}",
				"原因", "{{ reason }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 通道 {{ channel_name }}（#{{ channel_id }}）進入額度冷卻",
			HTML: emailHTMLLayout("通道進入額度冷卻", "上游方案額度耗盡，該通道已被暫時限流。", emailDetailTable(
				"通道", "{{ channel_name }}（#{{ channel_id }}）",
				"通道類型", "{{ channel_type }}",
				"預計恢復時間", "{{ cooldown_until }}",
				"原因", "{{ reason }}",
			)),
		},
	}
}

func buildChannelTestResultTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Channel test completed: {{ succeeded_channels }}/{{ tested_channels }} passed",
			HTML: emailHTMLLayout("Channel test completed", "The scheduled or manually triggered channel health check has finished.", emailDetailTable(
				"Mode", "{{ test_mode }}",
				"Tested", "{{ tested_channels }}",
				"Succeeded", "{{ succeeded_channels }}",
				"Failed", "{{ failed_channels }}",
				"Automatically disabled", "{{ disabled_channels }}",
				"Automatically restored", "{{ enabled_channels }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 通道测试完成：{{ succeeded_channels }}/{{ tested_channels }} 通过",
			HTML: emailHTMLLayout("通道测试已完成", "定时或手动触发的通道健康检查已经结束。", emailDetailTable(
				"测试模式", "{{ test_mode }}",
				"已测试", "{{ tested_channels }}",
				"成功", "{{ succeeded_channels }}",
				"失败", "{{ failed_channels }}",
				"自动禁用", "{{ disabled_channels }}",
				"自动恢复", "{{ enabled_channels }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 通道測試完成：{{ succeeded_channels }}/{{ tested_channels }} 通過",
			HTML: emailHTMLLayout("通道測試已完成", "定時或手動觸發的通道健康檢查已經結束。", emailDetailTable(
				"測試模式", "{{ test_mode }}",
				"已測試", "{{ tested_channels }}",
				"成功", "{{ succeeded_channels }}",
				"失敗", "{{ failed_channels }}",
				"自動停用", "{{ disabled_channels }}",
				"自動恢復", "{{ enabled_channels }}",
			)),
		},
	}
}

func buildChannelModelUpdateTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Upstream model inspection found {{ changed_channels }} changed channels",
			HTML: emailHTMLLayout("Upstream model inspection", "The latest inspection found model changes or channel failures that may need review.", emailDetailTable(
				"Checked channels", "{{ checked_channels }}",
				"Changed channels", "{{ changed_channels }}",
				"Models added / removed", "{{ detected_add_models }} / {{ detected_remove_models }}",
				"Models automatically added", "{{ auto_added_models }}",
				"Failed channels", "{{ failed_channels }}",
				"Changed channel details", "{{ changed_channel_details }}",
				"Added model samples", "{{ added_model_samples }}",
				"Removed model samples", "{{ removed_model_samples }}",
				"Failed channel IDs", "{{ failed_channel_ids }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 上游模型巡检发现 {{ changed_channels }} 个通道有变更",
			HTML: emailHTMLLayout("上游模型巡检", "本次巡检发现模型变更或通道失败，请及时检查。", emailDetailTable(
				"检查通道", "{{ checked_channels }}",
				"变更通道", "{{ changed_channels }}",
				"新增 / 删除模型", "{{ detected_add_models }} / {{ detected_remove_models }}",
				"自动新增模型", "{{ auto_added_models }}",
				"失败通道", "{{ failed_channels }}",
				"变更通道明细", "{{ changed_channel_details }}",
				"新增模型示例", "{{ added_model_samples }}",
				"删除模型示例", "{{ removed_model_samples }}",
				"失败通道 ID", "{{ failed_channel_ids }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 上游模型巡檢發現 {{ changed_channels }} 個通道有變更",
			HTML: emailHTMLLayout("上游模型巡檢", "本次巡檢發現模型變更或通道失敗，請及時檢查。", emailDetailTable(
				"檢查通道", "{{ checked_channels }}",
				"變更通道", "{{ changed_channels }}",
				"新增 / 刪除模型", "{{ detected_add_models }} / {{ detected_remove_models }}",
				"自動新增模型", "{{ auto_added_models }}",
				"失敗通道", "{{ failed_channels }}",
				"變更通道明細", "{{ changed_channel_details }}",
				"新增模型範例", "{{ added_model_samples }}",
				"刪除模型範例", "{{ removed_model_samples }}",
				"失敗通道 ID", "{{ failed_channel_ids }}",
			)),
		},
	}
}

func buildGeneralNotificationTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] {{ notification_title }}",
			HTML:    emailHTMLLayout("{{ notification_title }}", "Notification type: {{ notification_type }}", emailTextPanel("{{ notification_content }}")),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] {{ notification_title }}",
			HTML:    emailHTMLLayout("{{ notification_title }}", "通知类型：{{ notification_type }}", emailTextPanel("{{ notification_content }}")),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] {{ notification_title }}",
			HTML:    emailHTMLLayout("{{ notification_title }}", "通知類型：{{ notification_type }}", emailTextPanel("{{ notification_content }}")),
		},
	}
}

func buildBalanceLowTemplates(subscription bool) map[string]defaultEmailTemplate {
	nameEN := "balance"
	nameZhCN := "余额"
	nameZhTW := "餘額"
	if subscription {
		nameEN = "{{ subscription_name }} subscription balance"
		nameZhCN = "{{ subscription_name }} 订阅额度"
		nameZhTW = "{{ subscription_name }} 訂閱額度"
	}
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Your " + nameEN + " is {{ quota_status }}",
			HTML:    emailHTMLLayout("Your "+nameEN+" is {{ quota_status }}", "Your current balance is {{ current_balance }}. The configured reminder threshold is {{ threshold }}.", emailActionButton("Recharge now", "{{ recharge_url }}")+"\n"+`<p style="margin:20px 0 0;color:#667085;font-size:14px;line-height:22px;">Recharge to avoid an interruption to your service.</p>`),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 您的" + nameZhCN + "{{ quota_status }}",
			HTML:    emailHTMLLayout("您的"+nameZhCN+"{{ quota_status }}", "当前余额为 {{ current_balance }}，提醒阈值为 {{ threshold }}。", emailActionButton("立即充值", "{{ recharge_url }}")+"\n"+`<p style="margin:20px 0 0;color:#667085;font-size:14px;line-height:22px;">请补充余额，避免服务中断。</p>`),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 您的" + nameZhTW + "{{ quota_status }}",
			HTML:    emailHTMLLayout("您的"+nameZhTW+"{{ quota_status }}", "目前餘額為 {{ current_balance }}，提醒門檻為 {{ threshold }}。", emailActionButton("立即儲值", "{{ recharge_url }}")+"\n"+`<p style="margin:20px 0 0;color:#667085;font-size:14px;line-height:22px;">請補充餘額，避免服務中斷。</p>`),
		},
	}
}

func buildSubscriptionResetQuotaTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] {{ subscription_name }} plan quota is {{ quota_status }}",
			HTML: emailHTMLLayout("Plan quota is {{ quota_status }}", "This plan's quota will recover automatically; no wallet recharge is required for the reset.", emailDetailTable(
				"Plan", "{{ subscription_name }} (#{{ subscription_id }})",
				"Current quota", "{{ current_balance }}",
				"Reminder threshold", "{{ threshold }}",
				"Reset period", "{{ reset_period }}",
				"Available again at", "{{ reset_at }}",
				"Time remaining", "{{ reset_in }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] {{ subscription_name }} 套餐额度{{ quota_status }}",
			HTML: emailHTMLLayout("套餐周期额度{{ quota_status }}", "该套餐额度会按周期自动恢复，无需为本次恢复充值钱包。", emailDetailTable(
				"套餐", "{{ subscription_name }}（#{{ subscription_id }}）",
				"当前额度", "{{ current_balance }}",
				"提醒阈值", "{{ threshold }}",
				"重置周期", "{{ reset_period }}",
				"预计恢复时间", "{{ reset_at }}",
				"距离恢复", "{{ reset_in }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] {{ subscription_name }} 方案額度{{ quota_status }}",
			HTML: emailHTMLLayout("方案週期額度{{ quota_status }}", "該方案額度會依週期自動恢復，無需為本次恢復儲值錢包。", emailDetailTable(
				"方案", "{{ subscription_name }}（#{{ subscription_id }}）",
				"目前額度", "{{ current_balance }}",
				"提醒門檻", "{{ threshold }}",
				"重置週期", "{{ reset_period }}",
				"預計恢復時間", "{{ reset_at }}",
				"距離恢復", "{{ reset_in }}",
			)),
		},
	}
}

func buildSubscriptionSucceededTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] {{ subscription_name }} subscription activated",
			HTML: emailHTMLLayout("Subscription activated", "Your paid subscription is ready to use.", emailDetailTable(
				"Plan", "{{ subscription_name }} (#{{ plan_id }})",
				"Subscription ID", "{{ subscription_id }}",
				"Quota", "{{ amount_total }}",
				"Starts", "{{ start_at }}",
				"Ends", "{{ end_at }}",
				"Next quota reset", "{{ next_reset_at }}",
				"Reset period", "{{ reset_period }}",
				"Paid", "{{ payment_amount }} via {{ payment_method }} / {{ payment_provider }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] {{ subscription_name }} 订阅已开通",
			HTML: emailHTMLLayout("订阅开通成功", "您的付费订阅已经生效。", emailDetailTable(
				"套餐", "{{ subscription_name }}（#{{ plan_id }}）",
				"订阅 ID", "{{ subscription_id }}",
				"套餐额度", "{{ amount_total }}",
				"开始时间", "{{ start_at }}",
				"到期时间", "{{ end_at }}",
				"下次额度重置", "{{ next_reset_at }}",
				"重置周期", "{{ reset_period }}",
				"支付信息", "{{ payment_amount }}，{{ payment_method }} / {{ payment_provider }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] {{ subscription_name }} 訂閱已開通",
			HTML: emailHTMLLayout("訂閱開通成功", "您的付費訂閱已經生效。", emailDetailTable(
				"方案", "{{ subscription_name }}（#{{ plan_id }}）",
				"訂閱 ID", "{{ subscription_id }}",
				"方案額度", "{{ amount_total }}",
				"開始時間", "{{ start_at }}",
				"到期時間", "{{ end_at }}",
				"下次額度重置", "{{ next_reset_at }}",
				"重置週期", "{{ reset_period }}",
				"付款資訊", "{{ payment_amount }}，{{ payment_method }} / {{ payment_provider }}",
			)),
		},
	}
}

func buildSubscriptionExpiredTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] {{ subscription_name }} subscription expired",
			HTML: emailHTMLLayout("Subscription expired", "Your subscription is no longer active.", emailDetailTable(
				"Plan", "{{ subscription_name }} (#{{ plan_id }})",
				"Subscription ID", "{{ subscription_id }}",
				"Expired at", "{{ expired_at }}",
				"Source", "{{ subscription_source }}",
				"Wallet fallback", "{{ allow_wallet_overflow }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] {{ subscription_name }} 订阅已到期",
			HTML: emailHTMLLayout("订阅已到期", "该订阅已停止生效。", emailDetailTable(
				"套餐", "{{ subscription_name }}（#{{ plan_id }}）",
				"订阅 ID", "{{ subscription_id }}",
				"到期时间", "{{ expired_at }}",
				"订阅来源", "{{ subscription_source }}",
				"允许钱包接续", "{{ allow_wallet_overflow }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] {{ subscription_name }} 訂閱已到期",
			HTML: emailHTMLLayout("訂閱已到期", "該訂閱已停止生效。", emailDetailTable(
				"方案", "{{ subscription_name }}（#{{ plan_id }}）",
				"訂閱 ID", "{{ subscription_id }}",
				"到期時間", "{{ expired_at }}",
				"訂閱來源", "{{ subscription_source }}",
				"允許錢包接續", "{{ allow_wallet_overflow }}",
			)),
		},
	}
}

func buildTopUpSucceededTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Wallet top-up completed",
			HTML: emailHTMLLayout("Top-up completed", "Funds were added to your wallet successfully.", emailDetailTable(
				"Order", "{{ order_no }}",
				"Quota added", "{{ quota_added }}",
				"Payment amount", "{{ payment_amount }}",
				"Payment", "{{ payment_method }} / {{ payment_provider }}",
				"Completed at", "{{ completed_at }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 钱包充值成功",
			HTML: emailHTMLLayout("充值到账", "充值额度已成功加入您的钱包。", emailDetailTable(
				"订单号", "{{ order_no }}",
				"到账额度", "{{ quota_added }}",
				"支付金额", "{{ payment_amount }}",
				"支付方式", "{{ payment_method }} / {{ payment_provider }}",
				"到账时间", "{{ completed_at }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 錢包儲值成功",
			HTML: emailHTMLLayout("儲值到帳", "儲值額度已成功加入您的錢包。", emailDetailTable(
				"訂單號", "{{ order_no }}",
				"到帳額度", "{{ quota_added }}",
				"付款金額", "{{ payment_amount }}",
				"付款方式", "{{ payment_method }} / {{ payment_provider }}",
				"到帳時間", "{{ completed_at }}",
			)),
		},
	}
}

func buildUserDisabledTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Your account was disabled",
			HTML: emailHTMLLayout("Account disabled", "Your account can no longer access {{ site_name }}.", emailDetailTable(
				"Account", "{{ display_name }} ({{ username }}, #{{ user_id }})",
				"Reason", "{{ disable_reason }}",
				"Disabled at", "{{ disabled_at }}",
			)),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 您的账号已被封禁",
			HTML: emailHTMLLayout("账号已被封禁", "您的账号已无法继续访问 {{ site_name }}。", emailDetailTable(
				"账号", "{{ display_name }}（{{ username }}，#{{ user_id }}）",
				"封禁理由", "{{ disable_reason }}",
				"封禁时间", "{{ disabled_at }}",
			)),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 您的帳號已被停用",
			HTML: emailHTMLLayout("帳號已被停用", "您的帳號已無法繼續存取 {{ site_name }}。", emailDetailTable(
				"帳號", "{{ display_name }}（{{ username }}，#{{ user_id }}）",
				"停用理由", "{{ disable_reason }}",
				"停用時間", "{{ disabled_at }}",
			)),
		},
	}
}

func buildAccountAutoBannedTemplates() map[string]defaultEmailTemplate {
	return map[string]defaultEmailTemplate{
		i18n.LangEn: {
			Subject: "[{{ site_name }}] Your account was automatically banned",
			HTML: emailHTMLLayout("Account automatically banned", "Automated risk control disabled your account after detecting suspicious activity.", emailDetailTable(
				"Account", "{{ display_name }} ({{ username }}, #{{ user_id }})",
				"Source", "{{ ban_source }}",
				"Reason", "{{ ban_reason }}",
				"Permanent", "{{ is_permanent }}",
				"Duration", "{{ ban_duration }}",
				"Banned at", "{{ banned_at }}",
				"Unban at", "{{ unban_at }}",
				"Offense count", "{{ offense_count }}",
				"Rule", "{{ rule_name }} ({{ rule_id }})",
				"Trigger IP", "{{ trigger_ip }}",
			)+"\n"+emailTextPanel("{{ appeal_hint }}")),
		},
		i18n.LangZhCN: {
			Subject: "[{{ site_name }}] 您的账号已被自动封禁",
			HTML: emailHTMLLayout("账号已被自动封禁", "风控系统检测到异常行为后自动封禁了您的账号。", emailDetailTable(
				"账号", "{{ display_name }}（{{ username }}，#{{ user_id }}）",
				"来源", "{{ ban_source }}",
				"封禁理由", "{{ ban_reason }}",
				"是否永久", "{{ is_permanent }}",
				"封禁时长", "{{ ban_duration }}",
				"封禁时间", "{{ banned_at }}",
				"解封时间", "{{ unban_at }}",
				"违规次数", "{{ offense_count }}",
				"触发规则", "{{ rule_name }}（{{ rule_id }}）",
				"触发 IP", "{{ trigger_ip }}",
			)+"\n"+emailTextPanel("{{ appeal_hint }}")),
		},
		i18n.LangZhTW: {
			Subject: "[{{ site_name }}] 您的帳號已被自動封禁",
			HTML: emailHTMLLayout("帳號已被自動封禁", "風控系統偵測到異常行為後自動封禁了您的帳號。", emailDetailTable(
				"帳號", "{{ display_name }}（{{ username }}，#{{ user_id }}）",
				"來源", "{{ ban_source }}",
				"封禁理由", "{{ ban_reason }}",
				"是否永久", "{{ is_permanent }}",
				"封禁時長", "{{ ban_duration }}",
				"封禁時間", "{{ banned_at }}",
				"解封時間", "{{ unban_at }}",
				"違規次數", "{{ offense_count }}",
				"觸發規則", "{{ rule_name }}（{{ rule_id }}）",
				"觸發 IP", "{{ trigger_ip }}",
			)+"\n"+emailTextPanel("{{ appeal_hint }}")),
		},
	}
}

func emailHTMLLayout(title, intro, content string) string {
	return `<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width,initial-scale=1">
    <meta name="color-scheme" content="light">
  </head>
  <body style="margin:0;padding:0;background:#f4f6f8;color:#101828;font-family:Arial,'Helvetica Neue',sans-serif;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f4f6f8;">
      <tr>
        <td align="center" style="padding:32px 16px;">
          <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:600px;background:#ffffff;border:1px solid #e4e7ec;border-radius:8px;overflow:hidden;">
            <tr>
              <td style="padding:20px 32px;border-bottom:1px solid #eaecf0;">
                <table role="presentation" cellspacing="0" cellpadding="0">
                  <tr>
                    <td data-email-optional-url="{{ logo_url }}" style="padding-right:12px;vertical-align:middle;">
                      <img src="{{ logo_url }}" alt="{{ site_name }}" style="display:block;max-width:160px;max-height:36px;border:0;">
                    </td>
                    <td style="vertical-align:middle;font-size:16px;font-weight:700;color:#101828;">{{ site_name }}</td>
                  </tr>
                </table>
              </td>
            </tr>
            <tr>
              <td style="padding:36px 32px;">
                <h1 style="margin:0 0 14px;font-size:24px;line-height:32px;font-weight:700;color:#101828;">` + title + `</h1>
                <p style="margin:0;color:#475467;font-size:16px;line-height:26px;">` + intro + `</p>
` + indentEmailHTML(content, "                ") + `
              </td>
            </tr>
            <tr>
              <td style="padding:20px 32px;border-top:1px solid #eaecf0;color:#98a2b3;font-size:12px;line-height:18px;">&copy; {{ current_year }} {{ site_name }}</td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`
}

func emailActionButton(label, actionURL string) string {
	return `<table role="presentation" data-email-optional-url="` + actionURL + `" cellspacing="0" cellpadding="0" style="margin-top:26px;">
  <tr>
    <td style="border-radius:6px;background:#2563eb;">
      <a href="` + actionURL + `" target="_blank" style="display:inline-block;padding:12px 20px;color:#ffffff;text-decoration:none;font-size:15px;font-weight:700;line-height:20px;">` + label + `</a>
    </td>
  </tr>
</table>`
}

func emailDetailTable(items ...string) string {
	var rows strings.Builder
	for index := 0; index+1 < len(items); index += 2 {
		rows.WriteString(`<tr>
    <td style="width:38%;padding:10px 12px;border-bottom:1px solid #eaecf0;color:#667085;font-size:13px;line-height:20px;vertical-align:top;">` + items[index] + `</td>
    <td style="padding:10px 12px;border-bottom:1px solid #eaecf0;color:#101828;font-size:14px;line-height:20px;font-weight:600;vertical-align:top;white-space:pre-wrap;word-break:break-word;">` + items[index+1] + `</td>
  </tr>`)
	}
	return `<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="margin-top:24px;border:1px solid #eaecf0;border-radius:6px;border-collapse:separate;overflow:hidden;">
` + rows.String() + `
</table>`
}

func emailTextPanel(content string) string {
	return `<div style="margin-top:24px;padding:16px;border:1px solid #eaecf0;border-radius:6px;background:#f8fafc;color:#344054;font-size:14px;line-height:22px;white-space:pre-wrap;word-break:break-word;">` + content + `</div>`
}

func indentEmailHTML(source, indent string) string {
	lines := strings.Split(strings.TrimSpace(source), "\n")
	for index := range lines {
		lines[index] = indent + lines[index]
	}
	return strings.Join(lines, "\n")
}
