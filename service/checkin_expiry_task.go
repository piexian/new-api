package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	checkinExpiryTickInterval = 5 * time.Minute
	checkinExpiryBatchSize    = 200
)

var (
	checkinExpiryOnce    sync.Once
	checkinExpiryRunning atomic.Bool
)

// StartCheckinExpiryTask 启动签到额度次日清算任务。
// 每个 tick 找到最早仍有未清算记录的日期并逐批清算，清完推进到下一天；
// 功能未启用时空转，开启后能自动追上积压的历史日期。
func StartCheckinExpiryTask() {
	checkinExpiryOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("checkin expiry task started: tick=%s", checkinExpiryTickInterval))
			ticker := time.NewTicker(checkinExpiryTickInterval)
			defer ticker.Stop()

			runCheckinExpiryOnce()
			for range ticker.C {
				runCheckinExpiryOnce()
			}
		})
	})
}

func runCheckinExpiryOnce() {
	if !checkinExpiryRunning.CompareAndSwap(false, true) {
		return
	}
	defer checkinExpiryRunning.Store(false)

	ctx := context.Background()
	setting := operation_setting.GetCheckinSetting()
	if !setting.IsExpireEnabled() {
		return
	}

	today := time.Now().Format("2006-01-02")
	mode := setting.NormalizedExpireMode()
	for {
		date, err := model.OldestUnsettledCheckinDate(today)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("checkin expiry task query failed: %v", err))
			return
		}
		if date == "" {
			return
		}
		settled, reclaimed, err := model.SettleCheckinDate(date, mode, checkinExpiryBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("checkin expiry settle failed (date=%s): %v", date, err))
			return
		}
		if settled == 0 {
			// 防御：该日期存在但未清到任何记录（并发边界），跳出避免空转
			return
		}
		if common.DebugEnabled {
			logger.LogDebug(ctx, "checkin expiry: date=%s settled=%d reclaimed=%d", date, settled, reclaimed)
		}
	}
}
