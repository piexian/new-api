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

// setupCheckinLoanUser 迁移签到表并创建测试用户（贷款表由 createLoanTestUser 迁移）
func setupCheckinLoanUser(t *testing.T) *User {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Checkin{}))
	return createLoanTestUser(t)
}

// createLoanDebtAccount 创建带指定本金/债务的贷款账户
// LastSettledDay=今天，settle 当天不再计息，保证债务数值确定
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

// debt=0（无贷款账户）时 loanRepay=nil 且旧行为不变：全额入账、无还款台账、不给无贷用户建账户行
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

// 奖励 < 利息：全部抵息，本金不动，净额 0 入账
func TestUserCheckinRepayCoversInterestOnly(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	// 本金 10000，债务 20000 → 利息 10000，奖励 5000 全部抵息
	createLoanDebtAccount(t, user.Id, 10000, 20000)

	checkin, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.Equal(t, 5000, checkin.QuotaAwarded) // quota_awarded 保持 gross
	require.NotNil(t, repay)
	require.Equal(t, int64(5000), repay.Amount)
	require.Equal(t, int64(5000), repay.InterestPart)
	require.Equal(t, int64(0), repay.PrincipalPart)
	require.Equal(t, int64(15000), repay.DebtAfter)

	// 账户：利息被抵，本金不变
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(15000), acc.DebtQuota)
	require.Equal(t, int64(10000), acc.PrincipalQuota)
	require.Equal(t, int64(5000), acc.TotalRepaid)

	// 净额 = 5000 - 5000 = 0，用户余额不变
	require.Equal(t, 0, checkinUserQuota(t, user.Id))

	// 台账拆分正确
	var rec TokenLoanRecord
	require.NoError(t, DB.Where("user_id = ? AND type = ?", user.Id, "repay").First(&rec).Error)
	require.Equal(t, int64(5000), rec.Amount)
	require.Equal(t, int64(5000), rec.InterestPart)
	require.Equal(t, int64(0), rec.PrincipalPart)
	require.Equal(t, int64(15000), rec.DebtAfter)
	require.Equal(t, "checkin", rec.Source)
}

// 奖励 > 债务：清账后净额入账（DB quota 增量 = 净额）
func TestUserCheckinRepayClearsDebt(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	// 本金 2500，债务 3000 → 利息 500，奖励 5000 清账后净额 2000
	createLoanDebtAccount(t, user.Id, 2500, 3000)

	_, repay, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.NotNil(t, repay)
	require.Equal(t, int64(3000), repay.Amount)
	require.Equal(t, int64(500), repay.InterestPart)
	require.Equal(t, int64(2500), repay.PrincipalPart)
	require.Equal(t, int64(0), repay.DebtAfter)

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

// CheckinRepayEnabled=false：不还款，全额入账，账户与台账不变
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

// SQLite 回滚路径：台账写入失败 → 账户回滚（含 settle 落盘后的还款数值恢复）且签到记录被删
func TestUserCheckinRepayLedgerFailureRollback(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	createLoanDebtAccount(t, user.Id, 10000, 20000)
	renameTableForFailure(t, "token_loan_records")

	_, repay, err := UserCheckin(user.Id)
	require.Error(t, err)
	require.Nil(t, repay)

	// 账户回滚到还款前（LastSettledDay=今天，settle 无计息，数值与初始一致）
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(20000), acc.DebtQuota)
	require.Equal(t, int64(10000), acc.PrincipalQuota)
	require.Equal(t, int64(0), acc.TotalRepaid)

	// 签到记录已删除，用户余额未变
	hasChecked, err := HasCheckedInToday(user.Id)
	require.NoError(t, err)
	require.False(t, hasChecked)
	require.Equal(t, 0, checkinUserQuota(t, user.Id))
}

// SQLite 回滚路径：IncreaseUserQuota 失败 → 台账删除 + 账户回滚 + 签到记录删除
func TestUserCheckinRepayQuotaFailureRollback(t *testing.T) {
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	// 奖励 5000 > 债务 3000：走到 IncreaseUserQuota(净额 2000) 才失败
	createLoanDebtAccount(t, user.Id, 2500, 3000)
	renameTableForFailure(t, "users")

	_, repay, err := UserCheckin(user.Id)
	require.Error(t, err)
	require.Nil(t, repay)

	// 台账已删除，账户回滚到还款前
	require.Equal(t, int64(0), countLoanRecords(t, user.Id, "repay"))
	var acc TokenLoanAccount
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&acc).Error)
	require.Equal(t, int64(3000), acc.DebtQuota)
	require.Equal(t, int64(2500), acc.PrincipalQuota)
	require.Equal(t, int64(0), acc.TotalRepaid)

	// 签到记录已删除
	hasChecked, err := HasCheckedInToday(user.Id)
	require.NoError(t, err)
	require.False(t, hasChecked)
}

// SQLite 回滚路径：IncreaseUserQuota 在写库前会异步递增 Redis 余额缓存，
// 失败回滚时必须补偿递减，保证缓存 Quota 与 DB 一致（用 miniredis 观察缓存行为）
func TestUserCheckinQuotaFailureCompensatesCache(t *testing.T) {
	setupUserCacheVersionTest(t)
	withCheckinSetting(t, 5000)
	withLoanSetting(t, func(s *operation_setting.LoanSetting) {
		s.CheckinRepayEnabled = true
	})
	user := setupCheckinLoanUser(t)
	// 回收 id 名下可能残留的贷款数据，再建账
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanAccount{}).Error)
	require.NoError(t, DB.Where("user_id = ?", user.Id).Delete(&TokenLoanRecord{}).Error)
	// 奖励 5000 > 债务 3000：走到 IncreaseUserQuota(净额 2000) 才失败
	createLoanDebtAccount(t, user.Id, 2500, 3000)

	// 预置用户缓存，Quota 与 DB 一致（0）。
	// RedisHIncrBy 仅在 key 带 TTL 时才真正递增，需与生产缓存一致地设置过期时间
	ctx := context.Background()
	require.NoError(t, common.RDB.HSet(ctx, getUserCacheKey(user.Id), map[string]interface{}{
		"Id":     user.Id,
		"Role":   common.RoleCommonUser,
		"Quota":  0,
		"Status": common.UserStatusEnabled,
	}).Err())
	require.NoError(t, common.RDB.Expire(ctx, getUserCacheKey(user.Id), time.Duration(common.RedisKeyCacheSeconds())*time.Second).Err())

	renameTableForFailure(t, "users")

	_, _, err := UserCheckin(user.Id)
	require.Error(t, err)

	// 异步缓存递增与失败后的补偿递减相消（HINCRBY 可交换），最终回到 DB 值 0
	require.Eventually(t, func() bool {
		quota, err := common.RDB.HGet(ctx, getUserCacheKey(user.Id), "Quota").Int64()
		return err == nil && quota == 0
	}, time.Second, 10*time.Millisecond)
}
