package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolveGroupConcurrencyGroupPrefersAutoGroup(t *testing.T) {
	ctx := &gin.Context{}
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "user")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "actual")
	common.SetContextKey(ctx, constant.ContextKeyAutoGroup, "autoResolved")

	require.Equal(t, "autoResolved", resolveGroupConcurrencyGroup(ctx))
}

func TestResolveGroupConcurrencyGroupFallbacks(t *testing.T) {
	ctx := &gin.Context{}
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "user")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "actual")
	require.Equal(t, "actual", resolveGroupConcurrencyGroup(ctx))

	ctx = &gin.Context{}
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "user")
	require.Equal(t, "user", resolveGroupConcurrencyGroup(ctx))
}

func TestGroupConcurrencyAcquireMemory(t *testing.T) {
	const userId = 1
	const group = "vip"
	const limit = 2

	groupConcurrencyMemory.Lock()
	savedCounts := groupConcurrencyMemory.counts
	groupConcurrencyMemory.counts = make(map[string]int)
	groupConcurrencyMemory.Unlock()
	defer func() {
		groupConcurrencyMemory.Lock()
		groupConcurrencyMemory.counts = savedCounts
		groupConcurrencyMemory.Unlock()
	}()

	release1, allowed := groupConcurrencyAcquireMemory(userId, group, limit)
	require.True(t, allowed)
	require.NotNil(t, release1)

	release2, allowed := groupConcurrencyAcquireMemory(userId, group, limit)
	require.True(t, allowed)

	_, allowed = groupConcurrencyAcquireMemory(userId, group, limit)
	require.False(t, allowed)

	release2()
	release3, allowed := groupConcurrencyAcquireMemory(userId, group, limit)
	require.True(t, allowed)

	release1()
	release3()

	groupConcurrencyMemory.Lock()
	_, exists := groupConcurrencyMemory.counts["1:vip"]
	groupConcurrencyMemory.Unlock()
	require.False(t, exists)
}

func saveConcurrencySetting(t *testing.T) {
	t.Helper()
	common.RedisEnabled = false

	setting.UserGroupConcurrencyLimitMutex.Lock()
	savedLimit := setting.UserGroupConcurrencyLimit
	setting.UserGroupConcurrencyLimitMutex.Unlock()
	t.Cleanup(func() {
		setting.UserGroupConcurrencyLimitMutex.Lock()
		setting.UserGroupConcurrencyLimit = savedLimit
		setting.UserGroupConcurrencyLimitMutex.Unlock()
	})

	groupConcurrencyMemory.Lock()
	savedCounts := groupConcurrencyMemory.counts
	groupConcurrencyMemory.counts = make(map[string]int)
	groupConcurrencyMemory.Unlock()
	t.Cleanup(func() {
		groupConcurrencyMemory.Lock()
		groupConcurrencyMemory.counts = savedCounts
		groupConcurrencyMemory.Unlock()
	})
}

func buildGroupConcurrencyRouter(group string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("id", 42)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		c.Next()
	})
	r.POST("/v1/chat/completions", GroupConcurrencyLimit(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func TestGroupConcurrencyLimitMiddleware(t *testing.T) {
	saveConcurrencySetting(t)
	setting.UserGroupConcurrencyLimitMutex.Lock()
	setting.UserGroupConcurrencyLimit = map[string]int{"vip": 1}
	setting.UserGroupConcurrencyLimitMutex.Unlock()

	r := buildGroupConcurrencyRouter("vip")

	// 未占用并发额度时放行
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	require.Equal(t, http.StatusOK, w.Code)

	// 并发额度被占用时拒绝
	release, allowed := groupConcurrencyAcquireMemory(42, "vip", 1)
	require.True(t, allowed)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	release()

	// 释放后恢复放行
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestGroupConcurrencyLimitUnlimited(t *testing.T) {
	saveConcurrencySetting(t)
	setting.UserGroupConcurrencyLimitMutex.Lock()
	setting.UserGroupConcurrencyLimit = map[string]int{}
	setting.UserGroupConcurrencyLimitMutex.Unlock()

	r := buildGroupConcurrencyRouter("vip")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	require.Equal(t, http.StatusOK, w.Code)

	groupConcurrencyMemory.Lock()
	require.Empty(t, groupConcurrencyMemory.counts)
	groupConcurrencyMemory.Unlock()
}
