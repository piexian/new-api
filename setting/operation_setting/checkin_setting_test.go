package operation_setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApplyClientScore(t *testing.T) {
	s := CheckinSetting{MinQuota: 1000, MaxQuota: 10000}

	assert.Equal(t, 5000, s.ApplyClientScore(5000, 100), "满分不压制")
	assert.Equal(t, 5000, s.ApplyClientScore(5000, 120), "超过 100 按满分处理")
	assert.Equal(t, 1000, s.ApplyClientScore(5000, 0), "零分压到底")
	assert.Equal(t, 1000, s.ApplyClientScore(5000, -5), "负分按 0 处理")
	// 线性插值：1000 + (5000-1000)*50/100 = 3000
	assert.Equal(t, 3000, s.ApplyClientScore(5000, 50), "中间分线性插值")
	assert.Equal(t, 500, s.ApplyClientScore(500, 0), "奖励低于保底时原样返回")
}

func TestEffectiveDecayFloor(t *testing.T) {
	s := CheckinSetting{MinQuota: 1000}
	assert.Equal(t, 1000, s.EffectiveDecayFloor(), "floor 为 0 时取 MinQuota")
	s.DecayFloor = 2000
	assert.Equal(t, 2000, s.EffectiveDecayFloor())
}

func TestDecayedMax(t *testing.T) {
	s := CheckinSetting{
		MinQuota: 1000, MaxQuota: 10000,
		DecayEnabled: true, DecayRate: 0.85, DecayFloor: 0,
	}

	assert.Equal(t, 10000, s.DecayedMax(0), "第 0 周不衰减")
	assert.Equal(t, 8500, s.DecayedMax(1))
	assert.Equal(t, 7225, s.DecayedMax(2))

	// 逐周乘 0.85，最终停在 MinQuota 下限
	assert.Equal(t, 1000, s.DecayedMax(100), "长期不使用衰减到下限")

	// 自定义下限
	s.DecayFloor = 5000
	assert.Equal(t, 5000, s.DecayedMax(100), "停在自定义下限")

	// 未启用衰减恒为 MaxQuota
	s.DecayEnabled = false
	assert.Equal(t, 10000, s.DecayedMax(100))

	// 非法衰减系数回退 0.85
	s.DecayEnabled = true
	s.DecayFloor = 0
	s.DecayRate = 1.5
	assert.Equal(t, 8500, s.DecayedMax(1), "非法 rate 回退默认 0.85")
}

func TestBoostProbability(t *testing.T) {
	s := CheckinSetting{
		UsageBoostEnabled:   true,
		BaseHighProbability: 0.05,
		BoostMaxProbability: 0.80,
	}

	assert.InDelta(t, 0.05, s.BoostProbability(0, 0), 1e-9, "无连续使用保持基准")

	// 周加成爬满 4 周贡献区间 60%：0.05 + 0.75*0.6 = 0.50
	assert.InDelta(t, 0.50, s.BoostProbability(4, 0), 1e-9, "4 周爬满周加成")
	// 月加成爬满 3 个月贡献区间 40%：0.05 + 0.75*0.4 = 0.35
	assert.InDelta(t, 0.35, s.BoostProbability(0, 3), 1e-9, "3 个月爬满月加成")
	// 全部爬满 = 上限 0.80
	assert.InDelta(t, 0.80, s.BoostProbability(4, 3), 1e-9, "叠加达到上限")
	assert.InDelta(t, 0.80, s.BoostProbability(100, 100), 1e-9, "超过上限被封顶")

	// 半程：2 周 → 0.05 + 0.75*0.6*0.5 = 0.275
	assert.InDelta(t, 0.275, s.BoostProbability(2, 0), 1e-9)

	// 未启用加成恒为基准
	s.UsageBoostEnabled = false
	assert.InDelta(t, 0.05, s.BoostProbability(4, 3), 1e-9)

	// 上限低于基准时保持基准
	s.UsageBoostEnabled = true
	s.BoostMaxProbability = 0.01
	assert.InDelta(t, 0.05, s.BoostProbability(4, 3), 1e-9)
}

func TestRollReward(t *testing.T) {
	s := CheckinSetting{
		MinQuota: 1000, MaxQuota: 10000,
		HighRewardThreshold: 0.8,
	}

	assert.Equal(t, 1000, s.RollReward(1000, 1.0), "有效上限不超保底时恒为保底")
	assert.Equal(t, 1000, s.RollReward(500, 1.0), "有效上限低于保底时恒为保底")

	// highProb=1 时必落大额档 [threshold, effectiveMax]
	// threshold = 1000 + (5000-1000)*0.8 = 4200
	for i := 0; i < 200; i++ {
		r := s.RollReward(5000, 1.0)
		assert.GreaterOrEqual(t, r, 4200)
		assert.LessOrEqual(t, r, 5000)
	}
	// highProb=0 时必落普通档 [min, threshold)
	for i := 0; i < 200; i++ {
		r := s.RollReward(5000, 0.0)
		assert.GreaterOrEqual(t, r, 1000)
		assert.Less(t, r, 4200)
	}
	// 一般情况永远在 [min, effectiveMax] 内
	for i := 0; i < 200; i++ {
		r := s.RollReward(8000, 0.5)
		assert.GreaterOrEqual(t, r, 1000)
		assert.LessOrEqual(t, r, 8000)
	}
}

func TestIsSpecialRewardDay(t *testing.T) {
	s := CheckinSetting{SpecialEnabled: true, SpecialWeekday: "1", SpecialQuota: 5000}
	// 2026-08-24 是周一
	monday := mustParseTime(t, "2026-08-24 12:00:00")
	tuesday := mustParseTime(t, "2026-08-25 12:00:00")
	sunday := mustParseTime(t, "2026-08-30 12:00:00")

	assert.True(t, s.IsSpecialRewardDay(monday))
	assert.False(t, s.IsSpecialRewardDay(tuesday))
	assert.False(t, s.IsSpecialRewardDay(sunday))

	// 周日 = 7
	s.SpecialWeekday = "7"
	assert.True(t, s.IsSpecialRewardDay(sunday))

	// 未启用 / 无效值
	s.SpecialEnabled = false
	assert.False(t, s.IsSpecialRewardDay(sunday))
	s.SpecialEnabled = true
	s.SpecialWeekday = "9"
	assert.False(t, s.IsSpecialRewardDay(sunday))
}

func mustParseTime(t *testing.T, s string) (tm time.Time) {
	t.Helper()
	var err error
	tm, err = time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		t.Fatalf("parse time %s: %v", s, err)
	}
	return tm
}
