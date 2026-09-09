package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRelayErrorHandlerFiltersHTML(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		body   string
		want   string
	}{
		{"html without header", nil, "\xef\xbb\xbf <!-- proxy --> <!DOCTYPE HTML><HTML><body>PRIVATE_PAGE</body></HTML>", "上游返回 HTML（HTTP 403）"},
		{"html content type", http.Header{"Content-Type": {"Text/HTML; charset=utf-8"}}, "<div>PRIVATE_PAGE</div>", "上游返回 HTML（HTTP 403）"},
		{"xhtml", http.Header{"Content-Type": {"application/xhtml+xml"}}, "PRIVATE_PAGE", "上游返回 HTML（HTTP 403）"},
		{"cloudflare header", http.Header{"Cf-Mitigated": {"challenge"}}, "PRIVATE_PAGE", "上游返回 HTML（Cloudflare 人机验证拦截）（HTTP 403）"},
		{"cloudflare page", nil, `<html><script src="/cdn-cgi/challenge-platform/scripts/jsd/main.js"></script>PRIVATE_PAGE</html>`, "上游返回 HTML（Cloudflare 人机验证拦截）（HTTP 403）"},
		{"captcha page", nil, `<html><div class="g-recaptcha">PRIVATE_PAGE</div></html>`, "上游返回 HTML（人机验证拦截）（HTTP 403）"},
		{"human verification page", nil, `<html>Verify you are human PRIVATE_PAGE</html>`, "上游返回 HTML（人机验证拦截）（HTTP 403）"},
		{"json escaped html", nil, `{"error":{"message":"\u003chtml\u003ePRIVATE_PAGE\u003c/html\u003e","type":"server_error","metadata":{"secret":"PRIVATE_PAGE"}}}`, "上游返回 HTML（HTTP 403）"},
		{"json string html", nil, `{"message":"<html>PRIVATE_PAGE</html>","usage":{"cost_in_usd_ticks":123}}`, "上游返回 HTML（HTTP 403）"},
	}
	for _, tt := range tests {
		for _, showBody := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/showBody=%t", tt.name, showBody), func(t *testing.T) {
				resp := &http.Response{StatusCode: 403, Header: tt.header, Body: io.NopCloser(strings.NewReader(tt.body))}
				apiErr := RelayErrorHandler(context.Background(), resp, showBody)
				require.Equal(t, 403, apiErr.StatusCode)
				require.Equal(t, tt.want, apiErr.Error())
				require.Equal(t, string(types.ErrorTypeUpstreamError), apiErr.ToOpenAIError().Type)
				require.Equal(t, tt.want, apiErr.ToOpenAIError().Message)
				require.Equal(t, tt.want, apiErr.ToClaudeError().Message)
				require.Empty(t, apiErr.Metadata)
				encoded, err := common.Marshal(apiErr.ToOpenAIError())
				require.NoError(t, err)
				require.NotContains(t, string(encoded), "PRIVATE_PAGE")
			})
		}
	}
}

func TestRelayErrorHandlerDoesNotClassifyNormalAPIErrorsAsHTML(t *testing.T) {
	for _, message := range []string{"Cloudflare request rate limited", "captcha service unavailable", "HTML output is not supported", "invalid <html_model> parameter"} {
		body, err := common.Marshal(map[string]any{"error": map[string]any{"message": message, "type": "invalid_request_error", "code": "invalid_request"}})
		require.NoError(t, err)
		resp := &http.Response{StatusCode: 429, Header: http.Header{"Server": {"cloudflare"}}, Body: io.NopCloser(strings.NewReader(string(body)))}
		apiErr := RelayErrorHandler(context.Background(), resp, false)
		require.Equal(t, message, apiErr.ToOpenAIError().Message)
		require.Equal(t, "invalid_request_error", apiErr.ToOpenAIError().Type)
		require.Equal(t, 429, apiErr.StatusCode)
	}
}

func TestTaskErrorWrapperFiltersHTML(t *testing.T) {
	err := TaskErrorWrapper(errors.New("unmarshal response body failed, body: <html>PRIVATE_PAGE</html>"), "unmarshal_response_body_failed", 502)
	require.Equal(t, "上游返回 HTML（HTTP 502）", err.Message)
	require.NotContains(t, err.Error.Error(), "PRIVATE_PAGE")
	require.Equal(t, 502, err.StatusCode)
	require.False(t, err.LocalError)
}

func TestCloudflare524IsAnUpstreamTimeout(t *testing.T) {
	for _, body := range []string{"", "error code: 524", "<html>PRIVATE_PAGE A timeout occurred</html>", `{"error":{"message":"PRIVATE_PAGE timeout"}}`} {
		for _, showBody := range []bool{false, true} {
			resp := &http.Response{StatusCode: 524, Body: io.NopCloser(strings.NewReader(body))}
			apiErr := RelayErrorHandler(context.Background(), resp, showBody)
			require.Equal(t, 524, apiErr.StatusCode)
			require.Equal(t, types.ErrorCodeUpstreamTimeout, apiErr.GetErrorCode())
			require.Equal(t, "上游返回：Cloudflare 源站通信超时（HTTP 524）", apiErr.ToClientOpenAIError().Message)
			require.False(t, apiErr.IsLocalError())
			require.True(t, types.IsSkipRetryError(apiErr))
			require.Empty(t, apiErr.Metadata)
			ResetStatusCode(apiErr, `{"524":"502"}`)
			require.Equal(t, 502, apiErr.StatusCode)
			require.True(t, types.IsSkipRetryError(apiErr))
		}
	}
	taskErr := TaskErrorWrapper(errors.New("<html>PRIVATE_PAGE</html>"), "fetch_failed", 524)
	require.Equal(t, "upstream_timeout", taskErr.Code)
	require.Equal(t, "上游返回：Cloudflare 源站通信超时（HTTP 524）", taskErr.Message)
	require.False(t, taskErr.LocalError)
}
