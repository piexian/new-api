package model

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const (
	LogLanguageFollow = ""
	LogLanguageZH     = "zh"
	LogLanguageEN     = "en"
)

// rootLogLanguageCacheTTL bounds how long we trust the cached root admin
// fallback language between DB lookups.
const rootLogLanguageCacheTTL = 60 * time.Second

var (
	rootLogLanguageMu       sync.Mutex
	cachedRootLogLanguage   string
	cachedRootLogLanguageAt time.Time
)

// InvalidateRootLogLanguageCache makes changes to the root administrator's
// language preferences visible to subsequent log requests immediately.
func InvalidateRootLogLanguageCache() {
	rootLogLanguageMu.Lock()
	defer rootLogLanguageMu.Unlock()
	cachedRootLogLanguage = ""
	cachedRootLogLanguageAt = time.Time{}
}

// GetRootLogLanguageFallback returns the root admin's log language preference
// (falling back to the admin's interface language). Cached with a 60s TTL to
// avoid a DB round-trip on every log listing request.
func GetRootLogLanguageFallback() string {
	rootLogLanguageMu.Lock()
	defer rootLogLanguageMu.Unlock()
	if !cachedRootLogLanguageAt.IsZero() && time.Since(cachedRootLogLanguageAt) < rootLogLanguageCacheTTL {
		return cachedRootLogLanguage
	}
	fallback := ""
	if rootUser := GetRootUser(); rootUser != nil && rootUser.Id > 0 {
		setting := rootUser.GetSetting()
		fallback = setting.LogLanguage
		if fallback == "" {
			fallback = setting.Language
		}
	}
	cachedRootLogLanguage = fallback
	cachedRootLogLanguageAt = time.Now()
	return fallback
}

type logTranslationRule struct {
	pattern *regexp.Regexp
	format  string
}

type internalLogTranslationRule struct {
	zhPattern *regexp.Regexp
	enPattern *regexp.Regexp
	zhFormat  string
	enFormat  string
}

var logTranslationRulesEN = []logTranslationRule{
	{regexp.MustCompile(`^新用户注册赠送 (.+)$`), "New user registration bonus: %s"},
	{regexp.MustCompile(`^使用邀请码赠送 (.+)$`), "Invite code bonus: %s"},
	{regexp.MustCompile(`^邀请用户赠送 (.+)$`), "Inviter reward: %s"},
	{regexp.MustCompile(`^通过兑换码充值 (.+)，兑换码ID (\d+)$`), "Redeemed %s with redemption code ID %s"},
	{regexp.MustCompile(`^通过兑换码兑换套餐 (.+)，兑换码ID (\d+)$`), "Redeemed subscription plan %s with redemption code ID %s"},
	{regexp.MustCompile(`^使用在线充值成功，充值金额: (.+)，支付金额：(.+)$`), "Online top-up successful, quota: %s, paid: %s"},
	{regexp.MustCompile(`^使用Creem充值成功，充值额度: (.+)，支付金额：(.+)$`), "Creem top-up successful, quota: %s, paid: %s"},
	{regexp.MustCompile(`^Waffo充值成功，充值额度: (.+)，支付金额: (.+)$`), "Waffo top-up successful, quota: %s, paid: %s"},
	{regexp.MustCompile(`^Waffo Pancake充值成功，充值额度: (.+)，支付金额: (.+)$`), "Waffo Pancake top-up successful, quota: %s, paid: %s"},
	{regexp.MustCompile(`^管理员补单成功，充值金额: (.+)，支付金额：(.+)$`), "Admin completed top-up, quota: %s, paid: %s"},
	{regexp.MustCompile(`^使用钱包余额购买套餐成功，套餐: (.+)，支付金额: (.+)$`), "Subscription purchased with wallet balance, plan: %s, paid: %s"},
	{regexp.MustCompile(`^使用余额购买订阅成功，套餐: (.+)，支付金额: (.+)，扣除额度: (.+)$`), "Subscription purchased with balance, plan: %s, paid: %s, quota deducted: %s"},
	{regexp.MustCompile(`^订阅购买成功，套餐: (.+)，支付金额: (.+)，支付方式: (.+)$`), "Subscription purchased, plan: %s, paid: %s, payment method: %s"},
	{regexp.MustCompile(`^管理员重置订阅套餐 (.+)（ID: (\d+)）额度$`), "Admin reset quota for subscription plan %s (ID: %s)"},
	{regexp.MustCompile(`^用户签到，获得额度 (.+)$`), "Daily check-in reward: %s"},
	{regexp.MustCompile(`^开始设置两步验证$`), "Started two-factor authentication setup"},
	{regexp.MustCompile(`^成功启用两步验证$`), "Two-factor authentication enabled"},
	{regexp.MustCompile(`^禁用两步验证$`), "Two-factor authentication disabled"},
	{regexp.MustCompile(`^重新生成两步验证备用码$`), "Regenerated two-factor authentication backup codes"},
	{regexp.MustCompile(`^管理员强制禁用了用户的两步验证$`), "Admin forcibly disabled the user's two-factor authentication"},
	{regexp.MustCompile(`^查看渠道密钥信息 \(渠道ID: (\d+)\)$`), "Viewed channel key information (channel ID: %s)"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）进入套餐限额冷却，限流至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) entered plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）已处于套餐限额冷却，限流至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) is already in plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）进入套餐限额冷却，禁用至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) entered plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）已处于套餐限额冷却，禁用至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) is already in plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）密钥 #(\d+) 进入套餐限额冷却，禁用至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) key #%s entered plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）密钥 #(\d+) 已处于套餐限额冷却，禁用至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) key #%s is already in plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）模型「(.+)」进入套餐限额冷却，禁用至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) model \"%s\" entered plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）模型「(.+)」已处于套餐限额冷却，禁用至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) model \"%s\" is already in plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）密钥 #(\d+) 模型「(.+)」进入套餐限额冷却，禁用至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) key #%s model \"%s\" entered plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通道「(.+)」（#(\d+)）密钥 #(\d+) 模型「(.+)」已处于套餐限额冷却，禁用至 (.+)，原因：(.+)$`), "Channel \"%s\" (#%s) key #%s model \"%s\" is already in plan quota cooldown until %s, reason: %s"},
	{regexp.MustCompile(`^通用安全验证成功 \(验证方式: (.+)\)$`), "Security verification succeeded (method: %s)"},
	{regexp.MustCompile(`^用户自助修改用户名: (.+) -> (.+)$`), "User changed username: %s -> %s"},
	{regexp.MustCompile(`^用户自助修改密码$`), "User changed password"},
	{regexp.MustCompile(`^用户自助设置密码$`), "User set password"},
	{regexp.MustCompile(`^管理员增加用户额度 (.+)$`), "Admin increased user quota by %s"},
	{regexp.MustCompile(`^管理员减少用户额度 (.+)$`), "Admin decreased user quota by %s"},
	{regexp.MustCompile(`^管理员覆盖用户额度从 (.+) 为 (.+)$`), "Admin changed user quota from %s to %s"},
}

var internalLogTranslationRules = []internalLogTranslationRule{
	{
		zhPattern: regexp.MustCompile(`^token重算：tokens=(\d+), modelRatio=([0-9.]+), groupRatio=([0-9.]+), otherMultiplier=([0-9.]+)$`),
		enPattern: regexp.MustCompile(`^Token recalculation: tokens=(\d+), modelRatio=([0-9.]+), groupRatio=([0-9.]+), otherMultiplier=([0-9.]+)$`),
		zhFormat:  "token重算：tokens=%s, modelRatio=%s, groupRatio=%s, otherMultiplier=%s",
		enFormat:  "Token recalculation: tokens=%s, modelRatio=%s, groupRatio=%s, otherMultiplier=%s",
	},
	{
		zhPattern: regexp.MustCompile(`^adaptor计费调整$`),
		enPattern: regexp.MustCompile(`^Adaptor billing adjustment$`),
		zhFormat:  "adaptor计费调整",
		enFormat:  "Adaptor billing adjustment",
	},
	{
		zhPattern: regexp.MustCompile(`^构图失败$`),
		enPattern: regexp.MustCompile(`^Image generation failed$`),
		zhFormat:  "构图失败",
		enFormat:  "Image generation failed",
	},
	{
		zhPattern: regexp.MustCompile(`^任务超时（(\d+)分钟）$`),
		enPattern: regexp.MustCompile(`^Task timed out \((\d+) minutes\)$`),
		zhFormat:  "任务超时（%s分钟）",
		enFormat:  "Task timed out (%s minutes)",
	},
	{
		zhPattern: regexp.MustCompile(`^任务超时（旧系统遗留任务，不进行退款，请联系管理员）$`),
		enPattern: regexp.MustCompile(`^Task timed out \(legacy task; no refund issued; contact an administrator\)$`),
		zhFormat:  "任务超时（旧系统遗留任务，不进行退款，请联系管理员）",
		enFormat:  "Task timed out (legacy task; no refund issued; contact an administrator)",
	},
	{
		zhPattern: regexp.MustCompile(`^获取渠道信息失败，请联系管理员，渠道ID：(\d+)$`),
		enPattern: regexp.MustCompile(`^Failed to get channel (?:info, channel ID:|information; contact an administrator\. Channel ID:) (\d+)$`),
		zhFormat:  "获取渠道信息失败，请联系管理员，渠道ID：%s",
		enFormat:  "Failed to get channel information; contact an administrator. Channel ID: %s",
	},
	{
		zhPattern: regexp.MustCompile(`^上游任务超时（超过1小时）$`),
		enPattern: regexp.MustCompile(`^Upstream task timed out \(over 1 hour\)$`),
		zhFormat:  "上游任务超时（超过1小时）",
		enFormat:  "Upstream task timed out (over 1 hour)",
	},
	{
		zhPattern: regexp.MustCompile(`^上游返回了无法识别的消息$`),
		enPattern: regexp.MustCompile(`^upstream returned unrecognized message$`),
		zhFormat:  "上游返回了无法识别的消息",
		enFormat:  "upstream returned unrecognized message",
	},
	{
		zhPattern: regexp.MustCompile(`^违规费用已扣除$`),
		enPattern: regexp.MustCompile(`^Violation fee charged$`),
		zhFormat:  "违规费用已扣除",
		enFormat:  "Violation fee charged",
	},
	{
		zhPattern: regexp.MustCompile(`^SMTP 账号无效$`),
		enPattern: regexp.MustCompile(`^invalid SMTP account$`),
		zhFormat:  "SMTP 账号无效",
		enFormat:  "Invalid SMTP account",
	},
	{
		zhPattern: regexp.MustCompile(`^SMTP 服务器不支持 STARTTLS$`),
		enPattern: regexp.MustCompile(`^SMTP server does not support STARTTLS$`),
		zhFormat:  "SMTP 服务器不支持 STARTTLS",
		enFormat:  "SMTP server does not support STARTTLS",
	},
	{
		zhPattern: regexp.MustCompile(`^未配置 SMTP 服务器$`),
		enPattern: regexp.MustCompile(`^SMTP server not configured$`),
		zhFormat:  "未配置 SMTP 服务器",
		enFormat:  "SMTP server not configured",
	},
	{
		zhPattern: regexp.MustCompile(`^重复邮件已被抑制$`),
		enPattern: regexp.MustCompile(`^duplicate email suppressed$`),
		zhFormat:  "重复邮件已被抑制",
		enFormat:  "Duplicate email suppressed",
	},
	{
		zhPattern: regexp.MustCompile(`^已达到每日邮件发送上限（(\d+)/(\d+)）$`),
		enPattern: regexp.MustCompile(`^daily email sending limit reached \((\d+)/(\d+)\)$`),
		zhFormat:  "已达到每日邮件发送上限（%s/%s）",
		enFormat:  "Daily email sending limit reached (%s/%s)",
	},
	{
		zhPattern: regexp.MustCompile(`^检查每日邮件发送上限失败：(.+)$`),
		enPattern: regexp.MustCompile(`^failed to check daily email limit: (.+)$`),
		zhFormat:  "检查每日邮件发送上限失败：%s",
		enFormat:  "Failed to check daily email limit: %s",
	},
	{
		zhPattern: regexp.MustCompile(`^未配置 Cloudflare Account ID$`),
		enPattern: regexp.MustCompile(`^Cloudflare Account ID not configured$`),
		zhFormat:  "未配置 Cloudflare Account ID",
		enFormat:  "Cloudflare Account ID not configured",
	},
	{
		zhPattern: regexp.MustCompile(`^未配置 Cloudflare API Token$`),
		enPattern: regexp.MustCompile(`^Cloudflare API Token not configured$`),
		zhFormat:  "未配置 Cloudflare API Token",
		enFormat:  "Cloudflare API Token not configured",
	},
	{
		zhPattern: regexp.MustCompile(`^未配置 Cloudflare 发件地址$`),
		enPattern: regexp.MustCompile(`^Cloudflare From address not configured$`),
		zhFormat:  "未配置 Cloudflare 发件地址",
		enFormat:  "Cloudflare From address not configured",
	},
	{
		zhPattern: regexp.MustCompile(`^Cloudflare 邮件请求序列化失败：(.+)$`),
		enPattern: regexp.MustCompile(`^failed to marshal Cloudflare email request: (.+)$`),
		zhFormat:  "Cloudflare 邮件请求序列化失败：%s",
		enFormat:  "Failed to marshal Cloudflare email request: %s",
	},
	{
		zhPattern: regexp.MustCompile(`^创建 Cloudflare 请求失败：(.+)$`),
		enPattern: regexp.MustCompile(`^failed to create Cloudflare request: (.+)$`),
		zhFormat:  "创建 Cloudflare 请求失败：%s",
		enFormat:  "Failed to create Cloudflare request: %s",
	},
	{
		zhPattern: regexp.MustCompile(`^Cloudflare 请求失败：(.+)$`),
		enPattern: regexp.MustCompile(`^Cloudflare request failed: (.+)$`),
		zhFormat:  "Cloudflare 请求失败：%s",
		enFormat:  "Cloudflare request failed: %s",
	},
	{
		zhPattern: regexp.MustCompile(`^解析 Cloudflare 响应失败：(.+)$`),
		enPattern: regexp.MustCompile(`^failed to decode Cloudflare response: (.+)$`),
		zhFormat:  "解析 Cloudflare 响应失败：%s",
		enFormat:  "Failed to decode Cloudflare response: %s",
	},
	{
		zhPattern: regexp.MustCompile(`^未知的 SMTP AUTH LOGIN 质询$`),
		enPattern: regexp.MustCompile(`^unknown SMTP AUTH LOGIN challenge$`),
		zhFormat:  "未知的 SMTP AUTH LOGIN 质询",
		enFormat:  "Unknown SMTP AUTH LOGIN challenge",
	},
	{
		zhPattern: regexp.MustCompile(`^意外的 SMTP 认证质询$`),
		enPattern: regexp.MustCompile(`^unexpected SMTP auth challenge$`),
		zhFormat:  "意外的 SMTP 认证质询",
		enFormat:  "Unexpected SMTP auth challenge",
	},
	{
		zhPattern: regexp.MustCompile(`^未知的 SMTP 服务器质询$`),
		enPattern: regexp.MustCompile(`^unknown fromServer$`),
		zhFormat:  "未知的 SMTP 服务器质询",
		enFormat:  "Unknown SMTP server challenge",
	},
	{
		zhPattern: regexp.MustCompile(`^无法确定通道测试用户$`),
		enPattern: regexp.MustCompile(`^failed to resolve channel test user$`),
		zhFormat:  "无法确定通道测试用户",
		enFormat:  "Failed to resolve channel test user",
	},
	{
		zhPattern: regexp.MustCompile(`^无法确定通道测试用户：(.+)$`),
		enPattern: regexp.MustCompile(`^failed to resolve channel test user: (.+)$`),
		zhFormat:  "无法确定通道测试用户：%s",
		enFormat:  "Failed to resolve channel test user: %s",
	},
	{
		zhPattern: regexp.MustCompile(`^系统任务锁已丢失$`),
		enPattern: regexp.MustCompile(`^system task lock lost$`),
		zhFormat:  "系统任务锁已丢失",
		enFormat:  "System task lock lost",
	},
	{
		zhPattern: regexp.MustCompile(`^系统任务租约已过期$`),
		enPattern: regexp.MustCompile(`^task lease expired$`),
		zhFormat:  "系统任务租约已过期",
		enFormat:  "System task lease expired",
	},
	{
		zhPattern: regexp.MustCompile(`^缺少目标时间戳$`),
		enPattern: regexp.MustCompile(`^target timestamp is required$`),
		zhFormat:  "缺少目标时间戳",
		enFormat:  "Target timestamp is required",
	},
	{
		zhPattern: regexp.MustCompile(`^没有删除任何日志$`),
		enPattern: regexp.MustCompile(`^no log rows were deleted$`),
		zhFormat:  "没有删除任何日志",
		enFormat:  "No log rows were deleted",
	},
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
	localized := LocalizeInternalLogText(content, language)
	if localized != content {
		return localized
	}
	switch NormalizeLogLanguage(language) {
	case LogLanguageEN:
		return localizeLogContentToEN(content)
	default:
		return content
	}
}

func LocalizeInternalLogText(content string, language string) string {
	original := content
	content = strings.TrimSpace(content)
	if content == "" {
		return original
	}

	var targetPattern func(rule internalLogTranslationRule) *regexp.Regexp
	var targetFormat func(rule internalLogTranslationRule) string
	switch NormalizeLogLanguage(language) {
	case LogLanguageZH:
		targetPattern = func(rule internalLogTranslationRule) *regexp.Regexp { return rule.enPattern }
		targetFormat = func(rule internalLogTranslationRule) string { return rule.zhFormat }
	case LogLanguageEN:
		targetPattern = func(rule internalLogTranslationRule) *regexp.Regexp { return rule.zhPattern }
		targetFormat = func(rule internalLogTranslationRule) string { return rule.enFormat }
	default:
		return original
	}

	for _, rule := range internalLogTranslationRules {
		matches := targetPattern(rule).FindStringSubmatch(content)
		if matches == nil {
			continue
		}
		args := make([]any, 0, len(matches)-1)
		for _, match := range matches[1:] {
			args = append(args, match)
		}
		return fmt.Sprintf(targetFormat(rule), args...)
	}
	return original
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
		if log == nil {
			continue
		}

		var other struct {
			Op *struct {
				Action string                 `json:"action"`
				Params map[string]interface{} `json:"params"`
			} `json:"op"`
			ErrorType  string `json:"error_type"`
			ErrorCode  string `json:"error_code"`
			StatusCode int    `json:"status_code"`
		}
		if log.Other != "" {
			_ = common.UnmarshalJsonStr(log.Other, &other)
		}

		contentLocalized := false
		if (log.Type == LogTypeManage || log.Type == LogTypeLogin) && other.Op != nil {
			if content, ok := renderOperationLogContent(other.Op.Action, other.Op.Params, language); ok {
				log.Content = content
				contentLocalized = true
			}
		}
		if !contentLocalized && log.Type == LogTypeError && other.ErrorType == string(types.ErrorTypeNewAPIError) {
			if content, ok := renderNewAPIErrorLogContent(log.Content, other.ErrorCode, other.StatusCode, language); ok {
				log.Content = content
				contentLocalized = true
			}
		}
		if !contentLocalized {
			log.Content = LocalizeLogContent(log.Content, language)
		}
		localizeLogOtherReason(log, language)
	}
}

func localizeLogOtherReason(log *Log, language string) {
	if log.Other == "" {
		return
	}
	var other map[string]interface{}
	if err := common.UnmarshalJsonStr(log.Other, &other); err != nil {
		return
	}
	reason, ok := other["reason"].(string)
	if !ok {
		return
	}
	localized := LocalizeInternalLogText(reason, language)
	if localized == reason {
		return
	}
	other["reason"] = localized
	data, err := common.Marshal(other)
	if err == nil {
		log.Other = string(data)
	}
}
