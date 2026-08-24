# 签到功能增强方案

> 来源：分析 Futureppo/new-api 仓库签到相关提交 + 本项目新需求
> 状态：方案设计阶段，暂不改动代码
> 日期：2026-08-24

## 一、Futureppo 仓库签到功能分析

### 1.1 三大功能模块

| 功能 | 涉及文件 | 核心机制 |
|------|---------|---------|
| 特殊星期奖励 | `checkin_setting.go` | 指定某 weekday 发放固定大额奖励，覆盖随机区间 |
| 签到额度当日有效 | `checkin_setting.go` + `model/checkin.go` + `service/checkin_expiry_task.go` | 次日清算回收未消耗的签到额度，两种模式：`unused`（只回收未花完部分）/ `all`（全额回收）。Checkin 表新增 `ExpiredQuota`、`SettledAt` 字段，幂等清算避免重复扣减 |
| 反脚本检测 | `checkin_setting.go` + `service/checkin_client_score.go` + `service/checkin_behavior_score.go` | 双层评分（0-100），压低可疑请求的奖励而非拒绝 |

### 1.2 反脚本设计核心思路

```
CheckinClientScore = 请求头特征分(上限55) + 行为特征分(上限45)
```

- **请求头信号**（55分）：Sec-Fetch-Mode/Site/Dest、Accept-Language、Accept-Encoding(br/zstd)、Sec-Ch-Ua、Origin/Referer、UA浏览器特征、Accept具体类型
- **行为信号**（45分）：签到时刻圆周标准差（定时任务趋近0）、零点掐秒触发、7天内是否有真实消费

关键设计：线性映射无阈值跳变——改动任何单个请求头只小幅移动分数，无法用二分法定位判定规则。

### 1.3 Futureppo 有而我们没有的文件

1. `service/checkin_expiry_task.go` — 签到额度次日清算任务
2. `service/checkin_client_score.go` — 客户端环境评分（反脚本）
3. `service/checkin_client_score_test.go` — 客户端评分测试
4. `service/checkin_behavior_score.go` — 行为特征评分
5. `setting/operation_setting/checkin_setting_test.go` — 配置测试
6. `model/checkin_expiry_test.go` — 额度清算测试

### 1.4 CheckinSetting 结构体对比

| 字段 | Futureppo | 我们 |
|------|----------|------|
| `Enabled` | ✅ | ✅ |
| `MinQuota` | ✅ | ✅ |
| `MaxQuota` | ✅ | ✅ |
| `SpecialEnabled` | ✅ | ❌ |
| `SpecialWeekday` | ✅ | ❌ |
| `SpecialQuota` | ✅ | ❌ |
| `ExpireEnabled` | ✅ | ❌ |
| `ExpireMode` | ✅ | ❌ |
| `ClientCheckEnabled` | ✅ | ❌ |

### 1.5 架构差异

| 维度 | Futureppo | 我们 |
|------|----------|------|
| `UserCheckin()` 返回值 | `(*Checkin, error)` | `(*Checkin, *LoanRepayInfo, []LenderCredit, error)` |
| 签到自动还款 | ❌ | ✅ |
| Turnstile 作用域 | 通用 `TurnstileCheck()` | `TurnstileScopeCheckin` |
| 前端配置项 | 9 个 | 3 个 |
| 单元测试 | 完整覆盖 | 无 |
| `main.go` 清算任务 | ✅ `StartCheckinExpiryTask()` | ❌ |

## 二、接入冲突点

1. **`model/checkin.go` 函数签名不同**：Futureppo 返回 `(*Checkin, error)`，我们返回四元组含贷款还款结果。需保留贷款还款链路，在其基础上叠加新功能。
2. **Checkin 表新增列**：`ExpiredQuota` 和 `SettledAt` 需数据库迁移，验证 SQLite/MySQL/PG 三库兼容。
3. **额度清算与贷款还款的交互**：Futureppo 清算直接操作 `User.quota` 扣减，但我们的签到额度可能已通过贷款还款路径部分抵扣了债务。清算回收量需要考虑"净入账"而非"全额定发"。
4. **前端双主题**：`web/default/` 用 TSX + TanStack Form，`web/classic/` 用 JSX。Futureppo 前端代码是 JSX 风格，`web/default/` 需用 TSX 重写。

## 三、新增需求：自适应签到奖励

### 3.1 需求描述

在 Futureppo 签到增强的基础上，额外引入**自适应奖励机制**：

1. **只签不用 → 衰减**：用户连续签到但不使用额度，签到奖励逐日衰减，最终趋近 `MinQuota`。
2. **签到+使用 → 提升**：用户签到并且实际消费了额度，后续签到出大额奖励的概率提升，按连续签到天数计算。
3. **默认大额概率降低**：不引入自适应机制时，随机区间内出大额的基准概率应设得很低（防止白嫖）。

### 3.2 核心设计思路

#### 数据结构扩展

```
CheckinSetting 新增字段：
  DecayEnabled          bool    // 是否启用衰减机制
  DecayRate             float64 // 每周衰减系数（如 0.85 表示每周奖励上限乘以0.85）
  DecayFloor            int     // 衰减下限（达到此值后不再继续降，通常 = MinQuota）
  UsageBoostEnabled     bool    // 是否启用使用加成
  UsageBoostDays        int     // 需要连续签到+使用多少天才开始提升概率（如 3）
  HighRewardThreshold   float64 // "大额"定义：占区间 [Min, Max] 的比例（如 0.8 = 80%以上算大额）
  BaseHighProbability   float64 // 默认大额概率（如 0.05 = 5%）
  BoostMaxProbability   float64 // 加成后大额概率上限（如 0.80 = 80%）
  MakeUpEnabled         bool    // 是否允许补签
  MakeUpMaxDays         int     // 最多可补签前几天（如 3）
  MakeUpCountsTowardProgress bool // 补签是否计入连续签到进度（streak/周/月加成）

Checkin 表新增字段：
  StreakDays              int   // 连续签到天数
  ConsecutiveUsageWeeks  int   // 连续签到+消费的完整周数
  ConsecutiveUsageMonths int   // 连续签到+消费的完整月数
  IsMakeUp                bool  // 是否为补签记录（补签的当天实际未签到，事后补录）
```

#### 衰减逻辑（按周）

```
衰减 = 只签到不消费，按周累计

  第 1 周（0-6 天无消费）: 有效上限 = MaxQuota
  第 2 周（7-13 天无消费）: 有效上限 = MaxQuota * DecayRate^1
  第 3 周（14-20 天无消费）: 有效上限 = MaxQuota * DecayRate^2
  ...
  第 N 周: 有效上限 = max(MaxQuota * DecayRate^(N-1), DecayFloor)

一旦当天有消费记录 → 重置衰减计数器，恢复到满区间
```

#### 加成逻辑（按周提升 + 按月再提升，上限 80%）

```
两层叠加：
  1. 周加成：连续签到+消费满 UsageBoostDays 后，每经过一个完整周（7天）提升一档
  2. 月加成：在周加成基础上，每经过一个完整月（30天）再额外提升一档

周提升公式：
  weekProgress = min(floor(consecutiveUsageWeeks) / 4, 1.0)  // 4周爬满周加成
  weekBoost = BaseHighProbability + (BoostMaxProbability - BaseHighProbability) * 0.6 * weekProgress
  // 周加成贡献总区间的 60%

月提升公式：
  monthProgress = min(floor(consecutiveUsageMonths) / 3, 1.0)  // 3个月爬满月加成
  monthBoost = (BoostMaxProbability - BaseHighProbability) * 0.4 * monthProgress
  // 月加成贡献总区间的 40%

最终大额概率：
  p = min(weekBoost + monthBoost, BoostMaxProbability)
  // 周和月叠加，但不超过 BoostMaxProbability

示例（BaseHighProbability=0.05, BoostMaxProbability=0.80）：
  无加成:                    5%
  连续签到+消费 1 周:        5% + 0.75*0.6*0.25 = 5% + 11.25% = 16.25%
  连续签到+消费 4 周:        5% + 0.75*0.6*1.0  = 5% + 45%   = 50%
  连续签到+消费 4 周 + 1 月: 50% + 0.75*0.4*0.33 = 50% + 9.9% = 59.9%
  连续签到+消费 4 周 + 3 月: 50% + 0.75*0.4*1.0  = 50% + 30%  = 80%（满额）
```

#### 补签机制

用户断签后可以补签漏掉的日期，全部在管理后台设置中配置：

```
补签流程：
  1. 用户今天来签到时，如果之前有 ≤ MakeUpMaxDays 天的断签，可以选择补签
  2. 补签记录写入 checkins 表，IsMakeUp = true
  3. 补签奖励 = 漏签当天应得的奖励（按衰减逻辑计算），但补签不发特殊星期奖励
  4. 补签后连续签到天数是否恢复，取决于 MakeUpCountsTowardProgress

配置项：
  MakeUpEnabled              = false  // 默认关闭
  MakeUpMaxDays              = 3      // 最多补签前几天
  MakeUpCountsTowardProgress = false  // 补签是否计入 streak/周/月进度

两种策略：
  MakeUpCountsTowardProgress = false（推荐）：
    - 补签只是补回那天的额度，不恢复 streak
    - 用户断签的代价（进度归零）依然存在
    - 鼓励每天按时签到，补签只是"少亏一点"

  MakeUpCountsTowardProgress = true：
    - 补签后 streak 视为连续，周/月加成照常累计
    - 对用户更友好，但降低了每日签到的紧迫感
    - 需配合风控：防止批量补签刷进度

前端展示：
  - 签到日历上断签的日期显示"补签"按钮（仅 MakeUpEnabled 且在 MakeUpMaxDays 范围内）
  - 不说明补签是否计入进度——用户只能看到日历补上了，无法推断 streak 是否恢复
  - 补签按钮数量有限（MakeUpMaxDays），用完后断签日期灰色不可操作
```

#### 奖励计算流程

```
RewardQuota(now, userId) -> int:
  1. 基础区间 = [MinQuota, EffectiveMax]
     EffectiveMax = 衰减后的实际上限
  2. if 特殊星期命中: reward = SpecialQuota 经过衰减后发放（加成概率不影响固定奖励档位，但仍可叠加 clientScore）
  3. 大额判定：
     threshold = MinQuota + (EffectiveMax - MinQuota) * HighRewardThreshold
     p = boostProbability(streak, usageDays)
     if rand() < p: 在 [threshold, EffectiveMax] 随机
     else:           在 [MinQuota, threshold) 随机
  4. ApplyClientScore(reward, clientScore)  // 叠加反脚本评分
```

#### 前端展示策略

签到界面**只展示当前保底值**，不说明任何规则机制：

```
界面显示内容：
  ✅ 今日已签到 / 签到按钮
  📅 本月签到日历
  💰 今日保底: $0.01  ← 当前 EffectiveMax 换算后的展示值
  🔥 连续签到 X 天

界面不展示的内容：
  ❌ 衰减系数、衰减周数
  ❌ 加成概率、大额判定逻辑
  ❌ 风控状态、调用次数统计
  ❌ 任何规则说明文字
```

设计原则：
- 用户只能看到"今天签到至少能拿多少"（保底 = 衰减后的 EffectiveMax），无法推断奖励如何计算
- 连续签到天数仅作为激励展示，不附带"连续几天会有什么奖励"的说明
- 这样衰减和加成机制对用户是透明的（效果可感知但不透明），避免用户逆向规则来优化薅羊毛策略
- API 返回的 `min_quota` 字段改为返回当前 EffectiveMax（而非配置的 MinQuota），让前端展示真实保底

```

### 3.3 与现有功能的交互

| 维度 | 交互方式 |
|------|---------|
| 贷款还款 | 签到奖励先计算完毕，再进入还款分配流程；衰减/加成影响的是"发放总额"，还款逻辑不变 |
| 反脚本评分 | 先计算自适应奖励，再叠加 clientScore 压制；两层独立 |
| 额度次日回收 | 衰减降低了发放量，回收量也相应降低；加成提高了发放量，回收量也相应提高 |
| 特殊星期 | 固定大额不参与衰减，但可以被 clientScore 压制（Futureppo 原设计如此） |
| **风控系统** | 见 §3.5：签到+调用数据对比，长期"签到后只调一次"自动列入风控名单 |

### 3.4 默认配置建议

```go
DecayEnabled:          false,  // 默认关闭
DecayRate:              0.85,
DecayFloor:             0,      // 0 表示 = MinQuota
UsageBoostEnabled:      false,  // 默认关闭
UsageBoostDays:         3,
HighRewardThreshold:    0.8,
BaseHighProbability:    0.05,   // 5% 基准大额概率
BoostMaxProbability:    0.80,   // 满额加成后 80%
MakeUpEnabled:          false,  // 默认关闭
MakeUpMaxDays:          3,      // 最多补签前 3 天
MakeUpCountsTowardProgress: false, // 补签不恢复 streak（推荐）
```

### 3.5 签到风控联动

#### 触发条件

用户连续 N 天（默认 14 天）签到，且每天签到后实际 API 调用次数 ≤ 阈值（默认 1 次），自动列入风控观察名单。

这针对的是"签到后只调一次 API 来重置衰减、维持奖励"的薅羊毛模式：用户利用"当天有消费就重置衰减"的规则，每天只发一个极简请求就拿到满区间奖励。

#### 判定逻辑

```
风控触发 = 连续签到天数 ≥ RiskWatchDays
          且 期间每天 API 调用次数 ≤ RiskMinDailyCalls
          且 每次调用的总消费额度 ≤ RiskMinDailyQuota

RiskWatchDays      = 14    // 连续签到多少天后开始观察
RiskMinDailyCalls  = 1     // 每天调用次数低于等于此值视为"低使用"
RiskMinDailyQuota  = 100   // 每天消费额度低于此值也视为"低使用"（100 ≈ $0.002）
```

满足条件后：
1. 用户标记为 `checkin_abuse_risk`，写入风控名单
2. 该用户的签到奖励强制压到 `MinQuota`（不再参与加成，也不走衰减——直接锁底）
3. 管理员风控面板显示该用户的签到+调用对比数据

#### 风控面板展示数据

对每个风控用户，面板展示：

| 数据项 | 来源 | 说明 |
|--------|------|------|
| 连续签到天数 | `checkins` 表 | 当前 streak |
| 每日签到奖励 | `checkins.quota_awarded` | 每天发了多少 |
| 每日 API 调用次数 | `logs` 表 (LogTypeConsume) | 按天聚合 count |
| 每日消费额度 | `logs` 表 (LogTypeConsume) | 按天聚合 sum(quota) |
| 签到/消费比 | 计算 | 每日签到奖励 ÷ 每日消费，比值越高越可疑 |
| 风控触发原因 | 系统生成 | 如"连续14天签到，日均调用1次，日均消费$0.001" |
| 列入时间 | 自动记录 | 标记时间戳 |
| 当前状态 | 风控表 | 观察/限制/已解除 |

管理员可手动解除风控标记，解除后用户恢复正常自适应逻辑。

#### 配置项

```go
// CheckinSetting 新增
RiskWatchEnabled    bool    // 是否启用签到风控联动
RiskWatchDays       int     // 连续签到多少天后开始观察
RiskMinDailyCalls   int     // 每天调用次数 ≤ 此值视为低使用
RiskMinDailyQuota   int     // 每天消费额度 ≤ 此值也视为低使用
```

#### 与其他模块的交互

| 维度 | 交互方式 |
|------|---------|
| 衰减/加成 | 风控用户直接锁 `MinQuota`，跳过衰减和加成计算 |
| 反脚本评分 | 风控锁底在 clientScore 之后执行，两层叠加 |
| 贷款还款 | 不影响——锁的是发放额，还款分配逻辑不变 |
| 额度次日回收 | 风控用户发放 = MinQuota，回收量也相应极小 |

## 四、接入优先级

| 优先级 | 功能 | 理由 |
|--------|------|------|
| P0 | 特殊星期奖励 | 改动最小（仅配置层），无数据库迁移，无事务冲突 |
| P1 | 反脚本检测（客户端评分） | 新增 service/ 文件，对现有签到链路侵入小 |
| P2 | 反脚本检测（行为评分） | 依赖 P1 已建立的评分框架 |
| P3 | 自适应奖励（衰减+加成） | 本项目新需求，需要在签到+消费数据上做分析 |
| P4 | 签到风控联动 | 依赖 P3 的消费统计数据，需在现有风控面板新增签到维度 |
| P5 | 额度次日回收 | 改动最大，涉及 DB 迁移 + 定时任务 + 与贷款还款的交互分析 |

## 五、已确认问题

1. **"使用"定义**：还贷不算消费。用户必须有实际的 API 调用消费（LogTypeConsume）才算"使用"。项目有基础套餐保证用户总有可用额度，所以"只签不用"就是真的没用过。
2. **衰减重置条件**：当天有任意金额的实际 API 消费即算"使用"，无最低门槛。
3. **连续签到中断**：已确认——允许补签，补签天数和是否计入进度均在设置中配置。默认 `MakeUpMaxDays=3`，`MakeUpCountsTowardProgress=false`（补签只补额度不恢复进度，推荐配置）。
4. **特殊星期与衰减**：已确认——特殊星期固定大额同样参与衰减。衰减后的 SpecialQuota 作为新的固定奖励发放（即 `SpecialQuota * DecayRate^周数` 作为衰减后的值）。加成概率计算中大额判定区间仍使用该衰减后的 SpecialQuota 作为上限参照。
5. **加成与反脚本**：已确认 clientScore 先于自适应计算，保证脚本无法通过"假使用"获得加成。
6. **贷款还款场景**：还贷不算消费（见第1条），签到奖励被贷款还款全额抵扣不算"使用"。这是合理的——用户如果只签到还贷从不实际调 API，确实是"只签不用"。
7. **签到风控联动**：连续 14 天签到且每天 API 调用 ≤1 次、消费额度 ≤100，自动列入风控名单并锁底 MinQuota。管理员可手动解除。面板展示签到/调用对比数据。
8. **前端展示策略**：签到界面只展示当前保底值和连续签到天数，不说明衰减/加成/风控规则。用户能感知到奖励变化但无法逆向推断机制。
9. **补签机制**：允许补签，配置项：`MakeUpEnabled`（开关）、`MakeUpMaxDays`（最多补签前几天，默认3）、`MakeUpCountsTowardProgress`（补签是否计入streak/周/月进度，默认false）。补签记录标记 `IsMakeUp=true`，补签不发特殊星期奖励。前端不说明补签是否计入进度。
