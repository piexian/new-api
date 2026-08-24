package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// CheckinRewardPlan 一次签到的自适应奖励计算结果。
// 暴露在响应里的只有 EffectiveMax（作为"当前保底"展示），其余字段仅供内部/日志使用。
type CheckinRewardPlan struct {
	EffectiveMax    int     // 衰减后的有效上限（即前端展示的"当前保底"）
	HighProbability float64 // 大额档概率（基准 + 连续使用加成）
	RiskLocked      bool    // 风控锁底：奖励恒为 MinQuota
	DecayWeeks      int     // 当前衰减周数（诊断用）
	UsageWeeks      int     // 连续"签到+消费"周数（诊断用）
	UsageMonths     int     // 连续"签到+消费"月数（诊断用）
}

// ComputeCheckinRewardPlan 计算用户的签到奖励计划。
// 顺序：风控锁底 > 衰减定上限 > 加成定概率。查询失败一律按无增强处理（不惩罚用户）。
func ComputeCheckinRewardPlan(userId int, now time.Time) CheckinRewardPlan {
	setting := operation_setting.GetCheckinSetting()
	plan := CheckinRewardPlan{
		EffectiveMax:    setting.MaxQuota,
		HighProbability: setting.BaseHighProbability,
	}

	if model.IsUserCheckinRiskLocked(userId) {
		plan.RiskLocked = true
		plan.EffectiveMax = setting.MinQuota
		plan.HighProbability = 0
		return plan
	}

	if setting.DecayEnabled {
		if weeks, err := model.DecayWeeks(userId, now); err == nil {
			plan.DecayWeeks = weeks
			plan.EffectiveMax = setting.DecayedMax(weeks)
		}
	}

	if setting.UsageBoostEnabled {
		// 连续签到天数达到门槛后，周/月加成才开始生效
		streak, err := model.CurrentCheckinStreak(userId)
		if err == nil && streak >= setting.UsageBoostDays {
			if w, m, err := model.ConsecutiveUsageWeeksMonths(userId, now); err == nil {
				plan.UsageWeeks = w
				plan.UsageMonths = m
				plan.HighProbability = setting.BoostProbability(w, m)
			}
		}
	}

	return plan
}

// RollCheckinReward 摇出最终签到奖励。
// isMakeup=true 时跳过特殊星期固定档（补签不发特殊星期奖励）。
// 风控锁底直接返回 MinQuota；特殊星期固定大额同样被衰减上限截断。
func RollCheckinReward(userId int, now time.Time, isMakeup bool) (int, CheckinRewardPlan) {
	setting := operation_setting.GetCheckinSetting()
	plan := ComputeCheckinRewardPlan(userId, now)
	if plan.RiskLocked {
		return setting.MinQuota, plan
	}

	if !isMakeup && setting.IsSpecialRewardDay(now) && setting.SpecialQuota > 0 {
		reward := setting.SpecialQuota
		if reward > plan.EffectiveMax {
			reward = plan.EffectiveMax
		}
		if reward < setting.MinQuota {
			reward = setting.MinQuota
		}
		return reward, plan
	}

	return setting.RollReward(plan.EffectiveMax, plan.HighProbability), plan
}

// EvaluateCheckinRiskWatch 签到成功后评估是否纳入风控观察名单。
// 触发条件：连续签到达到 RiskWatchDays，且窗口内每天的调用次数与消费额度
// 都低于阈值——即「签到后只调一次」的薅羊毛模式。列入后签到奖励锁底。
// 设计为签到事务提交后异步调用，失败只记日志不影响签到结果。
func EvaluateCheckinRiskWatch(userId int) {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("checkin risk watch evaluate panic (user %d): %v", userId, r))
		}
	}()

	setting := operation_setting.GetCheckinSetting()
	if !setting.RiskWatchEnabled || setting.RiskWatchDays <= 0 {
		return
	}

	// CurrentCheckinStreak 从昨天往前数，本次签到已写入，实际连续天数 +1
	streak, err := model.CurrentCheckinStreak(userId)
	if err != nil {
		common.SysError(fmt.Sprintf("checkin risk watch streak query failed (user %d): %v", userId, err))
		return
	}
	streak++
	if streak < setting.RiskWatchDays {
		return
	}

	// 观察窗口取连续签到天数，上限 30 天防止超大窗口拖慢查询
	window := streak
	if window > 30 {
		window = 30
	}
	startDate := time.Now().AddDate(0, 0, -(window - 1)).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	stats, err := model.DailyUsageStatsBetween(userId, startDate, endDate)
	if err != nil {
		common.SysError(fmt.Sprintf("checkin risk watch usage query failed (user %d): %v", userId, err))
		return
	}

	var totalCalls, totalQuota int64
	lowUsage := true
	for i := 0; i < window; i++ {
		day := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		s := stats[day] // 无消费的日期为零值，天然满足低使用
		if s.Calls > setting.RiskMinDailyCalls || s.Quota > int64(setting.RiskMinDailyQuota) {
			lowUsage = false
			break
		}
		totalCalls += int64(s.Calls)
		totalQuota += s.Quota
	}
	if !lowUsage {
		return
	}

	avgCalls := totalCalls / int64(window)
	avgQuota := totalQuota / int64(window)
	awardSum, err := model.CheckinAwardSumBetween(userId, startDate, endDate)
	if err != nil {
		awardSum = 0 // 统计失败不阻断风控判定
	}
	avgAwarded := awardSum / int64(window)
	reason := fmt.Sprintf("连续签到 %d 天，观察期 %d 天内日均调用 %d 次、日均消费 %d 额度，均低于阈值（%d 次 / %d 额度）",
		streak, window, avgCalls, avgQuota, setting.RiskMinDailyCalls, setting.RiskMinDailyQuota)
	if err := model.UpsertCheckinRiskWatch(userId, streak, int(avgCalls), avgQuota, avgAwarded, reason); err != nil {
		common.SysError(fmt.Sprintf("checkin risk watch upsert failed (user %d): %v", userId, err))
		return
	}
	common.SysLog(fmt.Sprintf("user %d added to checkin risk watch: %s", userId, reason))
}
