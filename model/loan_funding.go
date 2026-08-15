package model

import (
	"math"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
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
