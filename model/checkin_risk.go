package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
)

// 签到风控观察名单状态
const (
	CheckinRiskStatusWatching = "watching" // 观察中（已锁底，持续收集数据）
	CheckinRiskStatusLocked   = "locked"   // 确认限制（管理员确认）
	CheckinRiskStatusReleased = "released" // 已解除（管理员手动解除）
)

// CheckinRiskWatch 签到风控观察名单。
// 触发条件：连续签到 RiskWatchDays 天且每天 API 调用 ≤ RiskMinDailyCalls、
// 消费额度 ≤ RiskMinDailyQuota——即「签到后只调一次」的薅羊毛模式。
// 列入后签到奖励锁底 MinQuota，管理员可在风控面板查看签到/调用对比并手动解除。
type CheckinRiskWatch struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex"`
	Status       string `json:"status" gorm:"type:varchar(16);not null;default:'watching';index"`
	StreakDays   int    `json:"streak_days"`                    // 触发时的连续签到天数
	AvgCalls     int    `json:"avg_calls"`                      // 观察期内日均调用次数
	AvgQuota     int64  `json:"avg_quota"`                      // 观察期内日均消费额度
	AvgAwarded   int64  `json:"avg_awarded"`                    // 观察期内日均签到奖励
	Reason       string `json:"reason" gorm:"type:text"`        // 触发原因描述
	ReleasedBy   int    `json:"released_by"`                    // 解除操作的管理员 id
	ReleasedNote string `json:"released_note" gorm:"type:text"` // 解除备注
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
	ReleasedAt   int64  `json:"released_at" gorm:"bigint"`
}

func (CheckinRiskWatch) TableName() string {
	return "checkin_risk_watches"
}

// GetActiveCheckinRiskWatch 返回用户当前生效的风控记录（watching/locked），无则返回 nil。
func GetActiveCheckinRiskWatch(userId int) (*CheckinRiskWatch, error) {
	var w CheckinRiskWatch
	err := DB.Where("user_id = ? AND status IN ?", userId,
		[]string{CheckinRiskStatusWatching, CheckinRiskStatusLocked}).
		First(&w).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// IsUserCheckinRiskLocked 用户签到奖励是否被风控锁底。
// 查询失败时放行（不惩罚用户）。
func IsUserCheckinRiskLocked(userId int) bool {
	w, err := GetActiveCheckinRiskWatch(userId)
	return err == nil && w != nil
}

// UpsertCheckinRiskWatch 创建或刷新观察记录（触发时调用）。
func UpsertCheckinRiskWatch(userId int, streak, avgCalls int, avgQuota, avgAwarded int64, reason string) error {
	now := common.GetTimestamp()
	var w CheckinRiskWatch
	err := DB.Where("user_id = ?", userId).First(&w).Error
	if err == nil {
		// 已有记录：已解除的不重新触发（尊重管理员决定）；观察中的刷新数据
		if w.Status == CheckinRiskStatusReleased {
			return nil
		}
		return DB.Model(&w).Updates(map[string]interface{}{
			"streak_days": streak,
			"avg_calls":   avgCalls,
			"avg_quota":   avgQuota,
			"avg_awarded": avgAwarded,
			"reason":      reason,
			"updated_at":  now,
		}).Error
	}
	w = CheckinRiskWatch{
		UserId:     userId,
		Status:     CheckinRiskStatusWatching,
		StreakDays: streak,
		AvgCalls:   avgCalls,
		AvgQuota:   avgQuota,
		AvgAwarded: avgAwarded,
		Reason:     reason,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return DB.Create(&w).Error
}

// ReleaseCheckinRiskWatch 管理员手动解除风控。
func ReleaseCheckinRiskWatch(userId int, adminId int, note string) error {
	now := common.GetTimestamp()
	res := DB.Model(&CheckinRiskWatch{}).
		Where("user_id = ? AND status IN ?", userId,
			[]string{CheckinRiskStatusWatching, CheckinRiskStatusLocked}).
		Updates(map[string]interface{}{
			"status":        CheckinRiskStatusReleased,
			"released_by":   adminId,
			"released_note": note,
			"released_at":   now,
			"updated_at":    now,
		})
	return res.Error
}

// CheckinRiskWatchListItem 风控面板行：观察记录 + 最近签到/调用对比数据
type CheckinRiskWatchListItem struct {
	CheckinRiskWatch
	Username string `json:"username"`
}

// ListCheckinRiskWatches 分页列出风控观察名单。
func ListCheckinRiskWatches(page, pageSize int, status string) ([]CheckinRiskWatchListItem, int64, error) {
	var total int64
	q := DB.Model(&CheckinRiskWatch{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []CheckinRiskWatchListItem
	// 过滤条件必须与上面 Count 保持一致，否则会出现"过滤后的 total + 未过滤的数据"
	list := DB.Table("checkin_risk_watches").
		Select("checkin_risk_watches.*, users.username").
		Joins("LEFT JOIN users ON users.id = checkin_risk_watches.user_id")
	if status != "" {
		list = list.Where("checkin_risk_watches.status = ?", status)
	}
	err := list.
		Order("checkin_risk_watches.updated_at desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Scan(&items).Error
	return items, total, err
}

// CheckinDailyContrast 风控面板用：某用户最近 N 天每天的签到奖励与调用数据对比
type CheckinDailyContrast struct {
	Date         string `json:"date"`
	QuotaAwarded int    `json:"quota_awarded"` // 当天签到奖励（无签到为 0）
	IsMakeUp     bool   `json:"is_makeup"`
	Calls        int    `json:"calls"` // 当天 API 调用次数
	Quota        int64  `json:"quota"` // 当天消费额度
}

// GetCheckinDailyContrast 返回用户最近 days 天的签到/调用逐日对比。
func GetCheckinDailyContrast(userId int, days int) ([]CheckinDailyContrast, error) {
	if days <= 0 || days > 90 {
		days = 30
	}
	startDate := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")

	var checkins []Checkin
	if err := DB.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?",
		userId, startDate, endDate).Find(&checkins).Error; err != nil {
		return nil, err
	}
	checkinMap := make(map[string]Checkin, len(checkins))
	for _, c := range checkins {
		checkinMap[c.CheckinDate] = c
	}

	usage, err := DailyUsageStatsBetween(userId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	result := make([]CheckinDailyContrast, 0, days)
	for i := 0; i < days; i++ {
		day := time.Now().AddDate(0, 0, -(days - 1 - i)).Format("2006-01-02")
		row := CheckinDailyContrast{Date: day}
		if c, ok := checkinMap[day]; ok {
			row.QuotaAwarded = c.QuotaAwarded
			row.IsMakeUp = c.IsMakeUp
		}
		if u, ok := usage[day]; ok {
			row.Calls = u.Calls
			row.Quota = u.Quota
		}
		result = append(result, row)
	}
	return result, nil
}
