package model

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	LogLanguageFollow = ""
	LogLanguageZH     = "zh"
	LogLanguageEN     = "en"
)

type logTranslationRule struct {
	pattern *regexp.Regexp
	format  string
}

var logTranslationRulesEN = []logTranslationRule{
	{regexp.MustCompile(`^新用户注册赠送 (.+)$`), "New user registration bonus: %s"},
	{regexp.MustCompile(`^使用邀请码赠送 (.+)$`), "Invite code bonus: %s"},
	{regexp.MustCompile(`^邀请用户赠送 (.+)$`), "Inviter reward: %s"},
	{regexp.MustCompile(`^通过兑换码充值 (.+)，兑换码ID (\d+)$`), "Redeemed %s with redemption code ID %s"},
	{regexp.MustCompile(`^使用在线充值成功，充值金额: (.+)，支付金额：(.+)$`), "Online top-up successful, quota: %s, paid: %s"},
	{regexp.MustCompile(`^使用Creem充值成功，充值额度: (.+)，支付金额：(.+)$`), "Creem top-up successful, quota: %s, paid: %s"},
	{regexp.MustCompile(`^Waffo充值成功，充值额度: (.+)，支付金额: (.+)$`), "Waffo top-up successful, quota: %s, paid: %s"},
	{regexp.MustCompile(`^Waffo Pancake充值成功，充值额度: (.+)，支付金额: (.+)$`), "Waffo Pancake top-up successful, quota: %s, paid: %s"},
	{regexp.MustCompile(`^管理员补单成功，充值金额: (.+)，支付金额：(.+)$`), "Admin completed top-up, quota: %s, paid: %s"},
	{regexp.MustCompile(`^使用钱包余额购买套餐成功，套餐: (.+)，支付金额: (.+)$`), "Subscription purchased with wallet balance, plan: %s, paid: %s"},
	{regexp.MustCompile(`^订阅购买成功，套餐: (.+)，支付金额: (.+)，支付方式: (.+)$`), "Subscription purchased, plan: %s, paid: %s, payment method: %s"},
	{regexp.MustCompile(`^用户签到，获得额度 (.+)$`), "Daily check-in reward: %s"},
	{regexp.MustCompile(`^开始设置两步验证$`), "Started two-factor authentication setup"},
	{regexp.MustCompile(`^成功启用两步验证$`), "Two-factor authentication enabled"},
	{regexp.MustCompile(`^禁用两步验证$`), "Two-factor authentication disabled"},
	{regexp.MustCompile(`^重新生成两步验证备用码$`), "Regenerated two-factor authentication backup codes"},
	{regexp.MustCompile(`^管理员强制禁用了用户的两步验证$`), "Admin forcibly disabled the user's two-factor authentication"},
	{regexp.MustCompile(`^查看渠道密钥信息 \(渠道ID: (\d+)\)$`), "Viewed channel key information (channel ID: %s)"},
	{regexp.MustCompile(`^通用安全验证成功 \(验证方式: (.+)\)$`), "Security verification succeeded (method: %s)"},
	{regexp.MustCompile(`^用户自助修改用户名: (.+) -> (.+)$`), "User changed username: %s -> %s"},
	{regexp.MustCompile(`^用户自助修改密码$`), "User changed password"},
	{regexp.MustCompile(`^用户自助设置密码$`), "User set password"},
	{regexp.MustCompile(`^管理员增加用户额度 (.+)$`), "Admin increased user quota by %s"},
	{regexp.MustCompile(`^管理员减少用户额度 (.+)$`), "Admin decreased user quota by %s"},
	{regexp.MustCompile(`^管理员覆盖用户额度从 (.+) 为 (.+)$`), "Admin changed user quota from %s to %s"},
}

func NormalizeLogLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(language, "_", "-")))
	switch {
	case language == "":
		return LogLanguageFollow
	case strings.HasPrefix(language, "zh"):
		return LogLanguageZH
	case strings.HasPrefix(language, "en"):
		return LogLanguageEN
	default:
		return LogLanguageFollow
	}
}

func ResolveEffectiveLogLanguage(userSetting, fallback string) string {
	if normalized := NormalizeLogLanguage(userSetting); normalized != LogLanguageFollow {
		return normalized
	}
	if normalized := NormalizeLogLanguage(fallback); normalized != LogLanguageFollow {
		return normalized
	}
	return LogLanguageZH
}

func LocalizeLogContent(content string, language string) string {
	switch NormalizeLogLanguage(language) {
	case LogLanguageEN:
		return localizeLogContentToEN(content)
	default:
		return content
	}
}

func localizeLogContentToEN(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return content
	}
	for _, rule := range logTranslationRulesEN {
		matches := rule.pattern.FindStringSubmatch(content)
		if matches == nil {
			continue
		}
		args := make([]any, 0, len(matches)-1)
		for _, match := range matches[1:] {
			args = append(args, match)
		}
		return fmt.Sprintf(rule.format, args...)
	}
	return content
}

func LocalizeLogs(logs []*Log, language string) {
	for _, log := range logs {
		if log != nil {
			log.Content = LocalizeLogContent(log.Content, language)
		}
	}
}
