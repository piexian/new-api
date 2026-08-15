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
