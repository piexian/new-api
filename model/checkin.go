package model

import (
	"errors"
	"math/rand"
	"time"

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
	// 签到实际入账净额（奖励 - 贷款自动还款）。额度次日清算以此为准，
	// 老数据默认为 0 表示不参与回收，避免追溯扣减未被告知的额度。
	NetCredited int `json:"net_credited" gorm:"not null;default:0"`
	// 以下字段服务于「签到额度当日有效」，未启用该功能时保持为 0
	ExpiredQuota int   `json:"expired_quota" gorm:"not null;default:0"`           // 清算时实际回收的额度
	SettledAt    int64 `json:"settled_at" gorm:"bigint;not null;default:0;index"` // 清算时间戳，0 表示尚未清算
	// 是否为补签记录（补签的当天实际未签到，事后补录）
	IsMakeUp bool `json:"is_makeup" gorm:"column:is_makeup;not null;default:false"`
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

// UserCheckin 执行用户签到，返回签到记录、签到自动还款结果（无还款时 loanRepay 为 nil）
// 与放贷人入账清单（按放贷人聚合，供 controller 写入充值日志；无入账时为空切片）。
// quotaAwarded 由调用方（controller 的自适应奖励计算）给出；<= 0 时按配置区间
// 随机摇一个（兼容旧行为与既有测试）。
// 所有数据库共用单一事务路径：GORM 事务在 SQLite 单写者模型下同样可用（BorrowLoan/
// RepayLoan 同模式），事务内一律经 tx 访问 DB（lockForUpdate 在 SQLite 下退化为普通
// 查询）。旧版为 SQLite 单独保留的顺序执行 + 手动回滚分支，Task 10 起还款涉及多行写
// （funding/offer/放贷人入账）手动回滚难以保全，已合并删除。
func UserCheckin(userId int, quotaAwarded int) (*Checkin, *LoanRepayInfo, []LenderCredit, error) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, nil, nil, errors.New("签到功能未启用")
	}

	// 检查今天是否已签到
	hasChecked, err := HasCheckedInToday(userId)
	if err != nil {
		return nil, nil, nil, err
	}
	if hasChecked {
		return nil, nil, nil, errors.New("今日已签到")
	}

	// 未指定奖励时按配置区间随机
	if quotaAwarded <= 0 {
		quotaAwarded = setting.MinQuota
		if setting.MaxQuota > setting.MinQuota {
			quotaAwarded = setting.MinQuota + rand.Intn(setting.MaxQuota-setting.MinQuota+1)
		}
	}

	today := time.Now().Format("2006-01-02")
	checkin := &Checkin{
		UserId:       userId,
		CheckinDate:  today,
		QuotaAwarded: quotaAwarded,
		CreatedAt:    time.Now().Unix(),
	}

	return userCheckinWithTransaction(checkin, userId, quotaAwarded)
}

// UserMakeupCheckin 补签：为指定历史日期补录一条签到记录（IsMakeUp=true）。
// 日期合法性（格式、范围、是否已有记录）由 controller 经 MakeupEligibleDates 校验，
// 这里只做最后防线：必须是今天之前的合法日期；(user_id, checkin_date) 唯一约束
// 兜底防并发重复。
func UserMakeupCheckin(userId int, date string, quotaAwarded int) (*Checkin, *LoanRepayInfo, []LenderCredit, error) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, nil, nil, errors.New("签到功能未启用")
	}
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return nil, nil, nil, errors.New("补签日期格式错误")
	}
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if !day.Before(todayStart) {
		return nil, nil, nil, errors.New("只能补签今天之前的日期")
	}
	if quotaAwarded <= 0 {
		quotaAwarded = setting.MinQuota
	}

	checkin := &Checkin{
		UserId:       userId,
		CheckinDate:  date,
		QuotaAwarded: quotaAwarded,
		CreatedAt:    time.Now().Unix(),
		IsMakeUp:     true,
	}

	return userCheckinWithTransaction(checkin, userId, quotaAwarded)
}

// userCheckinWithTransaction 在单一事务内执行签到（所有数据库共用）：
//  1. 创建签到记录——唯一约束 (user_id, checkin_date) 防止并发重复签到；
//  2. 签到自动还款（spec §7.6）：仅已有贷款账户才进入还款路径，不给无贷用户建行。
//     债务以 fundings 为准（Task 8 起账户行为纯投影）：锁全部 active/overdue fundings
//     → 逐条 settleFunding（结算有变动的行落盘）→ syncAccountFromFundings 同步账户投影
//     → repay = min(奖励, Σ债务) → distributeRepayment（按 funding pro-rata、每条先息后本）
//     → settleRepayAllocations（放贷人入账 + offer 回补 + 台账 repay 行）。
//     违约期间签到收入 100% 用于还款：该公式在正常/逾期模式下均全额抵债（净额 =
//     奖励 - repay，奖励大于债务时超额仍入账，还款恒不超债务），逾期仅经 settleFunding
//     的罚息放大债务，不改变分配公式，故无需按 overdue 分支处理；
//  3. 事务内入账净额（奖励 - 还款），杜绝"先全额发放再扣款"的崩溃漏出窗口。
func userCheckinWithTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, *LoanRepayInfo, []LenderCredit, error) {
	var repayInfo *LoanRepayInfo
	var lenderCredits []LenderCredit
	netQuota := quotaAwarded
	var flipped []TokenLoanFunding // 本次新翻转的逾期 funding（Task 15 官方处置派发）
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 步骤1: 创建签到记录
		if err := tx.Create(checkin).Error; err != nil {
			return errors.New("签到失败，请稍后重试")
		}

		// 步骤2: 签到自动还款（spec 4.4 / §7.6）：仅已有账户才进入还款路径，不给无贷用户建行
		if operation_setting.GetLoanSetting().CheckinRepayEnabled {
			now := time.Now()
			acc, err := getLoanAccountTx(tx, userId)
			if err != nil {
				return err
			}
			if acc != nil {
				fundings, err := loadUserFundingsTx(tx, userId)
				if err != nil {
					return err
				}
				// 逐条结算（platform 传 acc 提供有效利率/宽限输入）；结算有变动的行落盘
				for i := range fundings {
					before := fundings[i]
					settleFunding(&fundings[i], acc, now)
					if fundings[i].DebtQuota != before.DebtQuota || fundings[i].LastSettledDay != before.LastSettledDay {
						if err := tx.Save(&fundings[i]).Error; err != nil {
							return err
						}
					}
				}
				// 逾期状态机（Task 11）：今天过期的 active funding 在此翻转为 overdue
				// （幂等），使签到路径的 funding 状态与到期日对齐；翻转不影响结算/分配
				// 数学，逾期 funding 被本次签到全额结清时照常转 repaid。
				// 新翻转列表供 Task 15 官方处置派发（distributeRepayment 内部二次翻转
				// 幂等为空，本处结果即本次新翻转全集）。
				flipped, err = flipOverdueFundingsTx(tx, acc.UserId, fundings, now)
				if err != nil {
					return err
				}
				syncAccountFromFundings(acc, fundings)
				if acc.DebtQuota > 0 {
					// repay = min(奖励, Σ债务)：100% 扣还，超额仍入账
					repay := min(int64(quotaAwarded), acc.DebtQuota)
					info, allocs, _, err := distributeRepayment(tx, acc, fundings, repay, now, "checkin", false)
					if err != nil {
						return err
					}
					if info != nil {
						credits, err := settleRepayAllocations(tx, userId, allocs, "checkin", nil, 0)
						if err != nil {
							return err
						}
						repayInfo = info
						netQuota = quotaAwarded - int(info.Amount)
						lenderCredits = credits
					}
				}
				// 账户行是 fundings 的投影：结算可能推进了 LastSettledDay/债务，统一在此落盘
				acc.UpdatedAt = now.Unix()
				if err := tx.Save(acc).Error; err != nil {
					return err
				}
			}
		}

		// 步骤3: 在事务中增加用户额度（净额 = 奖励 - 还款）
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", netQuota)).Error; err != nil {
			return errors.New("签到失败：更新额度出错")
		}

		// 记录实际入账净额，供额度次日清算按"净入账"回收而非按全额定发
		if err := tx.Model(&Checkin{}).Where("id = ?", checkin.Id).
			Update("net_credited", netQuota).Error; err != nil {
			return errors.New("签到失败：更新净入账出错")
		}
		checkin.NetCredited = netQuota

		return nil
	})

	if err != nil {
		// 放贷人入账溢出（签到人无过错）：事务已回滚，异步通知管理员/放贷人介入
		notifyLenderOverflowAsync(err)
		return nil, nil, nil, err
	}

	// 事务提交后异步同步缓存：借款人按净额递增，各放贷人按入账清单递增
	// （镜像 RepayLoan 的缓存副作用；失败路径不触碰缓存，无需补偿）
	go func() {
		_ = cacheIncrUserQuota(userId, int64(netQuota))
		for _, c := range lenderCredits {
			_ = cacheIncrUserQuota(c.UserId, c.Amount)
		}
	}()
	// 本次新翻转的 platform 逾期 funding 异步派发官方处置（Task 15，提交后派发）
	dispatchPlatformOverdueAsync(flipped)

	return checkin, repayInfo, lenderCredits, nil
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
