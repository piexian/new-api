package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// 封禁日志维度与来源常量。
const (
	RiskBanDimensionIP   = "ip"
	RiskBanDimensionUser = "user"

	RiskBanSourceProbeGuard   = "probe_guard"
	RiskBanSourceErrorBan     = "error_ban"
	RiskBanSourceIPMiddleware = "ip_middleware"
	RiskBanSourceManual       = "manual"
)

// RiskBanLog 统一记录风控中心产生的所有封禁/处罚事件，供审计与解封查询。
type RiskBanLog struct {
	Id              int    `json:"id"`
	Dimension       string `json:"dimension" gorm:"type:varchar(16);index"`
	TargetIP        string `json:"target_ip" gorm:"type:varchar(64);index"`
	UserId          int    `json:"user_id" gorm:"index"`
	Username        string `json:"username" gorm:"type:varchar(128)"`
	Source          string `json:"source" gorm:"type:varchar(32);index"`
	RuleId          string `json:"rule_id" gorm:"type:varchar(64)"`
	RuleName        string `json:"rule_name" gorm:"type:varchar(128)"`
	Action          string `json:"action" gorm:"type:varchar(32)"`
	DurationMinutes int    `json:"duration_minutes"`
	IsPermanent     bool   `json:"is_permanent"`
	UnbanAt         int64  `json:"unban_at"`
	OffenseCount    int    `json:"offense_count"`
	Reason          string `json:"reason" gorm:"type:text"`
	ErrorSample     string `json:"error_sample" gorm:"type:text"`
	Models          string `json:"models" gorm:"type:text"`
	OperatorId      int    `json:"operator_id"`
	DryRun          bool   `json:"dry_run"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
}

// CreateRiskBanLog 写入一条封禁日志。
func CreateRiskBanLog(log *RiskBanLog) error {
	if log.CreatedAt == 0 {
		log.CreatedAt = common.GetTimestamp()
	}
	return DB.Create(log).Error
}

// GetRiskBanLogById 按 ID 查询单条封禁日志。
func GetRiskBanLogById(id int) (*RiskBanLog, error) {
	var log RiskBanLog
	err := DB.Where("id = ?", id).First(&log).Error
	return &log, err
}

// RiskBanLogFilter 封禁日志查询过滤条件。
type RiskBanLogFilter struct {
	Dimension string
	Source    string
	Keyword   string
	DryRun    *bool
	StartAt   int64
	EndAt     int64
}

// GetRiskBanLogs 分页查询封禁日志，支持多维度过滤。
func GetRiskBanLogs(startIdx, num int, filter RiskBanLogFilter) ([]*RiskBanLog, int64, error) {
	var logs []*RiskBanLog
	var total int64
	tx := DB.Model(&RiskBanLog{})
	if filter.Dimension != "" {
		tx = tx.Where("dimension = ?", filter.Dimension)
	}
	if filter.Source != "" {
		tx = tx.Where("source = ?", filter.Source)
	}
	if filter.DryRun != nil {
		tx = tx.Where("dry_run = ?", *filter.DryRun)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("target_ip LIKE ? OR username LIKE ? OR reason LIKE ? OR rule_id LIKE ?", like, like, like, like)
	}
	if filter.StartAt > 0 {
		tx = tx.Where("created_at >= ?", filter.StartAt)
	}
	if filter.EndAt > 0 {
		tx = tx.Where("created_at <= ?", filter.EndAt)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("created_at DESC").Limit(num).Offset(startIdx).Find(&logs).Error
	return logs, total, err
}

// RiskBanLogStatsResult 封禁日志统计数据。
type RiskBanLogStatsResult struct {
	Total       int64            `json:"total"`
	DryRunCount int64            `json:"dry_run_count"`
	Permanent   int64            `json:"permanent"`
	Today       int64            `json:"today"`
	ByDimension map[string]int64 `json:"by_dimension"`
	BySource    map[string]int64 `json:"by_source"`
}

type groupCount struct {
	Key   string
	Total int64
}

// GetRiskBanLogStats 汇总封禁日志统计信息。
func GetRiskBanLogStats() (*RiskBanLogStatsResult, error) {
	stats := &RiskBanLogStatsResult{
		ByDimension: make(map[string]int64),
		BySource:    make(map[string]int64),
	}
	if err := DB.Model(&RiskBanLog{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&RiskBanLog{}).Where("dry_run = ?", true).Count(&stats.DryRunCount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&RiskBanLog{}).Where("is_permanent = ?", true).Count(&stats.Permanent).Error; err != nil {
		return nil, err
	}
	todayStart := common.GetTimestamp() - 86400
	if err := DB.Model(&RiskBanLog{}).Where("created_at >= ?", todayStart).Count(&stats.Today).Error; err != nil {
		return nil, err
	}
	var dimensionRows []groupCount
	if err := DB.Model(&RiskBanLog{}).
		Select("dimension AS key, COUNT(*) AS total").
		Group("dimension").Scan(&dimensionRows).Error; err != nil {
		return nil, err
	}
	for _, row := range dimensionRows {
		stats.ByDimension[row.Key] = row.Total
	}
	var sourceRows []groupCount
	if err := DB.Model(&RiskBanLog{}).
		Select("source AS key, COUNT(*) AS total").
		Group("source").Scan(&sourceRows).Error; err != nil {
		return nil, err
	}
	for _, row := range sourceRows {
		stats.BySource[row.Key] = row.Total
	}
	return stats, nil
}

// DeleteRiskBanLogsBefore 删除指定时间之前的封禁日志，用于数据保留清理。
func DeleteRiskBanLogsBefore(before int64) (int64, error) {
	result := DB.Where("created_at < ?", before).Delete(&RiskBanLog{})
	return result.RowsAffected, result.Error
}

// HasRiskBanLogForUser 判断是否存在某用户的风控封禁记录，用于解封校验。
func HasRiskBanLogForUser(userId int) (bool, error) {
	var count int64
	err := DB.Model(&RiskBanLog{}).
		Where("user_id = ? AND dimension = ?", userId, RiskBanDimensionUser).
		Count(&count).Error
	return count > 0, err
}
