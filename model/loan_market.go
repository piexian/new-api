package model

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

// funding 还款计划
const (
	LoanRepayFull           = "full"            // 到期一次性还本息
	LoanRepayNoPenalty      = "no_penalty"      // 到期还本，宽限期内免息
	LoanRepayInterestFreeze = "interest_freeze" // 到期未还进入宽限期，利息冻结
	LoanRepayPrincipalOnly  = "principal_only"  // 每次仅还本金，利息另行结算
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
