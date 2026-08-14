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
	require.NoError(t, DB.AutoMigrate(&TokenLoanAccount{}, &TokenLoanRecord{}))
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
	_, err := BorrowLoan(user.Id, "1.00")
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
		_, err := BorrowLoan(user.Id, amt)
		require.ErrorIs(t, err, ErrLoanInvalidAmount, "amount %q should be rejected", amt)
	}
	// 两位小数与整数放行（但受额度上限约束，不能报金额错误）
	withLoanSetting(t, func(s *operation_setting.LoanSetting) { s.MaxTotal = math.MaxInt64 })
	for _, amt := range []string{"1.00", "0.01", "1.5", "2"} {
		_, err := BorrowLoan(user.Id, amt)
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
	_, err := BorrowLoan(user.Id, "10000.00")
	require.ErrorIs(t, err, ErrLoanQuotaOverflow)

	// 金额本身未超 int32，但 用户余额 + 借款 超上界
	user2 := createLoanTestUser(t)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user2.Id).
		Update("quota", math.MaxInt32-100).Error)
	_, err = BorrowLoan(user2.Id, "1.00") // 500000 quota
	require.ErrorIs(t, err, ErrLoanQuotaOverflow)
}

func TestBorrowLoanRequiresTermsAgreement(t *testing.T) {
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.Enabled = true
		s.TermsEnabled = true
		s.MaxTotal = 500000
	})
	user := createLoanTestUser(t)
	_, err := BorrowLoan(user.Id, "0.10")
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

	acc, err := BorrowLoan(user.Id, "0.10") // 50000 quota
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
	acc, err := BorrowLoan(user.Id, "0.10")
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
	_, err := BorrowLoan(user.Id, "1.01") // 505000 > 500000
	require.ErrorIs(t, err, ErrLoanLimitExceeded)

	// 首笔成功后，debt + amount 超上限的第二笔被拒
	_, err = BorrowLoan(user.Id, "0.60") // 300000
	require.NoError(t, err)
	_, err = BorrowLoan(user.Id, "0.50") // 300000 + 250000 = 550000 > 500000
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
	_, err := BorrowLoan(user.Id, "0.30") // 150000 > 单次上限 100000
	require.ErrorIs(t, err, ErrLoanLimitExceeded)
	_, err = BorrowLoan(user.Id, "0.20") // 100000，恰好等于上限放行
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

	acc, err := BorrowLoan(user.Id, "1.50") // 750000：超全局但在个人上限内
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
	_, err := BorrowLoan(user.Id, "0.10")
	require.ErrorIs(t, err, ErrLoanRegisterTooNew)

	// 注册满 30 天后放行
	old := time.Now().AddDate(0, 0, -31).Unix()
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("created_at", old).Error)
	_, err = BorrowLoan(user.Id, "0.10")
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
			_, errs[idx] = BorrowLoan(user.Id, "0.60")
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

	_, err := BorrowLoan(user.Id, "0.10")
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

	_, err := BorrowLoan(user.Id, "0.10")
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
