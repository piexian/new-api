package middleware

import (
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

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
