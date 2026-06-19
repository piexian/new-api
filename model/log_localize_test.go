package model

import "testing"

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := LocalizeLogContent(tt.content, tt.language); actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
