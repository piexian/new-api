package model

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TokenLoanAccount 词元贷账户（每用户一行）
type TokenLoanAccount struct {
	UserId            int     `json:"user_id" gorm:"primaryKey"`
	PrincipalQuota    int64   `json:"principal_quota" gorm:"bigint"`  // 未还本金
	DebtQuota         int64   `json:"debt_quota" gorm:"bigint"`       // 债务总额（本金+利息），debt >= principal 恒成立
	LastSettledDay    int     `json:"last_settled_day"`               // 上次惰性结算的 loanDay
	CustomMaxTotal    int64   `json:"custom_max_total" gorm:"bigint"` // AI 授予的个人总额上限覆盖，0 = 用全局配置
	CustomDailyRate   float64 `json:"custom_daily_rate"`              // AI 授予的个人日利率覆盖，0 = 用全局配置
	InterestFreeUntil int     `json:"interest_free_until"`            // 宽限期截止 loanDay（该日之前不计息），0 = 无
	TermsAgreedAt     int64   `json:"terms_agreed_at" gorm:"bigint"`  // 同意条款的时间戳，0 = 未同意
	TotalBorrowed     int64   `json:"total_borrowed" gorm:"bigint"`   // 累计借款
	TotalRepaid       int64   `json:"total_repaid" gorm:"bigint"`     // 累计还款
	CreatedAt         int64   `json:"created_at" gorm:"bigint"`       // 秒级时间戳
	UpdatedAt         int64   `json:"updated_at" gorm:"bigint"`       // 秒级时间戳
}

func (TokenLoanAccount) TableName() string {
	return "token_loan_accounts"
}

// TokenLoanRecord 词元贷台账（借款/还款变动记录）
type TokenLoanRecord struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId        int    `json:"user_id" gorm:"not null;index"`
	Type          string `json:"type" gorm:"type:varchar(16);not null"`   // borrow / repay
	Amount        int64  `json:"amount" gorm:"bigint"`                    // 本次变动总额
	InterestPart  int64  `json:"interest_part" gorm:"bigint"`             // 其中抵息部分（borrow 为 0）
	PrincipalPart int64  `json:"principal_part" gorm:"bigint"`            // 其中抵本部分（borrow 为 amount）
	DebtAfter     int64  `json:"debt_after" gorm:"bigint"`                // 变动后债务总额
	Source        string `json:"source" gorm:"type:varchar(16);not null"` // manual / checkin / ai
	RefId         int64  `json:"ref_id" gorm:"bigint"`                    // source=ai 时为申请 id，其余为 0
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`                // 秒级时间戳
}

func (TokenLoanRecord) TableName() string {
	return "token_loan_records"
}

// loanDay 返回 t 所在服务器本地日的日序号（与签到 CheckinDate 的本地日对齐，不用 UTC）
func loanDay(t time.Time) int {
	y, m, d := t.In(time.Local).Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.Local).Unix() / 86400)
}

// effectiveRate 返回有效日利率：个人覆盖 (>0) 与全局利率取较小者
func effectiveRate(acc *TokenLoanAccount) float64 {
	global := operation_setting.GetLoanSetting().DailyRate
	if acc.CustomDailyRate > 0 && acc.CustomDailyRate < global {
		return acc.CustomDailyRate
	}
	return global
}

// settle 惰性结算：把债务按日复利推进到 now 所在本地日。
// days = max(0, loanDay(now) - max(LastSettledDay, InterestFreeUntil))；
// 宽限期内 days=0 不计息，但 LastSettledDay 照常推进（防止宽限结束后一次性补算）。
// 仅修改内存中的 acc，落盘由调用方负责。
func settle(acc *TokenLoanAccount, now time.Time) {
	today := loanDay(now)
	base := acc.LastSettledDay
	if acc.InterestFreeUntil > base {
		base = acc.InterestFreeUntil
	}
	if days := today - base; days > 0 && acc.DebtQuota > 0 {
		rate := effectiveRate(acc)
		// math.Round 远离零取整到整数 quota；真值 >= principal 且 principal 为整数，
		// 故 debt >= principal 不变式恒成立
		acc.DebtQuota = int64(math.Round(float64(acc.DebtQuota) * math.Pow(1+rate, float64(days))))
	}
	acc.LastSettledDay = today
}

// getOrCreateLoanAccountTx 在事务内读取（或创建）用户贷款账户，读路径经 lockForUpdate 加行锁
func getOrCreateLoanAccountTx(tx *gorm.DB, userId int) (*TokenLoanAccount, error) {
	var acc TokenLoanAccount
	err := lockForUpdate(tx).Where("user_id = ?", userId).First(&acc).Error
	if err == nil {
		return &acc, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := time.Now()
	acc = TokenLoanAccount{
		UserId:         userId,
		LastSettledDay: loanDay(now),
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}
	if err := tx.Create(&acc).Error; err != nil {
		return nil, err
	}
	return &acc, nil
}

// ProjectLoanStatus 只读投影：返回 now 时刻的债务总额与其中利息部分，不修改 acc、不落盘
func ProjectLoanStatus(acc *TokenLoanAccount, now time.Time) (debt, interest int64) {
	projected := *acc
	settle(&projected, now)
	return projected.DebtQuota, projected.DebtQuota - projected.PrincipalQuota
}

// ===== Task 3: 同意声明与借款 =====

// 词元贷哨兵错误，controller 层映射为 i18n 响应
var (
	ErrLoanDisabled       = errors.New("loan feature is disabled")
	ErrLoanTermsNotAgreed = errors.New("loan terms not agreed")
	ErrLoanLimitExceeded  = errors.New("loan limit exceeded")
	ErrLoanInvalidAmount  = errors.New("invalid loan amount")
	ErrLoanRegisterTooNew = errors.New("account registered too recently to borrow")
	ErrLoanQuotaOverflow  = errors.New("loan amount overflows user quota range")
)

// isLoanDuplicateKeyErr 识别各数据库方言的主键/唯一键冲突错误
func isLoanDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || // MySQL
		strings.Contains(msg, "duplicate key") || // PostgreSQL
		strings.Contains(msg, "UNIQUE constraint failed") // SQLite
}

// getOrCreateLoanAccountTxSafe 处理并发首建撞主键：创建冲突时在事务内重读一次。
// 注意：PostgreSQL 出错后整事务不可用，重读会失败并回滚，由调用方重试整笔操作。
func getOrCreateLoanAccountTxSafe(tx *gorm.DB, userId int) (*TokenLoanAccount, error) {
	acc, err := getOrCreateLoanAccountTx(tx, userId)
	if err == nil || !isLoanDuplicateKeyErr(err) {
		return acc, err
	}
	var reread TokenLoanAccount
	if err := lockForUpdate(tx).Where("user_id = ?", userId).First(&reread).Error; err != nil {
		return nil, err
	}
	return &reread, nil
}

// AgreeLoanTerms 幂等记录用户同意条款的时间；账户不存在时顺带创建
func AgreeLoanTerms(userId int) error {
	now := time.Now()
	return DB.Transaction(func(tx *gorm.DB) error {
		acc, err := getOrCreateLoanAccountTxSafe(tx, userId)
		if err != nil {
			return err
		}
		if acc.TermsAgreedAt != 0 {
			return nil // 幂等：已同意不覆盖首次时间
		}
		acc.TermsAgreedAt = now.Unix()
		acc.UpdatedAt = now.Unix()
		return tx.Save(acc).Error
	})
}

// BorrowLoan 借款主流程：
//  1. 事务外无状态校验：功能开关、amount_usd 解析（decimal，最多两位小数）、int32 上界
//  2. 事务内 lockForUpdate 锁定账户 → settle → 条款/注册天数/额度校验 → 更新账户 + 写台账
//  3. 事务提交后 IncreaseUserQuota(userId, amount, true) 加余额：该函数走全局 DB
//     独立写入（model/user.go:1434），无法并入事务；失败时用 rollbackBorrow 补偿回滚
func BorrowLoan(userId int, amountUsd string) (*TokenLoanAccount, error) {
	loanSetting := operation_setting.GetLoanSetting()
	if !loanSetting.Enabled {
		return nil, ErrLoanDisabled
	}

	// amount_usd 为字符串，decimal 解析；非正数或超过两位小数一律拒绝
	usd, err := decimal.NewFromString(amountUsd)
	if err != nil || !usd.IsPositive() || usd.Exponent() < -2 {
		return nil, ErrLoanInvalidAmount
	}
	quotaDec := usd.Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	amount, clamp := common.QuotaFromDecimalChecked(quotaDec)
	if clamp != nil {
		return nil, ErrLoanQuotaOverflow
	}
	if amount <= 0 {
		return nil, ErrLoanInvalidAmount
	}

	now := time.Now()
	var acc *TokenLoanAccount
	recordId := 0
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 事务内读用户当前值：注册天数与余额 int32 上界校验
		var user User
		if err := tx.Select("id", "quota", "created_at").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if loanSetting.MinRegisterDays > 0 &&
			now.Unix()-user.CreatedAt < int64(loanSetting.MinRegisterDays)*86400 {
			return ErrLoanRegisterTooNew
		}
		if int64(user.Quota)+int64(amount) > common.MaxQuota {
			return ErrLoanQuotaOverflow
		}

		acc, err = getOrCreateLoanAccountTxSafe(tx, userId)
		if err != nil {
			return err
		}
		if loanSetting.TermsEnabled && acc.TermsAgreedAt == 0 {
			return ErrLoanTermsNotAgreed
		}

		settle(acc, now)

		// 个人上限覆盖全局；单次上限缺省跟随有效总额上限
		effectiveMax := loanSetting.MaxTotal
		if acc.CustomMaxTotal > 0 {
			effectiveMax = acc.CustomMaxTotal
		}
		perBorrowCap := effectiveMax
		if loanSetting.MaxPerBorrow > 0 {
			perBorrowCap = loanSetting.MaxPerBorrow
		}
		if int64(amount) > perBorrowCap || acc.DebtQuota+int64(amount) > effectiveMax {
			return ErrLoanLimitExceeded
		}

		acc.PrincipalQuota += int64(amount)
		acc.DebtQuota += int64(amount)
		acc.TotalBorrowed += int64(amount)
		acc.UpdatedAt = now.Unix()
		if err := tx.Save(acc).Error; err != nil {
			return err
		}

		record := &TokenLoanRecord{
			UserId:        userId,
			Type:          "borrow",
			Amount:        int64(amount),
			InterestPart:  0,
			PrincipalPart: int64(amount),
			DebtAfter:     acc.DebtQuota,
			Source:        "manual",
			RefId:         0,
			CreatedAt:     now.Unix(),
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		recordId = record.Id
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := IncreaseUserQuota(userId, amount, true); err != nil {
		if rbErr := rollbackBorrow(userId, recordId, int64(amount)); rbErr != nil {
			common.SysError(fmt.Sprintf("loan borrow rollback failed for user %d: %v", userId, rbErr))
		}
		return nil, err
	}
	return acc, nil
}

// rollbackBorrow 借款后 IncreaseUserQuota 失败时的补偿：回滚账户数值并删除台账
func rollbackBorrow(userId int, recordId int, amount int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var acc TokenLoanAccount
		if err := lockForUpdate(tx).Where("user_id = ?", userId).First(&acc).Error; err != nil {
			return err
		}
		acc.PrincipalQuota -= amount
		acc.DebtQuota -= amount
		acc.TotalBorrowed -= amount
		acc.UpdatedAt = time.Now().Unix()
		if err := tx.Save(&acc).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", recordId).Delete(&TokenLoanRecord{}).Error
	})
}
