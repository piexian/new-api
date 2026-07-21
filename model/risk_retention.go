package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// 数据保留窗口。
const (
	riskBanLogRetentionSeconds = 90 * 86400 // 封禁日志保留 90 天
	riskStateRetentionSeconds  = 30 * 86400 // 已归零的状态记录保留 30 天
)

// CleanupRiskData 清理过期的风控数据：
//   - 90 天前的封禁日志；
//   - 30 天前且计数已归零的违规/窗口状态记录。
func CleanupRiskData() {
	now := common.GetTimestamp()

	if deleted, err := DeleteRiskBanLogsBefore(now - riskBanLogRetentionSeconds); err != nil {
		common.SysError("risk ban log cleanup failed: " + err.Error())
	} else if deleted > 0 {
		common.SysLog(fmt.Sprintf("risk retention: cleaned %d expired ban logs", deleted))
	}

	stateBefore := now - riskStateRetentionSeconds
	if res := DB.Where("last_offense_at < ? AND window_count = 0", stateBefore).Delete(&ErrorBanIPState{}); res.Error != nil {
		common.SysError("error ban ip state cleanup failed: " + res.Error.Error())
	}
	if res := DB.Where("last_offense_at < ? AND window_count = 0", stateBefore).Delete(&ErrorBanUserState{}); res.Error != nil {
		common.SysError("error ban user state cleanup failed: " + res.Error.Error())
	}
	if res := DB.Where("last_offense_at < ? AND offense_count = 0", stateBefore).Delete(&ProbeIPAbuseState{}); res.Error != nil {
		common.SysError("probe ip abuse cleanup failed: " + res.Error.Error())
	}
	if res := DB.Where("last_offense_at < ? AND offense_count = 0", stateBefore).Delete(&ProbeUserAbuseState{}); res.Error != nil {
		common.SysError("probe user abuse cleanup failed: " + res.Error.Error())
	}
}

// StartRiskDataRetention 周期性清理风控过期数据。
func StartRiskDataRetention(intervalHours int) {
	if intervalHours <= 0 {
		intervalHours = 1
	}
	for {
		time.Sleep(time.Duration(intervalHours) * time.Hour)
		CleanupRiskData()
	}
}
