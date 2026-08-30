package setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func saveRateLimitSettings(t *testing.T) {
	t.Helper()
	savedEnabled := ModelRequestRateLimitEnabled
	savedDuration := ModelRequestRateLimitDurationMinutes
	savedCount := ModelRequestRateLimitCount
	savedGroup := ModelRequestRateLimitGroup
	t.Cleanup(func() {
		ModelRequestRateLimitEnabled = savedEnabled
		ModelRequestRateLimitDurationMinutes = savedDuration
		ModelRequestRateLimitCount = savedCount
		ModelRequestRateLimitGroup = savedGroup
	})
}

func TestGetGroupRPM(t *testing.T) {
	saveRateLimitSettings(t)

	ModelRequestRateLimitEnabled = true
	ModelRequestRateLimitDurationMinutes = 5
	ModelRequestRateLimitCount = 300
	require.NoError(t, UpdateModelRequestRateLimitGroupByJSONString(`{"vip":[600,100],"free":[0,10]}`))

	// 分组配置按周期换算为一分钟：600/5min = 120 RPM
	require.Equal(t, 120.0, GetGroupRPM("vip"))
	// 分组显式配置 0 = 不限制
	require.Equal(t, 0.0, GetGroupRPM("free"))
	// 未配置分组回退全局：300/5min = 60 RPM
	require.Equal(t, 60.0, GetGroupRPM("default"))

	// 全局限流关闭时不限制
	ModelRequestRateLimitEnabled = false
	require.Equal(t, 0.0, GetGroupRPM("vip"))
}

func TestGetGroupRPMDurationDefaultsToOne(t *testing.T) {
	saveRateLimitSettings(t)

	ModelRequestRateLimitEnabled = true
	ModelRequestRateLimitDurationMinutes = 0
	ModelRequestRateLimitCount = 100
	require.NoError(t, UpdateModelRequestRateLimitGroupByJSONString(`{}`))

	require.Equal(t, 100.0, GetGroupRPM("default"))
}
