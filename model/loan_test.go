package model

import (
	"testing"
	"time"

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
