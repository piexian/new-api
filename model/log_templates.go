package model

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/types"
)

type localizedLogText struct {
	ZH string
	EN string
}

var operationLogTemplates = map[string]localizedLogText{
	"login": {ZH: "通过 ${method} 登录成功", EN: "Logged in successfully via ${method}"},

	"user.create":           {ZH: "创建了用户 ${username}（角色 ${role}）", EN: "Created user ${username} (role ${role})"},
	"user.update":           {ZH: "更新了用户 ${username}（ID：${id}）", EN: "Updated user ${username} (ID: ${id})"},
	"user.delete":           {ZH: "删除了用户 ${username}（ID：${id}）", EN: "Deleted user ${username} (ID: ${id})"},
	"user.manage":           {ZH: "对用户 ${username}（ID：${id}）执行了 ${action} 操作", EN: "Performed ${action} on user ${username} (ID: ${id})"},
	"user.quota_add":        {ZH: "增加了用户额度 ${quota}", EN: "Increased user quota by ${quota}"},
	"user.quota_subtract":   {ZH: "减少了用户额度 ${quota}", EN: "Decreased user quota by ${quota}"},
	"user.quota_override":   {ZH: "将用户额度从 ${from} 覆盖为 ${to}", EN: "Overrode user quota from ${from} to ${to}"},
	"user.binding_clear":    {ZH: "清除了用户 ${username} 的 ${bindingType} 绑定", EN: "Cleared ${bindingType} binding for user ${username}"},
	"user.2fa_disable":      {ZH: "强制禁用了用户的两步验证", EN: "Force-disabled two-factor authentication for the user"},
	"user.passkey_register": {ZH: "注册了通行密钥", EN: "Registered a passkey"},
	"user.passkey_delete":   {ZH: "删除了通行密钥", EN: "Deleted a passkey"},
	"user.topup_complete":   {ZH: "完成了用户充值订单补单", EN: "Completed top-up order for the user"},
	"user.reset_passkey":    {ZH: "重置了用户通行密钥", EN: "Reset the user passkey"},
	"user.oauth_unbind":     {ZH: "移除了用户的 OAuth 绑定", EN: "Removed an OAuth binding for the user"},

	"option.update":                {ZH: "更新了系统设置 ${key}", EN: "Updated system setting ${key}"},
	"option.payment_compliance":    {ZH: "确认了支付合规设置", EN: "Confirmed payment compliance"},
	"option.reset_ratio":           {ZH: "重置了模型倍率", EN: "Reset model ratios"},
	"option.clear_affinity_cache":  {ZH: "清除了渠道亲和缓存", EN: "Cleared channel affinity cache"},
	"email.test":                   {ZH: "使用 ${provider} 发送了测试邮件", EN: "Sent a test email using ${provider}"},
	"email.template_update":        {ZH: "更新了邮件模板 ${event}（${locale}）", EN: "Updated email template ${event} (${locale})"},
	"email.template_restore":       {ZH: "恢复了邮件模板 ${event}（${locale}）", EN: "Restored email template ${event} (${locale})"},
	"custom_oauth.create":          {ZH: "创建了自定义 OAuth 提供方", EN: "Created a custom OAuth provider"},
	"custom_oauth.update":          {ZH: "更新了自定义 OAuth 提供方", EN: "Updated a custom OAuth provider"},
	"custom_oauth.delete":          {ZH: "删除了自定义 OAuth 提供方", EN: "Deleted a custom OAuth provider"},
	"performance.clear_disk_cache": {ZH: "清除了磁盘缓存", EN: "Cleared disk cache"},
	"performance.gc":               {ZH: "触发了垃圾回收", EN: "Triggered garbage collection"},
	"performance.clear_logs":       {ZH: "清除了运行日志文件", EN: "Cleared log files"},

	"channel.create":              {ZH: "创建了渠道 ${name}（类型 ${type}，数量 ${count}）", EN: "Created channel ${name} (type ${type}, count ${count})"},
	"channel.update":              {ZH: "更新了渠道 ${name}（ID：${id}）", EN: "Updated channel ${name} (ID: ${id})"},
	"channel.delete":              {ZH: "删除了渠道 ${name}（ID：${id}）", EN: "Deleted channel ${name} (ID: ${id})"},
	"channel.delete_batch":        {ZH: "批量删除了 ${count} 个渠道", EN: "Batch deleted ${count} channels"},
	"channel.delete_disabled":     {ZH: "删除了全部已禁用渠道（${count} 个）", EN: "Deleted all disabled channels (${count})"},
	"channel.key_view":            {ZH: "查看了渠道密钥 ${name}（ID：${id}）", EN: "Viewed channel key ${name} (ID: ${id})"},
	"channel.tag_disable":         {ZH: "禁用了标签为 ${tag} 的渠道", EN: "Disabled channels with tag ${tag}"},
	"channel.tag_enable":          {ZH: "启用了标签为 ${tag} 的渠道", EN: "Enabled channels with tag ${tag}"},
	"channel.tag_edit":            {ZH: "编辑了标签为 ${tag} 的渠道", EN: "Edited channels with tag ${tag}"},
	"channel.tag_batch_set":       {ZH: "为 ${count} 个渠道批量设置了标签", EN: "Batch set tag for ${count} channels"},
	"channel.copy":                {ZH: "复制了渠道（源 ID：${sourceId}）到 ${name}（新 ID：${id}）", EN: "Copied channel (source ID: ${sourceId}) to ${name} (new ID: ${id})"},
	"channel.multi_key_manage":    {ZH: "对渠道（ID：${id}）执行了多密钥操作 ${action}", EN: "Multi-key management ${action} on channel (ID: ${id})"},
	"channel.status_update":       {ZH: "更新了渠道状态（ID：${id}，状态：${status}）", EN: "Updated channel status (ID: ${id}, status: ${status})"},
	"channel.status_update_batch": {ZH: "批量更新了渠道状态（成功 ${success_count}，失败 ${failed_count}，状态：${status}）", EN: "Batch updated channel status (succeeded: ${success_count}, failed: ${failed_count}, status: ${status})"},
	"channel.upstream_apply":      {ZH: "将上游模型变更应用到渠道（ID：${id}）", EN: "Applied upstream model changes to channel (ID: ${id})"},
	"channel.upstream_apply_all":  {ZH: "将上游模型变更应用到 ${count} 个渠道", EN: "Applied upstream model changes to ${count} channels"},
	"channel.upstream_detect_all": {ZH: "启动了上游模型检测任务 ${task_id}", EN: "Started upstream model detection task ${task_id}"},

	"redemption.create":         {ZH: "创建了 ${count} 个名为 ${name} 的兑换码（每个 ${quota}）", EN: "Created ${count} redemption codes named ${name} (${quota} each)"},
	"redemption.update":         {ZH: "更新了兑换码", EN: "Updated a redemption code"},
	"redemption.delete":         {ZH: "删除了兑换码", EN: "Deleted a redemption code"},
	"redemption.delete_invalid": {ZH: "删除了无效兑换码", EN: "Deleted invalid redemption codes"},
	"prefill_group.create":      {ZH: "创建了预填组", EN: "Created a prefill group"},
	"prefill_group.update":      {ZH: "更新了预填组", EN: "Updated a prefill group"},
	"prefill_group.delete":      {ZH: "删除了预填组", EN: "Deleted a prefill group"},
	"vendor.create":             {ZH: "创建了供应商", EN: "Created a vendor"},
	"vendor.update":             {ZH: "更新了供应商", EN: "Updated a vendor"},
	"vendor.delete":             {ZH: "删除了供应商", EN: "Deleted a vendor"},
	"model.create":              {ZH: "创建了模型", EN: "Created a model"},
	"model.update":              {ZH: "更新了模型", EN: "Updated a model"},
	"model.delete":              {ZH: "删除了模型", EN: "Deleted a model"},
	"model.sync_upstream":       {ZH: "同步了上游模型", EN: "Synced upstream models"},
	"deployment.create":         {ZH: "创建了部署", EN: "Created a deployment"},
	"deployment.update":         {ZH: "更新了部署", EN: "Updated a deployment"},
	"deployment.delete":         {ZH: "删除了部署", EN: "Deleted a deployment"},

	"subscription.plan_create":     {ZH: "创建了订阅套餐", EN: "Created a subscription plan"},
	"subscription.plan_update":     {ZH: "更新了订阅套餐", EN: "Updated a subscription plan"},
	"subscription.bind":            {ZH: "绑定了订阅", EN: "Bound a subscription"},
	"subscription.plan_reset":      {ZH: "重置了套餐 ${plan_id} 的有效订阅", EN: "Reset active subscriptions for plan ${plan_id}"},
	"subscription.user_plan_reset": {ZH: "重置了用户 ${target_user_id} 在套餐 ${plan_id} 下的有效订阅", EN: "Reset active plan ${plan_id} subscriptions for user ${target_user_id}"},
	"log.clear":                    {ZH: "清除了历史日志", EN: "Cleared historical logs"},
	"generic":                      {ZH: "${method} ${route}", EN: "${method} ${route}"},
}

var newAPIErrorSummaries = map[string]localizedLogText{
	string(types.ErrorCodeInvalidRequest):               {ZH: "请求无效", EN: "Invalid request"},
	string(types.ErrorCodeSensitiveWordsDetected):       {ZH: "检测到敏感词", EN: "Sensitive words detected"},
	string(types.ErrorCodeViolationFeeGrokCSAM):         {ZH: "检测到违规内容（CSAM）", EN: "Policy-violating content detected (CSAM)"},
	string(types.ErrorCodeViolationFeeGrokModeration):   {ZH: "检测到违规内容（内容审核）", EN: "Policy-violating content detected (moderation)"},
	string(types.ErrorCodeCountTokenFailed):             {ZH: "计算令牌数失败", EN: "Failed to count tokens"},
	string(types.ErrorCodeModelPriceError):              {ZH: "计算模型价格失败", EN: "Failed to calculate model price"},
	string(types.ErrorCodeInvalidApiType):               {ZH: "API 类型无效", EN: "Invalid API type"},
	string(types.ErrorCodeJsonMarshalFailed):            {ZH: "JSON 序列化失败", EN: "Failed to encode JSON"},
	string(types.ErrorCodeDoRequestFailed):              {ZH: "发送请求失败", EN: "Failed to send request"},
	string(types.ErrorCodeGetChannelFailed):             {ZH: "获取渠道失败", EN: "Failed to get channel"},
	string(types.ErrorCodeGenRelayInfoFailed):           {ZH: "生成中继信息失败", EN: "Failed to generate relay information"},
	string(types.ErrorCodeChannelNoAvailableKey):        {ZH: "渠道没有可用密钥", EN: "No channel key is available"},
	string(types.ErrorCodeChannelParamOverrideInvalid):  {ZH: "渠道参数覆盖配置无效", EN: "Invalid channel parameter override"},
	string(types.ErrorCodeChannelHeaderOverrideInvalid): {ZH: "渠道请求头覆盖配置无效", EN: "Invalid channel header override"},
	string(types.ErrorCodeChannelModelMappedError):      {ZH: "渠道模型映射失败", EN: "Channel model mapping failed"},
	string(types.ErrorCodeChannelAwsClientError):        {ZH: "创建渠道 AWS 客户端失败", EN: "Failed to create channel AWS client"},
	string(types.ErrorCodeChannelInvalidKey):            {ZH: "渠道密钥无效", EN: "Invalid channel key"},
	string(types.ErrorCodeChannelResponseTimeExceeded):  {ZH: "渠道响应时间超限", EN: "Channel response time exceeded"},
	string(types.ErrorCodeReadRequestBodyFailed):        {ZH: "读取请求正文失败", EN: "Failed to read request body"},
	string(types.ErrorCodeConvertRequestFailed):         {ZH: "转换请求失败", EN: "Failed to convert request"},
	string(types.ErrorCodeAccessDenied):                 {ZH: "访问被拒绝", EN: "Access denied"},
	string(types.ErrorCodeBadRequestBody):               {ZH: "请求正文无效", EN: "Invalid request body"},
	string(types.ErrorCodeReadResponseBodyFailed):       {ZH: "读取响应正文失败", EN: "Failed to read response body"},
	string(types.ErrorCodeBadResponseStatusCode):        {ZH: "响应状态码异常", EN: "Unexpected response status code"},
	string(types.ErrorCodeBadResponse):                  {ZH: "响应无效", EN: "Invalid response"},
	string(types.ErrorCodeBadResponseBody):              {ZH: "响应正文无效", EN: "Invalid response body"},
	string(types.ErrorCodeEmptyResponse):                {ZH: "响应为空", EN: "Empty response"},
	string(types.ErrorCodeAwsInvokeError):               {ZH: "调用 AWS 服务失败", EN: "Failed to invoke AWS service"},
	string(types.ErrorCodeModelNotFound):                {ZH: "未找到模型", EN: "Model not found"},
	string(types.ErrorCodePromptBlocked):                {ZH: "提示词被拦截", EN: "Prompt blocked"},
	string(types.ErrorCodeQueryDataError):               {ZH: "查询数据失败", EN: "Failed to query data"},
	string(types.ErrorCodeUpdateDataError):              {ZH: "更新数据失败", EN: "Failed to update data"},
	string(types.ErrorCodeInsufficientUserQuota):        {ZH: "用户额度不足", EN: "Insufficient user quota"},
	string(types.ErrorCodePreConsumeTokenQuotaFailed):   {ZH: "预扣费失败", EN: "Failed to pre-consume quota"},
}

var statusCodePrefixPattern = regexp.MustCompile(`^status_code=(\d+),?\s*`)

func localizedText(text localizedLogText, language string) string {
	if NormalizeLogLanguage(language) == LogLanguageEN {
		return text.EN
	}
	return text.ZH
}

func renderOperationLogContent(action string, params map[string]interface{}, language string) (string, bool) {
	template, ok := operationLogTemplates[action]
	if !ok {
		return "", false
	}
	rendered := os.Expand(localizedText(template, language), func(key string) string {
		if value, exists := params[key]; exists {
			return fmt.Sprintf("%v", value)
		}
		return ""
	})
	return rendered, true
}

// RenderOperationLogContent renders a stable operation action for storage or
// API consumers. Unknown actions remain readable as their action identifier.
func RenderOperationLogContent(action string, params map[string]interface{}, language string) string {
	if rendered, ok := renderOperationLogContent(action, params, language); ok {
		return rendered
	}
	return action
}

func renderNewAPIErrorLogContent(content string, errorCode string, statusCode int, language string) (string, bool) {
	summaryText, ok := newAPIErrorSummaries[errorCode]
	if !ok {
		return "", false
	}
	summary := localizedText(summaryText, language)
	detail := strings.TrimSpace(content)
	if matches := statusCodePrefixPattern.FindStringSubmatch(detail); len(matches) > 0 {
		detail = strings.TrimSpace(statusCodePrefixPattern.ReplaceAllString(detail, ""))
		if statusCode == 0 {
			statusCode, _ = strconv.Atoi(matches[1])
		}
	}

	if detail == "" || detail == errorCode || detail == summaryText.ZH || detail == summaryText.EN {
		detail = ""
	}
	if detail != "" {
		if NormalizeLogLanguage(language) == LogLanguageEN {
			summary += "; Original detail: " + detail
		} else {
			summary += "；原始详情：" + detail
		}
	}
	if statusCode > 0 {
		summary = fmt.Sprintf("status_code=%d, %s", statusCode, summary)
	}
	return summary, true
}
