package middleware

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const turnstileSiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var (
	turnstileSiteVerifyClient       = &http.Client{Timeout: 10 * time.Second}
	turnstileDirectSiteVerifyClient = &http.Client{
		Timeout:   10 * time.Second,
		Transport: newTurnstileDirectTransport(),
	}
)

type turnstileCheckResponse struct {
	Success bool `json:"success"`
}

type TurnstileScope string

const (
	TurnstileScopeLegacy                    TurnstileScope = "legacy"
	TurnstileScopeLogin                     TurnstileScope = "login"
	TurnstileScopeRegister                  TurnstileScope = "register"
	TurnstileScopeRegisterEmailVerification TurnstileScope = "register_email_verification"
	TurnstileScopeEmailBindingVerification  TurnstileScope = "email_binding_verification"
	TurnstileScopePasswordReset             TurnstileScope = "password_reset"
	TurnstileScopeCheckin                   TurnstileScope = "checkin"
	TurnstileScopeSensitiveUpdate           TurnstileScope = "sensitive_update"
)

func ValidateTurnstile(c *gin.Context) bool {
	return validateTurnstile(c, true, TurnstileScopeLegacy, common.TurnstileCheckEnabled)
}

func ValidateTurnstileNoSession(c *gin.Context) bool {
	return validateTurnstile(c, false, TurnstileScopeLegacy, common.TurnstileCheckEnabled)
}

func ValidateTurnstileForScope(c *gin.Context, scope TurnstileScope) bool {
	return validateTurnstile(c, true, scope, isTurnstileScopeEnabled(scope))
}

func ValidateTurnstileNoSessionForScope(c *gin.Context, scope TurnstileScope) bool {
	return validateTurnstile(c, false, scope, isTurnstileScopeEnabled(scope))
}

func isTurnstileScopeEnabled(scope TurnstileScope) bool {
	switch scope {
	case TurnstileScopeLogin:
		return common.TurnstileLoginEnabled
	case TurnstileScopeRegister:
		return common.TurnstileRegisterEnabled
	case TurnstileScopeRegisterEmailVerification:
		return common.TurnstileRegisterEmailVerificationEnabled
	case TurnstileScopeEmailBindingVerification:
		return common.TurnstileEmailBindingVerificationEnabled
	case TurnstileScopePasswordReset:
		return common.TurnstilePasswordResetEnabled
	case TurnstileScopeCheckin:
		return common.TurnstileCheckinEnabled
	case TurnstileScopeSensitiveUpdate:
		return common.TurnstileSensitiveUpdateEnabled
	default:
		return common.TurnstileCheckEnabled
	}
}

func turnstileSessionKey(scope TurnstileScope) string {
	if scope == "" || scope == TurnstileScopeLegacy {
		return "turnstile"
	}
	return "turnstile:" + string(scope)
}

func turnstileEmailVerificationScope(c *gin.Context) TurnstileScope {
	if c != nil && strings.EqualFold(strings.TrimSpace(c.Query("purpose")), "bind") {
		return TurnstileScopeEmailBindingVerification
	}
	return TurnstileScopeRegisterEmailVerification
}

func validateTurnstile(c *gin.Context, allowSessionReuse bool, scope TurnstileScope, enabled bool) bool {
	if !enabled {
		return true
	}

	var session sessions.Session
	if allowSessionReuse {
		session = sessions.Default(c)
		turnstileChecked := session.Get(turnstileSessionKey(scope))
		if turnstileChecked != nil {
			return true
		}
	}

	response := c.Query("turnstile")
	if response == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Turnstile token 为空",
		})
		c.Abort()
		return false
	}

	rawRes, err := postTurnstileSiteVerify(url.Values{
		"secret":   {common.TurnstileSecretKey},
		"response": {response},
		"remoteip": {c.ClientIP()},
	})
	if err != nil {
		common.SysLog(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		c.Abort()
		return false
	}
	defer rawRes.Body.Close()

	var res turnstileCheckResponse
	err = common.DecodeJson(rawRes.Body, &res)
	if err != nil {
		common.SysLog(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		c.Abort()
		return false
	}
	if !res.Success {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Turnstile 校验失败，请刷新重试！",
		})
		c.Abort()
		return false
	}

	if allowSessionReuse {
		session.Set(turnstileSessionKey(scope), true)
		err = session.Save()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"message": "无法保存会话信息，请重试",
				"success": false,
			})
			c.Abort()
			return false
		}
	}
	return true
}

func postTurnstileSiteVerify(values url.Values) (*http.Response, error) {
	rawRes, err := postTurnstileSiteVerifyWithClient(turnstileSiteVerifyClient, values)
	if err == nil || !isTurnstileCertificateError(err) {
		return rawRes, err
	}
	if rawRes != nil && rawRes.Body != nil {
		_ = rawRes.Body.Close()
	}

	common.SysLog(fmt.Sprintf("Turnstile siteverify TLS certificate check failed through environment proxy, retrying without proxy: %v", err))
	directRes, directErr := postTurnstileSiteVerifyWithClient(turnstileDirectSiteVerifyClient, values)
	if directErr != nil {
		common.SysLog(fmt.Sprintf("Turnstile siteverify direct retry failed: %v", directErr))
		return nil, fmt.Errorf("turnstile siteverify failed through environment proxy: %w; direct retry failed: %v", err, directErr)
	}
	return directRes, nil
}

func postTurnstileSiteVerifyWithClient(client *http.Client, values url.Values) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	return client.PostForm(turnstileSiteVerifyURL, values)
}

func isTurnstileCertificateError(err error) bool {
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return true
	}
	var unknownAuthorityErr x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthorityErr) {
		return true
	}
	var certificateInvalidErr x509.CertificateInvalidError
	return errors.As(err, &certificateInvalidErr)
}

func newTurnstileDirectTransport() http.RoundTripper {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok || defaultTransport == nil {
		return &http.Transport{Proxy: nil}
	}
	transport := defaultTransport.Clone()
	transport.Proxy = nil
	return transport
}

func TurnstileCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ValidateTurnstile(c) {
			return
		}
		c.Next()
	}
}

func TurnstileCheckForScope(scope TurnstileScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ValidateTurnstileForScope(c, scope) {
			return
		}
		c.Next()
	}
}

func TurnstileCheckNoSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ValidateTurnstileNoSession(c) {
			return
		}
		c.Next()
	}
}

func TurnstileCheckNoSessionForScope(scope TurnstileScope) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ValidateTurnstileNoSessionForScope(c, scope) {
			return
		}
		c.Next()
	}
}

func TurnstileEmailVerificationCheckNoSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ValidateTurnstileNoSessionForScope(c, turnstileEmailVerificationScope(c)) {
			return
		}
		c.Next()
	}
}
