package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ProbeIPAbuseState 记录某 IP 的批量模型探测违规次数与处罚阶梯。
type ProbeIPAbuseState struct {
	Id            int    `json:"id"`
	TargetIP      string `json:"target_ip" gorm:"type:varchar(64);uniqueIndex"`
	LastUserId    int    `json:"last_user_id" gorm:"index"`
	OffenseCount  int    `json:"offense_count"`
	LastOffenseAt int64  `json:"last_offense_at" gorm:"index"`
	LastModels    string `json:"last_models" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// ProbeUserAbuseState 记录某用户的批量模型探测违规次数。
type ProbeUserAbuseState struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id" gorm:"uniqueIndex"`
	OffenseCount  int    `json:"offense_count"`
	LastOffenseAt int64  `json:"last_offense_at" gorm:"index"`
	LastIP        string `json:"last_ip" gorm:"type:varchar(64)"`
	LastModels    string `json:"last_models" gorm:"type:text"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// joinModels 将模型列表拼接为逗号分隔字符串，用于审计展示。
func joinModels(models []string) string {
	if len(models) == 0 {
		return ""
	}
	return strings.Join(models, ",")
}

// truncateRunes 将字符串按 rune 截断到 n 个字符，避免超出 varchar 列宽。
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// IncrementProbeIPAbuseOffense 原子地对某 IP 的违规计数加一，不存在则创建。
// 通过 UPDATE ... offense_count + 1 保证并发安全，唯一索引冲突时回退为自增。
func IncrementProbeIPAbuseOffense(targetIP string, lastUserId int, models []string) (*ProbeIPAbuseState, error) {
	now := common.GetTimestamp()
	modelsStr := joinModels(models)

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := incrementProbeIP(tx, targetIP, lastUserId, modelsStr, now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	state := &ProbeIPAbuseState{}
	if err := DB.Where("target_ip = ?", targetIP).First(state).Error; err != nil {
		return nil, err
	}
	return state, nil
}

func incrementProbeIP(tx *gorm.DB, targetIP string, lastUserId int, modelsStr string, now int64) error {
	result := tx.Model(&ProbeIPAbuseState{}).
		Where("target_ip = ?", targetIP).
		Updates(map[string]interface{}{
			"offense_count":   gorm.Expr("offense_count + ?", 1),
			"last_user_id":    lastUserId,
			"last_offense_at": now,
			"last_models":     modelsStr,
			"updated_at":      now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	created := &ProbeIPAbuseState{
		TargetIP:      targetIP,
		LastUserId:    lastUserId,
		OffenseCount:  1,
		LastOffenseAt: now,
		LastModels:    modelsStr,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := tx.Create(created).Error; err != nil {
		// 并发创建冲突（唯一索引），回退为自增。
		retry := tx.Model(&ProbeIPAbuseState{}).
			Where("target_ip = ?", targetIP).
			Update("offense_count", gorm.Expr("offense_count + ?", 1))
		return retry.Error
	}
	return nil
}

// IncrementProbeUserAbuseOffense 原子地对某用户的违规计数加一，不存在则创建。
func IncrementProbeUserAbuseOffense(userId int, lastIP string, models []string) (*ProbeUserAbuseState, error) {
	now := common.GetTimestamp()
	modelsStr := joinModels(models)

	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ProbeUserAbuseState{}).
			Where("user_id = ?", userId).
			Updates(map[string]interface{}{
				"offense_count":   gorm.Expr("offense_count + ?", 1),
				"last_ip":         lastIP,
				"last_offense_at": now,
				"last_models":     modelsStr,
				"updated_at":      now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}
		created := &ProbeUserAbuseState{
			UserId:        userId,
			OffenseCount:  1,
			LastOffenseAt: now,
			LastIP:        lastIP,
			LastModels:    modelsStr,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(created).Error; err != nil {
			retry := tx.Model(&ProbeUserAbuseState{}).
				Where("user_id = ?", userId).
				Update("offense_count", gorm.Expr("offense_count + ?", 1))
			return retry.Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	state := &ProbeUserAbuseState{}
	if err := DB.Where("user_id = ?", userId).First(state).Error; err != nil {
		return nil, err
	}
	return state, nil
}

// UpsertProbeGuardIPBan 创建或升级一条 IP 封禁记录，并刷新封禁缓存。
// 升级只升不降：已永久封禁保持永久，临时封禁取较晚的到期时间，新永久封禁覆盖临时封禁。
func UpsertProbeGuardIPBan(target, reason string, expiresAt int64) error {
	normalized, err := NormalizeIPBanTarget(target)
	if err != nil {
		return err
	}
	reason = truncateRunes(reason, 255)
	now := common.GetTimestamp()

	err = DB.Transaction(func(tx *gorm.DB) error {
		var ban IPBan
		findErr := tx.Where("target = ?", normalized).First(&ban).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return tx.Create(&IPBan{
				Target:      normalized,
				Reason:      reason,
				ExpiresAt:   expiresAt,
				AutoBanUser: true,
				CreatedAt:   now,
				UpdatedAt:   now,
			}).Error
		}
		if findErr != nil {
			return findErr
		}
		newExpires := expiresAt
		switch {
		case ban.ExpiresAt == 0:
			newExpires = 0 // 已永久封禁
		case expiresAt == 0:
			newExpires = 0 // 升级为永久封禁
		case expiresAt < ban.ExpiresAt:
			newExpires = ban.ExpiresAt // 保留更长的封禁
		}
		return tx.Model(&ban).Updates(map[string]interface{}{
			"reason":        reason,
			"expires_at":    newExpires,
			"auto_ban_user": true,
			"updated_at":    now,
		}).Error
	})
	if err != nil {
		return err
	}
	InitIPBanCache()
	return nil
}

// ListProbeIPAbuseStates 分页查询 IP 违规记录，可按 IP 关键字过滤。
func ListProbeIPAbuseStates(keyword string, startIdx, num int) ([]*ProbeIPAbuseState, int64, error) {
	var states []*ProbeIPAbuseState
	var total int64
	tx := DB.Model(&ProbeIPAbuseState{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		tx = tx.Where("target_ip LIKE ?", "%"+keyword+"%")
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("last_offense_at DESC").Limit(num).Offset(startIdx).Find(&states).Error
	return states, total, err
}

// ListProbeUserAbuseStates 分页查询用户违规记录。
func ListProbeUserAbuseStates(keyword string, startIdx, num int) ([]*ProbeUserAbuseState, int64, error) {
	var states []*ProbeUserAbuseState
	var total int64
	tx := DB.Model(&ProbeUserAbuseState{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		tx = tx.Where("last_ip LIKE ?", "%"+keyword+"%")
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("last_offense_at DESC").Limit(num).Offset(startIdx).Find(&states).Error
	return states, total, err
}

// ResetProbeIPAbuse 删除某 IP 的违规记录。
func ResetProbeIPAbuse(targetIP string) error {
	return DB.Where("target_ip = ?", targetIP).Delete(&ProbeIPAbuseState{}).Error
}

// ResetProbeUserAbuse 重置某用户的违规计数（保留记录，计数清零）。
func ResetProbeUserAbuse(userId int) error {
	return DB.Model(&ProbeUserAbuseState{}).
		Where("user_id = ?", userId).
		Updates(map[string]interface{}{
			"offense_count": 0,
			"updated_at":    common.GetTimestamp(),
		}).Error
}

// ProbeGuardStatsResult 探测防护统计数据。
type ProbeGuardStatsResult struct {
	TotalIPStates   int64 `json:"total_ip_states"`
	TotalUserStates int64 `json:"total_user_states"`
	TotalOffenses   int64 `json:"total_offenses"`
	RecentOffenses  int64 `json:"recent_offenses"`
}

// GetProbeGuardStats 汇总探测防护统计信息。
func GetProbeGuardStats() (*ProbeGuardStatsResult, error) {
	stats := &ProbeGuardStatsResult{}
	if err := DB.Model(&ProbeIPAbuseState{}).Count(&stats.TotalIPStates).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&ProbeUserAbuseState{}).Count(&stats.TotalUserStates).Error; err != nil {
		return nil, err
	}
	var ipSum, userSum struct{ Total int64 }
	if err := DB.Model(&ProbeIPAbuseState{}).Select("COALESCE(SUM(offense_count),0) AS total").Scan(&ipSum).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&ProbeUserAbuseState{}).Select("COALESCE(SUM(offense_count),0) AS total").Scan(&userSum).Error; err != nil {
		return nil, err
	}
	stats.TotalOffenses = ipSum.Total + userSum.Total
	recentSince := common.GetTimestamp() - 86400
	if err := DB.Model(&ProbeIPAbuseState{}).
		Where("last_offense_at >= ?", recentSince).
		Count(&stats.RecentOffenses).Error; err != nil {
		return nil, err
	}
	return stats, nil
}
