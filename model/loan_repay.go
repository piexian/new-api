package model

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ===== Task 9: 还款按 funding pro-rata 分配 =====

// RepayAllocation 单条 funding 的还款分配：还款额按各 funding 结算后债务 pro-rata 拆分
// （最大余数法，Σ Amount ≡ 还款额），每条内先息后本
type RepayAllocation struct {
	FundingId     int64
	LenderId      int
	OfferId       int
	SourceType    string
	Amount        int64
	InterestPart  int64
	PrincipalPart int64
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
//     逾期后还清与低于金额门槛均不计分），仅改内存 acc，落盘由调用方统一 Save。
//
// 返回还款结果（Amount/InterestPart/PrincipalPart/DebtAfter 来自同步后的账户投影）与逐条分配。
// repay <= 0 或 Σdebt <= 0 时返回 (nil, nil, nil)，调用方须先保证有债务。
func distributeRepayment(tx *gorm.DB, acc *TokenLoanAccount, fundings []TokenLoanFunding, repay int64, now time.Time) (*LoanRepayInfo, []RepayAllocation, error) {
	if repay <= 0 {
		return nil, nil, nil
	}
	// ① 逐条结算；结算有变动（债务增长/时钟推进）的行落盘。幂等：调用方已结算时
	//    days=0 无变动，不重复写
	for i := range fundings {
		before := fundings[i]
		settleFunding(&fundings[i], acc, now)
		if fundings[i].DebtQuota != before.DebtQuota || fundings[i].LastSettledDay != before.LastSettledDay {
			if err := tx.Save(&fundings[i]).Error; err != nil {
				return nil, nil, err
			}
		}
	}

	// ①.5 逾期状态机（Task 11）：今天过期的 active funding 翻转为 overdue（幂等条件
	// 更新，并发双还款不双翻）。翻转只改 status/penalty_started_day，不影响 ② 的
	// pro-rata 分配（分配基于结算后 debt_quota，与 status 无关）；③ 中 debt 归零照常
	// 置 repaid，故逾期 funding 全额结清自然完成 overdue → repaid 流转。
	if _, err := flipOverdueFundingsTx(tx, acc.UserId, fundings, now); err != nil {
		return nil, nil, err
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
		return nil, nil, nil
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
		payInterest := min(alloc, interest)
		payPrincipal := alloc - payInterest
		f.DebtQuota -= alloc
		f.PrincipalRemaining -= payPrincipal
		if f.DebtQuota == 0 {
			f.Status = LoanFundingRepaid
			repaidAny = true
		}
		if err := tx.Save(f).Error; err != nil {
			return nil, nil, err
		}
		totalInterest += payInterest
		totalPrincipal += payPrincipal
		allocs = append(allocs, RepayAllocation{
			FundingId:     f.Id,
			LenderId:      f.LenderId,
			OfferId:       f.OfferId,
			SourceType:    f.SourceType,
			Amount:        alloc,
			InterestPart:  payInterest,
			PrincipalPart: payPrincipal,
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
		if err := scoreBorrowEventRepaidTx(tx, acc, eventId, now); err != nil {
			return nil, nil, err
		}
	}

	// ④.5 黑名单出口（Task 12）：有 funding 本次结清（→repaid）时检查是否满足解除条件
	//     （永续全还清 → 立即解除；核销窗口内不解锁）。仅改内存 acc，落盘由调用方统一 Save
	if repaidAny {
		if err := maybeLiftBlacklistTx(tx, acc, now); err != nil {
			return nil, nil, err
		}
	}

	info := &LoanRepayInfo{
		Amount:        repay,
		InterestPart:  totalInterest,
		PrincipalPart: totalPrincipal,
		DebtAfter:     acc.DebtQuota,
	}
	return info, allocs, nil
}

// settleRepayAllocations 在事务内结算还款分配的资金去向（spec §7.4）：
//   - 利息：计入放贷人余额（users.quota）；offer 仍存在时累计 offer.TotalInterestEarned；
//     platform 无入账（债务销毁）；
//   - 本金：offer 非 closed → 回补 offer.AmountAvailable（amount_total 不变）；
//     offer closed → 计入放贷人余额且 amount_total -= 本金（钱离开 offer 账面）；
//     platform → 无入账；
//   - 台账：每条 alloc 一行 repay 记录（FundingId/LenderId 冗余），Source 由调用方传入
//     （manual / checkin），DebtAfter 取同事务内 funding 当前剩余债务。
//
// 锁序遵守全局约束 offers(id 升序) → users(id 升序)。返回按放贷人聚合的入账清单
// （user id 升序），供提交后异步 cacheIncrUserQuota。
func settleRepayAllocations(tx *gorm.DB, userId int, allocs []RepayAllocation, source string) ([]LenderCredit, error) {
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

	// 入账归集：利息（非 platform）+ closed offer 的本金；platform 本息归平台（债务销毁）
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
	}

	// 用户入账（锁序 users(id 升序)）：int32 上界校验镜像 BorrowLoan —— 溢出时整笔还款
	// 回滚（ErrLoanQuotaOverflow），绝不让余额静默截断。users.quota 列是 32 位 int，
	// 直接写超界大值在 MySQL/PG 会报错/回绕，因此必须事前锁行校验（非截断方案）。
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
		if int64(user.Quota)+amt > common.MaxQuota {
			return nil, ErrLoanQuotaOverflow
		}
		if err := tx.Model(&User{}).Where("id = ?", lid).
			Update("quota", gorm.Expr("quota + ?", amt)).Error; err != nil {
			return nil, err
		}
	}

	// 台账：每条 alloc 一行 repay（funding_id/lender_id 冗余）。手续费不落台账：
	// 提前还款手续费是平台收入，仅体现在 LoanRepayInfo.FeePart 与借款人余额扣款
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
