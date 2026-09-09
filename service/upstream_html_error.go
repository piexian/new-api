package service

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/types"
)

var upstreamHTMLDocument = regexp.MustCompile(`(?i)<(?:!doctype\s+html\b|html\b|head\b|body\b)`)

// Classify pages, not individual words such as "Cloudflare" in an API error.
func upstreamHTMLMessage(header http.Header, body string, statusCode int) string {
	if statusCode == 524 {
		return "上游返回：Cloudflare 源站通信超时（HTTP 524）"
	}
	cfChallenge := strings.EqualFold(strings.TrimSpace(header.Get("Cf-Mitigated")), "challenge")
	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(header.Get("Content-Type"), ";", 2)[0]))
	if !cfChallenge && contentType != "text/html" && contentType != "application/xhtml+xml" && !upstreamHTMLDocument.MatchString(body) {
		return ""
	}
	page := strings.ToLower(body)
	description := "上游返回 HTML"
	switch {
	case cfChallenge || strings.Contains(page, "/cdn-cgi/challenge-platform/") || strings.Contains(page, "cf-chl-") || strings.Contains(page, "_cf_chl_opt"):
		description += "（Cloudflare 人机验证拦截）"
	case strings.Contains(page, "g-recaptcha") || strings.Contains(page, "h-captcha") || strings.Contains(page, "hcaptcha") || strings.Contains(page, "cf-turnstile") || strings.Contains(page, "verify you are human") || strings.Contains(page, "verify that you are human") || strings.Contains(page, "checking your browser") || strings.Contains(page, "captcha") || strings.Contains(page, "人机验证"):
		description += "（人机验证拦截）"
	}
	return fmt.Sprintf("%s（HTTP %d）", description, statusCode)
}

func upstreamHTMLAPIError(message string, statusCode int) *types.NewAPIError {
	code := types.ErrorCodeBadResponseStatusCode
	var options []types.NewAPIErrorOptions
	if statusCode == 524 {
		code = types.ErrorCodeUpstreamTimeout
		// Preserve the timeout's no-retry behavior even after status-code mapping.
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	return types.WithOpenAIError(types.OpenAIError{
		Message: message,
		Type:    string(types.ErrorTypeUpstreamError),
		Code:    code,
	}, statusCode, options...)
}
