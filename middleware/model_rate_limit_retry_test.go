package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelRequestLimitsReturnRetryAfter(t *testing.T) {
	for _, redisMode := range []bool{false, true} {
		for _, successOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("redis=%t/successOnly=%t", redisMode, successOnly), func(t *testing.T) {
				total, success := 1, 0
				userID := 920001
				if successOnly {
					total, success, userID = 0, 1, 920002
				}
				var handler gin.HandlerFunc
				if redisMode {
					useRateLimitTestRedis(t)
					handler = redisRateLimitHandler(60, total, success)
				} else {
					handler = memoryRateLimitHandler(60, total, success)
				}
				r := gin.New()
				r.Use(func(c *gin.Context) { c.Set("id", userID) }, handler)
				r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
				send := func() *httptest.ResponseRecorder {
					w := httptest.NewRecorder()
					r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
					return w
				}
				require.Equal(t, 200, send().Code)
				blocked := send()
				require.Equal(t, 429, blocked.Code)
				require.Contains(t, []string{"59", "60"}, blocked.Header().Get("Retry-After"))
				require.Contains(t, blocked.Body.String(), `"new_api_error":true`)
			})
		}
	}
}

func TestModelMemorySuccessLimitDoesNotCountFailures(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("id", 920003) }, memoryRateLimitHandler(60, 0, 1))
	status := 500
	r.GET("/", func(c *gin.Context) { c.Status(status) })
	send := func() int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		return w.Code
	}
	require.Equal(t, 500, send())
	require.Equal(t, 500, send())
	status = 200
	require.Equal(t, 200, send())
	require.Equal(t, 429, send())
}
