package model

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ===== 借贷市场：资金供给方（offer）与投放记录（funding） =====

// 撮合模式
const (
	LoanOfferModePool  = "pool"  // 资金池自动撮合（按序/按需分配资金池额度）
	LoanOfferModeAi    = "ai"    // AI 审核撮合（放贷金额/利率由 AI 决定）
	LoanOfferModeOrder = "order" // 订单式撮合（明确指定 offer 匹配）
)

// offer 状态
const (
	LoanOfferStatusActive = "active" // 上架可撮合
	LoanOfferStatusPaused = "paused" // 暂停，不再参与新撮合
	LoanOfferStatusClosed = "closed" // 关闭下架
)

// funding 资金来源
const (
	LoanFundingPlatform = "platform" // 平台资金
	LoanFundingPool     = "pool"     // 资金池（offer mode=pool/ai 撮合）
	LoanFundingAi       = "ai"       // AI 决定投放
	LoanFundingOrder    = "order"    // 订单式指定投放
)

// funding 还款计划（结算语义见 spec §5）
const (
	LoanRepayFull           = "full"            // 正常复利+逾期罚息直到还清（默认）
	LoanRepayNoPenalty      = "no_penalty"      // 逾期后免罚息，只还本金+已结算利息（仍按 rate 计息）
	LoanRepayInterestFreeze = "interest_freeze" // 停止后续计息，只还本金+已结算利息
	LoanRepayPrincipalOnly  = "principal_only"  // 利息全免只还本金（改档时一次性核销未付利息）
)

// funding 状态
const (
	LoanFundingActive     = "active"      // 正常在贷
	LoanFundingOverdue    = "overdue"     // 逾期
	LoanFundingRepaid     = "repaid"      // 已结清
	LoanFundingWrittenOff = "written_off" // 已核销
)

// TokenLoanOffer 借贷市场供给方（放贷人）挂出的可撮合资金 offer
type TokenLoanOffer struct {
	Id                  int     `json:"id" gorm:"primaryKey;autoIncrement"`
	LenderId            int     `json:"lender_id" gorm:"not null;index"`               // 放贷人 user id
	Mode                string  `json:"mode" gorm:"type:varchar(16);not null"`         // pool / ai / order
	Status              string  `json:"status" gorm:"type:varchar(16);not null;index"` // active / paused / closed
	AmountTotal         int64   `json:"amount_total" gorm:"bigint"`                    // 挂出总额
	AmountAvailable     int64   `json:"amount_available" gorm:"bigint"`                // 剩余可撮合额度
	RateFixed           float64 `json:"rate_fixed"`                                    // 固定日利率（0 = 走区间竞价）
	RateMin             float64 `json:"rate_min"`                                      // 区间利率下限
	RateMax             float64 `json:"rate_max"`                                      // 区间利率上限
	PerLoanCap          int64   `json:"per_loan_cap" gorm:"bigint"`                    // 单笔上限
	MinCreditScore      int     `json:"min_credit_score"`                              // 最低可借信用分，-50 = 不限（spec §4.1；0 是罚分后的合法分值）
	TotalLent           int64   `json:"total_lent" gorm:"bigint"`                      // 累计放出
	TotalInterestEarned int64   `json:"total_interest_earned" gorm:"bigint"`           // 累计利息收入
	CreatedAt           int64   `json:"created_at" gorm:"bigint"`                      // 秒级时间戳
	UpdatedAt           int64   `json:"updated_at" gorm:"bigint"`                      // 秒级时间戳
}

func (TokenLoanOffer) TableName() string {
	return "token_loan_offers"
}

// TokenLoanFunding 单笔放贷投放记录：一次借款事件对应一条，记录资金去向与还款状态
type TokenLoanFunding struct {
	Id                 int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	LoanUserId         int     `json:"loan_user_id" gorm:"not null;index"`            // 借款人 user id
	BorrowEventId      int64   `json:"borrow_event_id" gorm:"bigint"`                 // 关联借款事件 id，0 = 非事件驱动（如平台直放）
	SourceType         string  `json:"source_type" gorm:"type:varchar(16);not null"`  // platform / pool / ai / order
	OfferId            int     `json:"offer_id" gorm:"index"`                         // 关联 offer id，platform 时为 0
	LenderId           int     `json:"lender_id" gorm:"index"`                        // 实际放贷方 user id，platform 时为 0
	Amount             int64   `json:"amount" gorm:"bigint"`                          // 投放本金
	PrincipalRemaining int64   `json:"principal_remaining" gorm:"bigint"`             // 未还本金
	DebtQuota          int64   `json:"debt_quota" gorm:"bigint"`                      // 未还本息总额（含利息）
	LastSettledDay     int     `json:"last_settled_day"`                              // 上次惰性结算的 loanDay
	Rate               float64 `json:"rate"`                                          // 实际执行的日利率
	RepayPlan          string  `json:"repay_plan" gorm:"type:varchar(16);not null"`   // full / no_penalty / interest_freeze / principal_only
	Status             string  `json:"status" gorm:"type:varchar(16);not null;index"` // active / overdue / repaid / written_off
	DueDay             int     `json:"due_day"`                                       // 应还日 loanDay，0 = 未定
	PenaltyStartedDay  int     `json:"penalty_started_day"`                           // 罚息起始 loanDay，0 = 无
	CreatedAt          int64   `json:"created_at" gorm:"bigint"`                      // 秒级时间戳
	UpdatedAt          int64   `json:"updated_at" gorm:"bigint"`                      // 秒级时间戳
}

func (TokenLoanFunding) TableName() string {
	return "token_loan_fundings"
}

// ===== Task 5: 存量迁移 =====

// loanFundingMigrationOptionKey 迁移哨兵：迁移整体只执行一次（含 credit_score 回填）。
// 信用分系统（Task 11）上线后 credit_score==0 是罚分后的合法分值，
// 严格禁止把 0 再次当作"未评估"整体重填为初始分。
const loanFundingMigrationOptionKey = "LoanFundingMigratedV1"

// MigrateLoanToFundings 存量迁移（spec §15）：一次性迁移，哨兵 Option 行
// （loanFundingMigrationOptionKey）置位后每次启动直接返回，不做任何动作。
//  1. 全量账户先惰性 settle 到当前本地日并落盘——存量利息落定，LastSettledDay 前推；
//  2. 对 settle 后仍 debt_quota > 0 且该用户尚无 platform funding 的账户，生成一条
//     platform funding：
//     Amount=PrincipalRemaining=PrincipalQuota、DebtQuota=settle 后债务、
//     Rate=当时 effectiveRate(acc)、LastSettledDay=账户 LastSettledDay、
//     DueDay=迁移日+LoanTermDays（>0 防御，见下）、RepayPlan=full、Status=active、
//     SourceType=platform、BorrowEventId=0（无历史借款事件）、OfferId/LenderId=0、
//     PenaltyStartedDay=0、时间戳=now。
//     InterestFreeUntil 宽限留在账户行上，settleFunding 的 platform 分支会读取它
//     （spec §15：存量宽限由 platform funding 继续承载，不复制到 funding 行）。
//  3. credit_score=0 的账户回填 CreditInitial（与第 1、2 步同轮完成）。
//
// 一次性是关键约束而非省事：Task 9 之后 TokenLoanAccount 的 DebtQuota 成为 fundings 的
// 纯投影（syncAccountFromFundings），若每次启动仍对账户做惰性结算，会与 funding 时钟
// 独立复利、双重计息。迁移只在部署时跑一次；失败整体回滚、哨兵不落库，下次启动自动重试。
func MigrateLoanToFundings() error {
	// 哨兵已置位：整体一次性迁移已完成，直接返回（先于任何 settle，保证零副作用）
	var existing Option
	if err := DB.Where("key = ?", loanFundingMigrationOptionKey).First(&existing).Error; err == nil {
		if existing.Value == "true" {
			return nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	now := time.Now()
	created := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 事务内二次确认哨兵：并发首启时避免重复回填（与既有 Once 迁移模式一致）
		creditPending := true
		if err := tx.Where("key = ?", loanFundingMigrationOptionKey).First(&existing).Error; err == nil {
			creditPending = existing.Value != "true"
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var accs []TokenLoanAccount
		if err := tx.Find(&accs).Error; err != nil {
			return err
		}

		loanSetting := operation_setting.GetLoanSetting()
		today := loanDay(now)
		// 防呆（评审发现）：LoanTermDays 被配置为 0/负值时 formula 会得到当日/过去到期日，
		// full 计划下迁移当日即整段按罚息计息；至少推到明天，保证 DueDay>0 且当日不触发罚息。
		dueDay := today + loanSetting.LoanTermDays
		if dueDay <= today {
			dueDay = today + 1
		}

		for i := range accs {
			acc := &accs[i]
			settle(acc, now)
			if creditPending && acc.CreditScore == 0 {
				acc.CreditScore = loanSetting.CreditInitial
			}
			if err := tx.Save(acc).Error; err != nil {
				return err
			}
			if acc.DebtQuota <= 0 {
				continue
			}
			var fundingCnt int64
			if err := tx.Model(&TokenLoanFunding{}).
				Where("loan_user_id = ? AND source_type = ?", acc.UserId, LoanFundingPlatform).
				Count(&fundingCnt).Error; err != nil {
				return err
			}
			if fundingCnt > 0 {
				continue // 幂等：已转化过，不重复生成
			}
			if err := tx.Create(&TokenLoanFunding{
				LoanUserId:         acc.UserId,
				BorrowEventId:      0,
				SourceType:         LoanFundingPlatform,
				Amount:             acc.PrincipalQuota,
				PrincipalRemaining: acc.PrincipalQuota,
				DebtQuota:          acc.DebtQuota,
				LastSettledDay:     acc.LastSettledDay,
				Rate:               effectiveRate(acc),
				RepayPlan:          LoanRepayFull,
				Status:             LoanFundingActive,
				DueDay:             dueDay,
				CreatedAt:          now.Unix(),
				UpdatedAt:          now.Unix(),
			}).Error; err != nil {
				return err
			}
			created++
		}

		if !creditPending {
			return nil
		}
		opt := Option{Key: loanFundingMigrationOptionKey, Value: "true"}
		if err := tx.FirstOrCreate(&opt, Option{Key: loanFundingMigrationOptionKey}).Error; err != nil {
			return err
		}
		return tx.Model(&Option{}).Where("key = ?", loanFundingMigrationOptionKey).Update("value", "true").Error
	})
	if err != nil {
		return err
	}
	if created > 0 {
		common.SysLog(fmt.Sprintf("loan funding migration: created %d platform funding(s) for legacy loans", created))
	}
	return nil
}

// ===== Task 6: offer 生命周期 =====

// 借贷市场哨兵错误，controller 层映射为 i18n 响应。
// 金额类错误复用 model/loan.go 的 ErrLoanInvalidAmount / ErrLoanInsufficientBalance /
// ErrLoanQuotaOverflow / ErrLoanUserDisabled。
var (
	ErrLoanMarketDisabled     = errors.New("loan market is disabled")
	ErrLoanDisclaimerRequired = errors.New("lender disclaimer not agreed")
	ErrLoanOfferNotFound      = errors.New("loan offer not found")
	ErrLoanOfferInvalidParams = errors.New("invalid loan offer parameters")
	ErrLoanOfferNotActive     = errors.New("loan offer is not in an operable state")
	ErrLoanNothingToWithdraw  = errors.New("loan offer has no idle balance to withdraw")

	// 禁止二次挂市场（P1-10）：可放贷额度 = 实际余额 - 未还借款本金，超出即拒绝
	ErrLoanLendBorrowedNotAllowed = errors.New("borrowed quota cannot be re-lent on the market")

	// Task 12：逾期债权处置（spec §9）
	ErrLoanFundingNotOverdue    = errors.New("loan funding is not overdue")            // 仅 overdue 可处置
	ErrLoanInvalidDefaultAction = errors.New("invalid loan default action")            // 非法动作 / extendDays 越界
	ErrLoanNotFundingOwner      = errors.New("loan funding does not belong to lender") // 非本人或平台债权

	// Task 14：repay_plan 调整（spec §8）
	ErrLoanInvalidRepayPlan = errors.New("invalid loan repay plan") // 非法 plan / 终态 funding / P2P 越权改档
)

// AgreeLenderDisclaimer 幂等记录放贷人同意免责声明的时间（spec §4.3/§11）；
// 账户不存在时顺带创建，镜像 AgreeLoanTerms。
func AgreeLenderDisclaimer(userId int) error {
	now := time.Now()
	return DB.Transaction(func(tx *gorm.DB) error {
		acc, err := getOrCreateLoanAccountTxSafe(tx, userId)
		if err != nil {
			return err
		}
		if acc.LenderDisclaimerAgreedAt != 0 {
			return nil // 幂等：已同意不覆盖首次时间
		}
		acc.LenderDisclaimerAgreedAt = now.Unix()
		acc.UpdatedAt = now.Unix()
		return tx.Save(acc).Error
	})
}

// CreateLoanOffer 放贷人挂出供给单（spec §3.1/§4.1）：
//  1. 事务外校验：市场开关、模式、金额 decimal 解析（镜像 BorrowLoan：正数、最多两位
//     小数、int32 上界）、利率区间、单笔上限、信用分门槛钳制；
//  2. 事务内 lockForUpdate 锁 users 行：状态正常、余额充足 → 扣 quota；读/建贷款账户
//     并校验免责声明 → 禁止二次挂市场（默认开启：可放贷额度 = 实际余额 - 未还借款本金，
//     超出报 ErrLoanLendBorrowedNotAllowed；MarketAllowLendBorrowed=true 时跳过）→ 建
//     offer（AmountTotal=AmountAvailable=amount，扣款与建行同事务，失败整体回滚）；
//  3. 提交后异步同步 Redis 余额缓存（镜像 BorrowLoan 的副作用）。
//
// 校验规则：
//   - pool/order：rateFixed ∈ [LenderRateMin, LenderRateMax]；perLoanCap=0 时取
//     PerLoanCapDefault 缺省（可能仍为 0 = 不限）；
//   - ai：rateMin/rateMax 区间 ⊆ [LenderRateMin, LenderRateMax] 且 perLoanCap > 0；
//   - minCreditScore 钳制到 [-50, 100]，低于 -50 视为"不限制"。
func CreateLoanOffer(lenderId int, mode string, amountUsd, rateFixed string, rateMin, rateMax float64, perLoanCap int64, minCreditScore int) (*TokenLoanOffer, error) {
	loanSetting := operation_setting.GetLoanSetting()
	if !loanSetting.MarketEnabled {
		return nil, ErrLoanMarketDisabled
	}
	if mode != LoanOfferModePool && mode != LoanOfferModeAi && mode != LoanOfferModeOrder {
		return nil, ErrLoanOfferInvalidParams
	}

	// 金额解析与 BorrowLoan 同一套：非正数或超过两位小数一律拒绝
	usd, err := decimal.NewFromString(amountUsd)
	if err != nil || !usd.IsPositive() || usd.Exponent() < -2 {
		return nil, ErrLoanInvalidAmount
	}
	quotaDec := usd.Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	amount, clamp := common.QuotaFromDecimalChecked(quotaDec)
	if clamp != nil {
		return nil, ErrLoanQuotaOverflow
	}
	// QuotaPerUnit 是运行时可调配置（model/option.go），换算后 amount 可能为 0，必须显式拒绝
	if amount <= 0 {
		return nil, ErrLoanInvalidAmount
	}
	if int64(amount) < loanSetting.LenderMinAmount {
		return nil, ErrLoanOfferInvalidParams
	}
	// 挂出金额必须落在 int32 quota 上界内（clamp 后恒成立，防御性校验）
	if int64(amount) > common.MaxQuota {
		return nil, ErrLoanQuotaOverflow
	}

	var rateFixedVal float64
	switch mode {
	case LoanOfferModePool, LoanOfferModeOrder:
		rf, err := strconv.ParseFloat(rateFixed, 64)
		if err != nil || rf < loanSetting.LenderRateMin || rf > loanSetting.LenderRateMax {
			return nil, ErrLoanOfferInvalidParams
		}
		rateFixedVal = rf
	case LoanOfferModeAi:
		if rateMin > rateMax || rateMin < loanSetting.LenderRateMin || rateMax > loanSetting.LenderRateMax {
			return nil, ErrLoanOfferInvalidParams
		}
		if perLoanCap <= 0 {
			return nil, ErrLoanOfferInvalidParams
		}
	}
	// pool/order 的 perLoanCap=0 表示跟随全局缺省；ai 已强制 perLoanCap>0，无需替换
	if perLoanCap == 0 && mode != LoanOfferModeAi {
		perLoanCap = loanSetting.PerLoanCapDefault
	}
	// 信用分门槛钳制 [-50, 100]；-50 即"不限制"（spec §4.1）
	if minCreditScore < -50 {
		minCreditScore = -50
	} else if minCreditScore > 100 {
		minCreditScore = 100
	}

	now := time.Now()
	var offer *TokenLoanOffer
	err = DB.Transaction(func(tx *gorm.DB) error {
		// 事务内锁 users 行：状态与余额（同 BorrowLoan 的读模式 + FOR UPDATE，
		// 并发同放贷人挂单在行锁上串行，扣减不丢失）
		var user User
		if err := lockForUpdate(tx).Select("id", "quota", "status").Where("id = ?", lenderId).First(&user).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return ErrLoanUserDisabled
		}
		if int64(user.Quota) < int64(amount) {
			return ErrLoanInsufficientBalance
		}

		acc, err := getOrCreateLoanAccountTxSafe(tx, lenderId)
		if err != nil {
			return err
		}
		if acc.LenderDisclaimerAgreedAt == 0 {
			return ErrLoanDisclaimerRequired
		}

		// 禁止二次挂市场（默认开启）：可放贷额度 = 实际余额 - 未还借款本金（floor 0）。
		// 放贷人可能同时是借款人，其名下 active/overdue funding 的未还本金即借来的钱，
		// 不得再用于放贷。用户行已 FOR UPDATE 锁定，还款/借款路径同样先锁用户行，
		// 本查询与并发写序列化，读取一致。MarketAllowLendBorrowed=true 时跳过该检查。
		if !loanSetting.MarketAllowLendBorrowed {
			var outstanding int64
			if err := tx.Model(&TokenLoanFunding{}).
				Where("loan_user_id = ? AND status IN ?", lenderId, []string{LoanFundingActive, LoanFundingOverdue}).
				Select("COALESCE(SUM(principal_remaining), 0)").
				Row().Scan(&outstanding); err != nil {
				return err
			}
			lendable := int64(user.Quota) - outstanding
			if lendable < 0 {
				lendable = 0
			}
			if int64(amount) > lendable {
				return ErrLoanLendBorrowedNotAllowed
			}
		}

		// 扣 quota 与建 offer 同一事务，失败整体回滚
		if err := tx.Model(&User{}).Where("id = ?", lenderId).
			Update("quota", gorm.Expr("quota - ?", amount)).Error; err != nil {
			return err
		}
		offer = &TokenLoanOffer{
			LenderId:        lenderId,
			Mode:            mode,
			Status:          LoanOfferStatusActive,
			AmountTotal:     int64(amount),
			AmountAvailable: int64(amount),
			RateFixed:       rateFixedVal,
			RateMin:         rateMin,
			RateMax:         rateMax,
			PerLoanCap:      perLoanCap,
			MinCreditScore:  minCreditScore,
			CreatedAt:       now.Unix(),
			UpdatedAt:       now.Unix(),
		}
		return tx.Create(offer).Error
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后异步同步 Redis 余额缓存（镜像 BorrowLoan 的缓存副作用）
	go func() {
		_ = cacheDecrUserQuota(lenderId, int64(amount))
	}()
	return offer, nil
}

// SetLoanOfferStatus offer 状态流转：仅 active ⇄ paused（关闭是终态，走 CloseLoanOffer）。
// 已关闭的 offer 再操作报 ErrLoanOfferNotActive；非法目标状态报 ErrLoanOfferInvalidParams；
// 非本人 offer 一律 ErrLoanOfferNotFound（not-found-not-forbidden，与 GetLoanApplicationById 同风格）。
func SetLoanOfferStatus(lenderId int, offerId int, status string) error {
	if status != LoanOfferStatusActive && status != LoanOfferStatusPaused {
		return ErrLoanOfferInvalidParams
	}
	now := time.Now()
	return DB.Transaction(func(tx *gorm.DB) error {
		var offer TokenLoanOffer
		if err := lockForUpdate(tx).Where("id = ? AND lender_id = ?", offerId, lenderId).First(&offer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrLoanOfferNotFound
			}
			return err
		}
		if offer.Status == LoanOfferStatusClosed {
			return ErrLoanOfferNotActive
		}
		if offer.Status == status {
			return nil // 幂等：已是目标状态
		}
		return tx.Model(&TokenLoanOffer{}).Where("id = ?", offerId).
			Updates(map[string]interface{}{"status": status, "updated_at": now.Unix()}).Error
	})
}

// CloseLoanOffer 关闭 offer（终态，spec §4.1）：事务内锁 offer 行，闲置额度
// AmountAvailable 退回用户余额，amount_total 同步核减（钱离开 offer 账面，与核销
// "amount_total 同步减"同语义，保持不变式 amount_total = amount_available + Σ 未还本金），
// 存续 funding 不受影响（后续本金直接回放贷人余额属 Task 9 还款分配）。提交后异步同步余额缓存。
// 返回退回的闲置额度（无闲置额度时返回 0），供 controller 写入充值日志。
func CloseLoanOffer(lenderId int, offerId int) (int64, error) {
	return closeOrWithdrawOffer(lenderId, offerId, true)
}

// WithdrawLoanOffer 撤回 offer 的全部闲置额度到用户余额（v1 简化：撤回后 offer 保留原状态，
// AmountAvailable=0 且不支持再充值），amount_total 同步核减；返回撤回额度。
func WithdrawLoanOffer(lenderId int, offerId int) (int64, error) {
	return closeOrWithdrawOffer(lenderId, offerId, false)
}

// closeOrWithdrawOffer 关闭/撤回共用实现：锁 offer 行（归属校验内置 WHERE，非本人一律
// ErrLoanOfferNotFound）→ 校验状态与闲置额度 → 退回余额（int32 上界校验镜像 BorrowLoan
// 的 quota+amount 检查）→ 核减 amount_total / 清零 amount_available。
func closeOrWithdrawOffer(lenderId int, offerId int, closing bool) (int64, error) {
	now := time.Now()
	var refunded int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var offer TokenLoanOffer
		if err := lockForUpdate(tx).Where("id = ? AND lender_id = ?", offerId, lenderId).First(&offer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrLoanOfferNotFound
			}
			return err
		}
		if offer.Status == LoanOfferStatusClosed {
			return ErrLoanOfferNotActive
		}
		refund := offer.AmountAvailable
		if !closing && refund <= 0 {
			return ErrLoanNothingToWithdraw
		}
		if refund > 0 {
			var user User
			if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", lenderId).First(&user).Error; err != nil {
				return err
			}
			// 入账走 int32 上界校验（镜像 BorrowLoan 的 quota+amount 检查）
			if int64(user.Quota)+refund > common.MaxQuota {
				return ErrLoanQuotaOverflow
			}
			if err := tx.Model(&User{}).Where("id = ?", lenderId).
				Update("quota", gorm.Expr("quota + ?", refund)).Error; err != nil {
				return err
			}
		}
		updates := map[string]interface{}{
			"amount_available": int64(0),
			"amount_total":     offer.AmountTotal - refund,
			"updated_at":       now.Unix(),
		}
		if closing {
			updates["status"] = LoanOfferStatusClosed
		}
		if err := tx.Model(&TokenLoanOffer{}).Where("id = ?", offerId).Updates(updates).Error; err != nil {
			return err
		}
		refunded = refund
		return nil
	})
	if err != nil {
		return 0, err
	}
	if refunded > 0 {
		go func() {
			_ = cacheIncrUserQuota(lenderId, refunded)
		}()
	}
	return refunded, nil
}

// GetUserLoanOffers 返回放贷人全部 offer，id 倒序（最新在前）
func GetUserLoanOffers(lenderId int) ([]TokenLoanOffer, error) {
	var offers []TokenLoanOffer
	err := DB.Where("lender_id = ?", lenderId).Order("id DESC").Find(&offers).Error
	return offers, err
}

// GetLoanOfferById 按 id 查询 offer（管理端只读），不存在时透出 gorm.ErrRecordNotFound
func GetLoanOfferById(id int) (*TokenLoanOffer, error) {
	var offer TokenLoanOffer
	if err := DB.First(&offer, id).Error; err != nil {
		return nil, err
	}
	return &offer, nil
}

// ListActiveOrderOffers 市场浏览：返回全部 active 状态、order 模式且有可撮合额度的
// 挂单，按利率升序、id 升序（确定性平局，与撮合引擎统一市场排序一致）。
func ListActiveOrderOffers() ([]TokenLoanOffer, error) {
	var offers []TokenLoanOffer
	err := DB.Where("status = ? AND mode = ? AND amount_available > 0",
		LoanOfferStatusActive, LoanOfferModeOrder).
		Order("rate_fixed ASC").Order("id ASC").
		Find(&offers).Error
	return offers, err
}

// ListActiveAiOffersForBorrow 借款前收集 ai 模式候选挂单：active、有可撮合额度、
// 排除借款人本人，按 updated_at 倒序（最新优先）；limit 越界/非正时钳制到 20。
func ListActiveAiOffersForBorrow(borrowerId int, limit int) ([]TokenLoanOffer, error) {
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	var offers []TokenLoanOffer
	err := DB.Where("status = ? AND mode = ? AND amount_available > 0 AND lender_id <> ?",
		LoanOfferStatusActive, LoanOfferModeAi, borrowerId).
		Order("updated_at DESC").
		Limit(limit).
		Find(&offers).Error
	return offers, err
}

// GetLenderFundings 分页返回放贷人名下全部投放记录（id 倒序，最新在前），附总数；
// 分页语义镜像 GetUserLoanRecords。
func GetLenderFundings(lenderId, page, pageSize int) ([]TokenLoanFunding, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var total int64
	if err := DB.Model(&TokenLoanFunding{}).Where("lender_id = ?", lenderId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var fundings []TokenLoanFunding
	err := DB.Where("lender_id = ?", lenderId).
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&fundings).Error
	return fundings, total, err
}

// ===== Task 12: 逾期债权处置（延长/核销/永续）+ 黑名单出口 =====

// 逾期债权处置动作（spec §9）
const (
	LoanDefaultActionExtend    = "extend"    // 延长：改写 due_day，status → active（已计罚息保留）
	LoanDefaultActionWriteoff  = "writeoff"  // 核销：终态 written_off，债务销毁 + 拉黑 + 扣分
	LoanDefaultActionPerpetual = "perpetual" // 永续：保持 overdue 继续计息，仅记录决策
)

// ResolveOverdueFunding 放贷人对本人逾期债权三选一处置（spec §9）：
//   - 仅 funding 的 lender 本人（LenderId == lenderId）可调用；platform funding
//     （LenderId==0）归 Task 15 官方流程（AI 审批员），一律 ErrLoanNotFundingOwner；
//   - 仅 status == overdue 可处置，否则 ErrLoanFundingNotOverdue；funding 不存在透出
//     gorm.ErrRecordNotFound；
//   - extend：extendDays ∈ (0, LoanTermDays]，DueDay = loanDay(now) + extendDays，
//     status → active；PenaltyStartedDay 保留（历史记录），按时还清加分的基准即新
//     DueDay（Task 13 读 max(due_day)）；
//   - writeoff：见 writeoffFundingTx（核销债务销毁 + offer 核减 + 拉黑 + 扣分）；
//   - perpetual：funding 状态不变（保持 overdue 继续计息，签到继续 100% 扣还），仅
//     SysLog 记录决策——决策不是资金变动，不写台账；审计日志属 Task 17 controller 侧。
func ResolveOverdueFunding(lenderId int, fundingId int64, action string, extendDays int) error {
	loanSetting := operation_setting.GetLoanSetting()
	now := time.Now()
	return DB.Transaction(func(tx *gorm.DB) error {
		var f TokenLoanFunding
		if err := lockForUpdate(tx).Where("id = ?", fundingId).First(&f).Error; err != nil {
			return err // gorm.ErrRecordNotFound 由 controller 映射
		}
		if f.LenderId != lenderId {
			// 含 platform（LenderId==0）：官方债权归 Task 15 审批员流程，不放贷人自处
			return ErrLoanNotFundingOwner
		}
		if f.Status != LoanFundingOverdue {
			return ErrLoanFundingNotOverdue
		}

		switch action {
		case LoanDefaultActionExtend:
			if extendDays <= 0 || extendDays > loanSetting.LoanTermDays {
				return ErrLoanInvalidDefaultAction
			}
			f.DueDay = loanDay(now) + extendDays
			f.Status = LoanFundingActive
			f.UpdatedAt = now.Unix()
			return tx.Save(&f).Error
		case LoanDefaultActionWriteoff:
			return writeoffFundingTx(tx, &f, now)
		case LoanDefaultActionPerpetual:
			common.SysLog(fmt.Sprintf("loan default decision: funding %d marked perpetual (stays overdue, keeps accruing)", f.Id))
			return nil
		default:
			return ErrLoanInvalidDefaultAction
		}
	})
}

// writeoffFundingTx 核销事务（f 为已锁定的 overdue P2P funding）：
//  1. settleFunding(f, nil, now) 冻结最终债务——P2P 用自身利率，acc 仅 platform 分支
//     使用故传 nil；冻结的债务留在 funding 行作历史记录，不在任何路径偿还；
//  2. funding → written_off 终态落盘；
//  3. offer 侧（锁 offer 行，offer 可能已关闭，两态同处理）：amount_total -=
//     principal_remaining（floor 0 防御）。钱在放款时已离开 offer 账面
//     （amount_available 已扣减），此处核减 amount_total 维持不变式
//     amount_total = amount_available + Σ(active/overdue 本金)；
//     offer 行缺失容忍跳过（生产代码无删除 offer 路径，仅防御）；
//  4. 借款人侧（锁贷款账户，缺失防御性创建——有 funding 必有账户）：
//     blacklisted_until_day = max(现值, today + BlacklistDaysOnDefault)，
//     credit_score -= CreditDefaultPenalty（下限 -50 截断，P1-9），并写一条
//     type=credit 台账行（Amount=实际生效扣分，DebtAfter=扣分后信用分，Source=writeoff）；
//  5. syncAccountFromFundings 汇总剩余 active/overdue fundings 回写账户投影，本笔核销
//     债务从投影销毁（deflation，spec §9）。
//
// 未付利息在借款人侧直接免除（放贷人从未收到，无账可冲），核销不写资金类台账行——
// 决策不是资金变动，但信用分是用户可见的资信变动，必须留痕（credit 行）。锁序：
// funding → offer → 借款人账户；与还款路径（用户 → 账户 → funding）在 (账户, funding)
// 上存在理论锁序倒挂，并发同用户核销+还款时数据库检测死锁并中止一方事务（整体回滚，
// 无数据损坏），调用方重试即可；v1 接受该瞬时失败。
func writeoffFundingTx(tx *gorm.DB, f *TokenLoanFunding, now time.Time) error {
	loanSetting := operation_setting.GetLoanSetting()

	settleFunding(f, nil, now) // 冻结最终债务（P2P 利率，无平台分支）
	f.Status = LoanFundingWrittenOff
	f.UpdatedAt = now.Unix()
	if err := tx.Save(f).Error; err != nil {
		return err
	}

	if f.OfferId > 0 {
		var offer TokenLoanOffer
		err := lockForUpdate(tx).Where("id = ?", f.OfferId).First(&offer).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			reduced := offer.AmountTotal - f.PrincipalRemaining
			if reduced < 0 {
				reduced = 0 // floor 防御，维持 amount_total >= 0
			}
			offer.AmountTotal = reduced
			offer.UpdatedAt = now.Unix()
			if err := tx.Save(&offer).Error; err != nil {
				return err
			}
		}
	}

	borrowerAcc, err := getOrCreateLoanAccountTx(tx, f.LoanUserId)
	if err != nil {
		return err
	}
	if bl := loanDay(now) + loanSetting.BlacklistDaysOnDefault; bl > borrowerAcc.BlacklistedUntilDay {
		borrowerAcc.BlacklistedUntilDay = bl
	}
	before := borrowerAcc.CreditScore
	borrowerAcc.CreditScore -= loanSetting.CreditDefaultPenalty
	if borrowerAcc.CreditScore < -50 {
		borrowerAcc.CreditScore = -50 // P1-9：下限 -50 截断
	}
	if err := writeCreditLedgerRowTx(tx, borrowerAcc, before, "writeoff", 0, now); err != nil {
		return err
	}

	// 投影同步：written_off 债务从账户投影销毁（deflation），不再计入任何债务
	remaining, err := loadUserFundingsTx(tx, f.LoanUserId)
	if err != nil {
		return err
	}
	syncAccountFromFundings(borrowerAcc, remaining)
	borrowerAcc.UpdatedAt = now.Unix()
	return tx.Save(borrowerAcc).Error
}

// maybeLiftBlacklistTx 黑名单出口（spec §9）：在 distributeRepayment 的 repaid 分支调用，
// 还款后若满足解除条件则清零 BlacklistedUntilDay。规则（评审钉死，见 plan Task 12）：
//   - 仅当 acc.BlacklistedUntilDay > 0 且用户已无任何 overdue funding（全部还清）时考虑解除；
//   - 追加守卫：当前黑名单不得由「窗口仍在运行」的核销设置——存在 written_off funding 且
//     其核销落账日 >= 黑名单起始日（blacklist_start = BlacklistedUntilDay -
//     BlacklistDaysOnDefault）时不解锁。核销拉黑不可逆（核销不可逆），必须跑满窗口自然到期；
//   - 永续全还清 → 立即解除：永续路径不产生 written_off 行，只要无 overdue 即满足守卫，
//     BlacklistedUntilDay 立即清零（还款激励）。
//
// 推导注记：blacklist_start 由 max(现值, today+N) 反推，最新一次核销的落账日恒 >= 该
// 起始日，故只要存在 written_off 行（黑名单由核销触发）就永不提前解除——这正是"核销
// 不可逆"；提前解除仅发生在无 written_off 的黑名单上（如外部/历史置位，或永续路径下
// 黑名单并非由本次核销窗口引发），窗口自然过完后 BorrowLoan 闸门
// （blacklisted_until_day > today）本就放行，此处清零仅为状态整洁。
//
// 仅修改内存 acc（BlacklistedUntilDay），落盘由调用方统一 tx.Save(acc)。
func maybeLiftBlacklistTx(tx *gorm.DB, acc *TokenLoanAccount, now time.Time) error {
	if acc.BlacklistedUntilDay <= 0 {
		return nil
	}
	hasOverdue, err := HasOverdueFundings(tx, acc.UserId)
	if err != nil {
		return err
	}
	if hasOverdue {
		return nil
	}
	start := acc.BlacklistedUntilDay - operation_setting.GetLoanSetting().BlacklistDaysOnDefault
	var writtenOff []TokenLoanFunding
	if err := tx.Select("updated_at").
		Where("loan_user_id = ? AND status = ?", acc.UserId, LoanFundingWrittenOff).
		Find(&writtenOff).Error; err != nil {
		return err
	}
	for _, f := range writtenOff {
		if loanDay(time.Unix(f.UpdatedAt, 0)) >= start {
			return nil // 核销窗口仍在运行，不提前解除
		}
	}
	acc.BlacklistedUntilDay = 0
	return nil
}

// ===== Task 14: repay_plan 调整（settle-first，spec §8） =====

// repayPlanOrder P2P 单向降档顺序（AI 审批员入口仅前三级：full < no_penalty <
// interest_freeze；principal_only 永远拒绝——见 SetFundingRepayPlanByOfficer）
var repayPlanOrder = map[string]int{
	LoanRepayFull:           0,
	LoanRepayNoPenalty:      1,
	LoanRepayInterestFreeze: 2,
}

// isValidRepayPlan plan 必须是四档常量之一
func isValidRepayPlan(plan string) bool {
	switch plan {
	case LoanRepayFull, LoanRepayNoPenalty, LoanRepayInterestFreeze, LoanRepayPrincipalOnly:
		return true
	}
	return false
}

// SetFundingRepayPlan 放贷人调整本人 P2P funding 的 repay_plan（spec §8 P0-2）：
//   - 仅 funding 的 lender 本人（LenderId == lenderId）可调用；platform funding
//     （LenderId==0）归 Task 15 官方流程，一律 ErrLoanNotFundingOwner（复用 Task 12 错误）；
//   - plan 必须四档常量之一（ErrLoanInvalidRepayPlan）；funding 必须 active/overdue，
//     repaid/written_off 终态不可改档（ErrLoanInvalidRepayPlan）；funding 不存在透出
//     gorm.ErrRecordNotFound；
//   - 同 plan 幂等（no-op，不结算不改动）；
//   - 否则 settle-first 改档，见 setFundingRepayPlanTx。
func SetFundingRepayPlan(lenderId int, fundingId int64, plan string) error {
	if !isValidRepayPlan(plan) {
		return ErrLoanInvalidRepayPlan
	}
	now := time.Now()
	return DB.Transaction(func(tx *gorm.DB) error {
		var f TokenLoanFunding
		if err := lockForUpdate(tx).Where("id = ?", fundingId).First(&f).Error; err != nil {
			return err // gorm.ErrRecordNotFound 由 controller 映射
		}
		if f.LenderId != lenderId {
			// 含 platform（LenderId==0）：官方债权归 Task 15 审批员流程，不放贷人自调
			return ErrLoanNotFundingOwner
		}
		if f.Status != LoanFundingActive && f.Status != LoanFundingOverdue {
			return ErrLoanInvalidRepayPlan
		}
		if f.RepayPlan == plan {
			return nil // 幂等：已是目标 plan
		}
		return setFundingRepayPlanTx(tx, &f, plan, now)
	})
}

// SetFundingRepayPlanByOfficer AI 审批员改档入口（Task 15 减免申诉裁决调用，spec §8 P0-2）：
//   - platform funding（LenderId==0）：四档全允许；
//   - P2P funding：仅允许 full → no_penalty → interest_freeze 单向降档（可跳档），
//     principal_only 永远拒绝（仅放贷人本人可核销利息，防 prompt 注入批量免息），
//     升档/同档以外的变更一律 ErrLoanInvalidRepayPlan；
//   - 与放贷人入口相同：plan 合法性、active/overdue 状态、同 plan 幂等。
func SetFundingRepayPlanByOfficer(fundingId int64, plan string) error {
	if !isValidRepayPlan(plan) {
		return ErrLoanInvalidRepayPlan
	}
	now := time.Now()
	return DB.Transaction(func(tx *gorm.DB) error {
		var f TokenLoanFunding
		if err := lockForUpdate(tx).Where("id = ?", fundingId).First(&f).Error; err != nil {
			return err // gorm.ErrRecordNotFound 由 controller 映射
		}
		if f.Status != LoanFundingActive && f.Status != LoanFundingOverdue {
			return ErrLoanInvalidRepayPlan
		}
		if f.LenderId != 0 { // P2P funding：权限边界
			if plan == LoanRepayPrincipalOnly {
				return ErrLoanInvalidRepayPlan // 永远拒绝（含同 plan 重试）
			}
			if f.RepayPlan == plan {
				return nil // 幂等：已是目标 plan
			}
			cur, ok := repayPlanOrder[f.RepayPlan]
			if !ok || cur >= repayPlanOrder[plan] {
				return ErrLoanInvalidRepayPlan // 升档（或当前档未知）拒绝
			}
		} else if f.RepayPlan == plan {
			return nil // platform 幂等
		}
		return setFundingRepayPlanTx(tx, &f, plan, now)
	})
}

// setFundingRepayPlanTx 改档共用事务实现（settle-first，spec §8）：调用方已完成归属/
// 权限/状态/幂等校验，这里按序执行：
//  1. 读/锁借款人账户（有 funding 必有账户，缺失防御性创建，镜像 writeoffFundingTx），
//     并以 acc 惰性结算到当天——先按旧 plan 结算，改档时点之前的利息/罚息不回溯
//     （已结算利息保留）；platform funding 用账户有效利率与宽限（与 settleFunding
//     全库语义一致），P2P 恒用自身利率（acc 仅 platform 分支消费）；
//  2. principal_only：一次性核销未付利息，debt_quota := principal_remaining（此后冻结）；
//  3. 写新 plan 落盘 funding；
//  4. loadUserFundingsTx 汇总全部 active/overdue fundings 回写账户投影
//     （syncAccountFromFundings），核销/改档立即反映到账户行。
func setFundingRepayPlanTx(tx *gorm.DB, f *TokenLoanFunding, plan string, now time.Time) error {
	acc, err := getOrCreateLoanAccountTx(tx, f.LoanUserId)
	if err != nil {
		return err
	}
	settleFunding(f, acc, now) // settle-first：按旧 plan 结算到今天
	if plan == LoanRepayPrincipalOnly {
		f.DebtQuota = f.PrincipalRemaining // 一次性核销未付利息，此后冻结
	}
	f.RepayPlan = plan
	f.UpdatedAt = now.Unix()
	if err := tx.Save(f).Error; err != nil {
		return err
	}

	remaining, err := loadUserFundingsTx(tx, f.LoanUserId)
	if err != nil {
		return err
	}
	syncAccountFromFundings(acc, remaining)
	acc.UpdatedAt = now.Unix()
	if err := tx.Save(acc).Error; err != nil {
		return err
	}
	common.SysLog(fmt.Sprintf("loan repay plan: funding %d repay_plan -> %s", f.Id, plan))
	return nil
}

// ===== Task 15: 官方逾期处置（AI 审批员，spec §9） =====

// ResolvePlatformOverdueByOfficer AI 审批员处置平台官方逾期债权（spec §9，与放贷人
// ResolveOverdueFunding 镜像，仅平台债权走此路径）：
//   - 仅 SourceType == platform 的 funding 可处置（P2P 归放贷人路径），非平台债权
//     返回 ErrLoanNotFundingOwner；
//   - 仅 status == overdue 可处置；非 overdue 视为幂等 no-op（并发处置抢先或已处置）
//     ——官方处置是异步后台流程，重复派发不得产生副作用，故不报错；
//   - extend：extendDays 钳制到 [1, max(LoanTermDays, 1)]（期限配置 0/负值时至少延
//     一天，镜像迁移防呆），DueDay = loanDay(now) + 钳制后天数，status → active；
//     PenaltyStartedDay 保留（历史记录，与 ResolveOverdueFunding 同语义）；
//   - writeoff：见 writeoffFundingTx（冻结最终债务 + funding 终态 + offer 核减 +
//     借款人拉黑扣分 + 投影销毁）；
//   - perpetual：funding 状态不变（保持 overdue 继续计息），仅记录决策日志。
func ResolvePlatformOverdueByOfficer(fundingId int64, action string, extendDays int) error {
	loanSetting := operation_setting.GetLoanSetting()
	now := time.Now()
	return DB.Transaction(func(tx *gorm.DB) error {
		var f TokenLoanFunding
		if err := lockForUpdate(tx).Where("id = ?", fundingId).First(&f).Error; err != nil {
			return err // gorm.ErrRecordNotFound 由调用方日志记录
		}
		if f.SourceType != LoanFundingPlatform {
			return ErrLoanNotFundingOwner // 官方处置仅限平台债权
		}
		if f.Status != LoanFundingOverdue {
			return nil // 幂等：已处置（并发抢先）或非逾期，no-op
		}
		switch action {
		case LoanDefaultActionExtend:
			term := loanSetting.LoanTermDays
			if term < 1 {
				term = 1 // 防御：期限配置 0/负值时至少延一天
			}
			days := extendDays
			if days < 1 {
				days = 1
			}
			if days > term {
				days = term
			}
			f.DueDay = loanDay(now) + days
			f.Status = LoanFundingActive
			f.UpdatedAt = now.Unix()
			return tx.Save(&f).Error
		case LoanDefaultActionWriteoff:
			return writeoffFundingTx(tx, &f, now)
		case LoanDefaultActionPerpetual:
			common.SysLog(fmt.Sprintf("loan default decision: platform funding %d marked perpetual (stays overdue, keeps accruing)", f.Id))
			return nil
		default:
			return ErrLoanInvalidDefaultAction
		}
	})
}

// platformOverdueDispatcher 官方逾期处置实现（service 层进程启动时接线一次）。
// model 包不能反向依赖 service，用可注入接缝镜像 controller 接线
// service.RegisterLoanOfficerModelCaller 的模式。
var platformOverdueDispatcher func(fundingId int64)

// RegisterPlatformOverdueDispatcher 接线官方逾期处置实现（service 包 init 调用）
func RegisterPlatformOverdueDispatcher(f func(fundingId int64)) {
	platformOverdueDispatcher = f
}

// dispatchPlatformOverdueAsync 逾期翻转事务提交后调用：把本次新翻转的 platform funding
// 逐条异步派发官方处置（DisposePlatformOverdueFunding 自身幂等，重复派发安全）。
// 空列表 / 全 P2P / 未接线时 no-op。goroutine 带 panic 兜底，处置失败不影响主流程。
func dispatchPlatformOverdueAsync(flipped []TokenLoanFunding) {
	if platformOverdueDispatcher == nil {
		return
	}
	for i := range flipped {
		f := &flipped[i]
		if f.SourceType != LoanFundingPlatform {
			continue
		}
		fundingId := f.Id
		go func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysError(fmt.Sprintf("platform overdue disposal panicked for funding %d: %v", fundingId, r))
				}
			}()
			platformOverdueDispatcher(fundingId)
		}()
	}
}
