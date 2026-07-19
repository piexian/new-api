package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeLogLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty follows default", input: "", expected: LogLanguageFollow},
		{name: "zh cn normalizes to zh", input: "zh-CN", expected: LogLanguageZH},
		{name: "en us normalizes to en", input: "en-US", expected: LogLanguageEN},
		{name: "unsupported follows default", input: "ja", expected: LogLanguageFollow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := NormalizeLogLanguage(tt.input); actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestResolveEffectiveLogLanguage(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		fallback string
		expected string
	}{
		{name: "user setting wins", user: "en", fallback: "zh", expected: LogLanguageEN},
		{name: "fallback used when user follows", user: "", fallback: "en", expected: LogLanguageEN},
		{name: "zh default when both unset", user: "", fallback: "", expected: LogLanguageZH},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := ResolveEffectiveLogLanguage(tt.user, tt.fallback); actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestLocalizeLogContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		language string
		expected string
	}{
		{
			name:     "english quota log",
			content:  "新用户注册赠送 $1.00",
			language: "en",
			expected: "New user registration bonus: $1.00",
		},
		{
			name:     "zh keeps original",
			content:  "新用户注册赠送 $1.00",
			language: "zh",
			expected: "新用户注册赠送 $1.00",
		},
		{
			name:     "unknown keeps original",
			content:  "自定义日志",
			language: "en",
			expected: "自定义日志",
		},
		{
			name:     "english channel cooldown already active",
			content:  "通道「智谱coding」（#782）已处于套餐限额冷却，禁用至 2026-06-24T01:20:15+08:00，原因：status_code=429",
			language: "en",
			expected: "Channel \"智谱coding\" (#782) is already in plan quota cooldown until 2026-06-24T01:20:15+08:00, reason: status_code=429",
		},
		{
			name:     "english channel rate limit cooldown entered",
			content:  "通道「智谱coding」（#782）进入套餐限额冷却，限流至 2026-06-24T01:20:15+08:00，原因：status_code=429",
			language: "en",
			expected: "Channel \"智谱coding\" (#782) entered plan quota cooldown until 2026-06-24T01:20:15+08:00, reason: status_code=429",
		},
		{
			name:     "english multi key cooldown entered",
			content:  "通道「智谱coding」（#782）密钥 #2 进入套餐限额冷却，禁用至 2026-06-24T01:20:15+08:00，原因：status_code=429",
			language: "en",
			expected: "Channel \"智谱coding\" (#782) key #2 entered plan quota cooldown until 2026-06-24T01:20:15+08:00, reason: status_code=429",
		},
		{
			name:     "english model cooldown entered",
			content:  "通道「MiniMax」（#783）模型「MiniMax-M2.7」进入套餐限额冷却，禁用至 2026-06-24T01:20:15+08:00，原因：status_code=429",
			language: "en",
			expected: "Channel \"MiniMax\" (#783) model \"MiniMax-M2.7\" entered plan quota cooldown until 2026-06-24T01:20:15+08:00, reason: status_code=429",
		},
		{
			name:     "english subscription balance purchase",
			content:  "使用余额购买订阅成功，套餐: Pro，支付金额: 9.99，扣除额度: 500000",
			language: "en",
			expected: "Subscription purchased with balance, plan: Pro, paid: 9.99, quota deducted: 500000",
		},
		{
			name:     "english admin subscription quota reset",
			content:  "管理员重置订阅套餐 Pro（ID: 12）额度",
			language: "en",
			expected: "Admin reset quota for subscription plan Pro (ID: 12)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := LocalizeLogContent(tt.content, tt.language); actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestLocalizeInternalLogText(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		language string
		expected string
	}{
		{
			name:     "task timeout to english",
			content:  "任务超时（30分钟）",
			language: LogLanguageEN,
			expected: "Task timed out (30 minutes)",
		},
		{
			name:     "channel lookup failure to chinese",
			content:  "Failed to get channel info, channel ID: 12",
			language: LogLanguageZH,
			expected: "获取渠道信息失败，请联系管理员，渠道ID：12",
		},
		{
			name:     "violation fee to chinese",
			content:  "Violation fee charged",
			language: LogLanguageZH,
			expected: "违规费用已扣除",
		},
		{
			name:     "email configuration error to chinese",
			content:  "SMTP server not configured",
			language: LogLanguageZH,
			expected: "未配置 SMTP 服务器",
		},
		{
			name:     "email daily limit to chinese",
			content:  "daily email sending limit reached (100/100)",
			language: LogLanguageZH,
			expected: "已达到每日邮件发送上限（100/100）",
		},
		{
			name:     "system task lease error to chinese",
			content:  "task lease expired",
			language: LogLanguageZH,
			expected: "系统任务租约已过期",
		},
		{
			name:     "unknown provider error is unchanged",
			content:  "Cloudflare API error: provider detail",
			language: LogLanguageZH,
			expected: "Cloudflare API error: provider detail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, LocalizeInternalLogText(tt.content, tt.language))
		})
	}
}

func TestUpdateUserSettingInvalidatesRootLogLanguageFallback(t *testing.T) {
	setupUserUpdateTestState(t)
	InvalidateRootLogLanguageCache()
	t.Cleanup(InvalidateRootLogLanguageCache)

	root := User{
		Id:       1,
		Username: "root-log-language",
		Password: "password",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
	}
	root.SetSetting(dto.UserSetting{Language: "zh", LogLanguage: "en"})
	require.NoError(t, DB.Create(&root).Error)
	require.Equal(t, LogLanguageEN, GetRootLogLanguageFallback())

	setting := root.GetSetting()
	setting.LogLanguage = "zh"
	require.NoError(t, UpdateUserSetting(root.Id, setting))
	require.Equal(t, LogLanguageZH, GetRootLogLanguageFallback())
}

func TestLocalizeLogsUsesStructuredOperationLanguage(t *testing.T) {
	tests := []struct {
		name     string
		log      *Log
		language string
		expected string
	}{
		{
			name: "manage log in chinese",
			log: &Log{
				Type:    LogTypeManage,
				Content: "Updated system setting LogLanguage",
				Other: common.MapToJsonStr(map[string]interface{}{
					"op": map[string]interface{}{
						"action": "option.update",
						"params": map[string]interface{}{"key": "LogLanguage"},
					},
				}),
			},
			language: LogLanguageZH,
			expected: "更新了系统设置 LogLanguage",
		},
		{
			name: "login log in english",
			log: &Log{
				Type:    LogTypeLogin,
				Content: "登录成功",
				Other: common.MapToJsonStr(map[string]interface{}{
					"op": map[string]interface{}{
						"action": "login",
						"params": map[string]interface{}{"method": "password"},
					},
				}),
			},
			language: LogLanguageEN,
			expected: "Logged in successfully via password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LocalizeLogs([]*Log{tt.log}, tt.language)
			assert.Equal(t, tt.expected, tt.log.Content)
		})
	}
}

func TestLocalizeLogsOnlyTranslatesNewAPIErrors(t *testing.T) {
	newAPILog := &Log{
		Type:    LogTypeError,
		Content: "status_code=403, 用户额度不足, 剩余额度: $0.00",
		Other: common.MapToJsonStr(map[string]interface{}{
			"error_type":  "new_api_error",
			"error_code":  "insufficient_user_quota",
			"status_code": 403,
		}),
	}
	upstreamLog := &Log{
		Type:    LogTypeError,
		Content: "status_code=429, upstream overloaded",
		Other: common.MapToJsonStr(map[string]interface{}{
			"error_type":  "openai_error",
			"error_code":  "bad_response_status_code",
			"status_code": 429,
		}),
	}

	LocalizeLogs([]*Log{newAPILog, upstreamLog}, LogLanguageEN)

	assert.Equal(t, "status_code=403, Insufficient user quota; Original detail: 用户额度不足, 剩余额度: $0.00", newAPILog.Content)
	assert.Equal(t, "status_code=429, upstream overloaded", upstreamLog.Content)
}

func TestLocalizeLogsTranslatesInternalTaskBillingFields(t *testing.T) {
	refundLog := &Log{
		Type: LogTypeRefund,
		Other: common.MapToJsonStr(map[string]interface{}{
			"task_id": "task-1",
			"reason":  "构图失败",
		}),
	}
	violationLog := &Log{
		Type:    LogTypeConsume,
		Content: "Violation fee charged",
	}
	upstreamReasonLog := &Log{
		Type: LogTypeRefund,
		Other: common.MapToJsonStr(map[string]interface{}{
			"reason": "provider overloaded",
		}),
	}

	LocalizeLogs([]*Log{refundLog}, LogLanguageEN)
	LocalizeLogs([]*Log{violationLog, upstreamReasonLog}, LogLanguageZH)

	var refundOther map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(refundLog.Other, &refundOther))
	assert.Equal(t, "Image generation failed", refundOther["reason"])
	assert.Equal(t, "违规费用已扣除", violationLog.Content)

	var upstreamOther map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(upstreamReasonLog.Other, &upstreamOther))
	assert.Equal(t, "provider overloaded", upstreamOther["reason"])
}

func TestLogTranslationCatalogsAreBilingual(t *testing.T) {
	for index, translation := range internalLogTranslationRules {
		t.Run(fmt.Sprintf("internal/%d", index), func(t *testing.T) {
			assert.NotNil(t, translation.zhPattern)
			assert.NotNil(t, translation.enPattern)
			assert.NotEmpty(t, translation.zhFormat)
			assert.NotEmpty(t, translation.enFormat)
			assert.Equal(t, translation.zhPattern.NumSubexp(), translation.enPattern.NumSubexp())
		})
	}
	for action, translation := range operationLogTemplates {
		t.Run("operation/"+action, func(t *testing.T) {
			assert.NotEmpty(t, translation.ZH)
			assert.NotEmpty(t, translation.EN)
		})
	}
	for code, translation := range newAPIErrorSummaries {
		t.Run("error/"+code, func(t *testing.T) {
			assert.NotEmpty(t, translation.ZH)
			assert.NotEmpty(t, translation.EN)
		})
	}
}
