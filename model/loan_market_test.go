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

// ===== Task 12: 逾期债权处置（延长/核销/永续）+ 黑名单出口 =====

// createP2POverdueFunding 建一条挂 offer 的 overdue P2P funding（处置测试）：
// LastSettledDay=今天使结算不再计息，债务数值确定；debt > principal 表示含未付利息
func createP2POverdueFunding(t *testing.T, borrowerId, lenderId, offerId int, principal, debt int64) *TokenLoanFunding {
	t.Helper()
	now := time.Now()
	day := loanDay(now)
	f := &TokenLoanFunding{
		LoanUserId:         borrowerId,
		SourceType:         LoanFundingPool,
		OfferId:            offerId,
		LenderId:           lenderId,
		Amount:             principal,
		PrincipalRemaining: principal,
		DebtQuota:          debt,
		LastSettledDay:     day,
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingOverdue,
		DueDay:             day - 5,
		PenaltyStartedDay:  day - 5,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}
	require.NoError(t, DB.Create(f).Error)
	return f
}

// ① 非 owner / platform / 不存在的 funding 一律拒绝（ErrLoanNotFundingOwner /
//
//	gorm.ErrRecordNotFound），且 funding 状态不被改动
func TestResolveOverdueFundingRejectsNonOwner(t *testing.T) {
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	other := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	cleanupLoanBorrowData(t, borrower.Id, other.Id)
	offer := createBorrowOffer(t, lender.Id, 1_000_000, 0.001)
	f := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)

	// 他人（非 funding 放贷人）处置被拒
	err := ResolveOverdueFunding(other.Id, f.Id, LoanDefaultActionExtend, 10)
	require.ErrorIs(t, err, ErrLoanNotFundingOwner)

	// funding 不存在
	err = ResolveOverdueFunding(lender.Id, 99999999, LoanDefaultActionExtend, 10)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// platform funding（LenderId==0）归 Task 15 官方流程，放贷人不可处置
	pf := createFlipFunding(t, borrower.Id, time.Now(), -1, 1000)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", pf.Id).
		Update("status", LoanFundingOverdue).Error)
	err = ResolveOverdueFunding(lender.Id, pf.Id, LoanDefaultActionPerpetual, 0)
	require.ErrorIs(t, err, ErrLoanNotFundingOwner)

	// 所有被拒路径都不改动 funding 状态
	for _, id := range []int64{f.Id, pf.Id} {
		var got TokenLoanFunding
		require.NoError(t, DB.First(&got, id).Error)
		require.Equal(t, LoanFundingOverdue, got.Status)
	}
}

// ② 非 overdue 状态拒绝（active / repaid 均报 ErrLoanFundingNotOverdue）
func TestResolveOverdueFundingRejectsNonOverdue(t *testing.T) {
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 1_000_000, 0.001)
	active := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", active.Id).
		Update("status", LoanFundingActive).Error)

	err := ResolveOverdueFunding(lender.Id, active.Id, LoanDefaultActionExtend, 10)
	require.ErrorIs(t, err, ErrLoanFundingNotOverdue)

	repaid := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 100_000, 105_000)
	require.NoError(t, DB.Model(&TokenLoanFunding{}).Where("id = ?", repaid.Id).
		Update("status", LoanFundingRepaid).Error)
	err = ResolveOverdueFunding(lender.Id, repaid.Id, LoanDefaultActionWriteoff, 0)
	require.ErrorIs(t, err, ErrLoanFundingNotOverdue)
}

// ③ 核销全链路：funding → written_off（冻结债务保留作历史，不偿还）；offer 侧
//
//	amount_total -= principal_remaining；借款人拉黑（today + BlacklistDaysOnDefault）、
//	扣分（50-20=30）；syncAccountFromFundings 后投影销毁核销债务；无任何台账行
func TestResolveOverdueFundingWriteoff(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.BlacklistDaysOnDefault = 30
		s.CreditDefaultPenalty = 20
	})
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 1_000_000, 0.001)
	createLoanDebtAccount(t, borrower.Id, 200_000, 210_000)
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", borrower.Id).
		Update("credit_score", 50).Error)
	f := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)

	require.NoError(t, ResolveOverdueFunding(lender.Id, f.Id, LoanDefaultActionWriteoff, 0))

	// funding：written_off 终态；冻结债务留在行上作历史（销毁的是账户投影与 offer 账面）
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, LoanFundingWrittenOff, got.Status)
	require.Equal(t, int64(210_000), got.DebtQuota)

	// offer：amount_total -= 200_000（钱离开 offer 账面）
	var o TokenLoanOffer
	require.NoError(t, DB.First(&o, offer.Id).Error)
	require.Equal(t, int64(800_000), o.AmountTotal)
	require.Equal(t, int64(1_000_000), o.AmountAvailable, "闲置额度不动（核销不影响未放款部分）")

	// 借款人：拉黑 + 扣分 + 投影销毁
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).First(&acc).Error)
	require.Equal(t, loanDay(time.Now())+30, acc.BlacklistedUntilDay)
	require.Equal(t, 30, acc.CreditScore)
	require.Zero(t, acc.DebtQuota, "核销债务从投影销毁（deflation）")
	require.Zero(t, acc.PrincipalQuota)

	// 决策不是资金变动：不写任何台账行
	var n int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).Where("user_id = ?", borrower.Id).Count(&n).Error)
	require.Zero(t, n)
}

// ③b 信用分扣分下限 -50 截断（P1-9）：-40 - 20 → -50，不继续下探
func TestResolveOverdueFundingWriteoffClampsCreditAtMinus50(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CreditDefaultPenalty = 20
	})
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 500_000, 0.001)
	createLoanDebtAccount(t, borrower.Id, 100_000, 105_000)
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", borrower.Id).
		Update("credit_score", -40).Error)
	f := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 100_000, 105_000)

	require.NoError(t, ResolveOverdueFunding(lender.Id, f.Id, LoanDefaultActionWriteoff, 0))

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).First(&acc).Error)
	require.Equal(t, -50, acc.CreditScore)
}

// ④ 已关闭 offer 的核销同样核减 amount_total（floor 0 防御：amount_total < 本金时归零）
func TestResolveOverdueFundingWriteoffClosedOfferFloorsTotal(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.BlacklistDaysOnDefault = 30 })
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 100_000, 0.001)
	require.NoError(t, DB.Model(&TokenLoanOffer{}).Where("id = ?", offer.Id).
		Updates(map[string]interface{}{
			"status":       LoanOfferStatusClosed,
			"amount_total": int64(100_000), // 账面已小于单笔本金，测试 floor
		}).Error)
	f := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)

	require.NoError(t, ResolveOverdueFunding(lender.Id, f.Id, LoanDefaultActionWriteoff, 0))

	var o TokenLoanOffer
	require.NoError(t, DB.First(&o, offer.Id).Error)
	require.Equal(t, LoanOfferStatusClosed, o.Status)
	require.Zero(t, o.AmountTotal, "amount_total - 200_000 后 floor 到 0")
}

// ⑤ 延长：due_day 前移、status → active、penalty_started_day 保留（历史）；
//
//	extendDays 越界（0 / > LoanTermDays）与非法动作报 ErrLoanInvalidDefaultAction
func TestResolveOverdueFundingExtend(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.LoanTermDays = 30 })
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 2_000_000, 0.001)
	today := loanDay(time.Now())

	ok := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)
	require.NoError(t, ResolveOverdueFunding(lender.Id, ok.Id, LoanDefaultActionExtend, 10))
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, ok.Id).Error)
	require.Equal(t, LoanFundingActive, got.Status)
	require.Equal(t, today+10, got.DueDay)
	require.Equal(t, today-5, got.PenaltyStartedDay, "已计罚息起始日保留（历史记录）")

	// 上界 LoanTermDays=30 允许
	maxDays := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)
	require.NoError(t, ResolveOverdueFunding(lender.Id, maxDays.Id, LoanDefaultActionExtend, 30))

	// extendDays=0 拒绝
	zero := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)
	err := ResolveOverdueFunding(lender.Id, zero.Id, LoanDefaultActionExtend, 0)
	require.ErrorIs(t, err, ErrLoanInvalidDefaultAction)

	// extendDays 超过 LoanTermDays 拒绝
	over := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)
	err = ResolveOverdueFunding(lender.Id, over.Id, LoanDefaultActionExtend, 31)
	require.ErrorIs(t, err, ErrLoanInvalidDefaultAction)

	// 非法动作拒绝
	bad := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)
	err = ResolveOverdueFunding(lender.Id, bad.Id, "delete", 0)
	require.ErrorIs(t, err, ErrLoanInvalidDefaultAction)

	// 被拒路径不改动 funding
	for _, id := range []int64{zero.Id, over.Id, bad.Id} {
		var f TokenLoanFunding
		require.NoError(t, DB.First(&f, id).Error)
		require.Equal(t, LoanFundingOverdue, f.Status)
		require.Equal(t, today-5, f.DueDay)
	}
}

// ⑥ 永续：funding 状态零变化（保持 overdue 继续计息），仅记录决策返回成功
func TestResolveOverdueFundingPerpetualKeepsState(t *testing.T) {
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 500_000, 0.001)
	f := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 100_000, 105_000)

	require.NoError(t, ResolveOverdueFunding(lender.Id, f.Id, LoanDefaultActionPerpetual, 0))

	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, LoanFundingOverdue, got.Status)
	require.Equal(t, f.DebtQuota, got.DebtQuota)
	require.Equal(t, f.DueDay, got.DueDay)
	require.Equal(t, f.PenaltyStartedDay, got.PenaltyStartedDay)
	var o TokenLoanOffer
	require.NoError(t, DB.First(&o, offer.Id).Error)
	require.Equal(t, int64(500_000), o.AmountTotal, "永续不改动 offer 账面")
}

// ⑦ 黑名单出口（永续全还清 → 立即解除）：黑名单置位（非核销触发，无 written_off 行）+
//
//	一条 overdue（永续）funding，全额还清后 distributeRepayment 的 repaid 分支经
//	maybeLiftBlacklistTx 清零 blacklisted_until_day
func TestBlacklistLiftsWhenPerpetualFundingsFullyRepaid(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.BlacklistDaysOnDefault = 30 })
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 500_000, 0.001)
	createLoanDebtAccount(t, borrower.Id, 100_000, 100_000)
	// 黑名单置位（30 天窗口内）+ 永续 funding：窗口内仍应提前解除（还款激励）
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", borrower.Id).
		Update("blacklisted_until_day", loanDay(time.Now())+30).Error)
	f := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 100_000, 100_000)

	now := time.Now()
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		acc, err := getLoanAccountTx(tx, borrower.Id)
		require.NoError(t, err)
		require.NotNil(t, acc)
		fundings, err := loadUserFundingsTx(tx, borrower.Id)
		require.NoError(t, err)
		info, _, err := distributeRepayment(tx, acc, fundings, 100_000, now)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, int64(100_000), info.Amount)
		require.NoError(t, tx.Save(acc).Error) // 真实调用方（RepayLoan/签到）统一落盘账户
		return nil
	}))

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).First(&acc).Error)
	require.Zero(t, acc.BlacklistedUntilDay, "永续 funding 全部还清 → 黑名单立即解除")
	require.Zero(t, acc.DebtQuota)
	var got TokenLoanFunding
	require.NoError(t, DB.First(&got, f.Id).Error)
	require.Equal(t, LoanFundingRepaid, got.Status)
}

// ⑧ 核销拉黑不可逆：核销（written_off 行落账日 >= blacklist_start）后窗口运行期内，
//
//	剩余 overdue funding 全部还清也不得提前解除（maybeLiftBlacklistTx 守卫拦截）
func TestBlacklistNotLiftedRightAfterWriteoff(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.BlacklistDaysOnDefault = 30
		s.CreditDefaultPenalty = 20
	})
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 1_000_000, 0.001)
	createLoanDebtAccount(t, borrower.Id, 300_000, 315_000)
	fA := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000) // 待核销
	fB := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 100_000, 105_000) // 永续，待还清
	require.NoError(t, ResolveOverdueFunding(lender.Id, fA.Id, LoanDefaultActionWriteoff, 0))

	now := time.Now()
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		acc, err := getLoanAccountTx(tx, borrower.Id)
		require.NoError(t, err)
		fundings, err := loadUserFundingsTx(tx, borrower.Id)
		require.NoError(t, err)
		require.Len(t, fundings, 1, "核销后的 funding 已退出 active/overdue 集合")
		info, _, err := distributeRepayment(tx, acc, fundings, 105_000, now)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.NoError(t, tx.Save(acc).Error)
		return nil
	}))

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", borrower.Id).First(&acc).Error)
	require.Equal(t, loanDay(time.Now())+30, acc.BlacklistedUntilDay,
		"核销拉黑窗口运行期内不提前解除（written_off 行落账日 >= blacklist_start）")
	require.Zero(t, acc.DebtQuota)
	var b TokenLoanFunding
	require.NoError(t, DB.First(&b, fB.Id).Error)
	require.Equal(t, LoanFundingRepaid, b.Status)
}

// ⑨ written_off funding 被排除出还款分配：loadUserFundingsTx 只载 active/overdue，
//
//	核销行的冻结债务永远不被分配/偿还（deflation，spec §9）
func TestWrittenOffFundingExcludedFromRepayment(t *testing.T) {
	borrower := createLoanTestUser(t)
	lender := createLoanTestUser(t)
	cleanupLoanBorrowData(t, borrower.Id, lender.Id)
	offer := createBorrowOffer(t, lender.Id, 1_000_000, 0.001)
	createLoanDebtAccount(t, borrower.Id, 300_000, 315_000)
	fA := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 200_000, 210_000)
	fB := createP2POverdueFunding(t, borrower.Id, lender.Id, offer.Id, 100_000, 105_000)
	require.NoError(t, ResolveOverdueFunding(lender.Id, fA.Id, LoanDefaultActionWriteoff, 0))

	now := time.Now()
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		acc, err := getLoanAccountTx(tx, borrower.Id)
		require.NoError(t, err)
		fundings, err := loadUserFundingsTx(tx, borrower.Id)
		require.NoError(t, err)
		require.Len(t, fundings, 1)
		require.Equal(t, fB.Id, fundings[0].Id)
		info, allocs, err := distributeRepayment(tx, acc, fundings, 105_000, now)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Len(t, allocs, 1)
		require.Equal(t, fB.Id, allocs[0].FundingId)
		require.NoError(t, tx.Save(acc).Error)
		return nil
	}))

	var a, b TokenLoanFunding
	require.NoError(t, DB.First(&a, fA.Id).Error)
	require.NoError(t, DB.First(&b, fB.Id).Error)
	require.Equal(t, LoanFundingWrittenOff, a.Status)
	require.Equal(t, int64(210_000), a.DebtQuota, "核销债务从未被分配/偿还")
	require.Equal(t, LoanFundingRepaid, b.Status)
	require.Zero(t, b.DebtQuota)
}
