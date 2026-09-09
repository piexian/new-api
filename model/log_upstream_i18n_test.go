package model

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUpstreamLogPrefixLocalization(t *testing.T) {
	cases := []struct{ zh, en string }{
		{"上游返回：原文 original\nsecond line (status_code=429)", "Upstream returned: 原文 original\nsecond line (status_code=429)"},
		{"上游返回 HTML（HTTP 502） (status_code=502)", "Upstream returned HTML (HTTP 502) (status_code=502)"},
		{"上游返回 HTML（Cloudflare 人机验证拦截）（HTTP 403）", "Upstream returned HTML (Cloudflare human verification challenge) (HTTP 403)"},
		{"上游返回 HTML（人机验证拦截）（HTTP 403）", "Upstream returned HTML (human verification challenge) (HTTP 403)"},
		{"上游返回：Cloudflare 源站通信超时（HTTP 524）", "Upstream returned: Cloudflare origin communication timed out (HTTP 524)"},
	}
	for _, tc := range cases {
		logs := []*Log{{Type: LogTypeError, Content: tc.zh, Other: `{"error_type":"new_api_error","error_code":"bad_response","new_api_error":false}`}}
		LocalizeLogs(logs, LogLanguageEN)
		require.Equal(t, tc.en, logs[0].Content)
		LocalizeLogs(logs, LogLanguageZH)
		require.Equal(t, tc.zh, logs[0].Content)
	}
}

func TestLocalErrorOriginControlsLogTranslation(t *testing.T) {
	logs := []*Log{{Type: LogTypeError, Content: "access_denied (status_code=403)", Other: `{"error_type":"openai_error","error_code":"access_denied","new_api_error":true,"status_code":403}`}}
	LocalizeLogs(logs, LogLanguageEN)
	require.Equal(t, "status_code=403, Access denied", logs[0].Content)
}
