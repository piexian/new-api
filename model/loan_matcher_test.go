package model

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// ===== Task 7: 撮合引擎测试 =====
// 纯撮合测试：直接向共享 SQLite 建 offer 行，DB.Begin() 开事务调用撮合器，
// 完成后 Rollback（撮合只读，不改库）。

// resetLoanOffers 清空全部 offer 行：共享内存库中其他用例（如 TestLoanMarketModels
// 冒烟用例）会残留 offer，撮合用例必须从干净状态开始（各用例自建 offer）。
// 先 AutoMigrate 保证表存在（TestMain 未迁移 token_loan_offers）。
func resetLoanOffers(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanOffer{}))
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanOffer{}).Error)
}

// createMatcherOffer 建一条 offer 测试行，测试结束删除（避免残留污染其他用例）
func createMatcherOffer(t *testing.T, lenderId int, mode, status string, available int64, rateFixed, rateMin, rateMax float64, perLoanCap int64, minCredit int) *TokenLoanOffer {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanOffer{}))
	now := time.Now()
	offer := &TokenLoanOffer{
		LenderId:        lenderId,
		Mode:            mode,
		Status:          status,
		AmountTotal:     available,
		AmountAvailable: available,
		RateFixed:       rateFixed,
		RateMin:         rateMin,
		RateMax:         rateMax,
		PerLoanCap:      perLoanCap,
		MinCreditScore:  minCredit,
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, DB.Create(offer).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("id = ?", offer.Id).Delete(&TokenLoanOffer{}).Error)
	})
	return offer
}

// runMatcher 开事务调用撮合器并回滚，返回计划与 drop 原因
func runMatcher(t *testing.T, borrowerId int, creditScore int, amount int64, intendedOrderId int, aiPriced []FundingPlan) ([]FundingPlan, []string, error) {
	t.Helper()
	tx := DB.Begin()
	require.NoError(t, tx.Error)
	plans, drops, err := MatchLoanFundings(tx, borrowerId, creditScore, amount, intendedOrderId, aiPriced)
	require.NoError(t, tx.Rollback().Error)
	return plans, drops, err
}

func planSum(plans []FundingPlan) int64 {
	var s int64
	for i := range plans {
		s += plans[i].Amount
	}
	return s
}

// ① 利率升序吃量顺序：低利率 offer 先吃满，高利率最后吃剩余
func TestLoanMatcherRateAscending(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	l3 := createLoanTestUser(t)
	high := createMatcherOffer(t, l1.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.003, 0, 0, 0, -50)
	low := createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.001, 0, 0, 0, -50)
	mid := createMatcherOffer(t, l3.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.002, 0, 0, 0, -50)

	plans, drops, err := runMatcher(t, borrower.Id, 80, 2500, 0, nil)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 3)
	// 顺序：low(0.001) → mid(0.002) → high(0.003)
	require.Equal(t, []int{low.Id, mid.Id, high.Id}, []int{plans[0].OfferId, plans[1].OfferId, plans[2].OfferId})
	require.Equal(t, []int64{1000, 1000, 500}, []int64{plans[0].Amount, plans[1].Amount, plans[2].Amount})
	require.Equal(t, LoanFundingPool, plans[0].SourceType)
	require.Equal(t, 2500, int(planSum(plans)))
}

// ② cap 与 available 限制：每笔 min(剩余, available, cap>0?cap:∞)
func TestLoanMatcherCapAndAvailable(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	// available 5000 但 cap 1000 → 只吃 1000
	capped := createMatcherOffer(t, l1.Id, LoanOfferModePool, LoanOfferStatusActive, 5000, 0.001, 0, 0, 1000, -50)
	// cap 0 = 不限，available 2000 → 吃满 2000
	availLimited := createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 2000, 0.002, 0, 0, 0, -50)

	plans, drops, err := runMatcher(t, borrower.Id, 80, 10000, 0, nil)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 2)
	require.Equal(t, []int{capped.Id, availLimited.Id}, []int{plans[0].OfferId, plans[1].OfferId})
	require.Equal(t, int64(1000), plans[0].Amount) // 受 cap 限制
	require.Equal(t, int64(2000), plans[1].Amount) // 受 available 限制
	require.Equal(t, int64(3000), planSum(plans))  // 来源不足，剩余交给调用方兜底
}

// ③ 信用分门槛过滤：低于 min_credit_score 跳过，-50 = 不限
func TestLoanMatcherCreditScoreFilter(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	l3 := createLoanTestUser(t)
	pass60 := createMatcherOffer(t, l1.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.001, 0, 0, 0, 60)
	noLimit := createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.002, 0, 0, 0, -50)
	createMatcherOffer(t, l3.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.003, 0, 0, 0, 90)

	plans, drops, err := runMatcher(t, borrower.Id, 70, 3000, 0, nil)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 2)
	require.Equal(t, []int{pass60.Id, noLimit.Id}, []int{plans[0].OfferId, plans[1].OfferId})
	require.Equal(t, int64(2000), planSum(plans)) // need90（90 > 70）被过滤
}

// ④ lender == borrower 跳过：不得向本人出资
func TestLoanMatcherSkipsOwnOffer(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	createMatcherOffer(t, borrower.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.001, 0, 0, 0, -50)
	other := createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.002, 0, 0, 0, -50)

	plans, drops, err := runMatcher(t, borrower.Id, 80, 2000, 0, nil)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 1)
	require.Equal(t, other.Id, plans[0].OfferId)
	require.Equal(t, int64(1000), plans[0].Amount)
}

// ⑤ AI 定价越界剔除：rate 超界 / amount 超 min(available, cap) / 非正数 → 剔除并记录原因
func TestLoanMatcherAiDropsInvalid(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	o1 := createMatcherOffer(t, l1.Id, LoanOfferModeAi, LoanOfferStatusActive, 2000, 0, 0.001, 0.002, 1000, -50)
	o2 := createMatcherOffer(t, l2.Id, LoanOfferModeAi, LoanOfferStatusActive, 2000, 0, 0.001, 0.002, 1000, -50)

	ai := []FundingPlan{
		{OfferId: o1.Id, LenderId: l1.Id, SourceType: LoanFundingAi, Amount: 800, Rate: 0.0015},  // 有效
		{OfferId: o1.Id, LenderId: l1.Id, SourceType: LoanFundingAi, Amount: 800, Rate: 0.003},   // rate 超 rate_max
		{OfferId: o1.Id, LenderId: l1.Id, SourceType: LoanFundingAi, Amount: 800, Rate: 0.0005},  // rate 低于 rate_min
		{OfferId: o2.Id, LenderId: l2.Id, SourceType: LoanFundingAi, Amount: 1200, Rate: 0.0015}, // amount 超 min(available, cap)
		{OfferId: o2.Id, LenderId: l2.Id, SourceType: LoanFundingAi, Amount: 0, Rate: 0.0015},    // 金额非正数
		{OfferId: o2.Id, LenderId: l2.Id, SourceType: LoanFundingAi, Amount: 500, Rate: 0.001},   // 有效
	}

	plans, drops, err := runMatcher(t, borrower.Id, 80, 10000, 0, ai)
	require.NoError(t, err)
	require.Len(t, drops, 4)
	// 有效条目按给定顺序保留
	require.Len(t, plans, 2)
	require.Equal(t, o1.Id, plans[0].OfferId)
	require.Equal(t, int64(800), plans[0].Amount)
	require.Equal(t, 0.0015, plans[0].Rate)
	require.Equal(t, LoanFundingAi, plans[0].SourceType)
	require.Equal(t, o2.Id, plans[1].OfferId)
	require.Equal(t, int64(500), plans[1].Amount)
	require.Equal(t, 0.001, plans[1].Rate)
}

// ⑥ 定向挂单优先：意向 order 先吃量（即使利率高于市场其他 offer）
func TestLoanMatcherIntendedOrderPriority(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	intended := createMatcherOffer(t, l1.Id, LoanOfferModeOrder, LoanOfferStatusActive, 5000, 0.003, 0, 0, 0, -50)
	createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 5000, 0.001, 0, 0, 0, -50)

	plans, drops, err := runMatcher(t, borrower.Id, 80, 3000, intended.Id, nil)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 1)
	require.Equal(t, intended.Id, plans[0].OfferId)
	require.Equal(t, LoanFundingOrder, plans[0].SourceType)
	require.Equal(t, int64(3000), plans[0].Amount)
	require.Equal(t, 0.003, plans[0].Rate) // 利率 = order 的 rate_fixed
	require.Equal(t, int64(3000), planSum(plans))
}

// ⑦ 全部来源不足：返回部分计划，缺额由调用方补 platform
func TestLoanMatcherPartialWhenInsufficient(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	createMatcherOffer(t, l1.Id, LoanOfferModePool, LoanOfferStatusActive, 500, 0.001, 0, 0, 0, -50)
	aiOffer := createMatcherOffer(t, l2.Id, LoanOfferModeAi, LoanOfferStatusActive, 3000, 0, 0.001, 0.002, 1000, -50)

	ai := []FundingPlan{{OfferId: aiOffer.Id, LenderId: l2.Id, SourceType: LoanFundingAi, Amount: 300, Rate: 0.0015}}
	plans, drops, err := runMatcher(t, borrower.Id, 80, 10000, 0, ai)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 2)
	require.Equal(t, int64(800), planSum(plans)) // 500 + 300 < 10000
}

// ⑧ 条数截断：超 MaxFundingsPerBorrow 时保留定向单 + 低利率计划，AI 在固定利率之后
func TestLoanMatcherTruncation(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.MaxFundingsPerBorrow = 2 })
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	l3 := createLoanTestUser(t)
	low := createMatcherOffer(t, l1.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.001, 0, 0, 0, -50)
	mid := createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.002, 0, 0, 0, -50)
	high := createMatcherOffer(t, l3.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.003, 0, 0, 0, -50)

	// 场景 A：无定向单 → 保留两个最低利率，被截断金额不重新分配
	plans, drops, err := runMatcher(t, borrower.Id, 80, 3000, 0, nil)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 2)
	require.Equal(t, []int{low.Id, mid.Id}, []int{plans[0].OfferId, plans[1].OfferId})
	require.Equal(t, int64(2000), planSum(plans)) // high 被截断，其额度不补位

	// 场景 B：定向单（利率高于市场）优先保留。清掉 mid/high，使固定利率吃完后
	// 剩余仍够 AI 参与（否则 AI 因 remaining=0 根本不被处理，截断掉的是 mid）。
	intended := createMatcherOffer(t, l1.Id, LoanOfferModeOrder, LoanOfferStatusActive, 1000, 0.005, 0, 0, 0, -50)
	aiOffer := createMatcherOffer(t, l3.Id, LoanOfferModeAi, LoanOfferStatusActive, 2000, 0, 0.001, 0.002, 1000, -50)
	ai := []FundingPlan{{OfferId: aiOffer.Id, LenderId: l3.Id, SourceType: LoanFundingAi, Amount: 500, Rate: 0.0015}}
	require.NoError(t, DB.Where("id IN ?", []int{mid.Id, high.Id}).Delete(&TokenLoanOffer{}).Error)
	plans, drops, err = runMatcher(t, borrower.Id, 80, 3000, intended.Id, ai)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 2)
	require.Equal(t, intended.Id, plans[0].OfferId) // 定向单第一
	require.Equal(t, low.Id, plans[1].OfferId)      // 其次最低利率
	require.Equal(t, int64(2000), planSum(plans))   // AI 计划（500）排最后，截断时被丢弃，缺额交调用方兜底
}

// 过期定向单（closed / 非 order 模式 / 不存在）跳过并记录原因，落到统一市场，不报错
func TestLoanMatcherStaleIntendedFallsThrough(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)

	// (a) 定向单已 closed
	closed := createMatcherOffer(t, l1.Id, LoanOfferModeOrder, LoanOfferStatusClosed, 5000, 0.003, 0, 0, 0, -50)
	poolA := createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.001, 0, 0, 0, -50)
	plans, drops, err := runMatcher(t, borrower.Id, 80, 2000, closed.Id, nil)
	require.NoError(t, err)
	require.Len(t, drops, 1)
	require.Contains(t, drops[0], "状态")
	require.Len(t, plans, 1)
	require.Equal(t, poolA.Id, plans[0].OfferId)
	// 清掉 poolA 避免干扰 (b)（同利率 0.001 且 id 更小会排在 intendedPool 前；t.Cleanup 重复删为空操作）
	require.NoError(t, DB.Where("id = ?", poolA.Id).Delete(&TokenLoanOffer{}).Error)

	// (b) 意向 offer 是 pool 模式（非 order）→ 落到统一市场按普通 offer 吃量
	intendedPool := createMatcherOffer(t, l1.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.001, 0, 0, 0, -50)
	poolB := createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.002, 0, 0, 0, -50)
	plans, drops, err = runMatcher(t, borrower.Id, 80, 2000, intendedPool.Id, nil)
	require.NoError(t, err)
	require.Len(t, drops, 1)
	require.Contains(t, drops[0], "模式")
	require.Len(t, plans, 2)
	require.Equal(t, intendedPool.Id, plans[0].OfferId) // 按利率升序仍排第一
	require.Equal(t, poolB.Id, plans[1].OfferId)

	// (c) 意向 offer 不存在 → 跳过继续
	plans, drops, err = runMatcher(t, borrower.Id, 80, 2000, 987654321, nil)
	require.NoError(t, err)
	require.Len(t, drops, 1)
	require.Contains(t, drops[0], "不存在")
	require.Len(t, plans, 2)
	require.Equal(t, int64(2000), planSum(plans))
}

// 定向单部分覆盖时，统一市场不得再次吃同一 offer（consumedOfferIds 守卫）：
// 定向阶段吃 min(剩余, available) 后，若剩余量足够大，统一市场按利率升序会再次
// 触达该 offer——必须跳过，否则同一 offer 总分配会超过 amount_available，
// Task 8 放款时二次校验必然失败
func TestLoanMatcherIntendedOfferConsumedOnce(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	// intended available 2000（rate 0.003），借款 5000：定向单吃满 2000 后剩余 3000，
	// 统一市场先吃 P(0.001) 1000，随后触达 intended(0.003)——无守卫会再吃 2000
	intended := createMatcherOffer(t, l1.Id, LoanOfferModeOrder, LoanOfferStatusActive, 2000, 0.003, 0, 0, 0, -50)
	createMatcherOffer(t, l2.Id, LoanOfferModePool, LoanOfferStatusActive, 1000, 0.001, 0, 0, 0, -50)

	plans, drops, err := runMatcher(t, borrower.Id, 80, 5000, intended.Id, nil)
	require.NoError(t, err)
	require.Empty(t, drops)
	require.Len(t, plans, 2)
	require.Equal(t, intended.Id, plans[0].OfferId) // 定向单优先
	require.Equal(t, int64(2000), plans[0].Amount)
	require.Equal(t, int64(1000), plans[1].Amount)
	// intended 恰好出现一次，总分配 = available，不超
	intendedCount, fromIntended := 0, int64(0)
	for i := range plans {
		if plans[i].OfferId == intended.Id {
			intendedCount++
			fromIntended += plans[i].Amount
		}
	}
	require.Equal(t, 1, intendedCount)
	require.Equal(t, int64(2000), fromIntended)
	require.Equal(t, int64(3000), planSum(plans)) // 来源不足，缺额交调用方兜底
}

// AI 出资同一 offer 的多条计划按累计校验：联合不得超过 min(available, cap)，
// 超出的条目剔除并记录原因（单条各自 ≤ 上限但联合超限的评审问题）
func TestLoanMatcherAiCumulativePerOffer(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	// o1：available 3000, cap 2000 → 按 cap 累计封顶
	o1 := createMatcherOffer(t, l1.Id, LoanOfferModeAi, LoanOfferStatusActive, 3000, 0, 0.001, 0.002, 2000, -50)
	// o2：available 1000, cap 0（防御不限）→ 按 available 累计封顶
	o2 := createMatcherOffer(t, l2.Id, LoanOfferModeAi, LoanOfferStatusActive, 1000, 0, 0.001, 0.002, 0, -50)

	ai := []FundingPlan{
		{OfferId: o1.Id, LenderId: l1.Id, SourceType: LoanFundingAi, Amount: 1500, Rate: 0.0015}, // 有效：1500 ≤ 2000
		{OfferId: o1.Id, LenderId: l1.Id, SourceType: LoanFundingAi, Amount: 1000, Rate: 0.0015}, // 累计 2500 > cap 2000 → 剔除
		{OfferId: o1.Id, LenderId: l1.Id, SourceType: LoanFundingAi, Amount: 400, Rate: 0.0015},  // 累计 1900 ≤ 2000 → 有效
		{OfferId: o2.Id, LenderId: l2.Id, SourceType: LoanFundingAi, Amount: 600, Rate: 0.0015},  // 有效：600 ≤ 1000
		{OfferId: o2.Id, LenderId: l2.Id, SourceType: LoanFundingAi, Amount: 600, Rate: 0.0015},  // 累计 1200 > available 1000 → 剔除
	}

	plans, drops, err := runMatcher(t, borrower.Id, 80, 10000, 0, ai)
	require.NoError(t, err)
	require.Len(t, drops, 2)
	require.Contains(t, strings.Join(drops, ";"), "超过剩余额度")
	require.Len(t, plans, 3)
	byOffer := map[int]int64{}
	for i := range plans {
		byOffer[plans[i].OfferId] += plans[i].Amount
	}
	require.Equal(t, int64(1900), byOffer[o1.Id]) // ≤ min(3000, 2000)
	require.Equal(t, int64(600), byOffer[o2.Id])  // ≤ min(1000, ∞)
	require.Equal(t, int64(2500), planSum(plans))
}

// AI 出资条目的信用分门槛（P1-2，spec §6）：与 ①② 阶段一致，
// 低于 offer.MinCreditScore 的条目剔除并记录原因（-50 = 不限）
func TestLoanMatcherAiCreditScoreFilter(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l1 := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	// 门槛 90 > 借款人 70 → 剔除
	highBar := createMatcherOffer(t, l1.Id, LoanOfferModeAi, LoanOfferStatusActive, 2000, 0, 0.001, 0.002, 1000, 90)
	// 门槛 60 ≤ 借款人 70 → 有效
	pass := createMatcherOffer(t, l2.Id, LoanOfferModeAi, LoanOfferStatusActive, 2000, 0, 0.001, 0.002, 1000, 60)

	ai := []FundingPlan{
		{OfferId: highBar.Id, LenderId: l1.Id, SourceType: LoanFundingAi, Amount: 800, Rate: 0.0015},
		{OfferId: pass.Id, LenderId: l2.Id, SourceType: LoanFundingAi, Amount: 800, Rate: 0.0015},
	}
	plans, drops, err := runMatcher(t, borrower.Id, 70, 10000, 0, ai)
	require.NoError(t, err)
	require.Len(t, drops, 1)
	require.Contains(t, drops[0], "信用分门槛")
	require.Len(t, plans, 1)
	require.Equal(t, pass.Id, plans[0].OfferId)
	require.Equal(t, int64(800), plans[0].Amount)
}

// AI 出资条目 lender == borrower 防御（P1-2，spec §6）：撮合器自洽，
// 放贷人不得向本人出资，此类条目剔除并记录原因（不依赖上游候选过滤）
func TestLoanMatcherAiSkipsOwnOffer(t *testing.T) {
	resetLoanOffers(t)
	borrower := createLoanTestUser(t)
	l2 := createLoanTestUser(t)
	// 借款人自己的 ai offer → 剔除
	own := createMatcherOffer(t, borrower.Id, LoanOfferModeAi, LoanOfferStatusActive, 2000, 0, 0.001, 0.002, 1000, -50)
	other := createMatcherOffer(t, l2.Id, LoanOfferModeAi, LoanOfferStatusActive, 2000, 0, 0.001, 0.002, 1000, -50)

	ai := []FundingPlan{
		{OfferId: own.Id, LenderId: borrower.Id, SourceType: LoanFundingAi, Amount: 800, Rate: 0.0015},
		{OfferId: other.Id, LenderId: l2.Id, SourceType: LoanFundingAi, Amount: 800, Rate: 0.0015},
	}
	plans, drops, err := runMatcher(t, borrower.Id, 80, 10000, 0, ai)
	require.NoError(t, err)
	require.Len(t, drops, 1)
	require.Contains(t, drops[0], "属于借款人本人")
	require.Len(t, plans, 1)
	require.Equal(t, other.Id, plans[0].OfferId)
	require.Equal(t, int64(800), plans[0].Amount)
}
