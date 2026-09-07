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
	savedSuccessCount := ModelRequestRateLimitSuccessCount
	savedGroup := ModelRequestRateLimitGroup
	t.Cleanup(func() {
		ModelRequestRateLimitEnabled = savedEnabled
		ModelRequestRateLimitDurationMinutes = savedDuration
		ModelRequestRateLimitCount = savedCount
		ModelRequestRateLimitSuccessCount = savedSuccessCount
		ModelRequestRateLimitGroup = savedGroup
	})
}

func TestGetGroupRPM(t *testing.T) {
	saveRateLimitSettings(t)

	ModelRequestRateLimitEnabled = true
	ModelRequestRateLimitDurationMinutes = 5
	ModelRequestRateLimitCount = 300
	ModelRequestRateLimitSuccessCount = 1000
	require.NoError(t, UpdateModelRequestRateLimitGroupByJSONString(`{"vip":[600,100],"free":[0,10],"max":[0,0]}`))

	// 优先展示成功数限制：100/5min = 20 RPM，而非含失败的 600/5min
	require.Equal(t, 20.0, GetGroupRPM("vip"))
	// 成功数 10/5min = 2 RPM，总数 0（不限）不影响展示
	require.Equal(t, 2.0, GetGroupRPM("free"))
	// 成功数与总数都未配置 = 不限制
	require.Equal(t, 0.0, GetGroupRPM("max"))
	// 未配置分组回退全局成功数限制：1000/5min = 200 RPM
	require.Equal(t, 200.0, GetGroupRPM("default"))

	// 全局成功数未配置时回退全局总数：300/5min = 60 RPM
	ModelRequestRateLimitSuccessCount = 0
	require.Equal(t, 60.0, GetGroupRPM("default"))

	// 全局限流关闭时不限制
	ModelRequestRateLimitEnabled = false
	require.Equal(t, 0.0, GetGroupRPM("vip"))
}

func TestGetGroupRPMDurationDefaultsToOne(t *testing.T) {
	saveRateLimitSettings(t)

	ModelRequestRateLimitEnabled = true
	ModelRequestRateLimitDurationMinutes = 0
	ModelRequestRateLimitSuccessCount = 0
	ModelRequestRateLimitCount = 100
	require.NoError(t, UpdateModelRequestRateLimitGroupByJSONString(`{}`))

	// duration 非法时按 1 分钟换算
	require.Equal(t, 100.0, GetGroupRPM("default"))
}
