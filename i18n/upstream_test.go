package i18n_test

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpstreamClientMessageLanguage(t *testing.T) {
	require.NoError(t, i18n.Init())
	for _, tc := range []struct{ language, want string }{
		{"en", "Upstream returned: 原文 original"},
		{"zh-CN", "上游返回：原文 original"},
		{"zh-TW", "上游回傳：原文 original"},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Accept-Language", tc.language)
		err := types.WithOpenAIError(types.OpenAIError{Message: "原文 original", Type: "upstream_error"}, 502)
		require.Equal(t, tc.want, err.ToClientOpenAIError(c).Message)
		require.Equal(t, tc.want, err.ToClientClaudeError(c).Message)
		require.Equal(t, "原文 original", err.Error())
	}
	for _, key := range []string{i18n.MsgUpstreamTimeout, i18n.MsgUpstreamHTML, i18n.MsgUpstreamCFChallenge, i18n.MsgUpstreamChallenge} {
		for _, lang := range []string{"en", "zh-CN", "zh-TW"} {
			require.NotEqual(t, key, i18n.Translate(lang, key, map[string]any{"Status": 524}))
		}
	}
}
