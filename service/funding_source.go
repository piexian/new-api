package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// ---------------------------------------------------------------------------
// FundingSource — 资金来源接口（钱包 or 订阅）
// ---------------------------------------------------------------------------

// FundingSource 抽象了预扣费的资金来源。
type FundingSource interface {
	// Source 返回资金来源标识："wallet" 或 "subscription"
	Source() string
	// PreConsume 从该资金来源预扣 amount 额度
	PreConsume(amount int) error
	// Settle 根据差额调整资金来源（正数补扣，负数退还）
	Settle(delta int) error
	// Refund 退还所有预扣费
	Refund() error
}

// ---------------------------------------------------------------------------
// WalletFunding — 钱包资金来源实现
// ---------------------------------------------------------------------------

type WalletFunding struct {
	userId   int
	consumed int // 实际预扣的用户额度
}

func (w *WalletFunding) Source() string { return BillingSourceWallet }

func (w *WalletFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	if err := model.DecreaseUserQuota(w.userId, amount, false); err != nil {
		return err
	}
	w.consumed = amount
	return nil
}

func (w *WalletFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return model.DecreaseUserQuota(w.userId, delta, false)
	}
	return model.IncreaseUserQuota(w.userId, -delta, false)
}

func (w *WalletFunding) Refund() error {
	if w.consumed <= 0 {
		return nil
	}
	// IncreaseUserQuota 是 quota += N 的非幂等操作，不能重试，否则会多退额度。
	// 订阅的 RefundSubscriptionPreConsume 有 requestId 幂等保护所以可以重试。
	return model.IncreaseUserQuota(w.userId, w.consumed, false)
}

// ---------------------------------------------------------------------------
// SubscriptionFunding — 订阅资金来源实现（支持跨订阅拆分 + 钱包兜底腿）
// ---------------------------------------------------------------------------

type SubscriptionFunding struct {
	requestId    string
	userId       int
	modelName    string
	requestGroup string
	amount       int64 // 预扣总额度（subConsume）
	requireFull  bool  // subscription_only：不允许钱包兜底
	// legs 为预扣在各订阅上的拆分；legConsumed 与 legs 平行，含结算/Reserve 补扣，作为退款上限
	legs        []model.SubscriptionConsumeLeg
	legConsumed []int64
	// wallet 为订阅未覆盖部分的钱包腿（订阅优先时），nil 表示全额由订阅承担
	wallet *WalletFunding
	// 以下字段在 PreConsume 成功后填充，供 RelayInfo 同步使用（均取首个扣减腿）
	subscriptionId  int
	preConsumed     int64 // 订阅腿预扣合计（不含钱包腿）
	AmountTotal     int64
	AmountUsedAfter int64
	PlanId          int
	PlanTitle       string
	NextResetTime   int64
	ResetPeriod     string
}

func (s *SubscriptionFunding) Source() string { return BillingSourceSubscription }

// HasSubscriptionLegs 返回本次请求是否实际扣到了订阅（false 表示整体走钱包）。
func (s *SubscriptionFunding) HasSubscriptionLegs() bool { return len(s.legs) > 0 }

// TotalLegConsumed 返回订阅腿累计扣减（预扣 + 补扣），用于日志与退款核算。
func (s *SubscriptionFunding) TotalLegConsumed() int64 {
	var total int64
	for _, v := range s.legConsumed {
		total += v
	}
	return total
}

func (s *SubscriptionFunding) PreConsume(_ int) error {
	// amount 参数被忽略，使用内部 s.amount（已在构造时根据 preConsumedQuota 计算）
	res, err := model.PreConsumeUserSubscriptionsSplit(s.requestId, s.userId, s.modelName, s.requestGroup, s.amount, s.requireFull)
	if err != nil {
		return err
	}
	s.legs = res.Legs
	s.legConsumed = make([]int64, len(res.Legs))
	for i, leg := range res.Legs {
		s.legConsumed[i] = leg.Amount
	}
	s.subscriptionId = res.UserSubscriptionId
	s.preConsumed = res.PreConsumed
	s.AmountTotal = res.AmountTotal
	s.AmountUsedAfter = res.AmountUsedAfter
	s.NextResetTime = res.NextResetTime
	s.ResetPeriod = res.QuotaResetPeriod
	// 获取订阅计划信息
	if res.UserSubscriptionId > 0 {
		if planInfo, err := model.GetSubscriptionPlanInfoByUserSubscriptionId(res.UserSubscriptionId); err == nil && planInfo != nil {
			s.PlanId = planInfo.PlanId
			s.PlanTitle = planInfo.PlanTitle
		}
	}
	if res.Remainder > 0 {
		if err := s.preConsumeWallet(res.Remainder); err != nil {
			// 钱包腿失败时回滚已预扣的订阅腿，避免悬空扣减
			s.rollbackSubscriptionLegs()
			return err
		}
	}
	return nil
}

// preConsumeWallet 预扣钱包腿。先做余额预检，避免 DecreaseUserQuota 静默扣成负数。
func (s *SubscriptionFunding) preConsumeWallet(amount int64) error {
	quota, err := model.GetUserQuota(s.userId, false)
	if err != nil {
		return err
	}
	if quota < int(amount) {
		return fmt.Errorf("wallet quota insufficient, need=%d remain=%d", amount, quota)
	}
	w := &WalletFunding{userId: s.userId}
	if err := w.PreConsume(int(amount)); err != nil {
		return err
	}
	s.wallet = w
	return nil
}

func (s *SubscriptionFunding) rollbackSubscriptionLegs() {
	if s.preConsumed <= 0 {
		return
	}
	if err := model.RefundSubscriptionPreConsume(s.requestId); err != nil {
		common.SysLog("error rolling back subscription legs after wallet pre-consume failure: " + err.Error())
	}
	s.preConsumed = 0
	for i := range s.legConsumed {
		s.legConsumed[i] = 0
	}
}

// topUp 追加扣减：先按腿顺序补订阅（每条扣到剩余容量为止），剩余落钱包。
func (s *SubscriptionFunding) topUp(delta int64) error {
	remaining := delta
	for i, leg := range s.legs {
		if remaining <= 0 {
			break
		}
		charged, err := model.PostConsumeUserSubscriptionDeltaUpTo(leg.UserSubscriptionId, remaining)
		if err != nil {
			return err
		}
		if charged > 0 {
			s.legConsumed[i] += charged
			remaining -= charged
		}
	}
	if remaining > 0 {
		if s.wallet == nil {
			s.wallet = &WalletFunding{userId: s.userId}
		}
		if err := model.DecreaseUserQuota(s.userId, int(remaining), false); err != nil {
			return err
		}
		s.wallet.consumed += int(remaining)
	}
	return nil
}

// refundDelta 退还扣减：与扣减顺序相反，先退钱包腿再逆序退订阅腿。
func (s *SubscriptionFunding) refundDelta(delta int64) error {
	remaining := delta
	if s.wallet != nil && s.wallet.consumed > 0 {
		w := remaining
		if int64(s.wallet.consumed) < w {
			w = int64(s.wallet.consumed)
		}
		if err := model.IncreaseUserQuota(s.userId, int(w), false); err != nil {
			return err
		}
		s.wallet.consumed -= int(w)
		remaining -= w
	}
	for i := len(s.legs) - 1; i >= 0 && remaining > 0; i-- {
		r := remaining
		if s.legConsumed[i] < r {
			r = s.legConsumed[i]
		}
		if r <= 0 {
			continue
		}
		if err := model.PostConsumeUserSubscriptionDelta(s.legs[i].UserSubscriptionId, -r); err != nil {
			return err
		}
		s.legConsumed[i] -= r
		remaining -= r
	}
	return nil
}

func (s *SubscriptionFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return s.topUp(int64(delta))
	}
	return s.refundDelta(int64(-delta))
}

func (s *SubscriptionFunding) Refund() error {
	var firstErr error
	if s.wallet != nil {
		if err := s.wallet.Refund(); err != nil {
			common.SysLog("error refunding subscription wallet leg: " + err.Error())
			firstErr = err
		}
	}
	if s.preConsumed > 0 {
		if err := refundWithRetry(func() error {
			return model.RefundSubscriptionPreConsume(s.requestId)
		}); err != nil {
			common.SysLog("error refunding subscription pre-consume: " + err.Error())
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	// 退回预扣之外的订阅补扣（Reserve 阶段产生）
	for i, leg := range s.legs {
		extra := s.legConsumed[i] - leg.Amount
		if extra <= 0 {
			continue
		}
		if err := model.PostConsumeUserSubscriptionDelta(leg.UserSubscriptionId, -extra); err != nil {
			common.SysLog("error refunding subscription reserve top-up: " + err.Error())
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// refundWithRetry 尝试多次执行退款操作以提高成功率，只能用于基于事务的退款函数！！！！！！
// try to refund with retries, only for refund functions based on transactions!!!
func refundWithRetry(fn func() error) error {
	if fn == nil {
		return nil
	}
	const maxAttempts = 3
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < maxAttempts-1 {
			time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
		}
	}
	return lastErr
}
