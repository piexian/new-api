package model

import (
	"errors"
	"math"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
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
