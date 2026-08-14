package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// AI 工单哨兵错误，controller 层映射为 i18n 响应
var (
	ErrLoanApplicationLimit = errors.New("loan application limit exceeded")
	ErrLoanAlreadyRated     = errors.New("loan application already rated or not closed")
	ErrLoanInvalidRating    = errors.New("loan application rating must be between 1 and 5")
)

// AI 工单状态
const (
	LoanAppStatusOpen   = "open"
	LoanAppStatusClosed = "closed"
)

// TokenLoanApplication AI 业务员工单（一次提额/降息/宽限申请）
type TokenLoanApplication struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId        int    `json:"user_id" gorm:"not null;index"`
	Topic         string `json:"topic" gorm:"type:varchar(256);not null"`
	Status        string `json:"status" gorm:"type:varchar(16);not null;index"` // open / closed
	ModelUsed     string `json:"model_used" gorm:"type:varchar(128)"`           // 实际对话使用的模型
	Decision      string `json:"decision" gorm:"type:text"`                     // AI 最终审批结论（JSON）
	Rating        int    `json:"rating"`                                        // 用户评分 1-5，0 = 未评
	RatingComment string `json:"rating_comment" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"` // 秒级时间戳
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint"` // 秒级时间戳
}

func (TokenLoanApplication) TableName() string {
	return "token_loan_applications"
}

// TokenLoanApplicationMessage AI 工单对话消息
type TokenLoanApplicationMessage struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ApplicationId int    `json:"application_id" gorm:"not null;index"`
	Role          string `json:"role" gorm:"type:varchar(16);not null"` // user / assistant
	Content       string `json:"content" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"` // 秒级时间戳
}

func (TokenLoanApplicationMessage) TableName() string {
	return "token_loan_application_messages"
}

// CreateLoanApplication 新建工单。并发安全的数量限制：事务内先 Create 再 Count，
// open 工单数超 AiMaxActiveApplications 或当日新建数（服务器本地日）超 AiDailyLimit
// 则回滚并返回 ErrLoanApplicationLimit。配置值 <= 0 表示不限制。
func CreateLoanApplication(userId int, topic, modelUsed string) (*TokenLoanApplication, error) {
	setting := operation_setting.GetLoanSetting()
	now := time.Now()
	app := &TokenLoanApplication{
		UserId:    userId,
		Topic:     topic,
		Status:    LoanAppStatusOpen,
		ModelUsed: modelUsed,
		CreatedAt: now.Unix(),
		UpdatedAt: now.Unix(),
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(app).Error; err != nil {
			return err
		}
		if setting.AiMaxActiveApplications > 0 {
			var openCount int64
			if err := tx.Model(&TokenLoanApplication{}).
				Where("user_id = ? AND status = ?", userId, LoanAppStatusOpen).
				Count(&openCount).Error; err != nil {
				return err
			}
			if openCount > int64(setting.AiMaxActiveApplications) {
				return ErrLoanApplicationLimit
			}
		}
		if setting.AiDailyLimit > 0 {
			// 当日新建数按服务器本地日 00:00 起算，与 loanDay 本地日基准一致；
			// 已关闭的工单同样计入当日额度
			y, m, d := now.In(time.Local).Date()
			dayStart := time.Date(y, m, d, 0, 0, 0, 0, time.Local).Unix()
			var todayCount int64
			if err := tx.Model(&TokenLoanApplication{}).
				Where("user_id = ? AND created_at >= ?", userId, dayStart).
				Count(&todayCount).Error; err != nil {
				return err
			}
			if todayCount > int64(setting.AiDailyLimit) {
				return ErrLoanApplicationLimit
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return app, nil
}

// AddLoanApplicationMessage 追加一条工单对话消息
func AddLoanApplicationMessage(appId int, role, content string) error {
	msg := &TokenLoanApplicationMessage{
		ApplicationId: appId,
		Role:          role,
		Content:       content,
		CreatedAt:     time.Now().Unix(),
	}
	return DB.Create(msg).Error
}

// GetLoanApplicationMessages 按 id 升序返回工单的全部对话消息
func GetLoanApplicationMessages(appId int) ([]TokenLoanApplicationMessage, error) {
	var msgs []TokenLoanApplicationMessage
	err := DB.Where("application_id = ?", appId).Order("id ASC").Find(&msgs).Error
	return msgs, err
}

// GetUserLoanApplications 分页返回用户工单，id 倒序（最新在前），page 从 1 开始
func GetUserLoanApplications(userId, page, pageSize int) ([]TokenLoanApplication, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var apps []TokenLoanApplication
	err := DB.Where("user_id = ?", userId).
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&apps).Error
	return apps, err
}

// RateLoanApplication 用户评分已关闭的工单。先 First 校验归属（不存在或非本人透出
// gorm.ErrRecordNotFound），再条件更新防并发重复评分：仅当 status=closed 且未评分
// （rating=0）时生效，否则返回 ErrLoanAlreadyRated。
func RateLoanApplication(userId, appId int, rating int, comment string) error {
	if rating < 1 || rating > 5 {
		return ErrLoanInvalidRating
	}
	var app TokenLoanApplication
	if err := DB.Select("id").Where("id = ? AND user_id = ?", appId, userId).First(&app).Error; err != nil {
		return err
	}
	res := DB.Model(&TokenLoanApplication{}).
		Where("id = ? AND user_id = ? AND status = ? AND rating = 0", appId, userId, LoanAppStatusClosed).
		Updates(map[string]interface{}{
			"rating":         rating,
			"rating_comment": comment,
			"updated_at":     time.Now().Unix(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrLoanAlreadyRated
	}
	return nil
}
