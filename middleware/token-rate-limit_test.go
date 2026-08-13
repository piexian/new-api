package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

func setupTokenRateLimitEngine(t *testing.T, tokenId int, rateLimit, ipRateLimit int) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("token_id", tokenId)
		c.Set("token_rate_limit", rateLimit)
		c.Set("token_ip_rate_limit", ipRateLimit)
		c.Next()
	})
	r.Use(TokenRateLimit())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func performTokenRateLimitRequest(t *testing.T, engine *gin.Engine, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}

func TestTokenRateLimitDisabledByDefault(t *testing.T) {
	engine := setupTokenRateLimitEngine(t, 900001, 0, 0)

	for i := 0; i < 5; i++ {
		recorder := performTokenRateLimitRequest(t, engine, "10.0.0.1:1234")
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected request %d to pass with limits disabled, got status %d", i+1, recorder.Code)
		}
	}
}

func TestTokenRateLimitRejectsOverLimit(t *testing.T) {
	engine := setupTokenRateLimitEngine(t, 900002, 2, 0)

	for i := 0; i < 2; i++ {
		recorder := performTokenRateLimitRequest(t, engine, "10.0.0.2:1234")
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected request %d within token rate limit to pass, got status %d", i+1, recorder.Code)
		}
	}

	recorder := performTokenRateLimitRequest(t, engine, "10.0.0.2:1234")
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected request over token rate limit to be rejected with 429, got status %d", recorder.Code)
	}
}

func TestTokenIpRateLimitRejectsOverLimit(t *testing.T) {
	engine := setupTokenRateLimitEngine(t, 900003, 0, 1)

	recorder := performTokenRateLimitRequest(t, engine, "10.0.0.3:1234")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected first request within ip rate limit to pass, got status %d", recorder.Code)
	}

	recorder = performTokenRateLimitRequest(t, engine, "10.0.0.3:1235")
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request from same ip to be rejected with 429, got status %d", recorder.Code)
	}
}

func TestTokenIpRateLimitIsPerIp(t *testing.T) {
	engine := setupTokenRateLimitEngine(t, 900004, 0, 1)

	for i, ip := range []string{"10.0.1.1:1234", "10.0.1.2:1234"} {
		recorder := performTokenRateLimitRequest(t, engine, ip)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected first request from ip %d to pass, got status %d", i, recorder.Code)
		}
	}
}

func TestTokenRateLimitIsPerToken(t *testing.T) {
	limited := setupTokenRateLimitEngine(t, 900005, 1, 0)
	other := setupTokenRateLimitEngine(t, 900006, 1, 0)

	if recorder := performTokenRateLimitRequest(t, limited, "10.0.2.1:1234"); recorder.Code != http.StatusOK {
		t.Fatalf("expected first request on limited token to pass, got status %d", recorder.Code)
	}
	if recorder := performTokenRateLimitRequest(t, limited, "10.0.2.1:1235"); recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request on limited token to be rejected with 429, got status %d", recorder.Code)
	}

	// 另一个令牌的计数互不影响
	if recorder := performTokenRateLimitRequest(t, other, "10.0.2.1:1236"); recorder.Code != http.StatusOK {
		t.Fatalf("expected request on other token to pass, got status %d", recorder.Code)
	}
}
