package model

import (
	"math"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// settleFunding 单条 funding 惰性结算（spec §5，镜像 settle()），仅修改内存中的 f，
// 落盘由调用方负责。按 base 日到 now 所在本地日的天数日复利推进 DebtQuota，
// LastSettledDay 就地推进。
//
// 利率与起算日：
//   - P2P（pool/ai/order）永远用 f.Rate，不受账户 CustomDailyRate/InterestFreeUntil 穿透
//   - platform 用账户有效利率 effectiveRate(acc)，且起算日上提 acc.InterestFreeUntil 宽限
//     （宽限期内 days<=0 不计息，LastSettledDay 照常推进，防止宽限结束后一次性补算）
//
// 逾期纯计算（P0-1，不翻转状态、不写 PenaltyStartedDay——那是 Task 11 的事）：
// loanDay(now) > f.DueDay 且 RepayPlan == full 时，罚息只作用于 due_day 之后的区间：
// [base, due_day] 按 base 利率复利、[due_day, today] 按 ×OverduePenaltyMultiplier 罚息利率复利
// （base 可能被 platform 宽限上提到 due_day 之后，此时整段均为罚息段，不会双重计息）；
// no_penalty 逾期仍按 base 利率；interest_freeze/principal_only 债务恒不增长。
// acc 仅为 platform 分支提供利率/宽限输入，P2P 分支可不传（nil 安全）。
func settleFunding(f *TokenLoanFunding, acc *TokenLoanAccount, now time.Time) {
	today := loanDay(now)
	base := f.LastSettledDay
	rate := f.Rate
	if f.SourceType == LoanFundingPlatform && acc != nil {
		rate = effectiveRate(acc)
		if acc.InterestFreeUntil > base {
			base = acc.InterestFreeUntil
		}
	}
	if days := today - base; days > 0 && f.DebtQuota > 0 {
		switch f.RepayPlan {
		case LoanRepayFull:
			if today > f.DueDay {
				// 分段复利：先按 base 利率计 [base, due_day)，再对结转后的
				// DebtQuota 按罚息利率计 [due_day, today)。每段各做一次
				// compoundRound（逐段取整符合既有整数 quota 取整风格）。
				penaltyRate := rate * operation_setting.GetLoanSetting().OverduePenaltyMultiplier
				// 外层已保证 today > base 且 today > DueDay，故 seg1 = DueDay - base
				if seg1 := f.DueDay - base; seg1 > 0 {
					f.DebtQuota = compoundRound(f.DebtQuota, rate, seg1)
				}
				if seg2 := today - max(base, f.DueDay); seg2 > 0 {
					f.DebtQuota = compoundRound(f.DebtQuota, penaltyRate, seg2)
				}
			} else {
				f.DebtQuota = compoundRound(f.DebtQuota, rate, days)
			}
		case LoanRepayNoPenalty:
			// 逾期不乘罚息倍率，按 base 利率计息
			f.DebtQuota = compoundRound(f.DebtQuota, rate, days)
		default: // interest_freeze / principal_only：债务冻结不增长
		}
	}
	f.LastSettledDay = today
}

// compoundRound 把 quota 按 rate 日复利 days 天并四舍五入到整数 quota。
// math.Round 远离零取整；真值 >= principal 且 principal 为整数，
// 故 debt >= principal 不变式恒成立（镜像 settle()）。
func compoundRound(quota int64, rate float64, days int) int64 {
	return int64(math.Round(float64(quota) * math.Pow(1+rate, float64(days))))
}

// ProjectFundingDebt 只读投影：返回 now 时刻该 funding 的债务总额，不修改 f、不落盘
func ProjectFundingDebt(f *TokenLoanFunding, acc *TokenLoanAccount, now time.Time) int64 {
	projected := *f
	settleFunding(&projected, acc, now)
	return projected.DebtQuota
}

// flipOverdueFundingsTx 逾期状态机（Task 11）：在写事务内把传入切片中「今天已过到期日
// （loanDay(now) > DueDay）且仍有债务（DebtQuota > 0）」的 active funding 条件更新为
// overdue 并落 penalty_started_day = loanDay(now)。
//
// 幂等性由条件更新保证（UPDATE ... WHERE id=? AND status='active'）：
//   - RowsAffected==1：本次调用完成了翻转，同步回写内存切片元素；
//   - RowsAffected==0：并发翻转抢先（或行已被其他路径改动），重读该行
//     status/penalty_started_day 回填切片，不视为本次新翻转。
//
// 重复调用（同日二次、跨路径）天然为空操作。返回本次调用新翻转的 funding 子集（内存态），
// 供信用分（Task 12）与平台资金处置（Task 15，按 SourceType==platform 过滤）消费；
// 其余调用方可直接忽略返回值。
//
// 注意：翻转只改 status/penalty_started_day，不影响利息数学——settleFunding 的罚息
// 由 today > DueDay 纯计算驱动、与 status 无关（见上），故翻转前后结算结果一致。
func flipOverdueFundingsTx(tx *gorm.DB, userId int, fundings []TokenLoanFunding, now time.Time) ([]TokenLoanFunding, error) {
	today := loanDay(now)
	var flipped []TokenLoanFunding
	for i := range fundings {
		f := &fundings[i]
		if f.Status != LoanFundingActive || today <= f.DueDay || f.DebtQuota <= 0 {
			continue
		}
		res := tx.Model(&TokenLoanFunding{}).
			Where("loan_user_id = ? AND id = ? AND status = ?", userId, f.Id, LoanFundingActive).
			Updates(map[string]interface{}{
				"status":              LoanFundingOverdue,
				"penalty_started_day": today,
				"updated_at":          now.Unix(),
			})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 1 {
			// 本次调用完成翻转：内存切片同步，调用方后续（闸门/分配）看到 overdue 态
			f.Status = LoanFundingOverdue
			f.PenaltyStartedDay = today
			f.UpdatedAt = now.Unix()
			flipped = append(flipped, *f)
			continue
		}
		// 并发翻转抢先：重读 status/penalty_started_day 回填切片，保证调用方视图与库内一致
		var cur TokenLoanFunding
		if err := tx.Select("status", "penalty_started_day", "updated_at").
			Where("id = ?", f.Id).First(&cur).Error; err != nil {
			return nil, err
		}
		f.Status = cur.Status
		f.PenaltyStartedDay = cur.PenaltyStartedDay
		f.UpdatedAt = cur.UpdatedAt
	}
	return flipped, nil
}

// syncAccountFromFundings 把给定 fundings 的债务/未还本金汇总回写账户投影
// （spec §4.5：账户 DebtQuota/PrincipalQuota 恒等于 Σ 该用户 active/overdue fundings）。
// 同时把 acc.LastSettledDay 推进到 fundings 中最大的 LastSettledDay 且不回拨，
// 保证旧账户时钟不滞后于任一 funding。仅改内存，落盘由调用方负责。
func syncAccountFromFundings(acc *TokenLoanAccount, fundings []TokenLoanFunding) {
	var debt, principal int64
	maxDay := acc.LastSettledDay
	for _, f := range fundings {
		debt += f.DebtQuota
		principal += f.PrincipalRemaining
		if f.LastSettledDay > maxDay {
			maxDay = f.LastSettledDay
		}
	}
	acc.DebtQuota = debt
	acc.PrincipalQuota = principal
	acc.LastSettledDay = maxDay
}
