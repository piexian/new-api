package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// ===== Task 13: 信用分引擎测试 =====
// 换算基准：common.QuotaPerUnit = 500000，即 1 USD = 500000 quota。
// 共享内存库中 SQLite 会复用被删用户 id，各用例在开头按 user_id 清理名下残留行。

// setupCreditTest 开启词元贷 + 显式固定信用分参数（防默认值漂移）
func setupCreditTest(t *testing.T) *User {
	t.Helper()
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.CreditInitial = 50
		s.CreditRepayBonus = 5
		s.CreditFastRepayPenalty = 2
		s.CreditDefaultPenalty = 20
		s.CreditMinHoldDays = 3
		s.CreditMinBorrowUsd = 1.0
		s.LoanTermDays = 30
	})
	user := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, 0)
	return user
}

// createCreditAccount 直接建信用分账户行（start 为初始分）
func createCreditAccount(t *testing.T, userId, start int) *TokenLoanAccount {
	t.Helper()
	now := time.Now()
	acc := &TokenLoanAccount{
		UserId:         userId,
		CreditScore:    start,
		LastSettledDay: loanDay(now),
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}
	require.NoError(t, DB.Create(acc).Error)
	return acc
}

// createCreditBorrowEvent 直接建 borrow 台账行（借款事件），返回事件 id
func createCreditBorrowEvent(t *testing.T, userId int, amount int64, createdAt time.Time) int64 {
	t.Helper()
	rec := &TokenLoanRecord{
		UserId:        userId,
		Type:          "borrow",
		Amount:        amount,
		PrincipalPart: amount,
		DebtAfter:     amount,
		Source:        "manual",
		CreatedAt:     createdAt.Unix(),
	}
	require.NoError(t, DB.Create(rec).Error)
	return int64(rec.Id)
}

// createCreditFunding 直接建一条事件 funding（active，债务=本金，当日结算无息）
func createCreditFunding(t *testing.T, userId int, eventId int64, principal int64, dueDay int) *TokenLoanFunding {
	t.Helper()
	now := time.Now()
	f := &TokenLoanFunding{
		LoanUserId:         userId,
		BorrowEventId:      eventId,
		SourceType:         LoanFundingPlatform,
		Amount:             principal,
		PrincipalRemaining: principal,
		DebtQuota:          principal,
		LastSettledDay:     loanDay(now),
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             dueDay,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	return f
}

// ① 按时全额还清且事件本金 ≥ 门槛 → +CreditRepayBonus
func TestCreditScoreOnTimeFullRepayBonus(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	// 持有 10 天（>= 3 天），本金 2 USD（>= 1 USD 门槛），到期日未过
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -10))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 55, acc.CreditScore, "按时还清 +5")
}

// ①b 加分上限 100 钳制：98 + 5 → 100
func TestCreditScoreBonusClampsCeiling100(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -10))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 98)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 100, acc.CreditScore, "98 + 5 钳制到上限 100")
}

// ② 持有 < CreditMinHoldDays 即全额还清 → -CreditFastRepayPenalty（且不得加分）
func TestCreditScoreFastRepayPenalty(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	// 持有 2 天（< 3 天），本金 2 USD 满足门槛——反刷分优先于加分
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -2))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 48, acc.CreditScore, "快速还清 -2，不得加分")
}

// ③ 事件本金低于 CreditMinBorrowUsd（刷分墙）→ 不加分不扣分
func TestCreditScoreBelowMinBorrowUsdNoScore(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	// 持有 10 天满足最短持有，但本金仅 0.2 USD（< 1.0 USD 门槛）
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*0.2), now.AddDate(0, 0, -10))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*0.2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 50, acc.CreditScore, "低于金额门槛不加分不扣分")
}

// ④ 逾期后全额还清 → 不加分不扣分（max(due_day) 已过）
func TestCreditScoreLateRepayNoChange(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	// 持有 10 天满足最短持有，但到期日 day-5 已过
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -10))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day-5)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 50, acc.CreditScore, "逾期后还清不加分不扣分")
}

// ⑤ 多 funding 事件在一次 distributeRepayment 中全部结清 → 事件级加分恰一次（+5 而非 +10）
func TestCreditScoreMultiFundingEventScoresOnce(t *testing.T) {
	borrower := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	// 事件拆成两条 funding：1.2 USD + 0.8 USD = 2 USD，同一 BorrowEventId
	eventId := createCreditBorrowEvent(t, borrower.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -10))
	f1 := createCreditFunding(t, borrower.Id, eventId, int64(common.QuotaPerUnit*1.2), day+30)
	f2 := createCreditFunding(t, borrower.Id, eventId, int64(common.QuotaPerUnit*0.8), day+30)

	acc := createCreditAccount(t, borrower.Id, 50)
	// 一次还款覆盖整事件（债务=本金无息，全额结清两条 funding）
	info, _, _, err := distributeRepayment(DB, acc, []TokenLoanFunding{*f1, *f2}, int64(common.QuotaPerUnit*2), now)
	require.NoError(t, err)
	require.Zero(t, info.DebtAfter)
	require.Equal(t, 55, acc.CreditScore, "事件级加分只结算一次（+5 而非 +10）")

	// 两条 funding 均已落盘为 repaid（此后无 active/overdue 行 → 不可能再触发第二次评分）
	var f1got, f2got TokenLoanFunding
	require.NoError(t, DB.First(&f1got, f1.Id).Error)
	require.NoError(t, DB.First(&f2got, f2.Id).Error)
	require.Equal(t, LoanFundingRepaid, f1got.Status)
	require.Equal(t, LoanFundingRepaid, f2got.Status)
}

// ⑤b 事件未完全结清 → 跳过评分；最后一条结清时才评分一次
func TestCreditScoreEventNotFullySettledSkips(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -10))
	f1 := createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit), day+30)
	f2 := createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", f1.Id).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	// 只有一条结清：事件未完成，不得评分
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 50, acc.CreditScore, "事件未完全结清时不得评分")

	// 第二条结清：此刻才评分恰一次
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", f2.Id).
		Update("status", LoanFundingRepaid).Error)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 55, acc.CreditScore, "最后一条结清时评分恰一次")
}

// ⑥ 账户不存在 → GetCreditScore 返回 CreditInitial；有账户时返回实际分值
func TestGetCreditScoreMissingAccountReturnsInitial(t *testing.T) {
	user := setupCreditTest(t)
	score, err := GetCreditScore(user.Id)
	require.NoError(t, err)
	require.Equal(t, 50, score, "无账户返回信用分初始值")

	createCreditAccount(t, user.Id, 77)
	score, err = GetCreditScore(user.Id)
	require.NoError(t, err)
	require.Equal(t, 77, score)
}

// ⑦ legacy funding（BorrowEventId=0，迁移生成）跳过评分
func TestCreditScoreLegacyFundingSkipped(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	createCreditFunding(t, user.Id, 0, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", 0).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, 0, now))
	require.Equal(t, 50, acc.CreditScore, "legacy funding 不参与事件评分")
}

// ⑧ 快速还清扣分下限 -50 钳制：-49 - 2 → -50，不再下探
func TestCreditScoreFastRepayClampsFloorMinus50(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -1))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, -49)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, -50, acc.CreditScore, "-49 - 2 → -50，不再下探")
}

// ⑨ borrow 台账行缺失（防御）→ 跳过评分
func TestCreditScoreMissingBorrowRecordSkipped(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	// 事件 id 无对应 borrow 行（数据异常场景）
	createCreditFunding(t, user.Id, 424242, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", 424242).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, 424242, now))
	require.Equal(t, 50, acc.CreditScore, "borrow 行缺失不评分")
}

// ===== 信用分变动入台账（type=credit）=====

// ⑩ 按时还清加分写 credit 台账行：+5，DebtAfter=变动后信用分，Source=repay_bonus，
// RefId=借款事件 id
func TestCreditLedgerRowOnRepayBonus(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -10))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 55, acc.CreditScore)

	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "credit").First(&rec).Error)
	require.Equal(t, int64(5), rec.Amount)
	require.Equal(t, int64(55), rec.DebtAfter)
	require.Equal(t, "repay_bonus", rec.Source)
	require.Equal(t, eventId, rec.RefId)
	require.Zero(t, rec.FundingId)
	require.Zero(t, rec.LenderId)
	require.Zero(t, rec.InterestPart)
	require.Zero(t, rec.PrincipalPart)
	require.Zero(t, rec.FeePart)
}

// ⑪ 快速还清扣分写 credit 台账行：-2，DebtAfter=变动后信用分，Source=fast_repay
func TestCreditLedgerRowOnFastRepayPenalty(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -2))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 50)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 48, acc.CreditScore)

	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "credit").First(&rec).Error)
	require.Equal(t, int64(-2), rec.Amount)
	require.Equal(t, int64(48), rec.DebtAfter)
	require.Equal(t, "fast_repay", rec.Source)
	require.Equal(t, eventId, rec.RefId)
}

// ⑫ 加分上限 100 钳制时记录实际生效的 delta：98 + 5 → 100，台账记 +2 而非 +5
func TestCreditLedgerRowBonusClampRecordsActualDelta(t *testing.T) {
	user := setupCreditTest(t)
	now := time.Now()
	day := loanDay(now)
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -10))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, 98)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, 100, acc.CreditScore)

	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "credit").First(&rec).Error)
	require.Equal(t, int64(2), rec.Amount, "钳制后记录实际生效的 delta（100 - 98），而非名义 +5")
	require.Equal(t, int64(100), rec.DebtAfter)
}

// ⑬ 扣分下限 -50 钳制时记录实际生效的 delta：-45 - 20 → -50，台账记 -5 而非 -20
func TestCreditLedgerRowFastRepayRecordsClampedDelta(t *testing.T) {
	user := setupCreditTest(t)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.CreditFastRepayPenalty = 20 })
	now := time.Now()
	day := loanDay(now)
	eventId := createCreditBorrowEvent(t, user.Id, int64(common.QuotaPerUnit*2), now.AddDate(0, 0, -2))
	createCreditFunding(t, user.Id, eventId, int64(common.QuotaPerUnit*2), day+30)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("borrow_event_id = ?", eventId).
		Update("status", LoanFundingRepaid).Error)

	acc := createCreditAccount(t, user.Id, -45)
	require.NoError(t, scoreBorrowEventRepaidTx(DB, acc, eventId, now))
	require.Equal(t, -50, acc.CreditScore, "-45 - 20 钳制到下限 -50")

	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "credit").First(&rec).Error)
	require.Equal(t, int64(-5), rec.Amount, "钳制后记录实际生效的 delta（-50 - (-45)），而非名义 -20")
	require.Equal(t, int64(-50), rec.DebtAfter)
	require.Equal(t, "fast_repay", rec.Source)
}
