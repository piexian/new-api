package middleware

import (
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestPostTurnstileSiteVerifyRetriesDirectOnCertificateError(t *testing.T) {
	originalClient := turnstileSiteVerifyClient
	originalDirectClient := turnstileDirectSiteVerifyClient
	t.Cleanup(func() {
		turnstileSiteVerifyClient = originalClient
		turnstileDirectSiteVerifyClient = originalDirectClient
	})

	var directCalls int32
	turnstileSiteVerifyClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{
				Op:  "Post",
				URL: turnstileSiteVerifyURL,
				Err: x509.HostnameError{
					Certificate: &x509.Certificate{},
					Host:        "challenges.cloudflare.com",
				},
			}
		}),
	}
	turnstileDirectSiteVerifyClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			atomic.AddInt32(&directCalls, 1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	rawRes, err := postTurnstileSiteVerify(url.Values{"response": {"token"}})
	require.NoError(t, err)
	require.NotNil(t, rawRes)
	require.Equal(t, int32(1), atomic.LoadInt32(&directCalls))
	_ = rawRes.Body.Close()
}

func TestPostTurnstileSiteVerifyDoesNotRetryDirectOnGenericError(t *testing.T) {
	originalClient := turnstileSiteVerifyClient
	originalDirectClient := turnstileDirectSiteVerifyClient
	t.Cleanup(func() {
		turnstileSiteVerifyClient = originalClient
		turnstileDirectSiteVerifyClient = originalDirectClient
	})

	var directCalls int32
	turnstileSiteVerifyClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		}),
	}
	turnstileDirectSiteVerifyClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			atomic.AddInt32(&directCalls, 1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	rawRes, err := postTurnstileSiteVerify(url.Values{"response": {"token"}})
	require.Error(t, err)
	require.Nil(t, rawRes)
	require.Equal(t, int32(0), atomic.LoadInt32(&directCalls))
}

func TestValidateTurnstileNoSessionForScopeUsesDedicatedSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalLogin := common.TurnstileLoginEnabled
	originalRegister := common.TurnstileRegisterEnabled
	t.Cleanup(func() {
		common.TurnstileLoginEnabled = originalLogin
		common.TurnstileRegisterEnabled = originalRegister
	})

	common.TurnstileLoginEnabled = false
	common.TurnstileRegisterEnabled = true

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/login", nil)
	require.True(t, ValidateTurnstileNoSessionForScope(ctx, TurnstileScopeLogin))

	recorder := httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", nil)
	require.False(t, ValidateTurnstileNoSessionForScope(ctx, TurnstileScopeRegister))
	require.Contains(t, recorder.Body.String(), "Turnstile token")
}

func TestTurnstileEmailVerificationScopeUsesPurpose(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		target string
		want   TurnstileScope
	}{
		{target: "/api/verification?purpose=bind", want: TurnstileScopeEmailBindingVerification},
		{target: "/api/verification?purpose=Bind", want: TurnstileScopeEmailBindingVerification},
		{target: "/api/verification?purpose=register", want: TurnstileScopeRegisterEmailVerification},
		{target: "/api/verification", want: TurnstileScopeRegisterEmailVerification},
	} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodGet, test.target, nil)
		require.Equal(t, test.want, turnstileEmailVerificationScope(ctx))
	}
}
