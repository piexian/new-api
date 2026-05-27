package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolveModelRequestRateLimitGroupPrefersUsingGroup(t *testing.T) {
	ctx := &gin.Context{}
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "user")
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "token")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "actual")

	require.Equal(t, "actual", resolveModelRequestRateLimitGroup(ctx))
}

func TestResolveModelRequestRateLimitGroupFallbacks(t *testing.T) {
	ctx := &gin.Context{}
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "user")
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "token")
	require.Equal(t, "token", resolveModelRequestRateLimitGroup(ctx))

	ctx = &gin.Context{}
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "user")
	require.Equal(t, "user", resolveModelRequestRateLimitGroup(ctx))
}
