package model

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// withDailyRate 临时设置全局日利率，测试结束后恢复
func withDailyRate(t *testing.T, rate float64) {
	t.Helper()
	setting := operation_setting.GetLoanSetting()
	old := setting.DailyRate
	setting.DailyRate = rate
	t.Cleanup(func() { setting.DailyRate = old })
}

// withLoanSetting 临时整体修改词元贷配置，测试结束后恢复
func withLoanSetting(t *testing.T, mutate func(s *operation_setting.LoanSetting)) {
	t.Helper()
	setting := operation_setting.GetLoanSetting()
	old := *setting
	mutate(setting)
	t.Cleanup(func() { *setting = old })
}

// createLoanTestUser 创建借款测试用户（用户名唯一，避开 aff_code 唯一索引空值冲突）
func createLoanTestUser(t *testing.T) *User {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &TokenLoanRecord{}, &TokenLoanOffer{}, &TokenLoanFunding{}))
	username := fmt.Sprintf("loan-test-%d", time.Now().UnixNano())
	user := &User{
		Username: username,
		Password: "loan-test-password",
		Status:   common.UserStatusEnabled,
		AffCode:  username + "-aff",
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestLoanDayUsesServerLocalDay(t *testing.T) {
	d0 := loanDay(time.Date(2026, 8, 1, 23, 59, 59, 0, time.Local))
	d1 := loanDay(time.Date(2026, 8, 2, 0, 0, 0, 0, time.Local))
	require.Equal(t, d0+1, d1)
	// 同一本地日内任意时刻映射到同一日序号
	require.Equal(t, d0, loanDay(time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)))
}

func TestLoanSettleCrossDayCompound(t *testing.T) {
	withDailyRate(t, 0.001)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
	acc := &TokenLoanAccount{
		PrincipalQuota: 1000000,
		DebtQuota:      1000000,
		LastSettledDay: loanDay(start),
	}
	now := start.AddDate(0, 0, 3)
	settle(acc, now)
	// round(1000000 * 1.001^3) = round(1003003.001) = 1003003
	require.Equal(t, int64(1003003), acc.DebtQuota)
	require.Equal(t, loanDay(now), acc.LastSettledDay)
}

func TestLoanSettleSameDayIdempotent(t *testing.T) {
	withDailyRate(t, 0.001)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
	acc := &TokenLoanAccount{
		PrincipalQuota: 1000000,
		DebtQuota:      1000000,
		LastSettledDay: loanDay(start),
	}
	next := start.AddDate(0, 0, 1)
	settle(acc, next)
	require.Equal(t, int64(1001000), acc.DebtQuota)
	// 同日再次结算不得重复计息
	settle(acc, next.Add(6*time.Hour))
	require.Equal(t, int64(1001000), acc.DebtQuota)
	require.Equal(t, loanDay(next), acc.LastSettledDay)
}

func TestLoanSettleGracePeriodSkipsInterest(t *testing.T) {
	withDailyRate(t, 0.001)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
	now := start.AddDate(0, 0, 3)
	acc := &TokenLoanAccount{
		PrincipalQuota:    1000000,
		DebtQuota:         1000000,
		LastSettledDay:    loanDay(start),
		InterestFreeUntil: loanDay(now) + 5, // 宽限期覆盖 now
	}
	settle(acc, now)
	// 宽限期内 days=0，债务不变，但 LastSettledDay 照常推进
	require.Equal(t, int64(1000000), acc.DebtQuota)
	require.Equal(t, loanDay(now), acc.LastSettledDay)
}

func TestLoanSettlePartialInterestAfterGrace(t *testing.T) {
	withDailyRate(t, 0.001)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
	acc := &TokenLoanAccount{
		PrincipalQuota:    1000000,
		DebtQuota:         1000000,
		LastSettledDay:    loanDay(start),
		InterestFreeUntil: loanDay(start) + 2,
	}
	now := start.AddDate(0, 0, 5)
	settle(acc, now)
	// days = 5 - max(0, 2) = 3 → round(1000000 * 1.001^3) = 1003003
	require.Equal(t, int64(1003003), acc.DebtQuota)
	require.Equal(t, loanDay(now), acc.LastSettledDay)
}

func TestLoanEffectiveRateMinSemantics(t *testing.T) {
	withDailyRate(t, 0.001)
	// 无个人覆盖 → 全局利率
	require.Equal(t, 0.001, effectiveRate(&TokenLoanAccount{}))
	// 个人利率更低 → 取个人
	require.Equal(t, 0.0005, effectiveRate(&TokenLoanAccount{CustomDailyRate: 0.0005}))
	// 个人利率更高 → 取全局（min 语义）
	require.Equal(t, 0.001, effectiveRate(&TokenLoanAccount{CustomDailyRate: 0.005}))
}

func TestLoanSettleMinimalDebtNoError(t *testing.T) {
	withDailyRate(t, 0.001)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
	acc := &TokenLoanAccount{
		PrincipalQuota: 1,
		DebtQuota:      1,
		LastSettledDay: loanDay(start),
	}
	now := start.AddDate(0, 0, 10)
	settle(acc, now)
	// round(1 * 1.001^10) = 1，个位数 quota 跨天不报错且 debt >= principal 不变式成立
	require.Equal(t, int64(1), acc.DebtQuota)
	require.GreaterOrEqual(t, acc.DebtQuota, acc.PrincipalQuota)
}

func TestLoanProjectStatusReadOnly(t *testing.T) {
	withDailyRate(t, 0.001)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
	acc := &TokenLoanAccount{
		PrincipalQuota: 1000000,
		DebtQuota:      1000000,
		LastSettledDay: loanDay(start),
	}
	now := start.AddDate(0, 0, 3)
	debt, interest := ProjectLoanStatus(acc, now)
	require.Equal(t, int64(1003003), debt)
	require.Equal(t, int64(3003), interest)
	// 只读投影：原账户不被修改
	require.Equal(t, int64(1000000), acc.DebtQuota)
	require.Equal(t, loanDay(start), acc.LastSettledDay)
}

func TestLoanGetOrCreateAccountTx(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &TokenLoanRecord{}))
	userId := 987654321

	var first *TokenLoanAccount
	err := DB.Transaction(func(tx *gorm.DB) error {
		acc, err := getOrCreateLoanAccountTx(tx, userId)
		if err != nil {
			return err
		}
		first = acc
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, userId, first.UserId)
	require.Equal(t, int64(0), first.DebtQuota)
	require.Equal(t, loanDay(time.Now()), first.LastSettledDay)

	// 再次调用返回同一行，不重复创建
	var second *TokenLoanAccount
	err = DB.Transaction(func(tx *gorm.DB) error {
		acc, err := getOrCreateLoanAccountTx(tx, userId)
		if err != nil {
			return err
		}
		second = acc
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, first.UserId, second.UserId)
	require.Equal(t, first.CreatedAt, second.CreatedAt)

	var count int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", userId).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

// ===== Task 3: 同意声明与借款 =====
// 换算基准：common.QuotaPerUnit = 500000，即 1 USD = 500000 quota

func TestBorrowLoanDisabled(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = false
		s.TermsEnabled = false
	})
	user := createLoanTestUser(t)
	_, _, err := BorrowLoan(user.Id, "1.00", 0, nil)
	require.ErrorIs(t, err, ErrLoanDisabled)
}

func TestBorrowLoanRejectsInvalidAmount(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
	})
	user := createLoanTestUser(t)
	// 非数字、负数、零、超过两位小数一律拒绝
	for _, amt := range []string{"", "abc", "-1.00", "0", "0.00", "1.005", "0.001", "1.234"} {
		_, _, err := BorrowLoan(user.Id, amt, 0, nil)
		require.ErrorIs(t, err, ErrLoanInvalidAmount, "amount %q should be rejected", amt)
	}
	// 两位小数与整数放行（但受额度上限约束，不能报金额错误）
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.MaxTotal = math.MaxInt64 })
	for _, amt := range []string{"1.00", "0.01", "1.5", "2"} {
		_, _, err := BorrowLoan(user.Id, amt, 0, nil)
		require.NotErrorIs(t, err, ErrLoanInvalidAmount, "amount %q should be accepted", amt)
	}
}

func TestBorrowLoanQuotaOverflow(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = math.MaxInt64
	})
	user := createLoanTestUser(t)
	// 10000 USD * 500000 = 5e9 quota，超 int32 上界
	_, _, err := BorrowLoan(user.Id, "10000.00", 0, nil)
	require.ErrorIs(t, err, ErrLoanQuotaOverflow)

	// 金额本身未超 int32，但 用户余额 + 借款 超上界
	user2 := createLoanTestUser(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user2.Id).
		Update("quota", math.MaxInt32-100).Error)
	_, _, err = BorrowLoan(user2.Id, "1.00", 0, nil) // 500000 quota
	require.ErrorIs(t, err, ErrLoanQuotaOverflow)
}

// QuotaPerUnit 是运行时可调配置：调成 1 后，"0.01" USD 换算为 0 quota，
// amount<=0 必须被 ErrLoanInvalidAmount 拒绝，不能放行 0 额度借款
func TestBorrowLoanZeroQuotaAmountRejected(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = math.MaxInt64
	})
	user := createLoanTestUser(t)
	// 回收 id 名下可能残留的贷款数据，保证账户为新建
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanRecord{}).Error)

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	_, _, err := BorrowLoan(user.Id, "0.01", 0, nil) // 0.01 USD * 1 = 0.01 quota，取整为 0
	require.ErrorIs(t, err, ErrLoanInvalidAmount)

	// 正数控制组：1.00 USD = 1 quota，仍可正常借款
	acc, _, err := BorrowLoan(user.Id, "1.00", 0, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), acc.DebtQuota)
}

func TestBorrowLoanRequiresTermsAgreement(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = true
		s.MaxTotal = 500000
	})
	user := createLoanTestUser(t)
	_, _, err := BorrowLoan(user.Id, "0.10", 0, nil)
	require.ErrorIs(t, err, ErrLoanTermsNotAgreed)
}

func TestAgreeLoanTermsIdempotent(t *testing.T) {
	user := createLoanTestUser(t)
	require.NoError(t, AgreeLoanTerms(user.Id))

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.NotZero(t, acc.TermsAgreedAt)
	first := acc.TermsAgreedAt

	// 幂等：再次同意不覆盖首次时间
	require.NoError(t, AgreeLoanTerms(user.Id))
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, first, acc.TermsAgreedAt)

	var count int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestBorrowLoanSucceedsAfterTermsAgreement(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = true
		s.MaxTotal = 500000
	})
	user := createLoanTestUser(t)
	require.NoError(t, AgreeLoanTerms(user.Id))

	acc, _, err := BorrowLoan(user.Id, "0.10", 0, nil) // 50000 quota
	require.NoError(t, err)
	require.Equal(t, int64(50000), acc.PrincipalQuota)
	require.Equal(t, int64(50000), acc.DebtQuota)
	require.Equal(t, int64(50000), acc.TotalBorrowed)

	// 台账记录
	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "borrow").First(&rec).Error)
	require.Equal(t, int64(50000), rec.Amount)
	require.Equal(t, int64(0), rec.InterestPart)
	require.Equal(t, int64(50000), rec.PrincipalPart)
	require.Equal(t, int64(50000), rec.DebtAfter)
	require.Equal(t, "manual", rec.Source)

	// 用户余额同步增加
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 50000, u.Quota)
}

func TestBorrowLoanSucceedsWhenTermsDisabled(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 500000
	})
	user := createLoanTestUser(t)
	acc, _, err := BorrowLoan(user.Id, "0.10", 0, nil)
	require.NoError(t, err)
	require.Equal(t, int64(50000), acc.DebtQuota)
	require.Zero(t, acc.TermsAgreedAt)
}

func TestBorrowLoanExceedsMaxTotal(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 500000
		s.MaxPerBorrow = 0
	})
	user := createLoanTestUser(t)

	// 单笔超总额上限
	_, _, err := BorrowLoan(user.Id, "1.01", 0, nil) // 505000 > 500000
	require.ErrorIs(t, err, ErrLoanLimitExceeded)

	// 首笔成功后，debt + amount 超上限的第二笔被拒
	_, _, err = BorrowLoan(user.Id, "0.60", 0, nil) // 300000
	require.NoError(t, err)
	_, _, err = BorrowLoan(user.Id, "0.50", 0, nil) // 300000 + 250000 = 550000 > 500000
	require.ErrorIs(t, err, ErrLoanLimitExceeded)

	// 失败借款不落台账、不加余额
	var count int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Equal(t, int64(1), count)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 300000, u.Quota)
}

func TestBorrowLoanMaxPerBorrow(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 500000
		s.MaxPerBorrow = 100000
	})
	user := createLoanTestUser(t)
	_, _, err := BorrowLoan(user.Id, "0.30", 0, nil) // 150000 > 单次上限 100000
	require.ErrorIs(t, err, ErrLoanLimitExceeded)
	_, _, err = BorrowLoan(user.Id, "0.20", 0, nil) // 100000，恰好等于上限放行
	require.NoError(t, err)
}

func TestBorrowLoanCustomMaxTotalOverride(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 500000
		s.MaxPerBorrow = 0
	})
	user := createLoanTestUser(t)
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:         user.Id,
		CustomMaxTotal: 1000000, // 个人上限覆盖全局 500000
		LastSettledDay: loanDay(now),
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}).Error)

	acc, _, err := BorrowLoan(user.Id, "1.50", 0, nil) // 750000：超全局但在个人上限内
	require.NoError(t, err)
	require.Equal(t, int64(750000), acc.DebtQuota)
}

func TestBorrowLoanRegisterTooNew(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 500000
		s.MinRegisterDays = 30
	})
	user := createLoanTestUser(t)
	_, _, err := BorrowLoan(user.Id, "0.10", 0, nil)
	require.ErrorIs(t, err, ErrLoanRegisterTooNew)

	// 注册满 30 天后放行
	old := time.Now().AddDate(0, 0, -31).Unix()
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("created_at", old).Error)
	_, _, err = BorrowLoan(user.Id, "0.10", 0, nil)
	require.NoError(t, err)
}

func TestBorrowLoanConcurrentRespectsCap(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 500000
		s.MaxPerBorrow = 0
	})
	user := createLoanTestUser(t)

	// 两个 goroutine 各借 60% 上限（300000），最多一笔成功
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, errs[idx] = BorrowLoan(user.Id, "0.60", 0, nil)
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	require.LessOrEqual(t, successes, 1)

	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.LessOrEqual(t, acc.TotalBorrowed, int64(500000))
	require.LessOrEqual(t, acc.DebtQuota, int64(500000))

	// 账户、台账、用户余额三方一致
	var count int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).
		Where("user_id = ? AND type = ?", user.Id, "borrow").Count(&count).Error)
	require.Equal(t, int64(successes), count)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, acc.TotalBorrowed, int64(u.Quota))
}

func TestBorrowLoanUserDisabled(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 500000
	})
	user := createLoanTestUser(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).
		Update("status", common.UserStatusDisabled).Error)

	_, _, err := BorrowLoan(user.Id, "0.10", 0, nil)
	require.ErrorIs(t, err, ErrLoanUserDisabled)

	// 被拒后无账户、无台账、无余额变动
	var count int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Equal(t, int64(0), count)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 0, u.Quota)
}

func TestBorrowLoanQuotaCreditFailureRollsBackAll(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
		s.MaxTotal = 500000
	})
	user := createLoanTestUser(t)

	// 注入 quota 入账失败：SQLite 触发器拦截 users.quota 更新
	require.NoError(t, DB.Exec(`CREATE TRIGGER loan_test_block_quota_update
		BEFORE UPDATE OF quota ON users
		BEGIN SELECT RAISE(ABORT, 'quota update blocked by test'); END`).Error)
	t.Cleanup(func() {
		_ = DB.Exec(`DROP TRIGGER IF EXISTS loan_test_block_quota_update`).Error
	})

	_, _, err := BorrowLoan(user.Id, "0.10", 0, nil)
	require.Error(t, err)

	// 入账与账户/台账在同一事务：失败后账户行与台账都不存在、用户 quota 不变
	var accCount int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", user.Id).Count(&accCount).Error)
	require.Equal(t, int64(0), accCount)
	var recCount int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).Where("user_id = ?", user.Id).Count(&recCount).Error)
	require.Equal(t, int64(0), recCount)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 0, u.Quota)
}

// setupRepayTestUser 创建带债务的还款测试用户：直接建行造 debt 债务（含 interest 利息），
// 用户余额设为 balance，避免走借款流程的耦合。Task 9 起债务以 funding 为准，因此同步创建
// 一条匹配的 platform funding（账户行 = fundings 投影），保证 RepayLoan 走 funding 分配路径。
func setupRepayTestUser(t *testing.T, debt, principal, balance int64) *User {
	t.Helper()
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
	})
	user := createLoanTestUser(t)
	cleanupLoanBorrowData(t, user.Id, 0)
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:         user.Id,
		PrincipalQuota: principal,
		DebtQuota:      debt,
		LastSettledDay: loanDay(now),
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}).Error)
	if debt > 0 {
		require.NoError(t, DB.Create(&TokenLoanFunding{
			LoanUserId:         user.Id,
			SourceType:         LoanFundingPlatform,
			Amount:             principal,
			PrincipalRemaining: principal,
			DebtQuota:          debt,
			LastSettledDay:     loanDay(now),
			Rate:               0.001,
			RepayPlan:          LoanRepayFull,
			Status:             LoanFundingActive,
			DueDay:             loanDay(now) + 30,
			CreatedAt:          now.Unix(),
			UpdatedAt:          now.Unix(),
		}).Error)
	}
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).
		Update("quota", balance).Error)
	return user
}

func TestRepayLoanDisabled(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = false
	})
	user := createLoanTestUser(t)
	_, _, err := RepayLoan(user.Id, "1.00")
	require.ErrorIs(t, err, ErrLoanDisabled)
}

func TestRepayLoanNoAccount(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
	})
	user := createLoanTestUser(t)
	_, _, err := RepayLoan(user.Id, "1.00")
	require.ErrorIs(t, err, ErrLoanNoDebt)

	// 不存在的账户不得被还款路径创建
	var count int64
	require.NoError(t, DB.Model(&TokenLoanAccount{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Equal(t, int64(0), count)
}

func TestRepayLoanZeroDebt(t *testing.T) {
	user := setupRepayTestUser(t, 0, 0, 100000)
	_, _, err := RepayLoan(user.Id, "1.00")
	require.ErrorIs(t, err, ErrLoanNoDebt)
}

func TestRepayLoanRejectsInvalidAmount(t *testing.T) {
	user := setupRepayTestUser(t, 100000, 90000, 500000)
	for _, amt := range []string{"", "abc", "-1.00", "0", "0.00", "1.005", "0.001"} {
		_, _, err := RepayLoan(user.Id, amt)
		require.ErrorIs(t, err, ErrLoanInvalidAmount, "amount %q should be rejected", amt)
	}
}

// QuotaPerUnit 调成 1 后，"0.01" USD 换算为 0 quota，RepayLoan 必须拒绝
func TestRepayLoanZeroQuotaAmountRejected(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = false
	})
	user := createLoanTestUser(t)
	// 回收 id 名下可能残留的贷款数据，避免与新建账户的主键冲突
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanRecord{}).Error)
	require.NoError(t, DB.Where("loan_user_id = ?", user.Id).Delete(&TokenLoanFunding{}).Error)
	now := time.Now()
	require.NoError(t, DB.Create(&TokenLoanAccount{
		UserId:         user.Id,
		PrincipalQuota: 90000,
		DebtQuota:      100000,
		LastSettledDay: loanDay(now),
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}).Error)
	// Task 9 起债务以 funding 为准：同步建一条匹配的 platform funding
	require.NoError(t, DB.Create(&TokenLoanFunding{
		LoanUserId:         user.Id,
		SourceType:         LoanFundingPlatform,
		Amount:             90000,
		PrincipalRemaining: 90000,
		DebtQuota:          100000,
		LastSettledDay:     loanDay(now),
		Rate:               0.001,
		RepayPlan:          LoanRepayFull,
		Status:             LoanFundingActive,
		DueDay:             loanDay(now) + 30,
		CreatedAt:          now.Unix(),
		UpdatedAt:          now.Unix(),
	}).Error)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).
		Update("quota", 500000).Error)

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	_, _, err := RepayLoan(user.Id, "0.01") // 换算后 0 quota
	require.ErrorIs(t, err, ErrLoanInvalidAmount)

	// 正数控制组：1.00 USD = 1 quota，可正常还款（债务减 1）
	acc, _, err := RepayLoan(user.Id, "1.00")
	require.NoError(t, err)
	require.Equal(t, int64(99999), acc.DebtQuota)
}

func TestRepayLoanInsufficientBalance(t *testing.T) {
	// 债务 100000，余额只有 30000，显式还 0.10 USD（50000）被拒
	user := setupRepayTestUser(t, 100000, 90000, 30000)
	_, _, err := RepayLoan(user.Id, "0.10")
	require.ErrorIs(t, err, ErrLoanInsufficientBalance)

	// 拒绝后债务、余额、台账均无变化
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(100000), acc.DebtQuota)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 30000, u.Quota)
	var count int64
	require.NoError(t, DB.Model(&TokenLoanRecord{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Equal(t, int64(0), count)
}

func TestRepayLoanPartialInterestFirst(t *testing.T) {
	// 债务 100000（本金 90000 + 利息 10000），还 0.08 USD = 40000：先抵息 10000 再抵本 30000，
	// 手续费 = round(30000 * 0.0001) = 3
	user := setupRepayTestUser(t, 100000, 90000, 500000)
	acc, info, err := RepayLoan(user.Id, "0.08")
	require.NoError(t, err)
	require.Equal(t, int64(40000), info.Amount)
	require.Equal(t, int64(10000), info.InterestPart)
	require.Equal(t, int64(30000), info.PrincipalPart)
	require.Equal(t, int64(3), info.FeePart)
	require.Equal(t, int64(60000), info.DebtAfter)
	require.Equal(t, int64(60000), acc.DebtQuota)
	require.Equal(t, int64(60000), acc.PrincipalQuota)
	require.Equal(t, int64(40000), acc.TotalRepaid)

	// 台账改为 funding 粒度：一条 repay 行（挂 funding_id）；手续费不落台账
	// （平台收入，仅体现在 info.FeePart 与余额扣款）
	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "repay").First(&rec).Error)
	require.Equal(t, "manual", rec.Source)
	require.Equal(t, int64(40000), rec.Amount)
	require.Zero(t, rec.FeePart)
	require.Equal(t, int64(60000), rec.DebtAfter)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 459997, u.Quota)
}

func TestRepayLoanAll(t *testing.T) {
	// 余额充足：all 全额还清（金额不受两位小数限制，精确到 quota）
	// 手续费 = round(90001 * 0.0001) = 9
	user := setupRepayTestUser(t, 100001, 90001, 500000)
	acc, info, err := RepayLoan(user.Id, "all")
	require.NoError(t, err)
	require.Equal(t, int64(100001), info.Amount)
	require.Equal(t, int64(9), info.FeePart)
	require.Equal(t, int64(0), acc.DebtQuota)
	require.Equal(t, int64(0), acc.PrincipalQuota)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 399990, u.Quota)
}

func TestRepayLoanAllClampedByBalance(t *testing.T) {
	// 余额不足覆盖债务：all 只还余额能覆盖的部分（还款额 + 手续费 <= 余额）
	// 迭代收敛：repay 39997 + fee round(29997*0.0001)=3 = 40000
	user := setupRepayTestUser(t, 100000, 90000, 40000)
	acc, info, err := RepayLoan(user.Id, "ALL") // 大小写不敏感
	require.NoError(t, err)
	require.Equal(t, int64(39997), info.Amount)
	require.Equal(t, int64(3), info.FeePart)
	require.Equal(t, int64(60003), acc.DebtQuota)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 0, u.Quota)
}

func TestRepayLoanAllZeroBalance(t *testing.T) {
	user := setupRepayTestUser(t, 100000, 90000, 0)
	_, _, err := RepayLoan(user.Id, "all")
	require.ErrorIs(t, err, ErrLoanInsufficientBalance)
}

func TestRepayLoanExplicitAmountClampedToDebt(t *testing.T) {
	// 显式金额超过债务时按债务截断；手续费 = round(50000 * 0.0001) = 5
	user := setupRepayTestUser(t, 50000, 50000, 500000)
	acc, info, err := RepayLoan(user.Id, "1.00") // 500000 quota > 债务 50000
	require.NoError(t, err)
	require.Equal(t, int64(50000), info.Amount)
	require.Equal(t, int64(5), info.FeePart)
	require.Equal(t, int64(0), acc.DebtQuota)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 449995, u.Quota)
}

func TestRepayLoanFeeDisabled(t *testing.T) {
	// 费率为 0 时不收手续费，行为与旧版一致
	user := setupRepayTestUser(t, 100000, 90000, 500000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.RepayFeeRate = 0 })
	_, info, err := RepayLoan(user.Id, "0.08")
	require.NoError(t, err)
	require.Equal(t, int64(0), info.FeePart)
	var u User
	require.NoError(t, DB.Select("quota").First(&u, user.Id).Error)
	require.Equal(t, 460000, u.Quota)
}

func TestRepayLoanFeeOnlyOnPrincipal(t *testing.T) {
	// 还款全部抵息时无抵本部分，手续费为 0
	user := setupRepayTestUser(t, 100000, 90000, 500000)
	_, info, err := RepayLoan(user.Id, "0.01") // 5000 quota，全抵息（利息 10000）
	require.NoError(t, err)
	require.Equal(t, int64(5000), info.InterestPart)
	require.Equal(t, int64(0), info.PrincipalPart)
	require.Equal(t, int64(0), info.FeePart)
}

// 新账户建行即带初始信用分（P1-1）：任意真实账户创建路径（同意条款 / 同意放贷声明）
// 都应产出 credit_initial，而不是 0（0 仅保留给迁移前的存量账户）
func TestGetOrCreateLoanAccountSetsCreditInitial(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.CreditInitial = 66 })

	// 路径一：同意借款条款
	u1 := createLoanTestUser(t)
	require.NoError(t, AgreeLoanTerms(u1.Id))
	var acc1 TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", u1.Id).First(&acc1).Error)
	require.Equal(t, 66, acc1.CreditScore)

	// 路径二：同意放贷免责声明
	u2 := createLoanTestUser(t)
	require.NoError(t, AgreeLenderDisclaimer(u2.Id))
	var acc2 TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", u2.Id).First(&acc2).Error)
	require.Equal(t, 66, acc2.CreditScore)

	// 幂等路径不重置信用分：再次同意不改写 CreditScore
	require.NoError(t, AgreeLoanTerms(u1.Id))
	require.NoError(t, DB.Where("user_id = ?", u1.Id).First(&acc1).Error)
	require.Equal(t, 66, acc1.CreditScore)
}
