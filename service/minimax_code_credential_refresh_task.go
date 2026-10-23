package service

// MiniMax Code 渠道凭证自动刷新：令牌临近过期时用 refresh_token 续期。

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	minimaxCodeRefreshTickInterval = 10 * time.Minute
	// 访问令牌有效期约 1 小时，阈值需大于巡检间隔且远小于有效期，避免每轮都刷新。
	minimaxCodeRefreshThreshold = 20 * time.Minute
	minimaxCodeRefreshBatchSize = 200
	minimaxCodeRefreshTimeout   = 20 * time.Second
)

var (
	minimaxCodeRefreshOnce    sync.Once
	minimaxCodeRefreshRunning atomic.Bool
)

func StartMiniMaxCodeCredentialAutoRefreshTask() {
	minimaxCodeRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("minimax code credential auto-refresh task started: tick=%s threshold=%s", minimaxCodeRefreshTickInterval, minimaxCodeRefreshThreshold))

			ticker := time.NewTicker(minimaxCodeRefreshTickInterval)
			defer ticker.Stop()

			runMiniMaxCodeCredentialAutoRefreshOnce()
			for range ticker.C {
				runMiniMaxCodeCredentialAutoRefreshOnce()
			}
		})
	})
}

func runMiniMaxCodeCredentialAutoRefreshOnce() {
	if !minimaxCodeRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer minimaxCodeRefreshRunning.Store(false)

	ctx := context.Background()
	now := time.Now()

	var refreshed int
	offset := 0
	for {
		var channels []*model.Channel
		err := model.DB.
			Select("id", "name", "key", "status", "channel_info").
			Where("type = ? AND (status = ? OR status = ?)",
				constant.ChannelTypeMiniMaxCode,
				common.ChannelStatusEnabled,
				common.ChannelStatusAutoDisabled,
			).
			Order("id asc").
			Limit(minimaxCodeRefreshBatchSize).
			Offset(offset).
			Find(&channels).Error
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("minimax code credential auto-refresh: query channels failed: %v", err))
			return
		}
		if len(channels) == 0 {
			break
		}
		offset += minimaxCodeRefreshBatchSize

		for _, ch := range channels {
			if ch == nil || ch.ChannelInfo.IsMultiKey {
				continue
			}
			rawKey := strings.TrimSpace(ch.Key)
			if rawKey == "" {
				continue
			}
			oauthKey, err := ParseMiniMaxCodeOAuthKey(rawKey)
			if err != nil || strings.TrimSpace(oauthKey.RefreshToken) == "" {
				continue
			}
			expiredAt, err := time.Parse(time.RFC3339, strings.TrimSpace(oauthKey.Expired))
			if err == nil && !expiredAt.IsZero() && expiredAt.Sub(now) > minimaxCodeRefreshThreshold {
				continue
			}

			refreshCtx, cancel := context.WithTimeout(ctx, minimaxCodeRefreshTimeout)
			newKey, _, err := RefreshMiniMaxCodeChannelCredential(refreshCtx, ch.Id, MiniMaxCodeCredentialRefreshOptions{ResetCaches: false})
			cancel()
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("minimax code credential auto-refresh: channel_id=%d name=%s refresh failed: %v", ch.Id, ch.Name, err))
				continue
			}
			refreshed++
			logger.LogInfo(ctx, fmt.Sprintf("minimax code credential auto-refresh: channel_id=%d name=%s refreshed, expires_at=%s", ch.Id, ch.Name, newKey.Expired))
		}
	}

	if refreshed > 0 {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.LogWarn(ctx, fmt.Sprintf("minimax code credential auto-refresh: InitChannelCache panic: %v", r))
				}
			}()
			model.InitChannelCache()
		}()
	}

	if common.DebugEnabled {
		logger.LogDebug(ctx, fmt.Sprintf("minimax code credential auto-refresh: refreshed=%d", refreshed))
	}
}
