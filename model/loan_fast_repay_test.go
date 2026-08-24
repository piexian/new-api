package model

import (
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// ===== 秒结清惩罚（fast-repay penalty）测试 =====
// 换算基准：common.QuotaPerUnit = 500000，即 1 USD = 500000 quota。
// 触发条件（最终语义，勿偏离）：仅手动提前还款（model.RepayLoan）路径下，本次转结清
// （Repaid）的 funding 满足 loanDay(now)-loanDay(CreatedAt) <= FastRepayWindowDays 且
// FastRepayPenaltyQuota > 0 → 该 funding 计罚。签到自动还款（checkin 路径）恒不触发。
// 惩罚从借款人余额扣除时不校验余额（允许扣成负数），按 funding 记台账 penalty_part，
// 按放贷人并入 LenderCredit（与利息/本金同一 64 位上界校验）。

// setupFastRepayTest 开启词元贷 + 市场，返回借款人/放贷人（共享库残留行已清理）。
// 手续费率置 0 消除干扰，使惩罚断言聚焦。
func setupFastRepayTest(t *testing.T) (*User, *User) {
	t.Helper()
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MarketEnabled = true
		s.MaxTotal = 10_000_000
		s.MaxPerBorrow = 0
		s.LoanTermDays = 30
		s.MaxFundingsPerBorrow = 5
		s.RepayFeeRate = 0
	})
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	return borrower, lender
}

// createPenaltyOffer 直接建一条带秒结清惩罚条款的 active pool offer（跳过
// CreateLoanOffer 的余额扣款/免责声明路径，测试聚焦还款侧；penalty 为整数 quota）
func createPenaltyOffer(t *testing.T, lenderId int, total, available, penalty int64, window int) *TokenLoanOffer {
	t.Helper()
	offer := createRepayOffer(t, lenderId, total, available)
	require.NoError(t, DB.Model(&TokenLoanOffer{}).Where("id = ?", offer.Id).
		Updates(map[string]interface{}{
			"fast_repay_penalty_quota": penalty,
			"fast_repay_window_days":   window,
		}).Error)
	return offer
}

// borrowFromMarket 从统一市场借一笔钱（单一 pool offer 必然整笔命中），返回新建 funding
func borrowFromMarket(t *testing.T, borrowerId int, usd string) []TokenLoanFunding {
	t.Helper()
	acc, fundings, err := BorrowLoan(borrowerId, usd, 0, nil)
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.NotEmpty(t, fundings)
	return fundings
}

// userQuota 读取用户当前余额（int64）
func userQuota(t *testing.T, userId int) int64 {
	t.Helper()
	var u User
	require.NoError(t, DB.Select("quota").First(&u, userId).Error)
	return int64(u.Quota)
}

// repayLedgerRows 返回借款人名下全部 repay 台账行（id 升序）
func repayLedgerRows(t *testing.T, userId int) []TokenLoanRecord {
	t.Helper()
	var rows []TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", userId, "repay").Order("id ASC").Find(&rows).Error)
	return rows
}

// ① 当天全额手动提前还款 → 惩罚生效：借款人余额被扣成负数（低于惩罚额度仍结清）、
// 放贷人收到惩罚、台账 penalty_part 落库、info.PenaltyPart 正确。
func TestFastRepayPenaltySameDayFullManualRepay(t *testing.T) {
	borrower, lender := setupFastRepayTest(t)
	// 挂单 2.00 USD（1,000,000 quota），惩罚 2.00 USD，窗口 3 天
	createPenaltyOffer(t, lender.Id, 1_000_000, 1_000_000, quotaOf(t, "2.00"), 3)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 4_000_000).Error)

	// 借 1.00 USD：借款人余额 0 + 借得 500,000 = 500,000（恰好覆盖债务，不覆盖惩罚）
	fundings := borrowFromMarket(t, borrower.Id, "1.00")
	require.Len(t, fundings, 1)
	require.Equal(t, quotaOf(t, "1.00"), fundings[0].Amount)

	acc, info, credits, err := RepayLoan(borrower.Id, "all")
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Equal(t, int64(500_000), info.Amount)
	require.Zero(t, info.InterestPart)
	require.Equal(t, int64(500_000), info.PrincipalPart)
	require.Zero(t, info.FeePart)
	require.Equal(t, quotaOf(t, "2.00"), info.PenaltyPart)
	require.Zero(t, info.DebtAfter)

	// 借款人余额：500,000 - 500,000(本) - 1,000,000(惩罚) = -1,000,000（允许为负）
	require.Equal(t, int64(-1_000_000), userQuota(t, borrower.Id))

	// funding 转结清
	var f TokenLoanFunding
	require.NoError(t, DB.First(&f, fundings[0].Id).Error)
	require.Equal(t, LoanFundingRepaid, f.Status)
	require.Zero(t, f.DebtQuota)

	// 放贷人入账 = 惩罚 1,000,000（同日无利息；本金回补 offer 不进余额）
	require.Len(t, credits, 1)
	require.Equal(t, lender.Id, credits[0].UserId)
	require.Equal(t, quotaOf(t, "2.00"), credits[0].Amount)
	require.Equal(t, int64(5_000_000), userQuota(t, lender.Id))

	// 台账：单条 repay 行携带 penalty_part
	rows := repayLedgerRows(t, borrower.Id)
	require.Len(t, rows, 1)
	require.Equal(t, int64(500_000), rows[0].Amount)
	require.Equal(t, quotaOf(t, "2.00"), rows[0].PenaltyPart)
}

// ② 超过惩罚窗口（funding 5 天前投放、窗口 2 天）全额还清 → 不计罚。
func TestFastRepayPenaltyOutsideWindow(t *testing.T) {
	borrower, lender := setupFastRepayTest(t)
	createPenaltyOffer(t, lender.Id, 1_000_000, 1_000_000, quotaOf(t, "2.00"), 2)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 4_000_000).Error)

	fundings := borrowFromMarket(t, borrower.Id, "1.00")
	// 回拨 funding 投放日到 5 天前：5 > 2 → 窗口外
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", fundings[0].Id).
		Update("created_at", time.Now().AddDate(0, 0, -5).Unix()).Error)

	acc, info, credits, err := RepayLoan(borrower.Id, "all")
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Zero(t, info.PenaltyPart)
	require.Equal(t, int64(500_000), info.Amount)

	// 只扣本息：500,000 - 500,000 = 0
	require.Equal(t, int64(0), userQuota(t, borrower.Id))
	// 放贷人无任何入账（无利息、无惩罚）
	require.Empty(t, credits)
	require.Equal(t, int64(4_000_000), userQuota(t, lender.Id))
	// 台账 penalty_part 为 0
	rows := repayLedgerRows(t, borrower.Id)
	require.Len(t, rows, 1)
	require.Zero(t, rows[0].PenaltyPart)
}

// ③ 部分还款（funding 未转结清）→ 不计罚。
func TestFastRepayPenaltyPartialRepay(t *testing.T) {
	borrower, lender := setupFastRepayTest(t)
	createPenaltyOffer(t, lender.Id, 2_000_000, 2_000_000, quotaOf(t, "2.00"), 3)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 4_000_000).Error)

	fundings := borrowFromMarket(t, borrower.Id, "2.00") // 借 2.00 = 1,000,000 quota
	require.Len(t, fundings, 1)

	// 还 1.00：债务剩 500,000，funding 未结清
	acc, info, credits, err := RepayLoan(borrower.Id, "1.00")
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Equal(t, int64(500_000), info.Amount)
	require.Zero(t, info.PenaltyPart)

	// 余额：借得 1,000,000 - 还 500,000 = 500,000（未扣惩罚）
	require.Equal(t, int64(500_000), userQuota(t, borrower.Id))
	require.Empty(t, credits)
	require.Equal(t, int64(4_000_000), userQuota(t, lender.Id))

	var f TokenLoanFunding
	require.NoError(t, DB.First(&f, fundings[0].Id).Error)
	require.Equal(t, LoanFundingActive, f.Status)
	rows := repayLedgerRows(t, borrower.Id)
	require.Len(t, rows, 1)
	require.Zero(t, rows[0].PenaltyPart)
}

// ④ 签到自动还款全额结清（即使 funding 带惩罚条款）→ 不计罚（与平台手续费同为
// 手动还款专属）。
func TestFastRepayPenaltyCheckinAutoRepay(t *testing.T) {
	withCheckinSetting(t, 1_000_000) // 奖励 1,000,000 quota ≥ 债务
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	borrower := setupCheckinLoanUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, 0, lender.Id)
	offer := createPenaltyOffer(t, lender.Id, 500_000, 500_000, quotaOf(t, "2.00"), 30)
	createLoanDebtAccount(t, borrower.Id, 500_000, 500_000)
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanFunding{
		LoanUserId:            borrower.Id,
		SourceType:            LoanFundingPool,
		OfferId:               offer.Id,
		LenderId:              lender.Id,
		Amount:                500_000,
		PrincipalRemaining:    500_000,
		DebtQuota:             500_000,
		LastSettledDay:        loanDay(now),
		Rate:                  0.001,
		RepayPlan:             LoanRepayFull,
		Status:                LoanFundingActive,
		DueDay:                loanDay(now) + 30,
		FastRepayPenaltyQuota: quotaOf(t, "2.00"),
		FastRepayWindowDays:   30,
		CreatedAt:             now.Unix(),
		UpdatedAt:             now.Unix(),
	}).Error)

	_, repay, credits, err := UserCheckin(borrower.Id, 0)
	require.NoError(t, err)
	require.NotNil(t, repay)
	require.Equal(t, int64(500_000), repay.Amount)
	require.Zero(t, repay.PenaltyPart, "签到自动还款不得触发秒结清惩罚")

	// 净入账 = 奖励 - 还款 = 500,000（若误扣惩罚会变成 -500,000）
	require.Equal(t, int64(500_000), userQuota(t, borrower.Id))
	// 放贷人只有利息（同日为 0），无惩罚入账
	for _, c := range credits {
		require.NotEqual(t, quotaOf(t, "2.00"), c.Amount)
	}
	require.Equal(t, int64(0), userQuota(t, lender.Id))
	// 台账 penalty_part 为 0
	rows := repayLedgerRows(t, borrower.Id)
	require.Len(t, rows, 1)
	require.Zero(t, rows[0].PenaltyPart)
}

// ⑤ 两条 funding、不同惩罚额度同时结清 → 各放贷人各得其所，借款人付总和。
func TestFastRepayPenaltyTwoFundingsDifferentPenalties(t *testing.T) {
	borrower, lender1 := setupFastRepayTest(t)
	lender2 := createLoanTestUser(t)
	cleanupLoanBorrowData(t, 0, lender2.Id)
	// 两条 pool offer 同利率、可撮合额度均小于借款额：撮合按 id 升序各吃一部分，
	// 保证两条 funding 都生成（单个 offer 不足以覆盖整笔借款）
	createPenaltyOffer(t, lender1.Id, 600_000, 600_000, quotaOf(t, "1.00"), 3) // 吃 600,000
	createPenaltyOffer(t, lender2.Id, 600_000, 600_000, quotaOf(t, "2.00"), 3) // 吃 400,000
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender1.Id).Update("quota", 4_000_000).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender2.Id).Update("quota", 4_000_000).Error)

	fundings := borrowFromMarket(t, borrower.Id, "2.00")
	require.Len(t, fundings, 2)
	// 撮合吃量顺序 = offer id 升序：第一条 600,000（lender1），第二条 400,000（lender2）
	require.Equal(t, int64(600_000), fundings[0].Amount)
	require.Equal(t, lender1.Id, fundings[0].LenderId)
	require.Equal(t, int64(400_000), fundings[1].Amount)
	require.Equal(t, lender2.Id, fundings[1].LenderId)

	acc, info, credits, err := RepayLoan(borrower.Id, "all")
	require.NoError(t, err)
	require.NotNil(t, acc)
	// 惩罚 = 两档之和；第二档 1,000,000 超 2×本金（2×400,000），按 800,000 封顶
	require.Equal(t, int64(1_300_000), info.PenaltyPart, "惩罚 = 两档之和（第二档 2×本金封顶）")

	// 借款人：借得 1,000,000 - 本息 1,000,000 - 惩罚 1,300,000 = -1,300,000
	require.Equal(t, int64(-1_300_000), userQuota(t, borrower.Id))

	// 各放贷人收到各自惩罚（同日无利息）
	require.Len(t, credits, 2)
	byLender := map[int]int64{}
	for _, c := range credits {
		byLender[c.UserId] = c.Amount
	}
	require.Equal(t, quotaOf(t, "1.00"), byLender[lender1.Id])
	require.Equal(t, int64(800_000), byLender[lender2.Id])
	require.Equal(t, int64(4_500_000), userQuota(t, lender1.Id))
	require.Equal(t, int64(4_800_000), userQuota(t, lender2.Id))

	// 台账两条 repay 行，各自携带自己的惩罚份额
	rows := repayLedgerRows(t, borrower.Id)
	require.Len(t, rows, 2)
	byFunding := map[int64]int64{}
	for _, r := range rows {
		byFunding[r.FundingId] = r.PenaltyPart
	}
	require.Equal(t, quotaOf(t, "1.00"), byFunding[fundings[0].Id])
	require.Equal(t, int64(800_000), byFunding[fundings[1].Id])
}

// ⑥ 窗口 0 = 仅当天：次日还款不计罚；同日还款照常计罚。
func TestFastRepayPenaltyWindowZero(t *testing.T) {
	// 次日还款 → 不计罚
	borrower, lender := setupFastRepayTest(t)
	createPenaltyOffer(t, lender.Id, 1_000_000, 1_000_000, quotaOf(t, "2.00"), 0)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 4_000_000).Error)

	fundings := borrowFromMarket(t, borrower.Id, "1.00")
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", fundings[0].Id).
		Update("created_at", time.Now().AddDate(0, 0, -1).Unix()).Error)

	acc, info, _, err := RepayLoan(borrower.Id, "all")
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Zero(t, info.PenaltyPart)
	require.Equal(t, int64(0), userQuota(t, borrower.Id))

	// 同日还款 → 计罚（窗口 0 允许当天）
	borrower2, lender2 := setupFastRepayTest(t)
	createPenaltyOffer(t, lender2.Id, 1_000_000, 1_000_000, quotaOf(t, "2.00"), 0)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender2.Id).Update("quota", 4_000_000).Error)
	borrowFromMarket(t, borrower2.Id, "1.00")

	_, info2, _, err := RepayLoan(borrower2.Id, "all")
	require.NoError(t, err)
	require.Equal(t, quotaOf(t, "2.00"), info2.PenaltyPart)
	require.Equal(t, int64(-1_000_000), userQuota(t, borrower2.Id))
}

// ⑦ 惩罚条款在放款时从 offer 复制到 funding（CreateLoanOffer → BorrowLoan 全链路）。
func TestFastRepayPenaltyCopiedOfferToFunding(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	borrower := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, 0)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 10_000_000
		s.MaxPerBorrow = 0
		s.LoanTermDays = 30
	})

	offer, err := CreateLoanOffer(lender.Id, LoanOfferModeOrder, "2.00", "0.001", 0, 0, 0, -50, "1.50", 7)
	require.NoError(t, err)
	require.Equal(t, quotaOf(t, "1.50"), offer.FastRepayPenaltyQuota)
	require.Equal(t, 7, offer.FastRepayWindowDays)

	acc, fundings, err := BorrowLoan(borrower.Id, "1.00", offer.Id, nil) // 定向挂单
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Len(t, fundings, 1)
	require.Equal(t, offer.Id, fundings[0].OfferId)
	require.Equal(t, offer.FastRepayPenaltyQuota, fundings[0].FastRepayPenaltyQuota, "惩罚额度应随放款复制")
	require.Equal(t, offer.FastRepayWindowDays, fundings[0].FastRepayWindowDays, "惩罚窗口应随放款复制")

	// platform 兜底 funding 恒无惩罚条款：关闭市场后整笔借款走平台兜底
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.MarketEnabled = false
	})
	_, pf, err := BorrowLoan(borrower.Id, "0.50", 0, nil)
	require.NoError(t, err)
	for _, f := range pf {
		if f.SourceType == LoanFundingPlatform {
			require.Zero(t, f.FastRepayPenaltyQuota)
			require.Zero(t, f.FastRepayWindowDays)
		}
	}
}

// CreateLoanOffer 惩罚参数校验：负数/超过两位小数/窗口越界 → ErrLoanOfferInvalidParams；
// 空惩罚 = 0 不收；合法值原样落库。
func TestCreateLoanOfferFastRepayParamsValidation(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))

	_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50, "", -1)
	require.ErrorIs(t, err, ErrLoanOfferInvalidParams)
	_, err = CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50, "", 366)
	require.ErrorIs(t, err, ErrLoanOfferInvalidParams)
	_, err = CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50, "-1.00", 3)
	require.ErrorIs(t, err, ErrLoanOfferInvalidParams)
	_, err = CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50, "1.005", 3)
	require.ErrorIs(t, err, ErrLoanOfferInvalidParams)

	offer, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50, "", 0)
	require.NoError(t, err)
	require.Zero(t, offer.FastRepayPenaltyQuota)
	require.Zero(t, offer.FastRepayWindowDays)

	offer2, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50, "1.50", 7)
	require.NoError(t, err)
	require.Equal(t, quotaOf(t, "1.50"), offer2.FastRepayPenaltyQuota)
	require.Equal(t, 7, offer2.FastRepayWindowDays)
}

// ⑧ 高余额放贷人回归（生产 bug）：放贷人余额超过 int32 上界时，借款人结清触发的
// 惩罚入账不得再按 int32 口径溢出回滚（生产库 quota 列为 bigint，词元贷 64 位口径）
func TestFastRepayPenaltyLenderBeyondInt32Quota(t *testing.T) {
	borrower, lender := setupFastRepayTest(t)
	// 隔离共享库中其他用例残留的 active offer，保证撮合只命中本用例的挂单
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanOffer{}).Error)
	createPenaltyOffer(t, lender.Id, 1_000_000, 1_000_000, quotaOf(t, "2.00"), 3)
	// 放贷人余额远超 int32 上界（模拟生产 root 的超大余额）
	highBalance := int64(math.MaxInt32) + 5_000_000
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", highBalance).Error)

	fundings := borrowFromMarket(t, borrower.Id, "1.00")
	require.Len(t, fundings, 1)

	acc, info, credits, err := RepayLoan(borrower.Id, "all")
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Equal(t, quotaOf(t, "2.00"), info.PenaltyPart)
	require.Len(t, credits, 1)
	require.Equal(t, lender.Id, credits[0].UserId)
	require.Equal(t, highBalance+quotaOf(t, "2.00"), userQuota(t, lender.Id))
}

// ⑨ 只接官方资金：市场开启且有 pool 挂单时，platformOnly 借款整笔走平台兜底，
// 不动用任何市场挂单（offer 余额不变），funding 无惩罚条款
func TestBorrowLoanPlatformOnlySkipsMarket(t *testing.T) {
	borrower, lender := setupFastRepayTest(t)
	// 隔离共享库中其他用例残留的 active offer
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanOffer{}).Error)
	offer := createPenaltyOffer(t, lender.Id, 1_000_000, 1_000_000, quotaOf(t, "2.00"), 3)

	acc, fundings, err := BorrowLoanWithOptions(borrower.Id, "1.00", 0, nil, true)
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Len(t, fundings, 1)
	require.Equal(t, LoanFundingPlatform, fundings[0].SourceType)
	require.Zero(t, fundings[0].OfferId)
	require.Zero(t, fundings[0].FastRepayPenaltyQuota)

	// offer 未被动用
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, int64(1_000_000), got.AmountAvailable)
	require.Zero(t, got.TotalLent)
}

// ⑩ 秒结清惩罚随放款复制时按"不超过本笔借出金额 2 倍"封顶：
// offer 惩罚 2.00 USD，借 0.50 → funding 惩罚封顶 1.00 USD
func TestFastRepayPenaltyCappedAtTwiceFundingAmount(t *testing.T) {
	borrower, lender := setupFastRepayTest(t)
	// 隔离共享库中其他用例残留的 active offer，保证撮合只命中本用例的挂单
	require.NoError(t, DB.Where("1 = 1").Delete(&TokenLoanOffer{}).Error)
	createPenaltyOffer(t, lender.Id, 1_000_000, 1_000_000, quotaOf(t, "2.00"), 3)

	fundings := borrowFromMarket(t, borrower.Id, "0.50") // 250,000 quota
	require.Len(t, fundings, 1)
	require.Equal(t, int64(500_000), fundings[0].FastRepayPenaltyQuota, "惩罚按 2×本金封顶")

	// 结清时实际收取封顶后的惩罚
	require.NoError(t, DB.Model(&User{}).Where("id = ?", borrower.Id).Update("quota", 2_000_000).Error)
	_, info, _, err := RepayLoan(borrower.Id, "all")
	require.NoError(t, err)
	require.Equal(t, int64(500_000), info.PenaltyPart)
}

// ⑪ CreateLoanOffer：惩罚超过挂出金额 2 倍 → ErrLoanOfferInvalidParams（penalty_exceeds）
func TestCreateLoanOfferPenaltyExceedsTwiceOfferAmount(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50, "2.01", 3)
	require.ErrorIs(t, err, ErrLoanOfferInvalidParams)
	var pe *LoanOfferParamError
	require.ErrorAs(t, err, &pe)
	require.Equal(t, "penalty_exceeds", pe.Reason)
	// 恰好 2 倍放行
	offer, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50, "2.00", 3)
	require.NoError(t, err)
	require.Equal(t, quotaOf(t, "2.00"), offer.FastRepayPenaltyQuota)
}
