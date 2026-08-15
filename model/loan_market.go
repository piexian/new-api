package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
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
	MinCreditScore      int     `json:"min_credit_score"`                              // 最低可借信用分，0 = 不限
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
