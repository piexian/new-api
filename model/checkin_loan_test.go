package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// ===== Task 10: 签到还款按 funding 分配 + 逾期 100% 扣还测试 =====
// Task 10 起签到还款走单一事务路径（与 BorrowLoan/RepayLoan 同模式），旧版 SQLite 的
// 顺序执行 + 手动回滚分支已合并删除，本文件全部用例在共享 SQLite（TestMain，
// MaxOpenConns=1）上运行，同时充当"单事务分支在 SQLite 下可用"的回归覆盖（⑤）。
// 共享内存库中 SQLite 会复用被删用户 id，各用例在开头按 user_id 清理名下残留行。

// withCheckinSetting 临时启用签到并固定奖励额度，测试结束后恢复
func withCheckinSetting(t *testing.T, quota int) {
	t.Helper()
	setting := operation_setting.GetCheckinSetting()
	old := *setting
	setting.Enabled = true
	setting.MinQuota = quota
	setting.MaxQuota = quota
	t.Cleanup(func() { *setting = old })
}

// setupCheckinLoanUser 迁移签到表并创建测试用户（贷款表由 createLoanTestUser 迁移）；
// 共享库 SQLite 复用被删用户 id，先清掉 id 名下残留的签到与贷款行
func setupCheckinLoanUser(t *testing.T) *User {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Checkin{}))
	user := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, 0)
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&Checkin{}).Error)
	return user
}

// createLoanDebtAccount 创建贷款账户行（getLoanAccountTx 需要账户存在才进入还款路径）。
// LastSettledDay=今天，结算当天不再计息，保证债务数值确定；账户行的债务/本金会被
// funding 投影（syncAccountFromFundings）覆盖，此处数值仅作初始输入。
func createLoanDebtAccount(t *testing.T, userId int, principal, debt int64) {
	t.Helper()
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:         userId,
		PrincipalQuota: principal,
		DebtQuota:      debt,
		TotalBorrowed:  principal,
		LastSettledDay: loanDay(now),
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}).Error)
}

// createCheckinFunding 直接建一条 platform funding：LastSettledDay=今天，债务数值确定。
// active 的 DueDay 推后 30 天；overdue 的 DueDay 置昨天（结算段长为 0，不产生罚息）。
func createCheckinFunding(t *testing.T, userId int, principal, debt int64, status string) *TokenLoanFunding {
	t.Helper()
	now := time.Now()
	day := loanDay(now)
	dueDay := day + 30
	if status == LoanFundingOverdue {
		dueDay = day - 1
	}
	f := &TokenLoanFunding{
		LoanUserId:         userId,
		SourceType:         LoanFundingPlatform,
		Amount:             principal,
		PrincipalRemaining: principal,
		DebtQuota:          debt,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             status,
		DueDay:             dueDay,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	return f
}

func checkinUserQuota(t *testing.T, userId int) int {
	t.Helper()
	var u User
	require.NoError(t, DB.Select("quota").First(&u, userId).Error)
	return u.Quota
}

func countLoanRecords(t *testing.T, userId int, recordType string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).
		Where("user_id = ? AND type = ?", userId, recordType).Count(&count).Error)
	return count
}

// ④ 回归：无贷款账户时 loanRepay=nil 且行为不变：全额入账、无还款台账、不给无贷用户建账户行
func TestUserCheckinNoDebtNoRepay(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)

	checkin, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.Nil(t, repay)
	require.Equal(t, 5000, checkin.QuotaAwarded)
	require.Equal(t, 5000, checkinUserQuota(t, user.Id))
	require.Equal(t, int64(0), countLoanRecords(t, user.Id, "repay"))

	// 无贷用户签到后不产生 token_loan_accounts 行
	var accCount int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", user.Id).Count(&accCount).Error)
	require.Equal(t, int64(0), accCount)
}

// ① funding 级 pro-rata 拆分：两条 platform funding 债务 [6000,4000]（利息各 2000/1000），
// 奖励 5000 < Σ债务 10000 → 按 3:2 分配：A 3000（息 2000 本 1000）、B 2000（息 1000 本 1000）；
// 净额 0；台账按 funding 各一行、挂 funding_id
func TestUserCheckinRepaySplitsAcrossFundings(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	fA := createCheckinFunding(t, user.Id, 4000, 6000, LoanFundingActive)
	fB := createCheckinFunding(t, user.Id, 3000, 4000, LoanFundingActive)
	createLoanDebtAccount(t, user.Id, 7000, 10000)

	checkin, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.Equal(t, 5000, checkin.QuotaAwarded) // quota_awarded 保持 gross
	require.NotNil(t, repay)
	require.Equal(t, int64(5000), repay.Amount)
	require.Equal(t, int64(3000), repay.InterestPart)
	require.Equal(t, int64(2000), repay.PrincipalPart)
	require.Equal(t, int64(5000), repay.DebtAfter)

	// 两条 funding 各自按配额定额，均未结清
	var a, b TokenLoanFunding
	require.NoError(t, DB.First(&a, fA.Id).Error)
	require.NoError(t, DB.First(&b, fB.Id).Error)
	require.Equal(t, int64(3000), a.DebtQuota)
	require.Equal(t, int64(3000), a.PrincipalRemaining)
	require.Equal(t, LoanFundingActive, a.Status)
	require.Equal(t, int64(2000), b.DebtQuota)
	require.Equal(t, int64(2000), b.PrincipalRemaining)
	require.Equal(t, LoanFundingActive, b.Status)

	// 账户投影：债务/本金 = Σ funding，TotalRepaid 累计
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(5000), acc.DebtQuota)
	require.Equal(t, int64(5000), acc.PrincipalQuota)
	require.Equal(t, int64(5000), acc.TotalRepaid)

	// 净额 0：用户余额不变
	require.Equal(t, 0, checkinUserQuota(t, user.Id))

	// 台账：两条 repay 行（checkin，挂 funding_id），Σ = 5000
	require.Equal(t, int64(2), countLoanRecords(t, user.Id, "repay"))
	var recs []TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "repay").
		Order("funding_id ASC").Find(&recs).Error)
	require.Len(t, recs, 2)
	require.Equal(t, fA.Id, recs[0].FundingId)
	require.Equal(t, int64(3000), recs[0].Amount)
	require.Equal(t, int64(2000), recs[0].InterestPart)
	require.Equal(t, int64(1000), recs[0].PrincipalPart)
	require.Equal(t, int64(3000), recs[0].DebtAfter)
	require.Equal(t, fB.Id, recs[1].FundingId)
	require.Equal(t, int64(2000), recs[1].Amount)
	require.Equal(t, int64(1000), recs[1].InterestPart)
	require.Equal(t, int64(1000), recs[1].PrincipalPart)
	require.Equal(t, int64(2000), recs[1].DebtAfter)
	require.Equal(t, "checkin", recs[0].Source)
}

// ② 放贷人入账 + offer 回补：债务 250000（本金 245000 + 利息 5000），奖励 10000
// → 抵息 5000、抵本 5000：放贷人余额 +5000（利息），offer.amount_available 回补 5000、
// amount_total 不变、TotalInterestEarned +5000；台账挂 funding_id/lender_id
func TestUserCheckinRepayCreditsLenderAndRefillsOffer(t *testing.T) {
	withCheckinSetting(t, 10000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, lender.Id)
	now := time.Now()
	day := loanDay(now)
	offer := createRepayOffer(t, lender.Id, 400000, 100000) // total 400000, available 100000
	f := &TokenLoanFunding{
		LoanUserId:         user.Id,
		SourceType:         LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           lender.Id,
		Amount:             245000,
		PrincipalRemaining: 245000,
		DebtQuota:          250000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	createLoanDebtAccount(t, user.Id, 245000, 250000)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 0).Error)
	// 清理本用例的 funding 行：共享库 SQLite 复用被删 offer id，残留的 offer 关联 funding
	// 会污染后续用例的 offer 不变式统计（activeOrOverduePrincipalSum）
	t.Cleanup(func() {
		_ = DB.Where("id = ?", f.Id).Delete(&TokenLoanFunding{}).Error
	})

	_, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.NotNil(t, repay)
	require.Equal(t, int64(10000), repay.Amount)
	require.Equal(t, int64(5000), repay.InterestPart)
	require.Equal(t, int64(5000), repay.PrincipalPart)
	require.Equal(t, int64(240000), repay.DebtAfter)

	// 放贷人余额：利息入账 5000
	var lu User
	require.NoError(t, DB.Select("quota").First(&lu, lender.Id).Error)
	require.Equal(t, 5000, lu.Quota)
	// offer：available 100000+5000=105000、total 不变、TotalInterestEarned=5000
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, int64(105000), got.AmountAvailable)
	require.Equal(t, int64(400000), got.AmountTotal)
	require.Equal(t, int64(5000), got.TotalInterestEarned)
	// funding：债务 240000/本金 240000，仍 active
	var gf TokenLoanFunding
	require.NoError(t, DB.First(&gf, f.Id).Error)
	require.Equal(t, int64(240000), gf.DebtQuota)
	require.Equal(t, int64(240000), gf.PrincipalRemaining)
	require.Equal(t, LoanFundingActive, gf.Status)

	// 净额 0：借款人余额不变；台账挂 funding_id/lender_id、source=checkin
	require.Equal(t, 0, checkinUserQuota(t, user.Id))
	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "repay").First(&rec).Error)
	require.Equal(t, "checkin", rec.Source)
	require.Equal(t, f.Id, rec.FundingId)
	require.Equal(t, lender.Id, rec.LenderId)
	require.Equal(t, int64(10000), rec.Amount)
	require.Equal(t, int64(5000), rec.InterestPart)
	require.Equal(t, int64(5000), rec.PrincipalPart)
	require.Equal(t, int64(240000), rec.DebtAfter)
}

// ③ 逾期 funding → 签到奖励 100% 抵债（spec §7.6）：奖励 5000 ≤ 债务 10000 时净额 0、
// 全额抵债。该公式与正常模式一致（repay = min(奖励, Σ债务)，奖励大于债务时超额仍入账），
// 逾期仅经 settleFunding 罚息放大债务、不改变分配公式——此处文档化等价性而非分支。
// 未全额结清时 funding 保持 overdue；HasOverdueFundings 识别违约期。
func TestUserCheckinRepayOverdueFullAwardConsumed(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	// 逾期 funding：本金 8000、债务 10000（利息 2000）；LastSettledDay=今天、DueDay=昨天，
	// 结算分段长为 0 不产生罚息，债务数值确定
	f := createCheckinFunding(t, user.Id, 8000, 10000, LoanFundingOverdue)
	createLoanDebtAccount(t, user.Id, 8000, 10000)

	// 前提：处于违约期
	overdue, err := HasOverdueFundings(DB, user.Id)
	require.NoError(t, err)
	require.True(t, overdue)

	checkin, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.Equal(t, 5000, checkin.QuotaAwarded)
	require.NotNil(t, repay)
	require.Equal(t, int64(5000), repay.Amount)
	require.Equal(t, int64(2000), repay.InterestPart)
	require.Equal(t, int64(3000), repay.PrincipalPart)
	require.Equal(t, int64(5000), repay.DebtAfter)

	// 净额 0：奖励全额抵债，用户余额不变
	require.Equal(t, 0, checkinUserQuota(t, user.Id))

	// 未结清：funding 债务 5000，状态保持 overdue（违约期仍在）
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, int64(5000), got.DebtQuota)
	require.Equal(t, int64(5000), got.PrincipalRemaining)
	require.Equal(t, LoanFundingOverdue, got.Status)
	overdue, err = HasOverdueFundings(DB, user.Id)
	require.NoError(t, err)
	require.True(t, overdue)

	// 台账一条 repay 行（checkin，挂 funding_id）
	require.Equal(t, int64(1), countLoanRecords(t, user.Id, "repay"))
}

// ⑤ 奖励 > 债务：清账后净额入账（DB quota 增量 = 净额），funding 转 repaid
func TestUserCheckinRepayClearsDebt(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	// 本金 2500，债务 3000（利息 500），奖励 5000 清账后净额 2000
	f := createCheckinFunding(t, user.Id, 2500, 3000, LoanFundingActive)
	createLoanDebtAccount(t, user.Id, 2500, 3000)

	_, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.NotNil(t, repay)
	require.Equal(t, int64(3000), repay.Amount)
	require.Equal(t, int64(500), repay.InterestPart)
	require.Equal(t, int64(2500), repay.PrincipalPart)
	require.Equal(t, int64(0), repay.DebtAfter)

	// funding 结清
	var gf TokenLoanFunding
	require.NoError(t, DB.First(&gf, f.Id).Error)
	require.Equal(t, LoanFundingRepaid, gf.Status)
	require.Zero(t, gf.DebtQuota)
	require.Zero(t, gf.PrincipalRemaining)

	// 账户清账
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(0), acc.DebtQuota)
	require.Equal(t, int64(0), acc.PrincipalQuota)
	require.Equal(t, int64(3000), acc.TotalRepaid)

	// DB quota 增量 = 净额 5000 - 3000 = 2000
	require.Equal(t, 2000, checkinUserQuota(t, user.Id))

	// 台账拆分正确
	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "repay").First(&rec).Error)
	require.Equal(t, int64(3000), rec.Amount)
	require.Equal(t, int64(500), rec.InterestPart)
	require.Equal(t, int64(2500), rec.PrincipalPart)
	require.Equal(t, int64(0), rec.DebtAfter)
	require.Equal(t, "checkin", rec.Source)
}

// CheckinRepayEnabled=false：不还款，全额入账，账户/funding 与台账不变
func TestUserCheckinRepayDisabled(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = false
	})
	user := setupCheckinLoanUser(t)
	createLoanDebtAccount(t, user.Id, 10000, 20000)

	_, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.Nil(t, repay)
	require.Equal(t, 5000, checkinUserQuota(t, user.Id))

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(20000), acc.DebtQuota)
	require.Equal(t, int64(10000), acc.PrincipalQuota)
	require.Equal(t, int64(0), acc.TotalRepaid)
	require.Equal(t, int64(0), countLoanRecords(t, user.Id, "repay"))
}

// renameTableForFailure 临时把表改名以注入写入失败，测试结束后恢复
func renameTableForFailure(t *testing.T, table string) {
	t.Helper()
	require.NoError(t, DB.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s_bak", table, table)).Error)
	t.Cleanup(func() {
		DB.Exec(fmt.Sprintf("ALTER TABLE %s_bak RENAME TO %s", table, table))
	})
}

// 事务回滚：台账写入失败（token_loan_records 改名）→ 整笔回滚：funding/账户原状、
// 签到记录删除、用户余额不变
func TestUserCheckinRepayLedgerFailureRollback(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	f := createCheckinFunding(t, user.Id, 4000, 5000, LoanFundingActive)
	createLoanDebtAccount(t, user.Id, 4000, 5000)
	renameTableForFailure(t, "token_loan_records")

	_, repay, err := UserCheckin(user.Id)
	require.Error(t, err)
	require.Nil(t, repay)

	// funding 回滚到还款前（含 status 与 LastSettledDay）
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, int64(5000), got.DebtQuota)
	require.Equal(t, int64(4000), got.PrincipalRemaining)
	require.Equal(t, LoanFundingActive, got.Status)
	require.Equal(t, loanDay(time.Now()), got.LastSettledDay)
	// 账户回滚
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(5000), acc.DebtQuota)
	require.Equal(t, int64(4000), acc.PrincipalQuota)
	require.Equal(t, int64(0), acc.TotalRepaid)
	// 签到记录已删除，用户余额未变
	hasChecked, err := HasCheckedInToday(user.Id)
	require.NoError(t, err)
	require.False(t, hasChecked)
	require.Equal(t, 0, checkinUserQuota(t, user.Id))
}

// 事务回滚：quota 入账失败（SQLite 触发器拦截 users.quota 更新）→ 整笔回滚：
// funding/账户原状、签到记录删除、余额不变
func TestUserCheckinRepayQuotaFailureRollback(t *testing.T) {
	withCheckinSetting(t, 10000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	// 奖励 10000 > 债务 3000：还款 3000 后净额 7000，走到 quota 入账才失败
	f := createCheckinFunding(t, user.Id, 2500, 3000, LoanFundingActive)
	createLoanDebtAccount(t, user.Id, 2500, 3000)

	// 注入 quota 更新失败：SQLite 触发器拦截 users.quota 更新（镜像 loan_test.go 模式）
	require.NoError(t, DB.Exec(`CREATE TRIGGER checkin_test_block_quota_update
		BEFORE UPDATE OF quota ON users
		BEGIN SELECT RAISE(ABORT, 'quota update blocked by test'); END`).Error)
	t.Cleanup(func() {
		_ = DB.Exec(`DROP TRIGGER IF EXISTS checkin_test_block_quota_update`).Error
	})

	_, repay, err := UserCheckin(user.Id)
	require.Error(t, err)
	require.Nil(t, repay)

	// funding 回滚
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, int64(3000), got.DebtQuota)
	require.Equal(t, int64(2500), got.PrincipalRemaining)
	require.Equal(t, LoanFundingActive, got.Status)
	// 账户回滚
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(3000), acc.DebtQuota)
	require.Equal(t, int64(2500), acc.PrincipalQuota)
	require.Equal(t, int64(0), acc.TotalRepaid)
	// 签到记录已删除，余额不变
	hasChecked, err := HasCheckedInToday(user.Id)
	require.NoError(t, err)
	require.False(t, hasChecked)
	require.Equal(t, 0, checkinUserQuota(t, user.Id))
}

// 提交后缓存（miniredis）：借款人按净额递增、放贷人按入账清单递增。
// 旧版 SQLite 分支"先异步递增缓存、失败再补偿"的路径已随分支合并删除：quota 更新在
// 事务内完成、提交前不触碰缓存，失败路径无需补偿。
func TestUserCheckinSuccessSyncsCache(t *testing.T) {
	setupUserCacheVersionTest(t)
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, lender.Id)
	now := time.Now()
	day := loanDay(now)
	offer := createRepayOffer(t, lender.Id, 400000, 100000)
	// 债务 3000（本金 2500 + 利息 500），奖励 5000 → 还款 3000（息 500 本 2500），
	// 净额 2000；放贷人入账 500（利息），offer 回补 2500
	f := &TokenLoanFunding{
		LoanUserId:         user.Id,
		SourceType:         LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           lender.Id,
		Amount:             2500,
		PrincipalRemaining: 2500,
		DebtQuota:          3000,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             day + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	createLoanDebtAccount(t, user.Id, 2500, 3000)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", 0).Error)
	// 清理本用例的 funding 行（同 TestUserCheckinRepayCreditsLenderAndRefillsOffer）
	t.Cleanup(func() {
		_ = DB.Where("id = ?", f.Id).Delete(&TokenLoanFunding{}).Error
	})

	// 预置借款人/放贷人缓存（Quota 0，带 TTL；RedisHIncrBy 仅在 key 带 TTL 时真正递增）
	ctx := context.Background()
	for _, uid := range []int{user.Id, lender.Id} {
		require.NoError(t, common.RDB.HSet(ctx, getUserCacheKey(uid), map[string]interface{}{
			"Id":     uid,
			"Role":   common.RoleCommonUser,
			"Quota":  0,
			"Status": common.UserStatusEnabled,
		}).Err())
		require.NoError(t, common.RDB.Expire(ctx, getUserCacheKey(uid),
			time.Duration(common.RedisKeyCacheSeconds())*time.Second).Err())
	}

	_, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.NotNil(t, repay)
	require.Equal(t, int64(3000), repay.Amount)

	// DB：借款人净额 2000、放贷人 500、offer 回补 2500
	require.Equal(t, 2000, checkinUserQuota(t, user.Id))
	var lu User
	require.NoError(t, DB.Select("quota").First(&lu, lender.Id).Error)
	require.Equal(t, 500, lu.Quota)
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, int64(102500), got.AmountAvailable)

	// 缓存：借款人 +2000、放贷人 +500（异步，Eventually 轮询）
	require.Eventually(t, func() bool {
		q, err := common.RDB.HGet(ctx, getUserCacheKey(user.Id), "Quota").Int64()
		return err == nil && q == 2000
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		q, err := common.RDB.HGet(ctx, getUserCacheKey(lender.Id), "Quota").Int64()
		return err == nil && q == 500
	}, time.Second, 10*time.Millisecond)
}
