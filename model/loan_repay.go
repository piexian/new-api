package model

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"gorm.io/gorm"
)

// ===== Task 9: 还款按 funding pro-rata 分配 =====

// RepayAllocation 单条 funding 的还款分配：还款额按各 funding 结算后债务 pro-rata 拆分
// （最大余数法，Σ Amount ≡ 还款额），每条内先息后本。
// Repaid 标记该 funding 是否在本次分配中转结清（debt 归零 → status=repaid）；
// 秒结清惩罚判定需要 funding 侧字段（窗口按 CreatedAt、条款取放款时从 offer 复制的值）。
type RepayAllocation struct {
	FundingId             int64
	LenderId              int
	OfferId               int
	SourceType            string
	Amount                int64
	InterestPart          int64
	PrincipalPart         int64
	Repaid                bool
	CreatedAt             int64
	FastRepayPenaltyQuota int64
	FastRepayWindowDays   int
}

// LenderCredit 事务内放贷人余额入账清单（按放贷人聚合），供提交后异步同步 Redis 缓存
type LenderCredit struct {
	UserId int
	Amount int64
}

// distributeRepayment 把一笔还款额按各 funding 当前债务（结算后 debt_quota）pro-rata 分配：
//  1. 逐条 settleFunding（platform 传 acc 提供有效利率/宽限输入）；结算有变动的行落盘；
//  2. 债务 0 的 funding 跳过；repay = min(repay, Σdebt)；最大余数法取整（big 精确除，防
//     长期复利债务下 int64 乘法溢出），Σ 分配 ≡ repay 且每条分配 <= 自身债务；
//  3. 每条内先息后本：payInterest = min(alloc, debt-principal)；debt -= alloc、
//     principal -= payInterest，debt 归零 → status = repaid；分配变动的行落盘；
//  4. syncAccountFromFundings 回写账户投影（仅内存，落盘由调用方负责），并累计 TotalRepaid；
//     4.5 黑名单出口（Task 12）：有 funding 本次结清时调用 maybeLiftBlacklistTx（永续全还清
//     → 立即解除；核销窗口内不解锁），仅改内存 acc，落盘由调用方统一 Save。
//  5. 信用分结算（Task 13）：本次转结清的 funding 所属借款事件（去重）调用
//     scoreBorrowEventRepaidTx——事件全部结清才评分（按时还清加分 / 快速还清扣分 /
//     逾期后还清与低于金额门槛均不计分；source == "checkin" 的快速结清积分中性，
//     不扣分也不加分），仅改内存 acc，落盘由调用方统一 Save。
//
// 返回还款结果（Amount/InterestPart/PrincipalPart/DebtAfter 来自同步后的账户投影）、
// 逐条分配与本次新翻转的逾期 funding 列表（Task 15 官方处置派发用，事务提交后由
// 调用方 dispatchPlatformOverdueAsync；事务内不可派发——overdue 尚未提交）。
// repay <= 0 或 Σdebt <= 0 时返回 (nil, nil, nil, nil)，调用方须先保证有债务。
// source 为还款来源（manual / checkin），仅用于信用分结算的快速还清豁免。
// principalFirst=true 时每条 funding 内先抵本后抵息（注销强制清算专用，出借人优先
// 拿回本金）；false 为既有先息后本（手动还款 / 签到扣还）。
func distributeRepayment(tx *gorm.DB, acc *TokenLoanAccount, fundings []TokenLoanFunding, repay int64, now time.Time, source string, principalFirst bool) (*LoanRepayInfo, []RepayAllocation, []TokenLoanFunding, error) {
	if repay <= 0 {
		return nil, nil, nil, nil
	}
	// ① 逐条结算；结算有变动（债务增长/时钟推进）的行落盘。幂等：调用方已结算时
	//    days=0 无变动，不重复写
	for i := range fundings {
		before := fundings[i]
		settleFunding(&fundings[i], acc, now)
		if fundings[i].DebtQuota != before.DebtQuota || fundings[i].LastSettledDay != before.LastSettledDay {
			if err := tx.Save(&fundings[i]).Error; err != nil {
				return nil, nil, nil, err
			}
		}
	}

	// ①.5 逾期状态机（Task 11）：今天过期的 active funding 翻转为 overdue（幂等条件
	// 更新，并发双还款不双翻）。翻转只改 status/penalty_started_day，不影响 ② 的
	// pro-rata 分配（分配基于结算后 debt_quota，与 status 无关）；③ 中 debt 归零照常
	// 置 repaid，故逾期 funding 全额结清自然完成 overdue → repaid 流转。
	// 新翻转列表随返回值带出（调用方事务提交后派发官方处置，见函数头注释）。
	flipped, err := flipOverdueFundingsTx(tx, acc.UserId, fundings, now)
	if err != nil {
		return nil, nil, nil, err
	}

	// ② pro-rata 分配（最大余数法）：
	//    floor_i = repay*debt_i/Σdebt（整数截断），余数按降序吃下 left = repay - Σfloor 的配额；
	//    left < 参与条数；并列余数按 funding id 升序（确定性）。乘法用 big 精确计算，
	//    避免 long 期复利债务下 int64 溢出破坏 Σ 守恒
	var total int64
	for i := range fundings {
		if fundings[i].DebtQuota > 0 {
			total += fundings[i].DebtQuota
		}
	}
	if total <= 0 {
		return nil, nil, nil, nil
	}
	if repay > total {
		repay = total
	}
	floors := make([]int64, len(fundings))
	remainders := make([]int64, len(fundings))
	bigRepay := big.NewInt(repay)
	bigTotal := big.NewInt(total)
	var floorSum int64
	participants := make([]int, 0, len(fundings))
	for i := range fundings {
		if fundings[i].DebtQuota <= 0 {
			continue
		}
		num := new(big.Int).Mul(bigRepay, big.NewInt(fundings[i].DebtQuota))
		quo, rem := new(big.Int).QuoRem(num, bigTotal, new(big.Int))
		floors[i] = quo.Int64()
		remainders[i] = rem.Int64()
		floorSum += floors[i]
		participants = append(participants, i)
	}
	if left := repay - floorSum; left > 0 {
		sort.Slice(participants, func(a, b int) bool {
			ia, ib := participants[a], participants[b]
			if remainders[ia] != remainders[ib] {
				return remainders[ia] > remainders[ib]
			}
			return fundings[ia].Id < fundings[ib].Id
		})
		for k := 0; k < int(left); k++ {
			floors[participants[k]]++
		}
	}

	// ③ 每条内先息后本；更新 funding 行并落盘，生成分配明细
	allocs := make([]RepayAllocation, 0, len(fundings))
	var totalInterest, totalPrincipal int64
	repaidAny := false // 是否有 funding 本次结清（黑名单解除钩子触发条件）
	for i := range fundings {
		f := &fundings[i]
		if f.DebtQuota <= 0 {
			continue
		}
		alloc := floors[i]
		if alloc <= 0 {
			continue
		}
		interest := f.DebtQuota - f.PrincipalRemaining // 当前未付利息
		var payInterest, payPrincipal int64
		if principalFirst {
			// 先本后息（注销强制清算）：出借人优先拿回本金
			payPrincipal = min(alloc, f.PrincipalRemaining)
			payInterest = alloc - payPrincipal
		} else {
			// 先息后本（手动还款 / 签到扣还）
			payInterest = min(alloc, interest)
			payPrincipal = alloc - payInterest
		}
		f.DebtQuota -= alloc
		f.PrincipalRemaining -= payPrincipal
		repaid := f.DebtQuota == 0
		if repaid {
			f.Status = LoanFundingRepaid
			repaidAny = true
		}
		if err := tx.Save(f).Error; err != nil {
			return nil, nil, nil, err
		}
		totalInterest += payInterest
		totalPrincipal += payPrincipal
		allocs = append(allocs, RepayAllocation{
			FundingId:             f.Id,
			LenderId:              f.LenderId,
			OfferId:               f.OfferId,
			SourceType:            f.SourceType,
			Amount:                alloc,
			InterestPart:          payInterest,
			PrincipalPart:         payPrincipal,
			Repaid:                repaid,
			CreatedAt:             f.CreatedAt,
			FastRepayPenaltyQuota: f.FastRepayPenaltyQuota,
			FastRepayWindowDays:   f.FastRepayWindowDays,
		})
	}

	// ④ 账户投影回写（仅内存，落盘由调用方负责）
	syncAccountFromFundings(acc, fundings)
	acc.TotalRepaid += repay

	// ③.5 信用分结算（Task 13）：本次转结清的 funding 所属借款事件（去重，事件级至多
	//     评一次）在事件全部结清时结算信用分。仅改内存 acc，落盘由调用方统一 Save
	scoredEvents := make(map[int64]struct{})
	for i := range fundings {
		if fundings[i].Status == LoanFundingRepaid && fundings[i].BorrowEventId > 0 {
			scoredEvents[fundings[i].BorrowEventId] = struct{}{}
		}
	}
	for eventId := range scoredEvents {
		if err := scoreBorrowEventRepaidTx(tx, acc, eventId, now, source); err != nil {
			return nil, nil, nil, err
		}
	}

	// ④.5 黑名单出口（Task 12）：有 funding 本次结清（→repaid）时检查是否满足解除条件
	//     （永续全还清 → 立即解除；核销窗口内不解锁）。仅改内存 acc，落盘由调用方统一 Save
	if repaidAny {
		if err := maybeLiftBlacklistTx(tx, acc, now); err != nil {
			return nil, nil, nil, err
		}
	}

	info := &LoanRepayInfo{
		Amount:        repay,
		InterestPart:  totalInterest,
		PrincipalPart: totalPrincipal,
		DebtAfter:     acc.DebtQuota,
	}
	return info, allocs, flipped, nil
}

// settleRepayAllocations 在事务内结算还款分配的资金去向（spec §7.4）：
//   - 利息：计入放贷人余额（users.quota）；offer 仍存在时累计 offer.TotalInterestEarned；
//     platform 无入账（债务销毁）；
//   - 本金：offer 非 closed → 回补 offer.AmountAvailable（amount_total 不变）；
//     offer closed → 计入放贷人余额且 amount_total -= 本金（钱离开 offer 账面）；
//     platform → 无入账；
//   - 秒结清惩罚（penaltyByFunding，按 funding id 给出本次转结清 funding 的惩罚额度）：
//     合并进放贷人入账（与利息/本金同一 64 位上界校验，溢出整体回滚），并写入
//     对应 repay 台账行的 penalty_part；platform funding 无惩罚（条款为 0，天然排除）；
//   - 台账：每条 alloc 一行 repay 记录（FundingId/LenderId 冗余），Source 由调用方传入
//     （manual / checkin），DebtAfter 取同事务内 funding 当前剩余债务；提前还款手续费
//     （fee，签到恒为 0）按各 funding 抵本部分 pro-rata 拆分（最大余数法，Σ ≡ fee）
//     落入各行 fee_part——手续费是平台收入，只记台账与借款人余额扣款，不产生放贷人入账。
//
// 锁序遵守全局约束 offers(id 升序) → users(id 升序)。返回按放贷人聚合的入账清单
// （user id 升序，含惩罚），供提交后异步 cacheIncrUserQuota。
func settleRepayAllocations(tx *gorm.DB, userId int, allocs []RepayAllocation, source string, penaltyByFunding map[int64]int64, fee int64) ([]LenderCredit, error) {
	now := time.Now()

	// 预扫描：按 offer 归集利息累计与本金回补，仅锁定涉及的 offer 行（id 升序）
	type offerDelta struct{ interest, principal int64 }
	offerDeltas := make(map[int]offerDelta)
	var offerIds []int
	for i := range allocs {
		a := &allocs[i]
		if a.OfferId <= 0 {
			continue
		}
		if _, seen := offerDeltas[a.OfferId]; !seen {
			offerIds = append(offerIds, a.OfferId)
		}
		d := offerDeltas[a.OfferId]
		d.interest += a.InterestPart
		d.principal += a.PrincipalPart
		offerDeltas[a.OfferId] = d
	}
	sort.Ints(offerIds)
	offerStatus := make(map[int]string, len(offerIds))
	for _, oid := range offerIds {
		var offer TokenLoanOffer
		if err := lockForUpdate(tx).Where("id = ?", oid).First(&offer).Error; err != nil {
			// offer 行缺失属于数据完整性问题（生产代码没有删除 offer 的路径），宁失败不吞钱
			return nil, err
		}
		delta := offerDeltas[oid]
		offer.TotalInterestEarned += delta.interest
		if delta.principal > 0 {
			if offer.Status == LoanOfferStatusClosed {
				offer.AmountTotal -= delta.principal
			} else {
				offer.AmountAvailable += delta.principal
			}
		}
		if err := tx.Save(&offer).Error; err != nil {
			return nil, err
		}
		offerStatus[oid] = offer.Status
	}

	// 入账归集：利息（非 platform）+ closed offer 的本金 + 秒结清惩罚；platform 本息归
	// 平台（债务销毁）。惩罚并入同一 credits map，64 位上界校验统一生效（溢出整笔
	// 还款回滚，绝不让余额静默截断），入账清单返回值自然携带惩罚供缓存/充值日志使用。
	credits := make(map[int]int64)
	for i := range allocs {
		a := &allocs[i]
		if a.SourceType == LoanFundingPlatform {
			continue
		}
		if a.OfferId <= 0 || a.LenderId <= 0 {
			// 非 platform funding 必然挂 offer 与放贷人（executeFundingPlans 保证）；
			// 缺失属于数据完整性错误，宁失败不吞钱（否则本金会被静默销毁）
			return nil, fmt.Errorf("loan funding %d: non-platform allocation missing offer/lender id", a.FundingId)
		}
		if a.InterestPart > 0 {
			credits[a.LenderId] += a.InterestPart
		}
		if a.PrincipalPart > 0 && offerStatus[a.OfferId] == LoanOfferStatusClosed {
			credits[a.LenderId] += a.PrincipalPart
		}
		if p := penaltyByFunding[a.FundingId]; p > 0 {
			credits[a.LenderId] += p
		}
	}

	// 用户入账（锁序 users(id 升序)）：64 位上界校验镜像 BorrowLoan —— 溢出时整笔还款
	// 回滚（LoanLenderOverflowError），绝不让余额静默截断。users.quota 按 64 位存储，
	// 不再沿用 int32 口径的 common.MaxQuota。溢出属放贷人侧异常（借款人无过错），
	// 错误携带放贷人 id 与金额，供调用方在回滚后通知管理员介入。
	lenderIds := make([]int, 0, len(credits))
	for lid := range credits {
		lenderIds = append(lenderIds, lid)
	}
	sort.Ints(lenderIds)
	for _, lid := range lenderIds {
		amt := credits[lid]
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", lid).First(&user).Error; err != nil {
			return nil, err
		}
		if amt > LoanQuotaCeiling-int64(user.Quota) {
			return nil, &LoanLenderOverflowError{LenderId: lid, Amount: amt}
		}
		if err := tx.Model(&User{}).Where("id = ?", lid).
			Update("quota", gorm.Expr("quota + ?", amt)).Error; err != nil {
			return nil, err
		}
	}

	// 台账：每条 alloc 一行 repay（funding_id/lender_id 冗余）。手续费按抵本部分
	// pro-rata 拆入各行 fee_part（Σ ≡ fee，手动还款才可能有值，签到恒 0）；
	// 秒结清惩罚落同一条 repay 行的 penalty_part（每 funding 携带自己的惩罚份额）。
	feeParts := distributeFeeByPrincipal(allocs, fee)
	for i := range allocs {
		a := &allocs[i]
		var f TokenLoanFunding
		if err := tx.Select("debt_quota").Where("id = ?", a.FundingId).First(&f).Error; err != nil {
			return nil, err
		}
		if err := tx.Create(&TokenLoanRecord{
			UserId:        userId,
			Type:          "repay",
			Amount:        a.Amount,
			InterestPart:  a.InterestPart,
			PrincipalPart: a.PrincipalPart,
			FeePart:       feeParts[a.FundingId],
			PenaltyPart:   penaltyByFunding[a.FundingId],
			DebtAfter:     f.DebtQuota,
			Source:        source,
			FundingId:     a.FundingId,
			LenderId:      a.LenderId,
			CreatedAt:     now.Unix(),
		}).Error; err != nil {
			return nil, err
		}
	}

	// 聚合入账清单（user id 升序），供提交后异步 cacheIncrUserQuota
	out := make([]LenderCredit, 0, len(credits))
	for _, lid := range lenderIds {
		out = append(out, LenderCredit{UserId: lid, Amount: credits[lid]})
	}
	return out, nil
}

// distributeFeeByPrincipal 把提前还款手续费按各 funding 的抵本部分 pro-rata 拆分
// （最大余数法取整，big 精确除；并列余数按 funding id 升序，Σ ≡ fee）。
// fee <= 0（签到/全抵息）或 Σ 抵本为 0 时返回空表。
func distributeFeeByPrincipal(allocs []RepayAllocation, fee int64) map[int64]int64 {
	parts := make(map[int64]int64)
	if fee <= 0 {
		return parts
	}
	var totalPrincipal int64
	for i := range allocs {
		totalPrincipal += allocs[i].PrincipalPart
	}
	if totalPrincipal <= 0 {
		return parts
	}
	bigFee := big.NewInt(fee)
	bigTotal := big.NewInt(totalPrincipal)
	indices := make([]int, 0, len(allocs))
	remainders := make([]int64, len(allocs))
	var floorSum int64
	for i := range allocs {
		a := &allocs[i]
		if a.PrincipalPart <= 0 {
			continue
		}
		num := new(big.Int).Mul(bigFee, big.NewInt(a.PrincipalPart))
		quo, rem := new(big.Int).QuoRem(num, bigTotal, new(big.Int))
		parts[a.FundingId] = quo.Int64()
		remainders[i] = rem.Int64()
		floorSum += quo.Int64()
		indices = append(indices, i)
	}
	if left := fee - floorSum; left > 0 {
		sort.Slice(indices, func(x, y int) bool {
			ix, iy := indices[x], indices[y]
			if remainders[ix] != remainders[iy] {
				return remainders[ix] > remainders[iy]
			}
			return allocs[ix].FundingId < allocs[iy].FundingId
		})
		for k := 0; k < int(left); k++ {
			parts[allocs[indices[k]].FundingId]++
		}
	}
	return parts
}

// ProjectFastRepayPenalty 只读估算：借款人此刻手动全额结清将触发的秒结清惩罚总额
// （active/overdue funding 中仍在惩罚窗口内的 FastRepayPenaltyQuota 之和，platform
// funding 条款恒为 0 天然排除）。仅供 status 投影展示；签到自动还款恒不触发惩罚。
func ProjectFastRepayPenalty(userId int, now time.Time) (int64, error) {
	var fundings []TokenLoanFunding
	if err := DB.Select("lender_id", "fast_repay_penalty_quota", "fast_repay_window_days", "created_at").
		Where("loan_user_id = ? AND status IN ? AND fast_repay_penalty_quota > 0",
			userId, []string{LoanFundingActive, LoanFundingOverdue}).
		Find(&fundings).Error; err != nil {
		return 0, err
	}
	today := loanDay(now)
	var total int64
	for i := range fundings {
		f := &fundings[i]
		if f.LenderId <= 0 {
			continue
		}
		if today-loanDay(time.Unix(f.CreatedAt, 0)) <= f.FastRepayWindowDays {
			total += f.FastRepayPenaltyQuota
		}
	}
	return total, nil
}

// computeFastRepayPenalties 计算手动提前还款的秒结清惩罚（仅 model.RepayLoan 调用；
// 签到自动还款经 checkin 路径不经过本函数，恒不触发）：
//   - 仅对本次分配中转结清（Repaid=true）的 funding 计罚；
//   - 窗口判定：loanDay(now) - loanDay(funding.CreatedAt) <= FastRepayWindowDays
//     （0 = 仅当天结清才计罚；放款日与还款日同本地日时差为 0）；
//   - FastRepayPenaltyQuota <= 0（未设置条款）或 LenderId <= 0（platform 无放贷人）
//     不计罚。
//
// 返回：按 funding id 的惩罚额（写台账 penalty_part）、按放贷人的惩罚聚合（并入入账
// 清单）、惩罚总额（借款人余额扣减 + LoanRepayInfo.PenaltyPart）。
func computeFastRepayPenalties(allocs []RepayAllocation, now time.Time) (map[int64]int64, map[int]int64, int64) {
	today := loanDay(now)
	byFunding := make(map[int64]int64)
	byLender := make(map[int]int64)
	var total int64
	for i := range allocs {
		a := &allocs[i]
		if !a.Repaid || a.FastRepayPenaltyQuota <= 0 || a.LenderId <= 0 {
			continue
		}
		if today-loanDay(time.Unix(a.CreatedAt, 0)) > a.FastRepayWindowDays {
			continue
		}
		byFunding[a.FundingId] = a.FastRepayPenaltyQuota
		byLender[a.LenderId] += a.FastRepayPenaltyQuota
		total += a.FastRepayPenaltyQuota
	}
	return byFunding, byLender, total
}
