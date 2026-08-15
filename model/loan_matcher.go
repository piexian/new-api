package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// ===== Task 7: 撮合引擎（spec §6） =====

// FundingPlan 单笔资金投放计划：撮合引擎的输出，Task 8 放款事务按计划执行
// （锁 offer 行二次校验 amount_available 后扣减）。SourceType 取
// LoanFundingPool / LoanFundingOrder / LoanFundingAi；platform 由调用方兜底补充。
type FundingPlan struct {
	OfferId    int     `json:"offer_id"`
	LenderId   int     `json:"lender_id"`
	SourceType string  `json:"source_type"`
	Amount     int64   `json:"amount"`
	Rate       float64 `json:"rate"`
}

// MatchLoanFundings 撮合引擎：为一次借款分配资金投放计划。
// 只读 + 内存计算，**不改库**；必须在调用方事务内调用（caller 已锁 borrower 行），
// 内部经 lockForUpdate 锁定 offer 行，保证 Task 8 放款时读到与撮合一致的行。
//
// 分配顺序（spec §6 两阶段 + AI 参与）：
//  1. 定向挂单（intendedOrderId > 0）：锁该 offer，校验 order 模式、active、
//     lender != borrower、信用分门槛、amount_available > 0 后按
//     min(剩余, available, cap>0?cap:∞) 吃量，利率 = rate_fixed；
//     任一校验失败仅跳过并记录 drop 原因，不使整笔借款失败（过期意向不阻断借款）；
//  2. 统一市场：active 的 pool/order offer 按 rate_fixed 升序、id 升序（确定性
//     平局）吃量，跳过 lender==borrower、信用分不足、amount_available==0
//     及已在定向阶段消费过的 offer（同一 offer 不重复吃量，防止超 available 分配）；
//  3. AI 出资方案（service 层事务外预先产出）：逐条校验 offer 存在且 active 且
//     ai 模式、rate ∈ [rate_min, rate_max]，越界剔除并记录 drop 原因；并按 offer
//     累计已投量动态校验 amount ≤ min(available, cap) - 累计（防止同一 offer 多条
//     计划联合超额度）；有效条目在固定利率计划之后按给定顺序吃量，剩余不足时
//     截断到最后剩余（AI 只参与剩余部分）。
//
// 计划总条数超过 MaxFundingsPerBorrow 时截断：保留定向单与低利率计划
// （构造顺序即定向 → 利率升序 → AI 给定顺序，直接保留前缀）；被截断的金额
// 不再重新分配，缺额由调用方补 platform funding。
//
// 返回：plans（来源不足时可能不满 amount）、dropReasons（审计日志用）、
// error 仅表示数据库故障（offer 行读取失败），业务性跳过一律不报错。
func MatchLoanFundings(tx *gorm.DB, borrowerId int, creditScore int, amount int64, intendedOrderId int, aiPriced []FundingPlan) ([]FundingPlan, []string, error) {
	remaining := amount
	plans := make([]FundingPlan, 0, 8)
	dropReasons := make([]string, 0, 4)
	consumedOfferIds := make(map[int]bool, 4) // 已消费 offer id，统一市场不再重复吃量

	// —— ① 定向挂单 ——
	if intendedOrderId > 0 {
		var offer TokenLoanOffer
		err := lockForUpdate(tx).Where("id = ?", intendedOrderId).First(&offer).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				dropReasons = append(dropReasons, fmt.Sprintf("定向挂单 offer %d 不存在", intendedOrderId))
			} else {
				return nil, dropReasons, err
			}
		} else if take, reason := intendedOrderTake(&offer, borrowerId, creditScore, remaining); reason != "" {
			dropReasons = append(dropReasons, reason)
		} else {
			plans = append(plans, FundingPlan{
				OfferId:    offer.Id,
				LenderId:   offer.LenderId,
				SourceType: LoanFundingOrder,
				Amount:     take,
				Rate:       offer.RateFixed,
			})
			remaining -= take
			consumedOfferIds[offer.Id] = true
		}
	}

	// —— ② 统一市场（pool/order 固定利率）——
	if remaining > 0 {
		var offers []TokenLoanOffer
		if err := lockForUpdate(tx).
			Where("status = ? AND mode IN ? AND amount_available > 0", LoanOfferStatusActive, []string{LoanOfferModePool, LoanOfferModeOrder}).
			Order("rate_fixed ASC").Order("id ASC").Find(&offers).Error; err != nil {
			return nil, dropReasons, err
		}
		for i := range offers {
			if remaining <= 0 {
				break
			}
			offer := &offers[i]
			if consumedOfferIds[offer.Id] {
				continue // 定向单已消费，不重复吃量
			}
			if offer.LenderId == borrowerId {
				continue // 放贷人不得向本人出资
			}
			if offer.MinCreditScore > -50 && creditScore < offer.MinCreditScore {
				continue // 信用分门槛过滤（-50 = 不限）
			}
			take := min(remaining, offer.AmountAvailable, perLoanLimit(offer.PerLoanCap))
			if take <= 0 {
				continue // 防御：available/cap 为 0 时无额度可吃
			}
			plans = append(plans, FundingPlan{
				OfferId:    offer.Id,
				LenderId:   offer.LenderId,
				SourceType: offer.Mode, // pool / order，与 offer 模式一致
				Amount:     take,
				Rate:       offer.RateFixed,
			})
			remaining -= take
		}
	}

	// —— ③ AI 出资方案（固定利率之后参与，只覆盖剩余部分）——
	aiConsumed := make(map[int]int64, 4) // 按 offer 累计已投量，防多条目联合超 available/cap
	for i := range aiPriced {
		if remaining <= 0 {
			break
		}
		entry := &aiPriced[i]
		var offer TokenLoanOffer
		if err := lockForUpdate(tx).Where("id = ?", entry.OfferId).First(&offer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				dropReasons = append(dropReasons, fmt.Sprintf("AI 出资 offer %d 不存在", entry.OfferId))
			} else {
				return nil, dropReasons, err
			}
			continue
		}
		if reason := aiEntryValid(entry, &offer); reason != "" {
			dropReasons = append(dropReasons, reason)
			continue
		}
		// 按 offer 累计校验：同一 offer 的多条 AI 计划联合不得超过 min(available, cap)，
		// 否则 Task 8 放款时二次校验必然失败（available 不足）或超单笔上限
		consumed := aiConsumed[offer.Id]
		allowance := min(offer.AmountAvailable, perLoanLimit(offer.PerLoanCap)) - consumed
		if allowance <= 0 || entry.Amount > allowance {
			dropReasons = append(dropReasons, fmt.Sprintf("AI 出资 offer %d 金额 %d 超过剩余额度 %d（累计已投 %d）",
				offer.Id, entry.Amount, allowance, consumed))
			continue
		}
		take := entry.Amount
		if take > remaining {
			take = remaining // 剩余不足时截断到最后剩余，不超借
		}
		aiConsumed[offer.Id] = consumed + take
		plans = append(plans, FundingPlan{
			OfferId:    offer.Id,
			LenderId:   offer.LenderId,
			SourceType: LoanFundingAi,
			Amount:     take,
			Rate:       entry.Rate,
		})
		remaining -= take
	}

	// —— 截断：总条数超限时保留前缀（定向单 → 利率升序 → AI 给定顺序）——
	// 被截断条目的金额不再重新分配，缺额由调用方补 platform funding。
	if max := operation_setting.GetLoanSetting().MaxFundingsPerBorrow; max > 0 && len(plans) > max {
		plans = plans[:max]
	}

	return plans, dropReasons, nil
}

// intendedOrderTake 定向单校验：返回可吃量；reason 非空表示校验失败原因
// （跳过不报错，过期意向不阻断借款）。
func intendedOrderTake(offer *TokenLoanOffer, borrowerId int, creditScore int, remaining int64) (int64, string) {
	if offer.Mode != LoanOfferModeOrder {
		return 0, fmt.Sprintf("定向挂单 offer %d 模式 %s，要求 order", offer.Id, offer.Mode)
	}
	if offer.Status != LoanOfferStatusActive {
		return 0, fmt.Sprintf("定向挂单 offer %d 状态 %s，要求 active", offer.Id, offer.Status)
	}
	if offer.LenderId == borrowerId {
		return 0, fmt.Sprintf("定向挂单 offer %d 属于借款人本人", offer.Id)
	}
	if offer.MinCreditScore > -50 && creditScore < offer.MinCreditScore {
		return 0, fmt.Sprintf("定向挂单 offer %d 信用分门槛 %d 高于借款人 %d", offer.Id, offer.MinCreditScore, creditScore)
	}
	if offer.AmountAvailable <= 0 {
		return 0, fmt.Sprintf("定向挂单 offer %d 无可撮合额度", offer.Id)
	}
	return min(remaining, offer.AmountAvailable, perLoanLimit(offer.PerLoanCap)), ""
}

// aiEntryValid AI 出资条目静态校验：通过返回 ""，否则返回 drop 原因。
// 校验范围：offer 存在且 active 且 ai 模式、rate ∈ [rate_min, rate_max]、
// amount > 0；额度上限（amount ≤ min(available, cap)）不在此校验——
// 由撮合循环按 offer 累计已投量动态校验（见 MatchLoanFundings ③）。
func aiEntryValid(entry *FundingPlan, offer *TokenLoanOffer) string {
	if offer.Mode != LoanOfferModeAi {
		return fmt.Sprintf("AI 出资 offer %d 模式 %s，要求 ai", offer.Id, offer.Mode)
	}
	if offer.Status != LoanOfferStatusActive {
		return fmt.Sprintf("AI 出资 offer %d 状态 %s，要求 active", offer.Id, offer.Status)
	}
	if entry.Amount <= 0 {
		return fmt.Sprintf("AI 出资 offer %d 金额 %d 非正数", offer.Id, entry.Amount)
	}
	if entry.Rate < offer.RateMin || entry.Rate > offer.RateMax {
		return fmt.Sprintf("AI 出资 offer %d 利率 %v 越界 [%v, %v]", offer.Id, entry.Rate, offer.RateMin, offer.RateMax)
	}
	return ""
}

// perLoanLimit 单笔上限归一化：per_loan_cap <= 0 表示不限（pool/order 跟随全局
// 缺省 PerLoanCapDefault 可能为 0 = 不限；ai 创建时强制 > 0，此处仅防御）。
func perLoanLimit(perLoanCap int64) int64 {
	if perLoanCap <= 0 {
		return math.MaxInt64
	}
	return perLoanCap
}
