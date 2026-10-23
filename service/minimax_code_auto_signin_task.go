package service

// MiniMax Code 渠道自动签到：渠道级开关 auto_signin_enabled 开启后每日领取一次积分签到。
// 上游签到按天幂等（重复 claim 返回当日同一 claim_id），本地按天去重仅是为了减少调用。

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
	minimaxCodeAutoSigninTickInterval = 30 * time.Minute
	minimaxCodeAutoSigninBatchSize    = 200
	minimaxCodeAutoSigninTimeout      = 15 * time.Second
)

var (
	minimaxCodeAutoSigninOnce    sync.Once
	minimaxCodeAutoSigninRunning atomic.Bool
	// channel id -> 最近一次签到日期（本地日期），跨天自动失效。
	minimaxCodeAutoSigninDays sync.Map
)

func StartMiniMaxCodeAutoSigninTask() {
	minimaxCodeAutoSigninOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("minimax code auto-signin task started: tick=%s", minimaxCodeAutoSigninTickInterval))

			ticker := time.NewTicker(minimaxCodeAutoSigninTickInterval)
			defer ticker.Stop()

			runMiniMaxCodeAutoSigninOnce()
			for range ticker.C {
				runMiniMaxCodeAutoSigninOnce()
			}
		})
	})
}

func runMiniMaxCodeAutoSigninOnce() {
	if !minimaxCodeAutoSigninRunning.CompareAndSwap(false, true) {
		return
	}
	defer minimaxCodeAutoSigninRunning.Store(false)

	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	var claimed int
	offset := 0
	for {
		var channels []*model.Channel
		err := model.DB.
			Select("id", "name", "key", "status", "setting", "channel_info").
			Where("type = ? AND (status = ? OR status = ?)",
				constant.ChannelTypeMiniMaxCode,
				common.ChannelStatusEnabled,
				common.ChannelStatusAutoDisabled,
			).
			Order("id asc").
			Limit(minimaxCodeAutoSigninBatchSize).
			Offset(offset).
			Find(&channels).Error
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("minimax code auto-signin: query channels failed: %v", err))
			return
		}
		if len(channels) == 0 {
			break
		}
		offset += minimaxCodeAutoSigninBatchSize

		for _, ch := range channels {
			if ch == nil || ch.ChannelInfo.IsMultiKey {
				continue
			}
			if !ch.GetSetting().AutoSigninEnabled {
				continue
			}
			if last, ok := minimaxCodeAutoSigninDays.Load(ch.Id); ok {
				if lastDay, _ := last.(string); lastDay == today {
					continue
				}
			}
			oauthKey, err := ParseMiniMaxCodeOAuthKey(ch.Key)
			if err != nil {
				continue
			}

			dayNo, points, err := claimMiniMaxCodeSignin(ctx, ch, oauthKey)
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("minimax code auto-signin: channel_id=%d name=%s claim failed: %v", ch.Id, ch.Name, err))
				continue
			}
			minimaxCodeAutoSigninDays.Store(ch.Id, today)
			claimed++
			logger.LogInfo(ctx, fmt.Sprintf("minimax code auto-signin: channel_id=%d name=%s claimed day_no=%s points=%s", ch.Id, ch.Name, dayNo, points))
		}
	}

	if claimed > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("minimax code auto-signin: claimed=%d", claimed))
	}
}

// claimMiniMaxCodeSignin 领取当日签到；访问令牌过期时刷新凭证重试一次。
func claimMiniMaxCodeSignin(ctx context.Context, ch *model.Channel, oauthKey *MiniMaxCodeOAuthKey) (dayNo string, points string, err error) {
	claimOnce := func(key *MiniMaxCodeOAuthKey) (string, string, error) {
		client, err := NewMiniMaxCodeAccountClient(ch.GetSetting().Proxy, key.Region, key.AccessToken, key.UserID)
		if err != nil {
			return "", "", err
		}
		claimCtx, cancel := context.WithTimeout(ctx, minimaxCodeAutoSigninTimeout)
		defer cancel()
		var result map[string]any
		if err := client.SigninClaim(claimCtx, &result); err != nil {
			return "", "", err
		}
		dayNo, points := miniMaxCodeSigninClaimSummary(result)
		return dayNo, points, nil
	}

	dayNo, points, err = claimOnce(oauthKey)
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		return dayNo, points, err
	}

	newKey, _, refreshErr := RefreshMiniMaxCodeChannelCredential(ctx, ch.Id, MiniMaxCodeCredentialRefreshOptions{ResetCaches: false})
	if refreshErr != nil {
		return "", "", fmt.Errorf("refresh credential: %w", refreshErr)
	}
	return claimOnce(newKey)
}

// miniMaxCodeSigninClaimSummary 从签到响应提取 day_no 与 points 用于日志（上游返回数字）。
func miniMaxCodeSigninClaimSummary(result map[string]any) (string, string) {
	return anyToString(result["day_no"]), miniMaxCodeAmountString(result["points"])
}
