package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	EmailLogStatusSuccess    = "success"
	EmailLogStatusFailed     = "failed"
	EmailLogStatusSuppressed = "suppressed"
)

type EmailLog struct {
	Id           int    `json:"id" gorm:"index:idx_email_logs_created_id,priority:1"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index:idx_email_logs_created_id,priority:2"`
	Provider     string `json:"provider" gorm:"type:varchar(32);index;default:''"`
	Receiver     string `json:"receiver" gorm:"type:varchar(191);index;default:''"`
	Subject      string `json:"subject" gorm:"type:varchar(255);index;default:''"`
	Status       string `json:"status" gorm:"type:varchar(32);index;default:''"`
	ErrorMessage string `json:"error_message" gorm:"type:varchar(500);default:''"`
	DurationMs   int64  `json:"duration_ms" gorm:"default:0"`
}

type EmailLogQueryParams struct {
	StartTimestamp int64
	EndTimestamp   int64
	Receiver       string
	Subject        string
	Status         string
	Provider       string
}

func RecordEmailLog(provider, receiver, subject, status string, durationMs int64, err error) {
	if LOG_DB == nil {
		return
	}
	log := &EmailLog{
		CreatedAt:    common.GetTimestamp(),
		Provider:     strings.TrimSpace(provider),
		Receiver:     truncateString(strings.TrimSpace(receiver), 191),
		Subject:      truncateString(strings.TrimSpace(subject), 255),
		Status:       strings.TrimSpace(status),
		DurationMs:   durationMs,
		ErrorMessage: "",
	}
	if err != nil {
		log.ErrorMessage = truncateString(err.Error(), 500)
	}
	if createErr := LOG_DB.Create(log).Error; createErr != nil {
		common.SysLog("failed to record email log: " + createErr.Error())
	}
}

func GetAllEmailLogs(startIdx int, num int, params EmailLogQueryParams) (logs []*EmailLog, total int64, err error) {
	tx := LOG_DB.Model(&EmailLog{})
	tx = applyEmailLogFilters(tx, params)
	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	return logs, total, err
}

func applyEmailLogFilters(tx *gorm.DB, params EmailLogQueryParams) *gorm.DB {
	if params.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", params.EndTimestamp)
	}
	if params.Receiver != "" {
		tx = tx.Where("receiver LIKE ?", "%"+params.Receiver+"%")
	}
	if params.Subject != "" {
		tx = tx.Where("subject LIKE ?", "%"+params.Subject+"%")
	}
	if params.Status != "" {
		tx = tx.Where("status = ?", params.Status)
	}
	if params.Provider != "" {
		tx = tx.Where("provider = ?", params.Provider)
	}
	return tx
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if maxLen <= 0 || len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
