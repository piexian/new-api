package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TokenLoanAccount 词元贷账户（每用户一行）
type TokenLoanAccount struct {
	UserId                   int     `json:"user_id" gorm:"primaryKey"`
	PrincipalQuota           int64   `json:"principal_quota" gorm:"bigint"`             // 未还本金
	DebtQuota                int64   `json:"debt_quota" gorm:"bigint"`                  // 债务总额（本金+利息），debt >= principal 恒成立
	LastSettledDay           int     `json:"last_settled_day"`                          // 上次惰性结算的 loanDay
	CustomMaxTotal           int64   `json:"custom_max_total" gorm:"bigint"`            // AI 授予的个人总额上限覆盖，0 = 用全局配置
	CustomDailyRate          float64 `json:"custom_daily_rate"`                         // AI 授予的个人日利率覆盖，0 = 用全局配置
	InterestFreeUntil        int     `json:"interest_free_until"`                       // 宽限期截止 loanDay（该日之前不计息），0 = 无
	CreditScore              int     `json:"credit_score"`                              // 贷方信用分，0 = 未评估（仅迁移前存量账户；新账户建行即 credit_initial）
	BlacklistedUntilDay      int     `json:"blacklisted_until_day"`                     // 信用拉黑截止 loanDay，0 = 未拉黑
	TermsAgreedAt            int64   `json:"terms_agreed_at" gorm:"bigint"`             // 同意借款条款的时间戳，0 = 未同意
	LenderDisclaimerAgreedAt int64   `json:"lender_disclaimer_agreed_at" gorm:"bigint"` // 同意放贷免责声明的时间戳，0 = 未同意
	TotalBorrowed            int64   `json:"total_borrowed" gorm:"bigint"`              // 累计借款
	TotalRepaid              int64   `json:"total_repaid" gorm:"bigint"`                // 累计还款
	CreatedAt                int64   `json:"created_at" gorm:"bigint"`                  // 秒级时间戳
	UpdatedAt                int64   `json:"updated_at" gorm:"bigint"`                  // 秒级时间戳
}

func (TokenLoanAccount) TableName() string {
	return "token_loan_accounts"
}

// TokenLoanRecord 词元贷台账（借款/还款/信用分变动记录）
type TokenLoanRecord struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId        int    `json:"user_id" gorm:"not null;index"`
	Type          string `json:"type" gorm:"type:varchar(16);not null"`   // borrow / repay / credit（credit = 信用分变动）
	Amount        int64  `json:"amount" gorm:"bigint"`                    // 本次变动总额；type=credit 时为带符号的实际生效加减分（钳制后 new-old）
	InterestPart  int64  `json:"interest_part" gorm:"bigint"`             // 其中抵息部分（borrow 为 0；credit 恒为 0）
	PrincipalPart int64  `json:"principal_part" gorm:"bigint"`            // 其中抵本部分（borrow 为 amount；credit 恒为 0）
	FeePart       int64  `json:"fee_part" gorm:"bigint"`                  // 提前还款手续费（仅手动还款可能 > 0；credit 恒为 0）
	PenaltyPart   int64  `json:"penalty_part" gorm:"bigint"`              // 秒结清惩罚（仅手动提前还款可能 > 0；credit 恒为 0）
	DebtAfter     int64  `json:"debt_after" gorm:"bigint"`                // 变动后债务总额；type=credit 时复用为变动后信用分
	Source        string `json:"source" gorm:"type:varchar(16);not null"` // manual / checkin / ai / repay_bonus / fast_repay / writeoff / account_closure
	RefId         int64  `json:"ref_id" gorm:"bigint"`                    // source=ai 时为申请 id；credit 时为借款事件 id（writeoff 为 0）；其余为 0
	FundingId     int64  `json:"funding_id" gorm:"bigint"`                // 关联 funding 行 id，0 = 非市场投放
	LenderId      int    `json:"lender_id"`                               // 放贷方 user id，0 = 平台/资金池
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

// LoanQuotaCeiling 词元贷额度算术的 64 位安全上界。users.quota 实际按 64 位存储
// （Go int 在 64 位平台即 int64，生产库列为 bigint），词元贷不再沿用 int32 口径的
// common.MaxQuota。取 MaxInt64/2 为单次加法留足余量，杜绝溢出回绕。
const LoanQuotaCeiling = int64(math.MaxInt64 / 2)

// LoanQuotaFromDecimal 把 USD×QuotaPerUnit 的 decimal 金额换算为 int64 quota：
// 四舍五入到整数；|d| 超过 LoanQuotaCeiling 时返回 overflow=true（调用方映射为
// ErrLoanQuotaOverflow / 参数错误），零值/负值由调用方按既有语义拒绝。
func LoanQuotaFromDecimal(d decimal.Decimal) (quota int64, overflow bool) {
	d = d.Round(0)
	if d.Abs().GreaterThan(decimal.NewFromInt(LoanQuotaCeiling)) {
		return 0, true
	}
	return d.IntPart(), false
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
		CreditScore:    operation_setting.GetLoanSetting().CreditInitial, // 新账户直接带初始信用分（存量 0 = 未评估，回填属迁移任务）
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

// LoanRepayInfo 还款结果（供 controller 透出，nil = 无还款）
type LoanRepayInfo struct {
	Amount        int64 `json:"amount"`
	InterestPart  int64 `json:"interest_part"`
	PrincipalPart int64 `json:"principal_part"`
	FeePart       int64 `json:"fee_part"`     // 提前还款手续费（签到自动还款恒为 0）
	PenaltyPart   int64 `json:"penalty_part"` // 秒结清惩罚总额（签到自动还款恒为 0）
	DebtAfter     int64 `json:"debt_after"`
}

// ===== Task 3: 同意声明与借款 =====

// 词元贷哨兵错误，controller 层映射为 i18n 响应
var (
	ErrLoanDisabled            = errors.New("loan feature is disabled")
	ErrLoanTermsNotAgreed      = errors.New("loan terms not agreed")
	ErrLoanLimitExceeded       = errors.New("loan limit exceeded")
	ErrLoanInvalidAmount       = errors.New("invalid loan amount")
	ErrLoanRegisterTooNew      = errors.New("account registered too recently to borrow")
	ErrLoanQuotaOverflow       = errors.New("loan amount overflows user quota range")
	ErrLoanUserDisabled        = errors.New("user account is not in normal status")
	ErrLoanNoDebt              = errors.New("no outstanding loan debt")
	ErrLoanInsufficientBalance = errors.New("insufficient balance to repay loan")
	ErrLoanBlacklisted         = errors.New("loan user is blacklisted")                       // 黑名单未解除（P1-8 借款闸门）
	ErrLoanHasOverdue          = errors.New("loan user has overdue funding")                  // 存在 overdue funding（P1-8 借款闸门）
	ErrLoanLenderQuotaOverflow = errors.New("lender quota overflow on loan repayment credit") // 放贷人入账溢出（借款人无过错）
	ErrLoanDebtInviteBlocked   = errors.New("loan debt blocks invite features")               // 有未还清贷款时禁止生成/使用邀请码
)

// LoanLenderOverflowError 放贷人入账溢出：还款分配的资金无法计入放贷人余额
// （触及 64 位上界）。整笔还款回滚，绝不静默截断；携带放贷人 id 与入账金额，
// 供调用方在事务回滚后通知管理员/放贷人介入处理。
type LoanLenderOverflowError struct {
	LenderId int
	Amount   int64
}

func (e *LoanLenderOverflowError) Error() string {
	return fmt.Sprintf("lender %d quota overflow on loan repayment credit of %d", e.LenderId, e.Amount)
}

func (e *LoanLenderOverflowError) Is(target error) bool {
	return target == ErrLoanLenderQuotaOverflow
}

// lenderOverflowNotifier 放贷人入账溢出通知钩子（事务回滚后异步触发）；
// 由 service 层注册（model 不依赖 service），未注册时静默跳过
var lenderOverflowNotifier func(lenderId int, amount int64)

// RegisterLenderOverflowNotifier 注册溢出通知钩子（启动时由 service 层调用一次）
func RegisterLenderOverflowNotifier(f func(lenderId int, amount int64)) {
	lenderOverflowNotifier = f
}

// notifyLenderOverflowAsync 若 err 为放贷人入账溢出则异步触发通知钩子
func notifyLenderOverflowAsync(err error) {
	var e *LoanLenderOverflowError
	if errors.As(err, &e) && lenderOverflowNotifier != nil {
		go lenderOverflowNotifier(e.LenderId, e.Amount)
	}
}

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

// BorrowLoan 借款主流程（Task 8 起按 funding 放款）：
//  1. 事务外无状态校验：功能开关、amount_usd 解析（decimal，最多两位小数）、64 位上界
//  2. 事务内 lockForUpdate 锁定用户行 → 读/建贷款账户 + 条款校验 → 借款闸门（黑名单/
//     逾期，P1-8）→ 全部 active/overdue fundings 惰性结算（settleFunding，替代旧 settle）
//     → 额度校验（用 funding 汇总的同步债务，公式与旧版一致）→ 市场撮合（MarketEnabled
//     时；定向挂单 → 统一市场 → AI 方案，来源不足由平台兜底）→ 先写台账 borrow 行取其 id
//     作事件 id → 按计划放款（锁 offer 行二次校验并扣减 + 建 funding）→ 汇总回写账户
//     → 用户 quota 入账（与账户/台账/funding 同一事务，失败整体回滚）
//  3. 事务提交后异步同步 Redis 余额缓存，并重置额度提醒锁
//     （对齐 IncreaseUserQuota 的副作用，model/user.go:1434）
//
// intendedOrderId > 0 时作为定向挂单意向传入撮合引擎（过期意向只跳过不阻断借款）；
// aiPriced 为 AI 出资方案（Task 15 产出，此处只按计划放款）。返回最新账户与本次新建的
// funding 列表（含 platform 兜底）。市场关闭时退化为纯官方放款：整笔金额生成一条
// platform funding，行为与旧版借款一致。
func BorrowLoan(userId int, amountUsd string, intendedOrderId int, aiPriced []FundingPlan) (*TokenLoanAccount, []TokenLoanFunding, error) {
	return BorrowLoanWithOptions(userId, amountUsd, intendedOrderId, aiPriced, false)
}

// BorrowLoanWithOptions 同 BorrowLoan；platformOnly=true（借款人选择"只接官方资金"）时
// 跳过市场撮合与 AI 定价方案，整笔由平台资金池放款（官方 funding 无秒结清惩罚条款）
func BorrowLoanWithOptions(userId int, amountUsd string, intendedOrderId int, aiPriced []FundingPlan, platformOnly bool) (*TokenLoanAccount, []TokenLoanFunding, error) {
	loanSetting := operation_setting.GetLoanSetting()
	if !loanSetting.Enabled {
		return nil, nil, ErrLoanDisabled
	}

	// amount_usd 为字符串，decimal 解析；非正数或超过两位小数一律拒绝
	usd, err := decimal.NewFromString(amountUsd)
	if err != nil || !usd.IsPositive() || usd.Exponent() < -2 {
		return nil, nil, ErrLoanInvalidAmount
	}
	quotaDec := usd.Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	amount, overflow := LoanQuotaFromDecimal(quotaDec)
	if overflow {
		return nil, nil, ErrLoanQuotaOverflow
	}
	// QuotaPerUnit 是运行时可调配置（model/option.go），换算后 amount 可能为 0，
	// 必须显式拒绝，避免记下 0 额度借款
	if amount <= 0 {
		return nil, nil, ErrLoanInvalidAmount
	}

	now := time.Now()
	var acc *TokenLoanAccount
	var newFundings []TokenLoanFunding
	var flipped []TokenLoanFunding // 本次新翻转的逾期 funding（Task 15 官方处置派发）
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 事务内读用户当前值：状态、注册天数与余额 64 位上界校验
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
		if amount > LoanQuotaCeiling-int64(user.Quota) {
			return ErrLoanQuotaOverflow
		}

		acc, err = getOrCreateLoanAccountTxSafe(tx, userId)
		if err != nil {
			return err
		}
		if loanSetting.TermsEnabled && acc.TermsAgreedAt == 0 {
			return ErrLoanTermsNotAgreed
		}

		// 借款闸门（P1-8）：黑名单未解除拒绝新借款（overdue 闸门在结算+翻转之后校验，
		// 使其能看到今天刚过期的 funding）。
		if acc.BlacklistedUntilDay > loanDay(now) {
			return ErrLoanBlacklisted
		}
		fundings, err := loadUserFundingsTx(tx, userId)
		if err != nil {
			return err
		}

		// 惰性结算下沉到 funding（spec §5）：逐条 settleFunding（platform 传 acc 提供
		// 有效利率与宽限期输入，P2P 用自身利率），仅持久化有变动的行
		for i := range fundings {
			before := fundings[i]
			settleFunding(&fundings[i], acc, now)
			if fundings[i].DebtQuota != before.DebtQuota || fundings[i].LastSettledDay != before.LastSettledDay {
				if err := tx.Save(&fundings[i]).Error; err != nil {
					return err
				}
			}
		}

		// 逾期状态机（Task 11）：今天过期的 active funding 在此翻转为 overdue 并落盘，
		// 使闸门能看到"今天刚过期"的 funding，杜绝借新还旧。幂等条件更新；拒绝路径
		// 整体回滚不落痕，下次写路径再翻。新翻转列表供 Task 12/15 消费。
		flipped, err = flipOverdueFundingsTx(tx, userId, fundings, now)
		if err != nil {
			return err
		}

		// 借款闸门（P1-8）：存在 overdue funding 拒绝新借款（黑名单已在上方校验）。
		// 检查落在结算+翻转后的 funding 列表上（含今天刚翻转的与既有全部 overdue 行）。
		for i := range fundings {
			if fundings[i].Status == LoanFundingOverdue {
				return ErrLoanHasOverdue
			}
		}

		// 额度校验用 funding 汇总口径的同步债务（settle 后 Σ，内存投影不落盘），
		// perBorrowCap/effectiveMax 公式与旧版逐字一致
		synced := *acc
		syncAccountFromFundings(&synced, fundings)
		effectiveMax := loanSetting.MaxTotal
		if acc.CustomMaxTotal > 0 {
			effectiveMax = acc.CustomMaxTotal
			// 借款侧兜底：AI 授予的个人上限不得突破信用分档位上限（档位作用于
			// CustomMaxTotal 累计值；CustomMaxTotal==0 的全局默认用户不受档位约束，
			// 基础额度与 AI 授予分开）。acc 已加载（新账户带 CreditInitial），无需额外查询。
			if tierMax := operation_setting.GetCreditTierMaxTotal(loanSetting, acc.CreditScore); tierMax < effectiveMax {
				effectiveMax = tierMax
			}
		}
		perBorrowCap := effectiveMax
		if loanSetting.MaxPerBorrow > 0 {
			perBorrowCap = loanSetting.MaxPerBorrow
		}
		if amount > perBorrowCap || synced.DebtQuota+amount > effectiveMax {
			return ErrLoanLimitExceeded
		}

		// 撮合（只读 + 内存）：市场开启且未勾选"只接官方资金"时定向挂单 → 统一市场 →
		// AI 方案；来源不足部分由平台兜底。dropReasons 供 Task 16 审计透出，业务性跳过
		// 不阻断借款。
		var plans []FundingPlan
		if loanSetting.MarketEnabled && !platformOnly {
			var dropReasons []string
			plans, dropReasons, err = MatchLoanFundings(tx, userId, acc.CreditScore, amount, intendedOrderId, aiPriced)
			if err != nil {
				return err
			}
			_ = dropReasons
		}
		if shortfall := amount - planTotal(plans); shortfall > 0 {
			plans = append(plans, FundingPlan{
				SourceType: LoanFundingPlatform,
				Amount:     shortfall,
				Rate:       effectiveRate(acc),
			})
		}

		// 先写台账 borrow 行（Source=manual）取其 id 作为本次放款事件 id，再落 funding
		record := &TokenLoanRecord{
			UserId:        userId,
			Type:          "borrow",
			Amount:        amount,
			InterestPart:  0,
			PrincipalPart: amount,
			DebtAfter:     synced.DebtQuota + amount,
			Source:        "manual",
			RefId:         0,
			CreatedAt:     now.Unix(),
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}

		// 按计划放款：非 platform 计划锁 offer 行二次校验并扣减，随后逐条落 funding
		newFundings, err = executeFundingPlans(tx, userId, int64(record.Id), plans, now)
		if err != nil {
			return err
		}

		// 重新载入全部 fundings（含新建）汇总回写账户投影并持久化；
		// DebtQuota/PrincipalQuota 恒等于 Σ active/overdue fundings（spec §4.5）
		all, err := loadUserFundingsTx(tx, userId)
		if err != nil {
			return err
		}
		syncAccountFromFundings(acc, all)
		acc.TotalBorrowed += amount
		acc.UpdatedAt = now.Unix()
		if err := tx.Save(acc).Error; err != nil {
			return err
		}

		// quota 入账与账户/台账/funding 同一事务（镜像签到模式），失败整体回滚
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", amount)).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// 事务提交后异步同步 Redis 余额缓存；并重置额度提醒锁，
	// 对齐 IncreaseUserQuota（model/user.go:1434）quota>0 时的副作用
	go func() {
		_ = cacheIncrUserQuota(userId, amount)
	}()
	// 本次新翻转的 platform 逾期 funding 异步派发官方处置（Task 15，提交后派发保证
	// overdue 状态已持久化；P2P 在 flip 结果里被过滤，全 P2P 时 no-op）
	dispatchPlatformOverdueAsync(flipped)
	common.ResetQuotaNotificationSendLocks(userId, "wallet", 0)
	return acc, newFundings, nil
}

// loadUserFundingsTx 在事务内按 id 升序载入用户全部 active/overdue fundings（FOR UPDATE），
// 供借款闸门、惰性结算与账户投影汇总共用
func loadUserFundingsTx(tx *gorm.DB, userId int) ([]TokenLoanFunding, error) {
	var fundings []TokenLoanFunding
	err := lockForUpdate(tx).
		Where("loan_user_id = ? AND status IN ?", userId, []string{LoanFundingActive, LoanFundingOverdue}).
		Order("id ASC").
		Find(&fundings).Error
	return fundings, err
}

// HasOverdueFundings 查询用户是否存在 overdue funding（P1-8 借款闸门判定 / spec §7.6
// 违约签到扣还的触发条件）。签到钩子不调用本函数：钩子已载入全部 active/overdue
// fundings（loadUserFundingsTx），且 §7.6 的 100% 扣还公式对正常/逾期模式一致，无需
// 二次查询；本函数供状态展示等只读路径直接判定"是否处于违约期"。
func HasOverdueFundings(tx *gorm.DB, userId int) (bool, error) {
	var count int64
	err := tx.Model(&TokenLoanFunding{}).
		Where("loan_user_id = ? AND status = ?", userId, LoanFundingOverdue).
		Count(&count).Error
	return count > 0, err
}

// HasOutstandingLoanDebt 判定用户是否有未还清债务（含正常在贷）。
// TokenLoanAccount 不变式 debt_quota >= principal_quota 恒成立，
// 故 debt_quota > 0 即准确的"有债"布尔值，无需触发利息结算。
// 供注册邀请码拦截与一次性邀请码生成拦截复用。
func HasOutstandingLoanDebt(tx *gorm.DB, userId int) (bool, error) {
	var count int64
	err := tx.Model(&TokenLoanAccount{}).
		Where("user_id = ? AND debt_quota > 0", userId).
		Count(&count).Error
	return count > 0, err
}

// planTotal 计划总金额（供平台兜底缺额计算）
func planTotal(plans []FundingPlan) int64 {
	var total int64
	for i := range plans {
		total += plans[i].Amount
	}
	return total
}

// executeFundingPlans 在事务内按撮合计划放款（spec §6）：
//   - 非 platform 计划：lockForUpdate 重读 offer 行二次校验 amount_available >= plan.Amount
//     （撮合是只读快照，available 可能已被并发借款占用，故此处必须加锁重验；
//     校验失败返回错误整体回滚），随后扣减 amount_available、累加 total_lent 并落库；
//     秒结清惩罚条款（FastRepayPenaltyQuota/FastRepayWindowDays）在此从 offer 复制到
//     funding（撮合引擎不感知惩罚，放款阶段以 offer 行锁定后的值为准）；惩罚额度
//     复制时按"不超过本笔借出金额的 2 倍"封顶（借款人侧最高承担 本金+2×本金）；
//   - 逐条创建 funding 行：Amount=PrincipalRemaining=DebtQuota=plan.Amount、
//     Rate=plan.Rate、LastSettledDay=loanDay(now)、DueDay=loanDay(now)+max(LoanTermDays,1)
//     （LoanTermDays<=0 时至少推到明天，防当日到期即整段罚息）、RepayPlan=full、
//     Status=active、BorrowEventId=本次借款事件 id、SourceType/OfferId/LenderId 取计划。
//
// 锁序遵守全局约束 offers(id 升序)：offer 锁定与扣减先按 OfferId 升序集中完成
// （与 settleRepayAllocations 的 offers(id 升序) 一致，避免并发借款与还款 AB-BA 死锁；
// 同一 offer 的多条计划相邻处理，后一条读到前一条扣减后的 available），
// funding 行再按计划原始顺序创建（借款事件内顺序稳定，行为不变）。
//
// 返回本次新建的 funding 列表（含 platform 兜底）。
func executeFundingPlans(tx *gorm.DB, userId int, borrowEventId int64, plans []FundingPlan, now time.Time) ([]TokenLoanFunding, error) {
	dueDay := loanDay(now) + max(operation_setting.GetLoanSetting().LoanTermDays, 1)

	// 第一遍：非 platform 计划按 OfferId 升序锁定并扣减 offer 行（全局锁序
	// offers(id 升序)）；同一 offer 的多条计划相邻处理，后一条读到前一条扣减后的 available。
	offerPlans := make([]*FundingPlan, 0, len(plans))
	for i := range plans {
		if plans[i].SourceType != LoanFundingPlatform {
			offerPlans = append(offerPlans, &plans[i])
		}
	}
	sort.Slice(offerPlans, func(a, b int) bool { return offerPlans[a].OfferId < offerPlans[b].OfferId })
	// 秒结清条款按 offer 记录，供第二遍建 funding 时复制（platform 兜底无 offer，恒为 0）
	offerPenalties := make(map[int]struct {
		quota  int64
		window int
	}, len(offerPlans))
	for _, plan := range offerPlans {
		var offer TokenLoanOffer
		if err := lockForUpdate(tx).Where("id = ?", plan.OfferId).First(&offer).Error; err != nil {
			return nil, err
		}
		if offer.AmountAvailable < plan.Amount {
			return nil, fmt.Errorf("loan offer %d available %d < plan amount %d", offer.Id, offer.AmountAvailable, plan.Amount)
		}
		offer.AmountAvailable -= plan.Amount
		offer.TotalLent += plan.Amount
		offer.UpdatedAt = now.Unix()
		if err := tx.Save(&offer).Error; err != nil {
			return nil, err
		}
		offerPenalties[offer.Id] = struct {
			quota  int64
			window int
		}{
			quota: offer.FastRepayPenaltyQuota, window: offer.FastRepayWindowDays,
		}
	}

	// 第二遍：按计划原始顺序创建 funding 行（含 platform 兜底），事件内顺序稳定
	newFundings := make([]TokenLoanFunding, 0, len(plans))
	for i := range plans {
		plan := &plans[i]
		penalty := offerPenalties[plan.OfferId]
		// 秒结清惩罚随放款复制时按"不超过本笔借出金额的 2 倍"封顶：
		// offer 条款是大单总惩罚，小额 funding 不得承担超过 2×本金的罚单
		if penalty.quota > 2*plan.Amount {
			penalty.quota = 2 * plan.Amount
		}
		funding := TokenLoanFunding{
			LoanUserId:            userId,
			BorrowEventId:         borrowEventId,
			SourceType:            plan.SourceType,
			OfferId:               plan.OfferId,
			LenderId:              plan.LenderId,
			Amount:                plan.Amount,
			PrincipalRemaining:    plan.Amount,
			DebtQuota:             plan.Amount,
			LastSettledDay:        loanDay(now),
			Rate:                  plan.Rate,
			RepayPlan:             LoanRepayFull,
			Status:                LoanFundingActive,
			DueDay:                dueDay,
			FastRepayPenaltyQuota: penalty.quota,
			FastRepayWindowDays:   penalty.window,
			CreatedAt:             now.Unix(),
			UpdatedAt:             now.Unix(),
		}
		if err := tx.Create(&funding).Error; err != nil {
			return nil, err
		}
		newFundings = append(newFundings, funding)
	}
	return newFundings, nil
}

// earlyRepayFee 手动提前还款手续费：按还款中的抵本部分 × 费率计算（四舍五入到整数 quota）。
// 签到自动还款不收取；费率为 0 或还款全部抵息时为 0。
func earlyRepayFee(acc *TokenLoanAccount, repay int64) int64 {
	rate := operation_setting.GetLoanSetting().RepayFeeRate
	if rate <= 0 || repay <= 0 {
		return 0
	}
	interest := acc.DebtQuota - acc.PrincipalQuota
	principalPart := repay - interest
	if principalPart <= 0 {
		return 0
	}
	return int64(math.Round(float64(principalPart) * rate))
}

// RepayLoan 手动提前还款：从用户余额扣款偿还债务，按各 funding 结算后债务 pro-rata
// 分配（最大余数法，先息后本），并按抵本部分收取手续费（RepayFeeRate，签到自动还款不收）。
// amountUsd 为 "all"（忽略大小写）时偿还 min(债务, 余额能覆盖的本息费)；否则按 decimal 解析
// （正数、最多两位小数），还款额 = min(金额, 债务)。余额不足以覆盖还款额+手续费时报
// ErrLoanInsufficientBalance（秒结清惩罚不参与该校验，见下）。
//
// 秒结清惩罚（放贷人自定义条款）：本次转结清的 funding 若仍在惩罚窗口内（loanDay 差 ≤
// FastRepayWindowDays）且惩罚额度 > 0，按 funding 逐条计罚——Σ 惩罚在余额扣款阶段一并
// 扣除（gorm.Expr("quota - ?")，不校验余额，允许扣成负数），按 funding 写入台账
// penalty_part，按放贷人并入 LenderCredit（利息/本金同一 64 位上界校验）。
//
// 事务内：锁用户行 → 读/锁贷款账户（不为无贷用户建行）→ 锁全部 active/overdue fundings
// （id 升序）→ 逐条 settleFunding 并落盘 → syncAccountFromFundings 同步账户投影（账户行
// 从此是 fundings 的纯投影，earlyRepayFee 的账户级利息 = Σ funding 利息，公式保持有效）
// → 还款额/手续费计算（既有逻辑）→ distributeRepayment（pro-rata 分配 + funding 行落盘）
// → 惩罚计算（computeFastRepayPenalties）→ settleRepayAllocations（放贷人入账 + offer 回补
// + 台账 repay 行含 penalty_part）→ 落盘账户投影 → 扣用户余额（含惩罚）。全部同一事务，
// 失败整体回滚。提交后异步同步缓存：借款人按还款额+手续费+惩罚扣减，各放贷人按入账清单
// 递增。第三返回值 credits 为按放贷人聚合的入账清单（含利息、已关闭 offer 的本金回补与
// 秒结清惩罚），供 controller 写入充值日志。
func RepayLoan(userId int, amountUsd string) (*TokenLoanAccount, *LoanRepayInfo, []LenderCredit, error) {
	loanSetting := operation_setting.GetLoanSetting()
	if !loanSetting.Enabled {
		return nil, nil, nil, ErrLoanDisabled
	}

	repayAll := strings.EqualFold(strings.TrimSpace(amountUsd), "all")
	var amount int64
	if !repayAll {
		// 与 BorrowLoan 同一套金额解析：非正数或超过两位小数一律拒绝
		usd, err := decimal.NewFromString(amountUsd)
		if err != nil || !usd.IsPositive() || usd.Exponent() < -2 {
			return nil, nil, nil, ErrLoanInvalidAmount
		}
		quotaDec := usd.Mul(decimal.NewFromFloat(common.QuotaPerUnit))
		a, overflow := LoanQuotaFromDecimal(quotaDec)
		if overflow {
			return nil, nil, nil, ErrLoanQuotaOverflow
		}
		amount = a
		// 同 BorrowLoan：QuotaPerUnit 运行时可调，换算后可能为 0，显式拒绝
		if amount <= 0 {
			return nil, nil, nil, ErrLoanInvalidAmount
		}
	}

	now := time.Now()
	var acc *TokenLoanAccount
	var info *LoanRepayInfo
	var credits []LenderCredit
	var flipped []TokenLoanFunding // 本次新翻转的逾期 funding（Task 15 官方处置派发）
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Select("id", "quota").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}

		var err error
		// 还款只面向已有账户，不为无贷用户建行
		acc, err = getLoanAccountTx(tx, userId)
		if err != nil {
			return err
		}
		if acc == nil {
			return ErrLoanNoDebt
		}
		// 债务以 fundings 为准（Task 8 起账户行为投影）：锁全部 active/overdue fundings
		// （id 升序）→ 逐条 settleFunding（platform 传 acc）→ 同步账户投影，据此计算
		// 还款额与手续费。结算有变动的行立即落盘（与 BorrowLoan 同模式），避免分配后
		// 未获配额的 funding 结算丢失
		fundings, err := loadUserFundingsTx(tx, userId)
		if err != nil {
			return err
		}
		for i := range fundings {
			before := fundings[i]
			settleFunding(&fundings[i], acc, now)
			if fundings[i].DebtQuota != before.DebtQuota || fundings[i].LastSettledDay != before.LastSettledDay {
				if err := tx.Save(&fundings[i]).Error; err != nil {
					return err
				}
			}
		}
		syncAccountFromFundings(acc, fundings)
		if acc.DebtQuota <= 0 {
			return ErrLoanNoDebt
		}

		repay := amount
		if repayAll {
			repay = min(acc.DebtQuota, int64(user.Quota))
		} else {
			repay = min(repay, acc.DebtQuota)
		}
		if repay <= 0 {
			// all 时余额为 0 视同为余额不足；显式金额理论上到不了这里（正数且债务为正）
			return ErrLoanInsufficientBalance
		}

		// 手续费按抵本部分计算；all 时余额需同时覆盖还款额与手续费，
		// 不足则下调还款额（手续费随还款额单调不增，数次迭代必收敛）
		fee := earlyRepayFee(acc, repay)
		if repayAll {
			for i := 0; i < 4 && repay > 0 && repay+fee > int64(user.Quota); i++ {
				repay = int64(user.Quota) - fee
				fee = earlyRepayFee(acc, repay)
			}
			if repay <= 0 {
				return ErrLoanInsufficientBalance
			}
		}
		if int64(user.Quota) < repay+fee {
			return ErrLoanInsufficientBalance
		}

		// pro-rata 分配（结算幂等，不会二次计息；变更的 funding 行在此落盘）。
		// info 必非 nil：repay > 0 且 Σdebt = acc.DebtQuota > 0
		var allocs []RepayAllocation
		info, allocs, flipped, err = distributeRepayment(tx, acc, fundings, repay, now, "manual", false)
		if err != nil {
			return err
		}
		if info == nil {
			return ErrLoanNoDebt
		}
		info.FeePart = fee

		// 秒结清惩罚（仅手动还款触发；签到自动还款经 checkin 路径不经过本函数）：
		// 识别本次转结清且仍在惩罚窗口内的 funding，Σ 惩罚从借款人余额扣除（不参与
		// 余额充足性校验，允许扣成负数——借款人用余额覆盖本息费即可结清，惩罚后扣），
		// 按 funding 写入台账 penalty_part，按放贷人并入入账清单（与利息/本金同一
		// 64 位上界校验，见 settleRepayAllocations）。
		penaltyByFunding, _, penaltyTotal := computeFastRepayPenalties(allocs, now)
		info.PenaltyPart = penaltyTotal

		// 放贷人入账 + offer 回补 + 台账 repay 行（含 fee_part；同事务，失败整体回滚）
		credits, err = settleRepayAllocations(tx, userId, allocs, "manual", penaltyByFunding, fee)
		if err != nil {
			return err
		}

		acc.UpdatedAt = now.Unix()
		if err := tx.Save(acc).Error; err != nil {
			return err
		}

		// 余额扣款（还款额+手续费+秒结清惩罚）与账户/台账/funding/offer/放贷人入账
		// 同一事务，失败整体回滚。惩罚不经余额充足性校验，允许扣成负数。
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota - ?", info.Amount+info.FeePart+penaltyTotal)).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// 放贷人入账溢出（借款人无过错）：事务已回滚，异步通知管理员/放贷人介入
		notifyLenderOverflowAsync(err)
		return nil, nil, nil, err
	}
	// 事务提交后异步同步 Redis 余额缓存（镜像 BorrowLoan 的缓存副作用）：
	// 借款人按 还款额+手续费+惩罚 扣减，各放贷人按入账清单递增（已含惩罚）
	go func() {
		_ = cacheDecrUserQuota(userId, info.Amount+info.FeePart+info.PenaltyPart)
		for _, c := range credits {
			_ = cacheIncrUserQuota(c.UserId, c.Amount)
		}
	}()
	// 本次新翻转的 platform 逾期 funding 异步派发官方处置（Task 15，提交后派发）
	dispatchPlatformOverdueAsync(flipped)
	return acc, info, credits, nil
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
