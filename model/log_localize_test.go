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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := LocalizeLogContent(tt.content, tt.language); actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
