package model

import (
	"strconv"
	"time"

	"gorm.io/gorm"
)

// AdminLoanAccountItem 管理端账户视图：账户行 + 用户名 + 当前时刻投影债务
type AdminLoanAccountItem struct {
	TokenLoanAccount
	Username    string `json:"username"`
	DebtNow     int64  `json:"debt_now"`
	InterestNow int64  `json:"interest_now"`
}

// AdminGetLoanAccounts 分页返回全部贷款账户（updated_at 倒序），附用户名与实时投影债务。
// keyword 为纯数字时同时匹配 user_id，否则按用户名模糊匹配；为空不过滤。
func AdminGetLoanAccounts(page, pageSize int, keyword string) ([]AdminLoanAccountItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	base := DB.Table("token_loan_accounts").
		Joins("LEFT JOIN users ON users.id = token_loan_accounts.user_id")
	if keyword != "" {
		if uid, err := strconv.Atoi(keyword); err == nil {
			base = base.Where("token_loan_accounts.user_id = ? OR users.username LIKE ?", uid, "%"+keyword+"%")
		} else {
			base = base.Where("users.username LIKE ?", "%"+keyword+"%")
		}
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []AdminLoanAccountItem
	err := base.Select("token_loan_accounts.*, users.username AS username").
		Order("token_loan_accounts.updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	// 只读投影当前时刻债务，不落盘
	now := time.Now()
	for i := range items {
		items[i].DebtNow, items[i].InterestNow = ProjectLoanStatus(&items[i].TokenLoanAccount, now)
	}
	return items, total, nil
}

// AdminLoanRecordItem 台账 + 用户名（管理端跨用户查询用）
type AdminLoanRecordItem struct {
	TokenLoanRecord
	Username string `json:"username"`
}

// AdminGetLoanRecords 分页返回台账（id 倒序）；userId > 0 时按用户过滤
func AdminGetLoanRecords(userId, page, pageSize int) ([]AdminLoanRecordItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	base := DB.Table("token_loan_records").
		Joins("LEFT JOIN users ON users.id = token_loan_records.user_id")
	if userId > 0 {
		base = base.Where("token_loan_records.user_id = ?", userId)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []AdminLoanRecordItem
	err := base.Select("token_loan_records.*, users.username AS username").
		Order("token_loan_records.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	return items, total, err
}

// AdminLoanApplicationItem 工单 + 用户名（管理端跨用户查询用）
type AdminLoanApplicationItem struct {
	TokenLoanApplication
	Username string `json:"username"`
}

// AdminGetLoanApplications 分页返回工单（id 倒序）；userId > 0 按用户过滤，
// status 仅放行 open/closed 两个合法值，其余忽略
func AdminGetLoanApplications(userId int, status string, page, pageSize int) ([]AdminLoanApplicationItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	base := DB.Table("token_loan_applications").
		Joins("LEFT JOIN users ON users.id = token_loan_applications.user_id")
	if userId > 0 {
		base = base.Where("token_loan_applications.user_id = ?", userId)
	}
	if status == LoanAppStatusOpen || status == LoanAppStatusClosed {
		base = base.Where("token_loan_applications.status = ?", status)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []AdminLoanApplicationItem
	err := base.Select("token_loan_applications.*, users.username AS username").
		Order("token_loan_applications.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	return items, total, err
}

// AdminLoanOfferItem 放贷挂单 + 放贷人用户名（管理端跨用户查询用）
type AdminLoanOfferItem struct {
	TokenLoanOffer
	Username string `json:"username"`
}

// AdminGetLoanOffers 分页返回全部放贷挂单（id 倒序），附放贷人用户名。
// keyword 为纯数字时按放贷人 user id 精确匹配，否则按用户名模糊匹配；为空不过滤。
func AdminGetLoanOffers(page, pageSize int, keyword string) ([]AdminLoanOfferItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	base := DB.Table("token_loan_offers").
		Joins("LEFT JOIN users ON users.id = token_loan_offers.lender_id")
	if keyword != "" {
		if uid, err := strconv.Atoi(keyword); err == nil {
			base = base.Where("token_loan_offers.lender_id = ?", uid)
		} else {
			base = base.Where("users.username LIKE ?", "%"+keyword+"%")
		}
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []AdminLoanOfferItem
	err := base.Select("token_loan_offers.*, users.username AS username").
		Order("token_loan_offers.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	return items, total, err
}

// AdminLoanFundingItem 投放记录 + 放贷人/借款人用户名（管理端跨用户查询用）
type AdminLoanFundingItem struct {
	TokenLoanFunding
	LenderUsername   string `json:"lender_username"`
	BorrowerUsername string `json:"borrower_username"`
}

// loanFundingAdminStatuses 管理端可过滤的 funding 状态白名单（其余值忽略，不做状态过滤）
var loanFundingAdminStatuses = map[string]bool{
	LoanFundingActive:     true,
	LoanFundingOverdue:    true,
	LoanFundingRepaid:     true,
	LoanFundingWrittenOff: true,
}

// AdminGetLoanFundings 分页返回全部投放记录（id 倒序），附放贷人/借款人用户名；
// lenderId / loanUserId > 0 时按对应用户过滤，status 仅放行白名单值，其余忽略。
func AdminGetLoanFundings(lenderId, loanUserId int, status string, page, pageSize int) ([]AdminLoanFundingItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	base := DB.Table("token_loan_fundings").
		Joins("LEFT JOIN users AS lender ON lender.id = token_loan_fundings.lender_id").
		Joins("LEFT JOIN users AS borrower ON borrower.id = token_loan_fundings.loan_user_id")
	if lenderId > 0 {
		base = base.Where("token_loan_fundings.lender_id = ?", lenderId)
	}
	if loanUserId > 0 {
		base = base.Where("token_loan_fundings.loan_user_id = ?", loanUserId)
	}
	if loanFundingAdminStatuses[status] {
		base = base.Where("token_loan_fundings.status = ?", status)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []AdminLoanFundingItem
	err := base.Select("token_loan_fundings.*, lender.username AS lender_username, borrower.username AS borrower_username").
		Order("token_loan_fundings.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	return items, total, err
}

// LoanMarketOverview 借贷市场总览（管理端只读聚合）
type LoanMarketOverview struct {
	OffersByStatus      map[string]int64 `json:"offers_by_status"`      // 各状态挂单数（active/paused/closed）
	FrozenIdle          int64            `json:"frozen_idle"`           // Σ amount_available：放贷人挂出但尚未投放的冻结闲置额度
	InLoanPrincipal     int64            `json:"in_loan_principal"`     // Σ active/overdue funding 本金：市场资金在贷规模
	TotalInterestEarned int64            `json:"total_interest_earned"` // Σ offer.total_interest_earned：放贷人累计利息收入
	OverdueFundings     int64            `json:"overdue_fundings"`      // 逾期 funding 笔数
	ActiveOffers        int64            `json:"active_offers"`         // active 状态挂单数（在售）
}

// AdminLoanMarketOverview 汇总借贷市场总览（空表返回全零；COALESCE 兜底跨 SQLite/MySQL/PG）
func AdminLoanMarketOverview() (LoanMarketOverview, error) {
	var overview LoanMarketOverview

	// 各状态挂单数
	type offerStatusCount struct {
		Status string
		Count  int64
	}
	var statusRows []offerStatusCount
	if err := DB.Table("token_loan_offers").Select("status, COUNT(*) AS count").Group("status").Scan(&statusRows).Error; err != nil {
		return overview, err
	}
	overview.OffersByStatus = make(map[string]int64, len(statusRows))
	for _, r := range statusRows {
		overview.OffersByStatus[r.Status] = r.Count
	}

	// 金额聚合：offer 侧闲置/利息，funding 侧在贷本金
	var offerSum struct{ FrozenIdle, TotalInterestEarned int64 }
	if err := DB.Table("token_loan_offers").
		Select("COALESCE(SUM(amount_available),0) AS frozen_idle, COALESCE(SUM(total_interest_earned),0) AS total_interest_earned").
		Scan(&offerSum).Error; err != nil {
		return overview, err
	}
	overview.FrozenIdle = offerSum.FrozenIdle
	overview.TotalInterestEarned = offerSum.TotalInterestEarned

	var inLoan struct{ Total int64 }
	if err := DB.Model(&TokenLoanFunding{}).
		Select("COALESCE(SUM(principal_remaining),0) AS total").
		Where("status IN ?", []string{LoanFundingActive, LoanFundingOverdue}).
		Scan(&inLoan).Error; err != nil {
		return overview, err
	}
	overview.InLoanPrincipal = inLoan.Total

	if err := DB.Model(&TokenLoanFunding{}).Where("status = ?", LoanFundingOverdue).Count(&overview.OverdueFundings).Error; err != nil {
		return overview, err
	}
	if err := DB.Model(&TokenLoanOffer{}).Where("status = ?", LoanOfferStatusActive).Count(&overview.ActiveOffers).Error; err != nil {
		return overview, err
	}
	return overview, nil
}
