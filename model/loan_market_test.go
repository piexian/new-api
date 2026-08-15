package model

import (
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
//   - 首轮迁移断言：全量账户 settle 落盘（LastSettledDay 前推、settle 计息正确、宽限内不计息）；
//     platform funding 字段逐一正确；债务 0 账户不生成 funding；已有 P2P funding 的账户仍补一条
//     platform funding（存在性守卫只认 source_type=platform）；credit_score==0 全部回填
//     CreditInitial
//   - 宽限承载：宽限账户的 funding 在宽限期内经 ProjectFundingDebt 结算不计息，
//     无宽限账户同期照常计息
//   - 一次性守卫：哨兵置位后二次执行必须是全量 no-op——过期未结算的新账户（LastSettledDay
//     落后 3 天、有债务、0 分）经再运行不得被 settle、不得建 funding、不得回填 credit
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

	// —— 一次性守卫：二次执行必须是全量 no-op ——
	// 哨兵置位后再建的过期未结算账户（LastSettledDay 落后 3 天、有债务、0 分）：
	// 迁移不得再 settle/转化 funding/回填 credit——函数入口即返回。
	// （若退化为"每次启动全量 settle"，此账户债务会被复利 3 天并生成 funding，测试即失败）
	fundingTotal := func() int64 {
		var n int64
		require.NoError(t, DB.Model(&TokenLoanFunding{}).Count(&n).Error)
		return n
	}
	staleUser := createLoanTestUser(t)
	require.NoError(t, DB.Where("loan_user_id = ?", staleUser.Id).Delete(&TokenLoanFunding{}).Error)
	require.NoError(t, DB.Where("user_id = ?", staleUser.Id).Delete(&TokenLoanAccount{}).Error)
	const staleDebt = int64(123_456)
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:         staleUser.Id,
		PrincipalQuota: staleDebt,
		DebtQuota:      staleDebt,
		LastSettledDay: today - 3,
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}).Error)

	before := fundingTotal()
	require.NoError(t, MigrateLoanToFundings())
	require.Equal(t, before, fundingTotal(), "哨兵置位后迁移不得新增 funding")
	var stale TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", staleUser.Id).First(&stale).Error)
	require.Equal(t, staleDebt, stale.DebtQuota, "哨兵置位后不得再 settle（债务保持未复利）")
	require.Equal(t, today-3, stale.LastSettledDay, "哨兵置位后不得推进结算时钟")
	require.Zero(t, stale.CreditScore, "回填只执行一次，不得重填 0 分")
	require.Empty(t, platformFundingsOf(t, staleUser.Id), "哨兵置位后不得转化 funding")

	// 再跑一次仍为 no-op：债务与首次 no-op 运行一致、各自用户 funding 条数不变
	require.NoError(t, MigrateLoanToFundings())
	var staleAgain TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", staleUser.Id).First(&staleAgain).Error)
	require.Equal(t, stale.DebtQuota, staleAgain.DebtQuota, "两次迁移后债务必须一致")
	require.Equal(t, before, fundingTotal(), "第三次迁移仍不得新增 funding")
	for _, uid := range []int{graceUser.Id, normalUser.Id, p2pUser.Id} {
		require.Len(t, platformFundingsOf(t, uid), 1)
	}
	require.Empty(t, platformFundingsOf(t, debtFreeUser.Id))
	require.Empty(t, platformFundingsOf(t, staleUser.Id))
}

// ===== Task 6: offer 生命周期 =====

// quotaOf 测试换算：把 USD 字符串按当前 QuotaPerUnit 精确换算为 quota（decimal，镜像生产路径）
func quotaOf(t *testing.T, usd string) int64 {
	t.Helper()
	d, err := decimal.NewFromString(usd)
	require.NoError(t, err)
	return d.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
}

// setupMarketLender 创建已开启市场、已同意免责声明、带余额的放贷人测试用户；
// SQLite 复用被删用户 id，先清掉名下残留的市场行再建行
func setupMarketLender(t *testing.T, balance int64) *User {
	t.Helper()
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.MarketEnabled = true
		s.LenderMinAmount = 50000
		s.LenderRateMin = 0.0005
		s.LenderRateMax = 0.003
		s.PerLoanCapDefault = 0
	})
	require.NoError(t, DB.AutoMigrate(&TokenLoanOffer{}, &TokenLoanFunding{}))
	lender := createLoanTestUser(t)
	require.NoError(t, DB.Where("lender_id = ?", lender.Id).Delete(&TokenLoanOffer{}).Error)
	require.NoError(t, DB.Where("lender_id = ?", lender.Id).Delete(&TokenLoanFunding{}).Error)
	require.NoError(t, DB.Where("user_id = ?", lender.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).Update("quota", balance).Error)
	require.NoError(t, AgreeLenderDisclaimer(lender.Id))
	return lender
}

// activeOrOverduePrincipalSum 统计 offer 名下 active/overdue funding 的未还本金之和
// （spec §4.1 不变式的 Σ 项）
func activeOrOverduePrincipalSum(t *testing.T, offerId int) int64 {
	t.Helper()
	var fundings []TokenLoanFunding
	require.NoError(t, DB.Where("offer_id = ? AND status IN ?", offerId, []string{LoanFundingActive, LoanFundingOverdue}).Find(&fundings).Error)
	var sum int64
	for _, f := range fundings {
		sum += f.PrincipalRemaining
	}
	return sum
}

// createOfferFunding 直接构造 offer 名下一条 active funding 并同步核减 amount_available，
// 保持不变式 amount_total = amount_available + Σ 未还本金（模拟 Task 8 放款扣减）
func createOfferFunding(t *testing.T, offer *TokenLoanOffer, borrowerId int, amount int64) *TokenLoanFunding {
	t.Helper()
	now := time.Now()
	f := &TokenLoanFunding{
		LoanUserId:         borrowerId,
		BorrowEventId:      int64(offer.Id),
		SourceType:         LoanFundingPool,
		OfferId:            offer.Id,
		LenderId:           offer.LenderId,
		Amount:             amount,
		PrincipalRemaining: amount,
		DebtQuota:          amount,
		LastSettledDay:     loanDay(now),
		Rate:               offer.RateFixed,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             loanDay(now) + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	require.NoError(t, DB.Model(&TokenLoanOffer{}).Where("id = ?", offer.Id).
		Update("amount_available", gorm.Expr("amount_available - ?", amount)).Error)
	offer.AmountAvailable -= amount
	return f
}

func TestAgreeLenderDisclaimerIdempotent(t *testing.T) {
	lender := createLoanTestUser(t)
	require.NoError(t, DB.Where("user_id = ?", lender.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, AgreeLenderDisclaimer(lender.Id))

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", lender.Id).First(&acc).Error)
	require.NotZero(t, acc.LenderDisclaimerAgreedAt)
	first := acc.LenderDisclaimerAgreedAt

	// 幂等：再次同意不覆盖首次时间
	require.NoError(t, AgreeLenderDisclaimer(lender.Id))
	require.NoError(t, DB.Where("user_id = ?", lender.Id).First(&acc).Error)
	require.Equal(t, first, acc.LenderDisclaimerAgreedAt)

	var count int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", lender.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestCreateLoanOfferMarketDisabled(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.MarketEnabled = false })
	lender := createLoanTestUser(t)
	_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50)
	require.ErrorIs(t, err, ErrLoanMarketDisabled)
}

func TestCreateLoanOfferDisclaimerRequired(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	// 撤销免责声明：删除账户行后 getOrCreateLoanAccountTxSafe 会重建（AgreedAt=0）
	require.NoError(t, DB.Where("user_id = ?", lender.Id).Delete(&TokenLoanAccount{}).Error)

	_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50)
	require.ErrorIs(t, err, ErrLoanDisclaimerRequired)

	// 拒绝后不落 offer、不动余额、不留账户（事务整体回滚）
	var n int64
	require.NoError(t, DB.Model(&TokenLoanOffer{}).Where("lender_id = ?", lender.Id).Count(&n).Error)
	require.Zero(t, n)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, lender.Id).Error)
	require.Equal(t, quotaOf(t, "10.00"), int64(u.Quota))
	var accCount int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", lender.Id).Count(&accCount).Error)
	require.Zero(t, accCount)
}

func TestCreateLoanOfferRejectsInvalidAmount(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	// 非数字、非正数、超过两位小数一律拒绝
	for _, amt := range []string{"", "abc", "-1.00", "0", "0.00", "1.005", "0.001", "1.234"} {
		_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, amt, "0.001", 0, 0, 0, -50)
		require.ErrorIs(t, err, ErrLoanInvalidAmount, "amount %q should be rejected", amt)
	}
	// 低于最小入池金额（LenderMinAmount=50000）
	_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "0.05", "0.001", 0, 0, 0, -50)
	require.ErrorIs(t, err, ErrLoanOfferInvalidParams)
	// int32 上界：10000 USD × 500000 = 5e9 quota 超 MaxQuota
	_, err = CreateLoanOffer(lender.Id, LoanOfferModePool, "10000.00", "0.001", 0, 0, 0, -50)
	require.ErrorIs(t, err, ErrLoanQuotaOverflow)
}

func TestCreateLoanOfferRejectsInvalidParams(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	// 非法模式
	_, err := CreateLoanOffer(lender.Id, "bogus", "1.00", "0.001", 0, 0, 0, -50)
	require.ErrorIs(t, err, ErrLoanOfferInvalidParams)
	// pool：rateFixed 低于下限 / 高于上限 / 非数字一律拒绝
	for _, rf := range []string{"0.0004", "0.0031", "abc"} {
		_, err = CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", rf, 0, 0, 0, -50)
		require.ErrorIs(t, err, ErrLoanOfferInvalidParams, "rateFixed %q should be rejected", rf)
	}
	// order 同 pool 校验
	_, err = CreateLoanOffer(lender.Id, LoanOfferModeOrder, "1.00", "0.004", 0, 0, 0, -50)
	require.ErrorIs(t, err, ErrLoanOfferInvalidParams)
	// ai：区间越界 / 区间倒挂 / perLoanCap<=0 一律拒绝
	for _, tc := range []struct {
		rateMin, rateMax float64
		cap              int64
	}{
		{0.0004, 0.002, 100000}, // 下限低于配置下限
		{0.001, 0.0031, 100000}, // 上限高于配置上限
		{0.002, 0.001, 100000},  // 区间倒挂
		{0.001, 0.002, 0},       // 单笔上限必须 > 0
		{0.001, 0.002, -100},    // 负数拒绝
	} {
		_, err = CreateLoanOffer(lender.Id, LoanOfferModeAi, "1.00", "", tc.rateMin, tc.rateMax, tc.cap, -50)
		require.ErrorIs(t, err, ErrLoanOfferInvalidParams, "ai %+v should be rejected", tc)
	}
}

func TestCreateLoanOfferInsufficientBalance(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "1.00"))
	_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "2.00", "0.001", 0, 0, 0, -50)
	require.ErrorIs(t, err, ErrLoanInsufficientBalance)

	// 拒绝后不落 offer、不动余额
	var n int64
	require.NoError(t, DB.Model(&TokenLoanOffer{}).Where("lender_id = ?", lender.Id).Count(&n).Error)
	require.Zero(t, n)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, lender.Id).Error)
	require.Equal(t, quotaOf(t, "1.00"), int64(u.Quota))
}

func TestCreateLoanOfferUserDisabled(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).
		Update("status", common.UserStatusDisabled).Error)
	_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50)
	require.ErrorIs(t, err, ErrLoanUserDisabled)
}

func TestCreateLoanOfferAtomicityOnQuotaFailure(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))

	// 注入 quota 扣款失败：SQLite 触发器拦截 users.quota 更新
	require.NoError(t, DB.Exec(`CREATE TRIGGER loan_market_test_block_quota_update
		BEFORE UPDATE OF quota ON users
		BEGIN SELECT RAISE(ABORT, 'quota update blocked by test'); END`).Error)
	t.Cleanup(func() {
		_ = DB.Exec(`DROP TRIGGER IF EXISTS loan_market_test_block_quota_update`).Error
	})

	_, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50)
	require.Error(t, err)

	// 扣款与建 offer 同一事务：失败后 offer 不落库、余额不变
	var n int64
	require.NoError(t, DB.Model(&TokenLoanOffer{}).Where("lender_id = ?", lender.Id).Count(&n).Error)
	require.Zero(t, n)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, lender.Id).Error)
	require.Equal(t, quotaOf(t, "10.00"), int64(u.Quota))
}

func TestCreateLoanOfferHappyPath(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.PerLoanCapDefault = 200000 })

	// pool：rateFixed 落库、perLoanCap=0 走全局缺省、MinCreditScore 低于 -50 钳制到 -50
	offer, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "2.00", "0.001", 0, 0, 0, -100)
	require.NoError(t, err)
	require.NotZero(t, offer.Id)
	require.Equal(t, lender.Id, offer.LenderId)
	require.Equal(t, LoanOfferModePool, offer.Mode)
	require.Equal(t, LoanOfferStatusActive, offer.Status)
	require.Equal(t, quotaOf(t, "2.00"), offer.AmountTotal)
	require.Equal(t, quotaOf(t, "2.00"), offer.AmountAvailable)
	require.Equal(t, 0.001, offer.RateFixed)
	require.Zero(t, offer.RateMin)
	require.Zero(t, offer.RateMax)
	require.Equal(t, int64(200000), offer.PerLoanCap, "pool 的 perLoanCap=0 应取全局缺省")
	require.Equal(t, -50, offer.MinCreditScore, "低于 -50 钳制到 -50（不限制）")
	require.NotZero(t, offer.CreatedAt)
	require.Equal(t, offer.CreatedAt, offer.UpdatedAt)
	// 余额扣减：10 - 2
	var u User
	require.NoError(t, DB.Select("quota").First(&u, lender.Id).Error)
	require.Equal(t, quotaOf(t, "8.00"), int64(u.Quota))

	// ai：区间与 perLoanCap 原样落库、RateFixed=0、MinCreditScore 超过 100 钳制到 100
	offer2, err := CreateLoanOffer(lender.Id, LoanOfferModeAi, "1.00", "0.001", 0.0008, 0.002, 300000, 150)
	require.NoError(t, err)
	require.Equal(t, 0.0008, offer2.RateMin)
	require.Equal(t, 0.002, offer2.RateMax)
	require.Equal(t, int64(300000), offer2.PerLoanCap)
	require.Zero(t, offer2.RateFixed)
	require.Equal(t, 100, offer2.MinCreditScore, "超过 100 钳制到 100")

	// order 也走固定利率路径
	offer3, err := CreateLoanOffer(lender.Id, LoanOfferModeOrder, "1.00", "0.0015", 0, 0, 0, -50)
	require.NoError(t, err)
	require.Equal(t, 0.0015, offer3.RateFixed)
	require.Equal(t, int64(200000), offer3.PerLoanCap)
}

func TestSetLoanOfferStatusTransitions(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	offer, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50)
	require.NoError(t, err)

	require.NoError(t, SetLoanOfferStatus(lender.Id, offer.Id, LoanOfferStatusPaused))
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, LoanOfferStatusPaused, got.Status)

	require.NoError(t, SetLoanOfferStatus(lender.Id, offer.Id, LoanOfferStatusActive))
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, LoanOfferStatusActive, got.Status)

	// 同状态幂等
	require.NoError(t, SetLoanOfferStatus(lender.Id, offer.Id, LoanOfferStatusActive))

	// 非法目标状态（含 closed，关闭必须走 CloseLoanOffer）
	require.ErrorIs(t, SetLoanOfferStatus(lender.Id, offer.Id, "bogus"), ErrLoanOfferInvalidParams)
	require.ErrorIs(t, SetLoanOfferStatus(lender.Id, offer.Id, LoanOfferStatusClosed), ErrLoanOfferInvalidParams)

	// 非本人 / 不存在 → ErrLoanOfferNotFound
	other := createLoanTestUser(t)
	require.ErrorIs(t, SetLoanOfferStatus(other.Id, offer.Id, LoanOfferStatusPaused), ErrLoanOfferNotFound)
	require.ErrorIs(t, SetLoanOfferStatus(lender.Id, 999999, LoanOfferStatusPaused), ErrLoanOfferNotFound)

	// 关闭后暂停/恢复被拒
	require.NoError(t, CloseLoanOffer(lender.Id, offer.Id))
	require.ErrorIs(t, SetLoanOfferStatus(lender.Id, offer.Id, LoanOfferStatusPaused), ErrLoanOfferNotActive)
	require.ErrorIs(t, SetLoanOfferStatus(lender.Id, offer.Id, LoanOfferStatusActive), ErrLoanOfferNotActive)
}

func TestCloseLoanOfferRefundsIdleAndKeepsInvariant(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	borrower := createLoanTestUser(t)
	require.NoError(t, DB.Where("lender_id = ? OR loan_user_id = ?", lender.Id, borrower.Id).
		Delete(&TokenLoanFunding{}).Error)

	offer, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "2.00", "0.001", 0, 0, 0, -50)
	require.NoError(t, err)
	f1 := createOfferFunding(t, offer, borrower.Id, quotaOf(t, "0.30")) // 150000
	createOfferFunding(t, offer, borrower.Id, quotaOf(t, "0.10"))       // 50000
	// 关闭前不变式成立：2 USD = available 0.6 USD + Σ 本金 0.4 USD
	require.Equal(t, offer.AmountTotal, offer.AmountAvailable+activeOrOverduePrincipalSum(t, offer.Id))

	require.NoError(t, CloseLoanOffer(lender.Id, offer.Id))

	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, LoanOfferStatusClosed, got.Status)
	require.Zero(t, got.AmountAvailable)
	require.Equal(t, activeOrOverduePrincipalSum(t, offer.Id), got.AmountTotal, "关闭核减 amount_total")
	// 关闭后不变式仍成立
	require.Equal(t, got.AmountTotal, got.AmountAvailable+activeOrOverduePrincipalSum(t, offer.Id))
	// 闲置额度退回余额：10 - 2(挂出) + 1.6(退回) = 9.6
	var u User
	require.NoError(t, DB.Select("quota").First(&u, lender.Id).Error)
	require.Equal(t, quotaOf(t, "9.60"), int64(u.Quota))
	// 存续 funding 不受影响
	var f TokenLoanFunding
	require.NoError(t, DB.First(&f, f1.Id).Error)
	require.Equal(t, LoanFundingActive, f.Status)
	require.Equal(t, quotaOf(t, "0.30"), f.PrincipalRemaining)
}

func TestCloseLoanOfferTwiceRejected(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	offer, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50)
	require.NoError(t, err)
	require.NoError(t, CloseLoanOffer(lender.Id, offer.Id))
	// 重复关闭与关闭后撤回均被拒
	require.ErrorIs(t, CloseLoanOffer(lender.Id, offer.Id), ErrLoanOfferNotActive)
	_, err = WithdrawLoanOffer(lender.Id, offer.Id)
	require.ErrorIs(t, err, ErrLoanOfferNotActive)
}

func TestWithdrawLoanOffer(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	borrower := createLoanTestUser(t)
	require.NoError(t, DB.Where("lender_id = ? OR loan_user_id = ?", lender.Id, borrower.Id).
		Delete(&TokenLoanFunding{}).Error)

	offer, err := CreateLoanOffer(lender.Id, LoanOfferModeAi, "3.00", "", 0.0008, 0.002, quotaOf(t, "1.00"), -50)
	require.NoError(t, err)
	createOfferFunding(t, offer, borrower.Id, quotaOf(t, "0.50"))
	createOfferFunding(t, offer, borrower.Id, quotaOf(t, "0.25"))

	// 撤回全部闲置额度：3 - 0.5 - 0.25 = 2.25
	refunded, err := WithdrawLoanOffer(lender.Id, offer.Id)
	require.NoError(t, err)
	require.Equal(t, quotaOf(t, "2.25"), refunded)

	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, LoanOfferStatusActive, got.Status, "撤回后 offer 保持原状态（v1 不支持撤回后再充值）")
	require.Zero(t, got.AmountAvailable)
	require.Equal(t, activeOrOverduePrincipalSum(t, offer.Id), got.AmountTotal, "撤回核减 amount_total")
	// 撤回后不变式仍成立
	require.Equal(t, got.AmountTotal, got.AmountAvailable+activeOrOverduePrincipalSum(t, offer.Id))
	// 余额：10 - 3(挂出) + 2.25(撤回) = 9.25
	var u User
	require.NoError(t, DB.Select("quota").First(&u, lender.Id).Error)
	require.Equal(t, quotaOf(t, "9.25"), int64(u.Quota))

	// 无可撤回额度 → ErrLoanNothingToWithdraw
	_, err = WithdrawLoanOffer(lender.Id, offer.Id)
	require.ErrorIs(t, err, ErrLoanNothingToWithdraw)
}

func TestCloseAndWithdrawOwnership(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	other := createLoanTestUser(t)
	offer, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50)
	require.NoError(t, err)

	require.ErrorIs(t, CloseLoanOffer(other.Id, offer.Id), ErrLoanOfferNotFound)
	_, err = WithdrawLoanOffer(other.Id, offer.Id)
	require.ErrorIs(t, err, ErrLoanOfferNotFound)
	require.ErrorIs(t, CloseLoanOffer(lender.Id, 999999), ErrLoanOfferNotFound)
	// 非本人操作不产生副作用
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, LoanOfferStatusActive, got.Status)
	require.Equal(t, quotaOf(t, "1.00"), got.AmountAvailable)
}

func TestCloseLoanOfferQuotaOverflow(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	offer, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "2.00", "0.001", 0, 0, 0, -50)
	require.NoError(t, err)
	// 挂出期间余额暴涨（模拟其他入账）到 int32 上界附近
	require.NoError(t, DB.Model(&User{}).Where("id = ?", lender.Id).
		Update("quota", int64(common.MaxQuota)-100).Error)
	// 关闭退回 1e6 → 2147483547 + 1000000 超 int32 上界
	require.ErrorIs(t, CloseLoanOffer(lender.Id, offer.Id), ErrLoanQuotaOverflow)
	// 失败后 offer 保持 active、余额不变
	var got TokenLoanOffer
	require.NoError(t, DB.First(&got, offer.Id).Error)
	require.Equal(t, LoanOfferStatusActive, got.Status)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, lender.Id).Error)
	require.Equal(t, int64(common.MaxQuota)-100, int64(u.Quota))
}

func TestGetUserLoanOffersAndGetLoanOfferById(t *testing.T) {
	lender := setupMarketLender(t, quotaOf(t, "10.00"))
	o1, err := CreateLoanOffer(lender.Id, LoanOfferModePool, "1.00", "0.001", 0, 0, 0, -50)
	require.NoError(t, err)
	o2, err := CreateLoanOffer(lender.Id, LoanOfferModeOrder, "2.00", "0.002", 0, 0, 0, -50)
	require.NoError(t, err)

	offers, err := GetUserLoanOffers(lender.Id)
	require.NoError(t, err)
	require.Len(t, offers, 2)
	require.Equal(t, o2.Id, offers[0].Id, "id 倒序（最新在前）")
	require.Equal(t, o1.Id, offers[1].Id)

	got, err := GetLoanOfferById(o1.Id)
	require.NoError(t, err)
	require.Equal(t, o1.Id, got.Id)
	require.Equal(t, lender.Id, got.LenderId)

	_, err = GetLoanOfferById(999999)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	// 他人 offer 列表互不可见
	other := createLoanTestUser(t)
	otherOffers, err := GetUserLoanOffers(other.Id)
	require.NoError(t, err)
	require.Empty(t, otherOffers)
}
