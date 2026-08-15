package model

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// Checkin 签到记录
type Checkin struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_checkin_date"`
	CheckinDate  string `json:"checkin_date" gorm:"type:varchar(10);not null;uniqueIndex:idx_user_checkin_date"` // 格式: YYYY-MM-DD
	QuotaAwarded int    `json:"quota_awarded" gorm:"not null"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
}

// CheckinRecord 用于API返回的签到记录（不包含敏感字段）
type CheckinRecord struct {
	CheckinDate  string `json:"checkin_date"`
	QuotaAwarded int    `json:"quota_awarded"`
}

func (Checkin) TableName() string {
	return "checkins"
}

// GetUserCheckinRecords 获取用户在指定日期范围内的签到记录
func GetUserCheckinRecords(userId int, startDate, endDate string) ([]Checkin, error) {
	var records []Checkin
	err := DB.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?",
		userId, startDate, endDate).
		Order("checkin_date DESC").
		Find(&records).Error
	return records, err
}

// HasCheckedInToday 检查用户今天是否已签到
func HasCheckedInToday(userId int) (bool, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, today).
		Count(&count).Error
	return count > 0, err
}

// UserCheckin 执行用户签到，返回签到记录与签到自动还款结果（无还款时 loanRepay 为 nil）
// MySQL 和 PostgreSQL 使用事务保证原子性
// SQLite 不支持嵌套事务，使用顺序操作 + 手动回滚
func UserCheckin(userId int) (*Checkin, *LoanRepayInfo, error) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, nil, errors.New("签到功能未启用")
	}

	// 检查今天是否已签到
	hasChecked, err := HasCheckedInToday(userId)
	if err != nil {
		return nil, nil, err
	}
	if hasChecked {
		return nil, nil, errors.New("今日已签到")
	}

	// 计算随机额度奖励
	quotaAwarded := setting.MinQuota
	if setting.MaxQuota > setting.MinQuota {
		quotaAwarded = setting.MinQuota + rand.Intn(setting.MaxQuota-setting.MinQuota+1)
	}

	today := time.Now().Format("2006-01-02")
	checkin := &Checkin{
		UserId:       userId,
		CheckinDate:  today,
		QuotaAwarded: quotaAwarded,
		CreatedAt:    time.Now().Unix(),
	}

	// 根据数据库类型选择不同的策略
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		// SQLite 不支持嵌套事务，使用顺序操作 + 手动回滚
		return userCheckinWithoutTransaction(checkin, userId, quotaAwarded)
	}

	// MySQL 和 PostgreSQL 支持事务，使用事务保证原子性
	return userCheckinWithTransaction(checkin, userId, quotaAwarded)
}

// userCheckinWithTransaction 使用事务执行签到（适用于 MySQL 和 PostgreSQL）
func userCheckinWithTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, *LoanRepayInfo, error) {
	var repayInfo *LoanRepayInfo
	netQuota := quotaAwarded
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 步骤1: 创建签到记录
		// 数据库有唯一约束 (user_id, checkin_date)，可以防止并发重复签到
		if err := tx.Create(checkin).Error; err != nil {
			return errors.New("签到失败，请稍后重试")
		}

		// 步骤2: 签到自动还款（spec 4.4）：仅已有账户才进入还款路径，不给无贷用户建行
		if operation_setting.GetLoanSetting().CheckinRepayEnabled {
			now := time.Now()
			acc, err := getLoanAccountTx(tx, userId)
			if err != nil {
				return err
			}
			if acc != nil {
				settle(acc, now)
				info := applyCheckinRepay(acc, int64(quotaAwarded))
				// settle 就地推进 LastSettledDay，已存在账户的利息时钟必须在同一事务内落盘
				acc.UpdatedAt = now.Unix()
				if err := tx.Save(acc).Error; err != nil {
					return err
				}
				if info != nil {
					repayInfo = info
					netQuota = quotaAwarded - int(info.Amount)
					if err := tx.Create(&TokenLoanRecord{
						UserId:        userId,
						Type:          "repay",
						Amount:        info.Amount,
						InterestPart:  info.InterestPart,
						PrincipalPart: info.PrincipalPart,
						DebtAfter:     info.DebtAfter,
						Source:        "checkin",
						CreatedAt:     now.Unix(),
					}).Error; err != nil {
						return err
					}
				}
			}
		}

		// 步骤3: 在事务中增加用户额度（净额 = 奖励 - 还款）
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", netQuota)).Error; err != nil {
			return errors.New("签到失败：更新额度出错")
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	// 事务成功后，异步更新缓存（净额）
	go func() {
		_ = cacheIncrUserQuota(userId, int64(netQuota))
	}()

	return checkin, repayInfo, nil
}

// userCheckinWithoutTransaction 不使用事务执行签到（适用于 SQLite）
func userCheckinWithoutTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, *LoanRepayInfo, error) {
	// 步骤1: 创建签到记录
	// 数据库有唯一约束 (user_id, checkin_date)，可以防止并发重复签到
	if err := DB.Create(checkin).Error; err != nil {
		return nil, nil, errors.New("签到失败，请稍后重试")
	}

	var repayInfo *LoanRepayInfo
	var repayRecord *TokenLoanRecord
	netQuota := quotaAwarded

	// 步骤2: 签到自动还款（spec 4.4）：与事务分支镜像，仅已有账户才进入，失败时手动回滚
	if operation_setting.GetLoanSetting().CheckinRepayEnabled {
		now := time.Now()
		// SQLite 无 FOR UPDATE，lockForUpdate 退化为普通查询，直接用全局 DB
		acc, err := getLoanAccountTx(DB, userId)
		if err != nil {
			DB.Delete(checkin)
			return nil, nil, errors.New("签到失败：更新额度出错")
		}
		if acc != nil {
			settle(acc, now)
			info := applyCheckinRepay(acc, int64(quotaAwarded))
			// settle 就地推进 LastSettledDay，已存在账户的利息时钟必须落盘
			acc.UpdatedAt = now.Unix()
			if err := DB.Save(acc).Error; err != nil {
				DB.Delete(checkin)
				return nil, nil, errors.New("签到失败：更新额度出错")
			}
			if info != nil {
				repayInfo = info
				netQuota = quotaAwarded - int(info.Amount)
				repayRecord = &TokenLoanRecord{
					UserId:        userId,
					Type:          "repay",
					Amount:        info.Amount,
					InterestPart:  info.InterestPart,
					PrincipalPart: info.PrincipalPart,
					DebtAfter:     info.DebtAfter,
					Source:        "checkin",
					CreatedAt:     now.Unix(),
				}
				if err := DB.Create(repayRecord).Error; err != nil {
					// 台账写入失败：回滚账户与签到记录
					rollbackCheckinRepay(acc, info)
					DB.Delete(checkin)
					return nil, nil, errors.New("签到失败：更新额度出错")
				}
			}
		}
	}

	// 步骤3: 增加用户额度（净额 = 奖励 - 还款）
	// 使用 db=true 强制直接写入数据库，不使用批量更新
	if err := IncreaseUserQuota(userId, netQuota, true); err != nil {
		// IncreaseUserQuota 会在写库前异步递增 Redis 余额缓存（model/user.go:1434），
		// 失败回滚时必须同步补偿递减，否则缓存与 DB 不一致（两者相消，与执行顺序无关）。
		// 仅 netQuota > 0 才需要补偿：netQuota < 0 时 IncreaseUserQuota 在触发异步递增
		// 之前就返回错误（quota 不能为负数），负值补偿会变成向上递增，反而污染缓存
		if netQuota > 0 {
			if err := cacheDecrUserQuota(userId, int64(netQuota)); err != nil {
				common.SysError("failed to compensate user quota cache after checkin rollback: " + err.Error())
			}
		}
		// 如果增加额度失败，需要回滚台账、账户与签到记录
		if repayRecord != nil {
			DB.Delete(repayRecord)
		}
		if repayInfo != nil {
			var acc TokenLoanAccount
			if err := DB.Where("user_id = ?", userId).First(&acc).Error; err == nil {
				rollbackCheckinRepay(&acc, repayInfo)
			}
		}
		DB.Delete(checkin)
		return nil, nil, errors.New("签到失败：更新额度出错")
	}

	return checkin, repayInfo, nil
}

// rollbackCheckinRepay SQLite 手动回滚：把账户恢复到还款前状态（台账由调用方删除）。
// 有意不回滚 settle 推进的 LastSettledDay：利息本应累计到当天，回退利息时钟会少计息。
func rollbackCheckinRepay(acc *TokenLoanAccount, info *LoanRepayInfo) {
	acc.PrincipalQuota += info.PrincipalPart
	acc.DebtQuota += info.Amount
	acc.TotalRepaid -= info.Amount
	acc.UpdatedAt = time.Now().Unix()
	if err := DB.Save(acc).Error; err != nil {
		common.SysError(fmt.Sprintf("checkin repay rollback failed for user %d (amount %d): %v",
			acc.UserId, info.Amount, err))
	}
}

// GetUserCheckinStats 获取用户签到统计信息
func GetUserCheckinStats(userId int, month string) (map[string]interface{}, error) {
	// 获取指定月份的所有签到记录
	startDate := month + "-01"
	endDate := month + "-31"

	records, err := GetUserCheckinRecords(userId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 转换为不包含敏感字段的记录
	checkinRecords := make([]CheckinRecord, len(records))
	for i, r := range records {
		checkinRecords[i] = CheckinRecord{
			CheckinDate:  r.CheckinDate,
			QuotaAwarded: r.QuotaAwarded,
		}
	}

	// 检查今天是否已签到
	hasCheckedToday, _ := HasCheckedInToday(userId)

	// 获取用户所有时间的签到统计
	var totalCheckins int64
	var totalQuota int64
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Count(&totalCheckins)
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Select("COALESCE(SUM(quota_awarded), 0)").Scan(&totalQuota)

	return map[string]interface{}{
		"total_quota":      totalQuota,      // 所有时间累计获得的额度
		"total_checkins":   totalCheckins,   // 所有时间累计签到次数
		"checkin_count":    len(records),    // 本月签到次数
		"checked_in_today": hasCheckedToday, // 今天是否已签到
		"records":          checkinRecords,  // 本月签到记录详情（不含id和user_id）
	}, nil
}
