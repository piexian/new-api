package setting

import (
	"fmt"
	"math"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// UserGroupConcurrencyLimit 用户分组并发限制：分组 -> 该分组下单个账号（用户）允许的最大同时进行中请求数
// 0 或未配置的分组表示不限制
var UserGroupConcurrencyLimit = map[string]int{}

var UserGroupConcurrencyLimitMutex sync.RWMutex

func UserGroupConcurrencyLimit2JSONString() string {
	UserGroupConcurrencyLimitMutex.RLock()
	defer UserGroupConcurrencyLimitMutex.RUnlock()

	jsonBytes, err := common.Marshal(UserGroupConcurrencyLimit)
	if err != nil {
		common.SysLog("error marshalling user group concurrency limit: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateUserGroupConcurrencyLimitByJSONString(jsonStr string) error {
	UserGroupConcurrencyLimitMutex.Lock()
	defer UserGroupConcurrencyLimitMutex.Unlock()

	UserGroupConcurrencyLimit = make(map[string]int)
	return common.Unmarshal([]byte(jsonStr), &UserGroupConcurrencyLimit)
}

func GetUserGroupConcurrencyLimit(group string) int {
	UserGroupConcurrencyLimitMutex.RLock()
	defer UserGroupConcurrencyLimitMutex.RUnlock()

	if UserGroupConcurrencyLimit == nil {
		return 0
	}
	return UserGroupConcurrencyLimit[group]
}

func CheckUserGroupConcurrencyLimit(jsonStr string) error {
	checkUserGroupConcurrencyLimit := make(map[string]int)
	err := common.Unmarshal([]byte(jsonStr), &checkUserGroupConcurrencyLimit)
	if err != nil {
		return err
	}
	for group, limit := range checkUserGroupConcurrencyLimit {
		if limit < 0 {
			return fmt.Errorf("group %s has negative concurrency limit: %d", group, limit)
		}
		if limit > math.MaxInt32 {
			return fmt.Errorf("group %s concurrency limit %d exceeds max value 2147483647", group, limit)
		}
	}
	return nil
}
