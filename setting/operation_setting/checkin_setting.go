package operation_setting

import (
	"errors"
	"math/rand"
	"time"

	"github.com/QuantumNous/new-api/setting/config"
)

// 签到额度当日有效回收模式
const (
	CheckinExpireModeUnused = "unused" // 只回收未消耗的部分
	CheckinExpireModeAll    = "all"    // 全额回收
)

// CheckinSetting 签到功能配置
//
// 字段分四组：
//   - 基础：enabled / min_quota / max_quota
//   - 特殊星期：特殊星期命中时以固定大额覆盖随机区间（同样参与衰减）
//   - 反脚本：client_check_enabled 启用客户端环境+行为评分，压低可疑签到奖励
//   - 自适应：decay_* 衰减（只签不用逐周降低上限）、boost_* 加成（签到+消费提升大额概率）
//   - 补签：makeup_* 断签补录，可配置是否计入进度
//   - 风控联动：risk_watch_* 长期"签到后只调一次"自动列入风控并锁底
//   - 额度过期：expire_* 次日清算回收未消耗部分
type CheckinSetting struct {
	Enabled  bool `json:"enabled"`   // 是否启用签到功能
	MinQuota int  `json:"min_quota"` // 签到最小额度奖励
	MaxQuota int  `json:"max_quota"` // 签到最大额度奖励

	// 特殊星期固定奖励（命中时覆盖随机区间，仍参与衰减与 clientScore 压制）
	SpecialEnabled bool   `json:"special_enabled"`
	SpecialWeekday string `json:"special_weekday"` // "1"-"7"，1=周一 ... 7=周日
	SpecialQuota   int    `json:"special_quota"`

	// 反脚本客户端检测：非浏览器环境压低奖励到区间下沿而非拒绝
	ClientCheckEnabled bool `json:"client_check_enabled"`

	// 自适应奖励：衰减（只签不用逐周降上限）
	DecayEnabled bool    `json:"decay_enabled"`
	DecayRate    float64 `json:"decay_rate"`  // 每周衰减系数，如 0.85
	DecayFloor   int     `json:"decay_floor"` // 衰减下限，0 表示取 MinQuota

	// 自适应奖励：加成（连续签到+消费提升大额概率）
	UsageBoostEnabled   bool    `json:"usage_boost_enabled"`
	UsageBoostDays      int     `json:"usage_boost_days"`      // 连续签到+消费满此天数后开始提升
	HighRewardThreshold float64 `json:"high_reward_threshold"` // 大额阈值占区间比例，如 0.8
	BaseHighProbability float64 `json:"base_high_probability"` // 基准大额概率，如 0.05
	BoostMaxProbability float64 `json:"boost_max_probability"` // 加成后上限，如 0.80

	// 补签
	MakeUpEnabled              bool `json:"makeup_enabled"`                // 是否允许补签
	MakeUpMaxDays              int  `json:"makeup_max_days"`               // 最多补签前几天
	MakeUpCountsTowardProgress bool `json:"makeup_counts_toward_progress"` // 补签是否计入 streak/周/月进度
	MakeUpRewardEnabled        bool `json:"makeup_reward_enabled"`        // 补签是否发放奖励（关闭时仅补进度，发放 0 额度）

	// 签到风控联动：长期"签到后只调一次"自动列入风控并锁底
	RiskWatchEnabled  bool `json:"risk_watch_enabled"`
	RiskWatchDays     int  `json:"risk_watch_days"`      // 连续签到多少天后开始观察
	RiskMinDailyCalls int  `json:"risk_min_daily_calls"` // 每天调用次数 ≤ 此值视为低使用
	RiskMinDailyQuota int  `json:"risk_min_daily_quota"` // 每天消费额度 ≤ 此值视为低使用

	// 签到额度当日有效（次日清算回收）
	ExpireEnabled bool   `json:"expire_enabled"`
	ExpireMode    string `json:"expire_mode"` // 见 CheckinExpireMode*
}

// 默认配置：所有增强功能默认关闭，保持既有站点行为不变
var checkinSetting = CheckinSetting{
	Enabled:  false,
	MinQuota: 1000,  // 约 0.002 USD
	MaxQuota: 10000, // 约 0.02 USD

	SpecialEnabled: false,
	SpecialWeekday: "1",
	SpecialQuota:   0,

	ClientCheckEnabled: false,

	DecayEnabled: false,
	DecayRate:    0.85,
	DecayFloor:   0,

	UsageBoostEnabled:   false,
	UsageBoostDays:      3,
	HighRewardThreshold: 0.8,
	BaseHighProbability: 0.05,
	BoostMaxProbability: 0.80,

	MakeUpEnabled:              false,
	MakeUpMaxDays:              3,
	MakeUpCountsTowardProgress: false,
	MakeUpRewardEnabled:        true, // 补签默认照常发放奖励，保持既有行为不变

	RiskWatchEnabled:  false,
	RiskWatchDays:     14,
	RiskMinDailyCalls: 1,
	RiskMinDailyQuota: 100,

	ExpireEnabled: false,
	ExpireMode:    CheckinExpireModeUnused,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("checkin_setting", &checkinSetting)
}

// GetCheckinSetting 获取签到配置
func GetCheckinSetting() *CheckinSetting {
	return &checkinSetting
}

// IsCheckinEnabled 是否启用签到功能
func IsCheckinEnabled() bool {
	return checkinSetting.Enabled
}

// GetCheckinQuotaRange 获取签到额度范围
func GetCheckinQuotaRange() (min, max int) {
	return checkinSetting.MinQuota, checkinSetting.MaxQuota
}

// CheckinWeekday 返回配置的特殊星期（1=周一 ... 7=周日），无效时返回 0
func (setting CheckinSetting) CheckinWeekday() int {
	if !setting.SpecialEnabled {
		return 0
	}
	switch setting.SpecialWeekday {
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	case "6":
		return 6
	case "7":
		return 7
	}
	return 0
}

// IsSpecialRewardDay 判断指定时间是否为特殊奖励日
func (setting CheckinSetting) IsSpecialRewardDay(now time.Time) bool {
	weekday := setting.CheckinWeekday()
	if weekday == 0 {
		return false
	}
	// time.Weekday: Sunday=0 ... Saturday=6，转为 1=周一 ... 7=周日
	day := int(now.Weekday())
	if day == 0 {
		day = 7
	}
	return day == weekday
}

// NormalizedExpireMode 返回合法的回收模式，无效值回退到 unused
func (setting CheckinSetting) NormalizedExpireMode() string {
	if setting.ExpireMode == CheckinExpireModeAll {
		return CheckinExpireModeAll
	}
	return CheckinExpireModeUnused
}

// IsExpireEnabled 是否启用签到额度当日有效
func (setting CheckinSetting) IsExpireEnabled() bool {
	return setting.ExpireEnabled
}

// SafeMinQuota 返回非负的最低奖励额度：MinQuota 为负会让签到倒扣余额，一律按 0 兜底。
func (setting CheckinSetting) SafeMinQuota() int {
	if setting.MinQuota < 0 {
		return 0
	}
	return setting.MinQuota
}

// clamp01 把概率/比例类配置夹到 [0,1]：超上限会让大额档 100% 命中。
func clamp01(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// ValidateCheckinSetting 保存路径的全量校验（controller/option.go 调用），
// 保证概率越界、负额度等配置无法入库。与运行时 clamp 双保险：
// LoadFromDB 直读的历史脏数据不经过这里，仍由 SafeMinQuota/clamp01 兜底。
func ValidateCheckinSetting(s *CheckinSetting) error {
	if s == nil {
		return errors.New("checkin_setting 配置为空")
	}
	if s.MinQuota < 0 {
		return errors.New("签到最小额度不能为负")
	}
	if s.MaxQuota < s.MinQuota {
		return errors.New("签到最大额度不能小于最小额度")
	}
	if s.SpecialQuota < 0 {
		return errors.New("特殊星期额度不能为负")
	}
	if s.DecayFloor < 0 {
		return errors.New("衰减下限不能为负")
	}
	if s.HighRewardThreshold < 0 || s.HighRewardThreshold > 1 {
		return errors.New("高奖励阈值必须在 0-1 之间")
	}
	if s.BaseHighProbability < 0 || s.BaseHighProbability > 1 {
		return errors.New("基础高奖励概率必须在 0-1 之间")
	}
	if s.BoostMaxProbability < 0 || s.BoostMaxProbability > 1 {
		return errors.New("加成概率上限必须在 0-1 之间")
	}
	if s.ExpireMode != CheckinExpireModeUnused && s.ExpireMode != CheckinExpireModeAll {
		return errors.New("过期模式只能是 unused 或 all")
	}
	return nil
}

// ApplyClientScore 按客户端环境分压制签到奖励，score 取值 0-100。
//
// score=100 原样返回；score=0 压到 MinQuota；中间线性插值，不设阈值。
// 结果始终落在 [MinQuota, reward] 内，因此从响应上无法与一次「运气不好的」
// 正常签到区分开——这正是「压低而非拦截」的关键。
func (setting CheckinSetting) ApplyClientScore(reward int, score int) int {
	if score >= 100 {
		return reward
	}
	if score < 0 {
		score = 0
	}
	floor := setting.MinQuota
	if floor < 0 {
		floor = 0
	}
	if reward <= floor {
		return reward
	}
	return floor + int(int64(reward-floor)*int64(score)/100)
}

// EffectiveDecayFloor 衰减下限实际值（配置为 0 时取 MinQuota）
func (setting CheckinSetting) EffectiveDecayFloor() int {
	if setting.DecayFloor > 0 {
		return setting.DecayFloor
	}
	return setting.SafeMinQuota()
}

// DecayedMax 计算衰减 N 周后的有效上限
func (setting CheckinSetting) DecayedMax(decayWeeks int) int {
	if !setting.DecayEnabled || decayWeeks <= 0 {
		return setting.MaxQuota
	}
	rate := setting.DecayRate
	if rate <= 0 || rate > 1 {
		rate = 0.85
	}
	floor := setting.EffectiveDecayFloor()
	max := setting.MaxQuota
	for i := 0; i < decayWeeks; i++ {
		max = int(float64(max) * rate)
		if max <= floor {
			return floor
		}
	}
	if max < floor {
		return floor
	}
	return max
}

// BoostProbability 计算加成后的大额概率。
// usageWeeks / usageMonths 为连续"签到+消费"的完整周数/月数。
// 周加成贡献区间的 60%（4 周爬满），月加成贡献 40%（3 个月爬满）。
func (setting CheckinSetting) BoostProbability(usageWeeks, usageMonths int) float64 {
	// 概率类配置先夹到 [0,1] 再参与计算：越界值（如 >1）会让大额档 100% 命中
	base := clamp01(setting.BaseHighProbability)
	maxP := clamp01(setting.BoostMaxProbability)
	if !setting.UsageBoostEnabled {
		return base
	}
	if maxP <= base {
		return base
	}
	weekProgress := float64(usageWeeks) / 4.0
	if weekProgress > 1 {
		weekProgress = 1
	}
	monthProgress := float64(usageMonths) / 3.0
	if monthProgress > 1 {
		monthProgress = 1
	}
	p := base + (maxP-base)*0.6*weekProgress + (maxP-base)*0.4*monthProgress
	if p > maxP {
		p = maxP
	}
	return p
}

// RollReward 在给定有效上限和大额概率下摇出奖励额度。
// highProb 为落在 [threshold, effectiveMax] 的概率；其余落在 [MinQuota, threshold)。
func (setting CheckinSetting) RollReward(effectiveMax int, highProb float64) int {
	min := setting.SafeMinQuota()
	if effectiveMax <= min {
		return min
	}
	threshold := min + int(float64(effectiveMax-min)*clamp01(setting.HighRewardThreshold))
	if threshold >= effectiveMax {
		threshold = effectiveMax
	}
	if threshold <= min {
		return min + rand.Intn(effectiveMax-min+1)
	}
	if rand.Float64() < highProb {
		// 大额档
		return threshold + rand.Intn(effectiveMax-threshold+1)
	}
	// 普通档
	return min + rand.Intn(threshold-min)
}

// RewardQuota 获取指定时间的签到奖励额度（不含衰减/加成/评分，纯基础+特殊星期）。
// 自适应逻辑由 service 层在调用前计算好 effectiveMax 和 highProb 后用 RollReward。
func (setting CheckinSetting) RewardQuota(now time.Time) int {
	if setting.IsSpecialRewardDay(now) && setting.SpecialQuota > 0 {
		return setting.SpecialQuota
	}
	min := setting.SafeMinQuota()
	max := setting.MaxQuota
	if max <= min {
		return min
	}
	return min + rand.Intn(max-min+1)
}
