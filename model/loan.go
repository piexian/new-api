package model

import (
	"errors"
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

// LoanDayOf 导出 loanDay，供 service 层做宽限期剩余天数等只读判断
func LoanDayOf(t time.Time) int {
	return loanDay(t)
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

// getLoanAccountTx 在事务内经 lockForUpdate 加行锁读取贷款账户；
// 账户不存在时返回 (nil, nil)，不建行（签到还款等路径不得给无贷用户建行）
func getLoanAccountTx(tx *gorm.DB, userId int) (*TokenLoanAccount, error) {
	var acc TokenLoanAccount
	err := lockForUpdate(tx).Where("user_id = ?", userId).First(&acc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// GetLoanAccountReadOnly 只读查询用户贷款账户（GET status 投影用）：
// 不加锁、不存在时返回 (nil, nil)，绝不建行或落盘
func GetLoanAccountReadOnly(userId int) (*TokenLoanAccount, error) {
	var acc TokenLoanAccount
	err := DB.Where("user_id = ?", userId).First(&acc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// getOrCreateLoanAccountTx 在事务内读取（或创建）用户贷款账户，读路径经 lockForUpdate 加行锁
func getOrCreateLoanAccountTx(tx *gorm.DB, userId int) (*TokenLoanAccount, error) {
	acc, err := getLoanAccountTx(tx, userId)
	if err != nil || acc != nil {
		return acc, err
	}
	now := time.Now()
	acc = &TokenLoanAccount{
		UserId:         userId,
		LastSettledDay: loanDay(now),
		CreatedAt:      now.Unix(),
		UpdatedAt:      now.Unix(),
	}
	if err := tx.Create(acc).Error; err != nil {
		return nil, err
	}
	return acc, nil
}

// ProjectLoanStatus 只读投影：返回 now 时刻的债务总额与其中利息部分，不修改 acc、不落盘
func ProjectLoanStatus(acc *TokenLoanAccount, now time.Time) (debt, interest int64) {
	projected := *acc
	settle(&projected, now)
	return projected.DebtQuota, projected.DebtQuota - projected.PrincipalQuota
}

// ===== Task 4: 签到还款 =====

// LoanRepayInfo 签到还款结果（供 controller 透出，nil = 无还款）
type LoanRepayInfo struct {
	Amount        int64 `json:"amount"`
	InterestPart  int64 `json:"interest_part"`
	PrincipalPart int64 `json:"principal_part"`
	DebtAfter     int64 `json:"debt_after"`
}

// applyCheckinRepay 在已 settle 的账户上执行还款拆分（spec 4.2）：
// repay = min(award, debt)，先息后本。仅修改内存中的 acc，落盘由调用方负责；
// 无债务或 award<=0 时返回 nil，此时 acc 仅有 settle 造成的内存变动、无需落盘。
func applyCheckinRepay(acc *TokenLoanAccount, award int64) *LoanRepayInfo {
	if award <= 0 || acc.DebtQuota <= 0 {
		return nil
	}
	repay := min(award, acc.DebtQuota)
	interest := acc.DebtQuota - acc.PrincipalQuota
	payInterest := min(repay, interest)
	payPrincipal := repay - payInterest
	acc.PrincipalQuota -= payPrincipal
	acc.DebtQuota -= repay
	acc.TotalRepaid += repay
	return &LoanRepayInfo{
		Amount:        repay,
		InterestPart:  payInterest,
		PrincipalPart: payPrincipal,
		DebtAfter:     acc.DebtQuota,
	}
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
	ErrLoanUserDisabled   = errors.New("user account is not in normal status")
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
//  2. 事务内 lockForUpdate 锁定账户 → settle → 条款/注册天数/额度校验 →
//     更新账户 + 写台账 + 用户 quota 入账（镜像签到模式，同事务保证原子性）
//  3. 事务提交后异步同步 Redis 余额缓存，并重置额度提醒锁
//     （对齐 IncreaseUserQuota 的副作用，model/user.go:1434）
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
	// usd > 0 且 QuotaPerUnit = 500000，最小 0.01 USD = 5000 quota，
	// 故此处 amount 必然 > 0，无需再判 amount <= 0

	now := time.Now()
	var acc *TokenLoanAccount
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 事务内读用户当前值：状态、注册天数与余额 int32 上界校验
		var user User
		if err := tx.Select("id", "quota", "created_at", "status").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return ErrLoanUserDisabled
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

		// quota 入账与账户/台账同一事务（镜像签到模式），失败整体回滚
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", int64(amount))).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后异步同步 Redis 余额缓存；并重置额度提醒锁，
	// 对齐 IncreaseUserQuota（model/user.go:1434）quota>0 时的副作用
	go func() {
		_ = cacheIncrUserQuota(userId, int64(amount))
	}()
	common.ResetQuotaNotificationSendLocks(userId, "wallet", 0)
	return acc, nil
}

// GetUserLoanRecords 分页返回用户台账，id 倒序（最新在前），page 从 1 开始；附总数用于分页
func GetUserLoanRecords(userId, page, pageSize int) ([]TokenLoanRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var total int64
	if err := DB.Model(&TokenLoanRecord{}).Where("user_id = ?", userId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []TokenLoanRecord
	err := DB.Where("user_id = ?", userId).
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error
	return records, total, err
}

// GetLoanApplicationById 按 id + userId 查询工单（归属校验内置于 WHERE），
// 不存在或非本人时透出 gorm.ErrRecordNotFound，由 controller 映射为 i18n 响应
func GetLoanApplicationById(userId, appId int) (*TokenLoanApplication, error) {
	var app TokenLoanApplication
	if err := DB.Where("id = ? AND user_id = ?", appId, userId).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}
