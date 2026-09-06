package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsFallbackToolCallID(t *testing.T) {
	t.Parallel()

	valid := NewFallbackToolCallID()
	require.True(t, IsFallbackToolCallID(valid))
	require.True(t, IsFallbackToolCallID("call_0123456789abcdef0123456789abcdef"))

	// 非 32 位小写十六进制的 id 都不算伪造,应原样回传上游
	require.False(t, IsFallbackToolCallID(""))
	require.False(t, IsFallbackToolCallID("call_"))
	require.False(t, IsFallbackToolCallID("call_0123456789abcdef0123456789abcde"))   // 31 位
	require.False(t, IsFallbackToolCallID("call_0123456789abcdef0123456789abcdef0")) // 33 位
	require.False(t, IsFallbackToolCallID("call_0123456789ABCDEF0123456789abcdef"))  // 含大写
	require.False(t, IsFallbackToolCallID("call_Z123456789abcdef0123456789abcdef"))  // 非十六进制
	require.False(t, IsFallbackToolCallID("call_zapier_1234"))                       // OpenAI 风格真实 id
	require.False(t, IsFallbackToolCallID("fc_0123456789abcdef0123456789abcdef"))    // 前缀不同
}

func TestNewFallbackToolCallIDShape(t *testing.T) {
	t.Parallel()

	id := NewFallbackToolCallID()
	require.True(t, IsFallbackToolCallID(id))
	require.NotEqual(t, id, NewFallbackToolCallID())
}
