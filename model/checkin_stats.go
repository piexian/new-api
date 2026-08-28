package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ---------------------------------------------------------------------------
// 签到自适应/风控/行为分析查询辅助
//
// 统一的数据口径：
//   - streak：从今天往前数连续有签到记录的天数（不含今天本身，签到前调用）。
//     MakeUpCountsTowardProgress=false 时补签记录不计入。
//   - 消费：logs 表 LogTypeConsume 记录。还贷产生的额度扣减不写 consume 日志，
//     因此天然满足「还贷不算消费」的口径。
//   - 衰减周数：最近一次消费距今的完整周数（无消费则从首次签到算起）。
// ---------------------------------------------------------------------------

// RecentCheckinTimestamps 返回该用户最近 limit 次签到的时间戳（行为特征分析用）。
// 调用发生在本次签到写入之前，因此返回的都是历史记录。
func RecentCheckinTimestamps(userId int, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 14
	}
	var timestamps []int64
	err := DB.Model(&Checkin{}).
		// 补签记录 created_at 集中在补签操作时刻，会干扰时刻离散度分析，
		// 一律排除；行为特征只分析真实签到时间。
		Where("user_id = ? AND created_at > 0 AND is_makeup = ?", userId, false).
		Order("created_at desc").
		Limit(limit).
		Pluck("created_at", &timestamps).Error
	return timestamps, err
}

// UserHasConsumptionSince 用户在给定时间点之后是否产生过实际消费。
func UserHasConsumptionSince(userId int, since int64) (bool, error) {
	var count int64
	err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ?", userId, LogTypeConsume, since).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

// FirstCheckinTimestamp 返回该用户最早一次签到的时间戳，0 表示从未签到。
func FirstCheckinTimestamp(userId int) (int64, error) {
	var timestamps []int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND created_at > 0", userId).
		Order("created_at asc").Limit(1).
		Pluck("created_at", &timestamps).Error
	if err != nil || len(timestamps) == 0 {
		return 0, err
	}
	return timestamps[0], nil
}

// checkinDatesDesc 返回用户所有签到日期（倒序），是否计入补签由 includeMakeup 控制。
func checkinDatesDesc(userId int, includeMakeup bool) ([]string, error) {
	q := DB.Model(&Checkin{}).Where("user_id = ?", userId)
	if !includeMakeup {
		q = q.Where("is_makeup = ?", false)
	}
	var dates []string
	err := q.Order("checkin_date desc").Pluck("checkin_date", &dates).Error
	return dates, err
}

// CurrentCheckinStreak 计算截至昨天为止的连续签到天数。
// 补签是否计入由 MakeUpCountsTowardProgress 配置决定。
func CurrentCheckinStreak(userId int) (int, error) {
	includeMakeup := operation_setting.GetCheckinSetting().MakeUpCountsTowardProgress
	dates, err := checkinDatesDesc(userId, includeMakeup)
	if err != nil || len(dates) == 0 {
		return 0, err
	}
	set := make(map[string]bool, len(dates))
	for _, d := range dates {
		set[d] = true
	}
	// 从昨天开始往前数；今天还没签到（签到前调用），不影响连续性
	cursor := time.Now().AddDate(0, 0, -1)
	streak := 0
	for {
		day := cursor.Format("2006-01-02")
		if !set[day] {
			break
		}
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return streak, nil
}

// LastConsumptionTimestamp 用户最近一次实际消费的时间戳，0 表示从未消费。
func LastConsumptionTimestamp(userId int) (int64, error) {
	var ts []int64
	err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ?", userId, LogTypeConsume).
		Order("created_at desc").Limit(1).
		Pluck("created_at", &ts).Error
	if err != nil || len(ts) == 0 {
		return 0, err
	}
	return ts[0], nil
}

// DecayWeeks 计算衰减周数：最近一次消费距今的完整周数。
// 从未消费的用户从首次签到开始计（首次签到前的一周视为宽限期，返回 0）。
func DecayWeeks(userId int, now time.Time) (int, error) {
	last, err := LastConsumptionTimestamp(userId)
	if err != nil {
		return 0, err
	}
	if last == 0 {
		// 从未消费：从首次签到起算，但留一周宽限
		first, err := FirstCheckinTimestamp(userId)
		if err != nil || first == 0 {
			return 0, err
		}
		weeks := int(now.Unix()-first) / (7 * 86400)
		if weeks <= 1 {
			return 0, nil
		}
		return weeks - 1, nil
	}
	elapsed := now.Unix() - last
	if elapsed < 7*86400 {
		return 0, nil
	}
	return int(elapsed / (7 * 86400)), nil
}

// checkinDateRange 把 YYYY-MM-DD 转为本地时区的 [start, end) 秒级时间戳。
// 使用本地时区是因为签到写入时用的就是 time.Now()，两侧必须一致。
func checkinDateRange(date string) (int64, int64, error) {
	start, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return 0, 0, err
	}
	return start.Unix(), start.AddDate(0, 0, 1).Unix(), nil
}

// weekHasCheckinAndConsumption 判断以 anchor 为周末的那一周（7 天窗口）
// 是否同时有签到记录和消费记录。
func weekHasCheckinAndConsumption(userId int, weekStart, weekEnd int64, includeMakeup bool) (bool, error) {
	q := DB.Model(&Checkin{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userId, weekStart, weekEnd)
	if !includeMakeup {
		q = q.Where("is_makeup = ?", false)
	}
	var checkinCount int64
	if err := q.Limit(1).Count(&checkinCount).Error; err != nil {
		return false, err
	}
	if checkinCount == 0 {
		return false, nil
	}
	return UserHasConsumptionSinceRange(userId, weekStart, weekEnd)
}

// UserHasConsumptionSinceRange 用户在 [start, end) 内是否有实际消费。
func UserHasConsumptionSinceRange(userId int, start, end int64) (bool, error) {
	var count int64
	err := LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?",
			userId, LogTypeConsume, start, end).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

// ConsecutiveUsageWeeksMonths 计算连续「签到+消费」的完整周数和月数。
// 从上一周/上一月开始往前数，当前进行中的周期不计入（未结束无法判定）。
func ConsecutiveUsageWeeksMonths(userId int, now time.Time) (weeks int, months int, err error) {
	includeMakeup := operation_setting.GetCheckinSetting().MakeUpCountsTowardProgress

	// 本周起点（本地时区周一 00:00）
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(weekday - 1))

	for i := 0; i < 52; i++ { // 最多回看一年，防御性上限
		start := weekStart.AddDate(0, 0, -7*(i+1))
		end := weekStart.AddDate(0, 0, -7*i)
		ok, err := weekHasCheckinAndConsumption(userId, start.Unix(), end.Unix(), includeMakeup)
		if err != nil {
			return weeks, months, err
		}
		if !ok {
			break
		}
		weeks++
	}

	// 本月起点
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for i := 0; i < 12; i++ {
		end := monthStart.AddDate(0, -i, 0)
		start := monthStart.AddDate(0, -(i + 1), 0)
		ok, err := weekHasCheckinAndConsumption(userId, start.Unix(), end.Unix(), includeMakeup)
		if err != nil {
			return weeks, months, err
		}
		if !ok {
			break
		}
		months++
	}
	return weeks, months, nil
}

// DailyUsageStat 某一天的调用次数与消费额度
type DailyUsageStat struct {
	Calls int   `json:"calls"`
	Quota int64 `json:"quota"`
}

// DailyUsageStatsBetween 返回用户在 [startDate, endDate] 内按天的消费统计。
// 返回 map[date]stat，无消费的日期不在 map 中。
func DailyUsageStatsBetween(userId int, startDate, endDate string) (map[string]DailyUsageStat, error) {
	start, _, err := checkinDateRange(startDate)
	if err != nil {
		return nil, err
	}
	_, end, err := checkinDateRange(endDate)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Day   string `gorm:"column:day"`
		Calls int    `gorm:"column:calls"`
		Quota int64  `gorm:"column:quota"`
	}
	// created_at 是秒级时间戳，按日志库服务器时区转日期分组；各库语法不同。
	// Log 表在 LOG_DB 中，需按日志库类型分支（含 ClickHouse）。
	var dayExpr string
	switch {
	case common.UsingLogDatabase(common.DatabaseTypeClickHouse):
		dayExpr = "formatDateTime(toDateTime(created_at), '%Y-%m-%d')"
	case common.UsingLogDatabase(common.DatabaseTypePostgreSQL):
		dayExpr = "to_char(to_timestamp(created_at), 'YYYY-MM-DD')"
	case common.UsingLogDatabase(common.DatabaseTypeMySQL):
		dayExpr = "FROM_UNIXTIME(created_at, '%Y-%m-%d')"
	default:
		dayExpr = "strftime('%Y-%m-%d', created_at, 'unixepoch', 'localtime')"
	}
	err = LOG_DB.Model(&Log{}).
		Select(dayExpr+" AS day, COUNT(*) AS calls, COALESCE(SUM(quota),0) AS quota").
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?",
			userId, LogTypeConsume, start, end).
		Group("day").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]DailyUsageStat, len(rows))
	for _, r := range rows {
		result[r.Day] = DailyUsageStat{Calls: r.Calls, Quota: r.Quota}
	}
	return result, nil
}

// CheckinAwardSumBetween 返回用户在 [startDate, endDate] 内的签到奖励总额。
func CheckinAwardSumBetween(userId int, startDate, endDate string) (int64, error) {
	var sum *int64
	err := DB.Model(&Checkin{}).
		Select("COALESCE(SUM(quota_awarded),0)").
		Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?", userId, startDate, endDate).
		Scan(&sum).Error
	if err != nil || sum == nil {
		return 0, err
	}
	return *sum, nil
}

// HasCheckinOnDate 用户在指定日期是否已有签到记录（含补签）。
func HasCheckinOnDate(userId int, date string) (bool, error) {
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, date).
		Count(&count).Error
	return count > 0, err
}

// MakeupEligibleDates 返回可补签的日期列表：最近 maxDays 天内没有签到记录的日期。
func MakeupEligibleDates(userId int, maxDays int) ([]string, error) {
	if maxDays <= 0 {
		return nil, nil
	}
	var dates []string
	err := DB.Model(&Checkin{}).
		Where("user_id = ?", userId).
		Pluck("checkin_date", &dates).Error
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(dates))
	for _, d := range dates {
		set[d] = true
	}
	eligible := make([]string, 0, maxDays)
	for i := 1; i <= maxDays; i++ {
		day := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if !set[day] {
			eligible = append(eligible, day)
		}
	}
	return eligible, nil
}
