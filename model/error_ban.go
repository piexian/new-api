package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ErrorBanIPState 记录某 IP 针对某条规则的违规与窗口统计。
type ErrorBanIPState struct {
	Id            int    `json:"id"`
	TargetIP      string `json:"target_ip" gorm:"type:varchar(64);uniqueIndex:idx_error_ban_ip"`
	RuleId        string `json:"rule_id" gorm:"type:varchar(64);uniqueIndex:idx_error_ban_ip"`
	OffenseCount  int    `json:"offense_count"`
	WindowCount   int    `json:"window_count"`
	WindowStart   int64  `json:"window_start"`
	LastError     string `json:"last_error" gorm:"type:text"`
	LastOffenseAt int64  `json:"last_offense_at" gorm:"index"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// ErrorBanUserState 记录某用户针对某条规则的违规与窗口统计。
type ErrorBanUserState struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id" gorm:"uniqueIndex:idx_error_ban_user"`
	RuleId        string `json:"rule_id" gorm:"type:varchar(64);uniqueIndex:idx_error_ban_user"`
	OffenseCount  int    `json:"offense_count"`
	WindowCount   int    `json:"window_count"`
	WindowStart   int64  `json:"window_start"`
	LastError     string `json:"last_error" gorm:"type:text"`
	LastOffenseAt int64  `json:"last_offense_at" gorm:"index"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// IncrementErrorBanIPState 原子地对某 IP+规则的违规计数加一，并记录窗口快照。
func IncrementErrorBanIPState(targetIP, ruleId string, windowCount int, windowStart int64, lastError string) (*ErrorBanIPState, error) {
	now := common.GetTimestamp()
	lastError = truncateRunes(lastError, 2048)

	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ErrorBanIPState{}).
			Where("target_ip = ? AND rule_id = ?", targetIP, ruleId).
			Updates(map[string]interface{}{
				"offense_count":   gorm.Expr("offense_count + ?", 1),
				"window_count":    windowCount,
				"window_start":    windowStart,
				"last_error":      lastError,
				"last_offense_at": now,
				"updated_at":      now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}
		created := &ErrorBanIPState{
			TargetIP:      targetIP,
			RuleId:        ruleId,
			OffenseCount:  1,
			WindowCount:   windowCount,
			WindowStart:   windowStart,
			LastError:     lastError,
			LastOffenseAt: now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(created).Error; err != nil {
			retry := tx.Model(&ErrorBanIPState{}).
				Where("target_ip = ? AND rule_id = ?", targetIP, ruleId).
				Update("offense_count", gorm.Expr("offense_count + ?", 1))
			return retry.Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	state := &ErrorBanIPState{}
	if err := DB.Where("target_ip = ? AND rule_id = ?", targetIP, ruleId).First(state).Error; err != nil {
		return nil, err
	}
	return state, nil
}

// IncrementErrorBanUserState 原子地对某用户+规则的违规计数加一，并记录窗口快照。
func IncrementErrorBanUserState(userId int, ruleId string, windowCount int, windowStart int64, lastError string) (*ErrorBanUserState, error) {
	now := common.GetTimestamp()
	lastError = truncateRunes(lastError, 2048)

	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ErrorBanUserState{}).
			Where("user_id = ? AND rule_id = ?", userId, ruleId).
			Updates(map[string]interface{}{
				"offense_count":   gorm.Expr("offense_count + ?", 1),
				"window_count":    windowCount,
				"window_start":    windowStart,
				"last_error":      lastError,
				"last_offense_at": now,
				"updated_at":      now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}
		created := &ErrorBanUserState{
			UserId:        userId,
			RuleId:        ruleId,
			OffenseCount:  1,
			WindowCount:   windowCount,
			WindowStart:   windowStart,
			LastError:     lastError,
			LastOffenseAt: now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(created).Error; err != nil {
			retry := tx.Model(&ErrorBanUserState{}).
				Where("user_id = ? AND rule_id = ?", userId, ruleId).
				Update("offense_count", gorm.Expr("offense_count + ?", 1))
			return retry.Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	state := &ErrorBanUserState{}
	if err := DB.Where("user_id = ? AND rule_id = ?", userId, ruleId).First(state).Error; err != nil {
		return nil, err
	}
	return state, nil
}

// ListErrorBanIPStates 分页查询 IP 错误封禁状态，可按 IP 或规则关键字过滤。
func ListErrorBanIPStates(keyword string, startIdx, num int) ([]*ErrorBanIPState, int64, error) {
	var states []*ErrorBanIPState
	var total int64
	tx := DB.Model(&ErrorBanIPState{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		tx = tx.Where("target_ip LIKE ? OR rule_id LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("last_offense_at DESC").Limit(num).Offset(startIdx).Find(&states).Error
	return states, total, err
}

// ListErrorBanUserStates 分页查询用户错误封禁状态。
func ListErrorBanUserStates(keyword string, startIdx, num int) ([]*ErrorBanUserState, int64, error) {
	var states []*ErrorBanUserState
	var total int64
	tx := DB.Model(&ErrorBanUserState{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		tx = tx.Where("rule_id LIKE ?", "%"+keyword+"%")
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("last_offense_at DESC").Limit(num).Offset(startIdx).Find(&states).Error
	return states, total, err
}

// ResetErrorBanIPStateById 删除指定 ID 的 IP 错误封禁状态。
func ResetErrorBanIPStateById(id int) error {
	return DB.Where("id = ?", id).Delete(&ErrorBanIPState{}).Error
}

// ResetErrorBanUserStateById 删除指定 ID 的用户错误封禁状态。
func ResetErrorBanUserStateById(id int) error {
	return DB.Where("id = ?", id).Delete(&ErrorBanUserState{}).Error
}

// ErrorBanStatsResult 错误封禁统计数据。
type ErrorBanStatsResult struct {
	TotalIPStates   int64 `json:"total_ip_states"`
	TotalUserStates int64 `json:"total_user_states"`
	TotalOffenses   int64 `json:"total_offenses"`
	ActiveRules     int64 `json:"active_rules"`
}

// GetErrorBanStats 汇总错误封禁统计信息。activeRules 由调用方根据配置填充。
func GetErrorBanStats() (*ErrorBanStatsResult, error) {
	stats := &ErrorBanStatsResult{}
	if err := DB.Model(&ErrorBanIPState{}).Count(&stats.TotalIPStates).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&ErrorBanUserState{}).Count(&stats.TotalUserStates).Error; err != nil {
		return nil, err
	}
	var ipSum, userSum struct{ Total int64 }
	if err := DB.Model(&ErrorBanIPState{}).Select("COALESCE(SUM(offense_count),0) AS total").Scan(&ipSum).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&ErrorBanUserState{}).Select("COALESCE(SUM(offense_count),0) AS total").Scan(&userSum).Error; err != nil {
		return nil, err
	}
	stats.TotalOffenses = ipSum.Total + userSum.Total
	return stats, nil
}

// ResetErrorBanIPStatesByIP 删除某 IP 的所有错误封禁状态（跨规则）。
func ResetErrorBanIPStatesByIP(targetIP string) error {
	return DB.Where("target_ip = ?", targetIP).Delete(&ErrorBanIPState{}).Error
}

// ResetErrorBanUserStatesByUser 删除某用户的所有错误封禁状态（跨规则）。
func ResetErrorBanUserStatesByUser(userId int) error {
	return DB.Where("user_id = ?", userId).Delete(&ErrorBanUserState{}).Error
}
