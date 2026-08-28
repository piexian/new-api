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
	checkinExpiryBatchSize         = 200
	checkinExpiryMaxBatchesPerTick = 100 // 每 tick 最多 100 批（2 万条），防历史积压拖垮单 tick
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
	// 单 tick 批次上限：历史积压大的站点首次启用时，避免在一个 tick 内同步跑数千批
	// 卡住协程；剩余积压顺延到下个 tick 继续。
	for batch := 1; ; batch++ {
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
		if batch >= checkinExpiryMaxBatchesPerTick {
			logger.LogInfo(ctx, fmt.Sprintf(
				"checkin expiry: per-tick batch cap reached (%d batches), continue next tick; last date=%s batch settled=%d reclaimed=%d",
				batch, date, settled, reclaimed))
			return
		}
	}
}
