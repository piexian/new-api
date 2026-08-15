package model

import (
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// TestLoanMarketModels 市场数据模型冒烟：建表、offer/funding CRUD 写读回、
// 既有表新列经 AutoMigrate 落库且可读写（credit_score 默认 0，回填属 Task 5 迁移）
func TestLoanMarketModels(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(
		&TokenLoanAccount{}, &TokenLoanRecord{},
		&TokenLoanOffer{}, &TokenLoanFunding{},
	))

	// 新列经 AutoMigrate 自动补默认值
	for _, col := range []string{"credit_score", "blacklisted_until_day", "lender_disclaimer_agreed_at"} {
		require.True(t, DB.Migrator().HasColumn(&TokenLoanAccount{}, col), col)
	}
	for _, col := range []string{"funding_id", "lender_id"} {
		require.True(t, DB.Migrator().HasColumn(&TokenLoanRecord{}, col), col)
	}

	// SQLite 复用被删用户 id，先清掉名下残留的市场行再建行
	lender := createLoanTestUser(t)
	borrower := createLoanTestUser(t)
	require.NoError(t, DB.Where("lender_id = ?", lender.Id).Delete(&TokenLoanOffer{}).Error)
	require.NoError(t, DB.Where("lender_id = ? OR loan_user_id = ?", lender.Id, borrower.Id).
		Delete(&TokenLoanFunding{}).Error)

	now := time.Now()
	offer := &TokenLoanOffer{
		LenderId:        lender.Id,
		Mode:            LoanOfferModePool,
		Status:          LoanOfferStatusActive,
		AmountTotal:     1_000_000,
		AmountAvailable: 1_000_000,
		RateFixed:       0.001,
		PerLoanCap:      200_000,
		MinCreditScore:  60,
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, DB.Create(offer).Error)
	require.NotZero(t, offer.Id)

	funding := &TokenLoanFunding{
		LoanUserId:         borrower.Id,
		BorrowEventId:      42,
		SourceType:         LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           lender.Id,
		Amount:             200_000,
		PrincipalRemaining: 200_000,
		DebtQuota:          200_000,
		LastSettledDay:     loanDay(now),
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             loanDay(now) + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(funding).Error)
	require.NotZero(t, funding.Id)

	// 读回断言
	var gotOffer TokenLoanOffer
	require.NoError(t, DB.First(&gotOffer, offer.Id).Error)
	require.Equal(t, lender.Id, gotOffer.LenderId)
	require.Equal(t, LoanOfferModePool, gotOffer.Mode)
	require.Equal(t, LoanOfferStatusActive, gotOffer.Status)
	require.Equal(t, int64(1_000_000), gotOffer.AmountTotal)
	require.Equal(t, int64(1_000_000), gotOffer.AmountAvailable)
	require.Equal(t, 0.001, gotOffer.RateFixed)
	require.Equal(t, int64(200_000), gotOffer.PerLoanCap)
	require.Equal(t, 60, gotOffer.MinCreditScore)

	var gotFunding TokenLoanFunding
	require.NoError(t, DB.First(&gotFunding, funding.Id).Error)
	require.Equal(t, borrower.Id, gotFunding.LoanUserId)
	require.Equal(t, int64(42), gotFunding.BorrowEventId)
	require.Equal(t, LoanFundingPool, gotFunding.SourceType)
	require.Equal(t, offer.Id, gotFunding.OfferId)
	require.Equal(t, lender.Id, gotFunding.LenderId)
	require.Equal(t, int64(200_000), gotFunding.Amount)
	require.Equal(t, int64(200_000), gotFunding.PrincipalRemaining)
	require.Equal(t, LoanRepayFull, gotFunding.RepayPlan)
	require.Equal(t, LoanFundingActive, gotFunding.Status)

	// 状态流转可更新
	overdueDay := loanDay(now) + 30
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", funding.Id).
		Updates(map[string]interface{}{"status": LoanFundingOverdue, "penalty_started_day": overdueDay}).Error)
	var updated TokenLoanFunding
	require.NoError(t, DB.First(&updated, funding.Id).Error)
	require.Equal(t, LoanFundingOverdue, updated.Status)
	require.Equal(t, overdueDay, updated.PenaltyStartedDay)

	// 账户新列：credit_score 默认 0（不回填），免责声明时间戳可写入读回
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:                   borrower.Id,
		LastSettledDay:           loanDay(now),
		LenderDisclaimerAgreedAt: now.Unix(),
		CreatedAt:                now.Unix(),
		UpdatedAt:                now.Unix(),
	}).Error)
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).First(&acc).Error)
	require.Zero(t, acc.CreditScore)
	require.Equal(t, now.Unix(), acc.LenderDisclaimerAgreedAt)
}

// platformFundingsOf 查询用户全部 platform funding（迁移测试辅助）
func platformFundingsOf(t *testing.T, userId int) []TokenLoanFunding {
	t.Helper()
	var fs []TokenLoanFunding
	require.NoError(t, DB.Where("loan_user_id = ? AND source_type = ?", userId, LoanFundingPlatform).Find(&fs).Error)
	return fs
}

// TestMigrateLoanToFundings 存量迁移测试（spec §15 + plan Task 5）：
//   - 预置 4 类账户：带宽限（interest_free_until）、无宽限、债务为 0、已有 P2P funding 仍带债务
//   - 迁移后断言：全量账户 settle 落盘（LastSettledDay 前推、settle 计息正确、宽限内不计息）；
//     platform funding 字段逐一正确；债务 0 账户不生成 funding；已有 P2P funding 的账户仍补一条
//     platform funding（存在性守卫只认 source_type=platform）；credit_score==0 全部回填
//     CreditInitial；二次执行不产生重复 funding（幂等）
//   - 宽限承载：宽限账户的 funding 在宽限期内经 ProjectFundingDebt 结算不计息，
//     无宽限账户同期照常计息
//   - 哨兵：回填只做一次——置位后再建的 0 分账户（即使带债务）不再被重填
func TestMigrateLoanToFundings(t *testing.T) {
	withDailyRate(t, 0.001)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.LoanTermDays = 30
		s.CreditInitial = 50
	})
	loanSetting := operation_setting.GetLoanSetting()

	require.NoError(t, DB.AutoMigrate(&Option{}, &TokenLoanAccount{}, &TokenLoanRecord{}, &TokenLoanFunding{}))

	// SQLite 会复用被删用户 id：先清掉名下残留的市场行再建行
	graceUser := createLoanTestUser(t)
	normalUser := createLoanTestUser(t)
	debtFreeUser := createLoanTestUser(t)
	p2pUser := createLoanTestUser(t)
	userIDs := []int{graceUser.Id, normalUser.Id, debtFreeUser.Id, p2pUser.Id}
	require.NoError(t, DB.Where("loan_user_id IN ?", userIDs).Delete(&TokenLoanFunding{}).Error)
	require.NoError(t, DB.Where("user_id IN ?", userIDs).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Where("key = ?", loanFundingMigrationOptionKey).Delete(&Option{}).Error)

	now := time.Now()
	today := loanDay(now)

	// 有宽限：5 天前借出，宽限到 today+5；迁移 settle 因仍在宽限期不计息
	grace := &TokenLoanAccount{
		UserId:            graceUser.Id,
		PrincipalQuota:    100_000,
		DebtQuota:         100_000,
		LastSettledDay:    today - 5,
		InterestFreeUntil: today + 5,
		CreatedAt:         now.Unix(),
		UpdatedAt:         now.Unix(),
	}
	// 无宽限：2 天前借出 → 迁移 settle 按 2 天复利
	normal := &TokenLoanAccount{
		UserId:         normalUser.Id,
		PrincipalQuota: 50_000,
		DebtQuota:      50_000,
		LastSettledDay: today - 2,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}
	// 无债务：不生成 funding，但 LastSettledDay 照常前推
	debtFree := &TokenLoanAccount{
		UserId:         debtFreeUser.Id,
		LastSettledDay: today - 5,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}
	// 已有 P2P funding 仍带债务：迁移仍补 platform funding（守卫只认 platform）
	p2p := &TokenLoanAccount{
		UserId:         p2pUser.Id,
		PrincipalQuota: 200_000,
		DebtQuota:      200_000,
		LastSettledDay: today - 1,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}
	require.NoError(t, DB.Create(grace).Error)
	require.NoError(t, DB.Create(normal).Error)
	require.NoError(t, DB.Create(debtFree).Error)
	require.NoError(t, DB.Create(p2p).Error)
	require.NoError(t, DB.Create(&TokenLoanFunding{
		LoanUserId:         p2pUser.Id,
		SourceType:         LoanFundingPool,
		Amount:             200_000,
		PrincipalRemaining: 200_000,
		DebtQuota:          200_000,
		LastSettledDay:     today - 1,
		Rate:               0.0008,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             today + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}).Error)

	require.NoError(t, MigrateLoanToFundings())

	// —— 全量账户 settle 落盘断言 ——
	var normalAfter TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", normalUser.Id).First(&normalAfter).Error)
	require.Equal(t, today, normalAfter.LastSettledDay)
	require.Equal(t, int64(math.Round(50_000*math.Pow(1.001, 2))), normalAfter.DebtQuota)
	var graceAfter TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", graceUser.Id).First(&graceAfter).Error)
	require.Equal(t, today, graceAfter.LastSettledDay)
	require.Equal(t, int64(100_000), graceAfter.DebtQuota, "宽限期内迁移 settle 不得计息")
	var debtFreeAfter TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", debtFreeUser.Id).First(&debtFreeAfter).Error)
	require.Equal(t, today, debtFreeAfter.LastSettledDay)
	require.Zero(t, debtFreeAfter.DebtQuota)

	// —— credit_score 回填 ——
	for _, uid := range []int{graceUser.Id, normalUser.Id, debtFreeUser.Id, p2pUser.Id} {
		var a TokenLoanAccount
		require.NoError(t, DB.Where("user_id = ?", uid).First(&a).Error)
		require.Equal(t, loanSetting.CreditInitial, a.CreditScore, "user %d", uid)
	}

	// —— platform funding 断言 ——
	graceFundings := platformFundingsOf(t, graceUser.Id)
	normalFundings := platformFundingsOf(t, normalUser.Id)
	p2pFundings := platformFundingsOf(t, p2pUser.Id)
	require.Len(t, graceFundings, 1)
	require.Len(t, normalFundings, 1)
	require.Len(t, p2pFundings, 1)
	require.Empty(t, platformFundingsOf(t, debtFreeUser.Id), "债务 0 账户不得生成 funding")

	gf := graceFundings[0]
	require.Equal(t, int64(100_000), gf.Amount)
	require.Equal(t, int64(100_000), gf.PrincipalRemaining)
	require.Equal(t, int64(100_000), gf.DebtQuota)
	require.Equal(t, 0.001, gf.Rate)
	require.Equal(t, today, gf.LastSettledDay)
	require.Equal(t, today+loanSetting.LoanTermDays, gf.DueDay)
	require.Equal(t, LoanRepayFull, gf.RepayPlan)
	require.Equal(t, LoanFundingActive, gf.Status)
	require.Equal(t, LoanFundingPlatform, gf.SourceType)
	require.Zero(t, gf.BorrowEventId, "存量迁移无历史借款事件")
	require.Zero(t, gf.OfferId)
	require.Zero(t, gf.LenderId)
	require.Zero(t, gf.PenaltyStartedDay)
	require.Equal(t, now.Unix(), gf.CreatedAt)
	require.Equal(t, now.Unix(), gf.UpdatedAt)

	nf := normalFundings[0]
	require.Equal(t, int64(50_000), nf.Amount)
	require.Equal(t, int64(50_000), nf.PrincipalRemaining)
	require.Equal(t, normalAfter.DebtQuota, nf.DebtQuota, "funding 债务必须等于 settle 落盘的账户债务")

	// —— 宽限承载：迁移后第 3 天 ——
	// 宽限账户 funding：base 被账户 InterestFreeUntil 上提 → 仍不计息；
	// 无宽限账户 funding：自 LastSettledDay 起 3 天复利。
	day3 := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local).AddDate(0, 0, 3)
	require.Equal(t, gf.DebtQuota, ProjectFundingDebt(&gf, &graceAfter, day3), "宽限期内 funding 不得计息")
	expected3 := int64(math.Round(float64(nf.DebtQuota) * math.Pow(1.001, 3)))
	require.Equal(t, expected3, ProjectFundingDebt(&nf, &normalAfter, day3))

	// —— 幂等：二次执行不产生重复 funding ——
	fundingTotal := func() int64 {
		var n int64
		require.NoError(t, DB.Model(&TokenLoanFunding{}).Count(&n).Error)
		return n
	}
	before := fundingTotal()
	require.NoError(t, MigrateLoanToFundings())
	require.Equal(t, before, fundingTotal(), "二次迁移不得新增 funding")
	for _, uid := range []int{graceUser.Id, normalUser.Id, p2pUser.Id} {
		require.Len(t, platformFundingsOf(t, uid), 1)
	}
	require.Empty(t, platformFundingsOf(t, debtFreeUser.Id))

	// —— 哨兵：credit 回填只做一次 ——
	// 哨兵置位后再建的 0 分账户（即使带债务可被转化 funding）不得被重填为初始分
	lateUser := createLoanTestUser(t)
	require.NoError(t, DB.Where("loan_user_id = ?", lateUser.Id).Delete(&TokenLoanFunding{}).Error)
	require.NoError(t, DB.Where("user_id = ?", lateUser.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:         lateUser.Id,
		PrincipalQuota: 10_000,
		DebtQuota:      10_000,
		LastSettledDay: today,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}).Error)
	require.NoError(t, MigrateLoanToFundings())
	var late TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", lateUser.Id).First(&late).Error)
	require.Zero(t, late.CreditScore, "回填只执行一次，不得重填 0 分")
	require.Len(t, platformFundingsOf(t, lateUser.Id), 1, "新债务仍应被转化为 platform funding")
}
