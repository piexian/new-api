package setting

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000
var ModelRequestRateLimitGroup = map[string][2]int{}
var ModelRequestRateLimitMutex sync.RWMutex

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(ModelRequestRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	ModelRequestRateLimitGroup = make(map[string][2]int)
	return json.Unmarshal([]byte(jsonStr), &ModelRequestRateLimitGroup)
}

func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	limits, found := ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

// GetGroupRPM 返回分组生效的每分钟完成请求数上限（固定换算为一分钟），0 表示不限速。
// 展示口径与限流执行一致：成功数限制才是每周期请求完成次数的主限制，
// 因此优先展示成功数；成功数未配置(0)时才回退到含失败的总数限制。
// 限流总开关关闭时不限；分组配置存在时以分组为准；分组未配置时回退到全局配置
func GetGroupRPM(group string) float64 {
	if !ModelRequestRateLimitEnabled {
		return 0
	}
	duration := ModelRequestRateLimitDurationMinutes
	if duration <= 0 {
		duration = 1
	}
	if total, success, found := GetGroupRateLimit(group); found {
		if success > 0 {
			return float64(success) / float64(duration)
		}
		if total > 0 {
			return float64(total) / float64(duration)
		}
		return 0
	}
	if ModelRequestRateLimitSuccessCount > 0 {
		return float64(ModelRequestRateLimitSuccessCount) / float64(duration)
	}
	if ModelRequestRateLimitCount > 0 {
		return float64(ModelRequestRateLimitCount) / float64(duration)
	}
	return 0
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	checkModelRequestRateLimitGroup := make(map[string][2]int)
	err := json.Unmarshal([]byte(jsonStr), &checkModelRequestRateLimitGroup)
	if err != nil {
		return err
	}
	for group, limits := range checkModelRequestRateLimitGroup {
		if limits[0] < 0 || limits[1] < 1 {
			return fmt.Errorf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1])
		}
		if limits[0] > math.MaxInt32 || limits[1] > math.MaxInt32 {
			return fmt.Errorf("group %s [%d, %d] has max rate limits value 2147483647", group, limits[0], limits[1])
		}
	}

	return nil
}
