# 词元贷第三方放贷市场设计（v2）

日期：2026-08-15
状态：已确认（用户 + pi/qwen3.8-max 两轮交叉核定）
前置：`docs/specs/2026-08-14-token-loan-design.md`（词元贷 v4）
修订：v2 吸收 pi 二轮复审的 P0×5 / P1×5 / P2×5，见各节标注。

## 1. 背景与目标

现有词元贷只有官方池：平台直接给借款人加余额。本设计引入第三方放贷市场——余额充足的用户可以把余额出借给其他用户赚利息，平台只做撮合与记账。纯娱乐玩法，平台不兜底、不抽成。

## 2. 已确认的核心决策

| 决策点 | 结论 |
|---|---|
| 违约风险 | 放贷人自担，平台不兜底 |
| 撮合模式 | 混合投放：P2P 池 + AI 空间 + 挂单市场，一次全做 |
| 利率 | 放贷人在平台区间 `[lender_rate_min, lender_rate_max]` 内自定；下限必须低于官方日利率且 > 0 |
| 流动性 | 闲置资金随时撤回余额；已放出部分锁定至该笔债权终结 |
| 实施范围 | 一次交付全部三种形态 |
| 违约处置 | 放贷人对每笔逾期债权三选一：延长 / 核销拉黑 / 永续；官方债权默认交 AI 审批员处置（AI 不可用时自动延长并告警） |
| 信用分 | -50 ~ 100，初始 50，公开可查；offer 可设最低分门槛 |
| 还款计划 | funding 粒度四档 `repay_plan`；放贷人全权调自有 funding，AI 权限受限（§8） |

## 3. 概念与角色

### 3.1 供给侧：loan_offers（放贷供给单）

放贷人余额划入即冻结（`amount_available`），三种模式：

- **pool（P2P 池）**：`rate_fixed` 固定利率。自动撮合时按利率升序吃量。
- **ai（AI 空间）**：`rate_min`/`rate_max` 区间 + `per_loan_cap`。勾选即授权 AI 审批员在边界内定价并决定投向。区间单在没有 AI 定价时**跳过不成交**。
- **order（挂单）**：`rate_fixed` + 公开展示。借款人浏览市场挑单发起申请，该单作为意向资金源优先撮资；挑中后仍走 AI 审批工单。

`ai` 与 `pool/order` 的统一视角：区间单就是"利率待 AI 定价的 offer"，撮合引擎只有一条路径（§6），`mode` 只做展示与校验分类。

### 3.2 需求侧

借款人流程不变：申请 → AI 审批工单 → 放款。新增可选入口"市场挑单"。一笔借款可由多个来源混合出资（1..N 条 funding，N ≤ `max_fundings_per_borrow`）。

**借款闸门（P1-8）**：存在 `overdue` funding 或 `blacklisted_until_day` 未过的用户拒绝新借款（新哨兵错误，与 `ErrLoanUserDisabled` 同层），杜绝"借新还旧"永续滚动。

## 4. 数据模型

### 4.1 token_loan_offers

| 字段 | 说明 |
|---|---|
| id, lender_id | 放贷人 |
| mode | `pool` / `ai` / `order` |
| status | `active` / `paused` / `closed` |
| amount_total | 入池总额（quota） |
| amount_available | 冻结中可放贷余额（quota） |
| rate_fixed | pool/order 固定日利率 |
| rate_min, rate_max | ai 模式利率区间 |
| per_loan_cap | 单笔出资上限，0 = 不限 |
| min_credit_score | 借款人信用分门槛，默认 -50（不限制） |
| total_lent, total_interest_earned | 累计放出 / 累计利息收益（对账） |
| created_at, updated_at | 秒级时间戳 |

不变式（P2-11）：`amount_total = amount_available + Σ(active/overdue funding 的 principal_remaining)`。offer 关闭：`amount_available` 退回余额、停止新投放；存续 funding 的后续本金直接回放贷人余额。

### 4.2 token_loan_fundings（借款资金构成）

| 字段 | 说明 |
|---|---|
| id, loan_user_id | 借款人 |
| borrow_event_id | 放款事件 id（台账 borrow 记录 id），同一事件 1..N 条 funding |
| source_type | `platform` / `pool` / `ai` / `order` |
| offer_id, lender_id | platform 时为 0 |
| amount | 原始本金（quota） |
| principal_remaining | 剩余本金（quota） |
| debt_quota | 当前债务（本金+应计利息），惰性结算承载字段，镜像 TokenLoanAccount |
| last_settled_day | 该 funding 的利息时钟 |
| rate | 执行日利率（ai 模式为 AI 定价结果） |
| repay_plan | `full` / `no_penalty` / `interest_freeze` / `principal_only`，见 §8 |
| status | `active` / `overdue` / `repaid` / `written_off` |
| due_day | 到期 loanDay（borrow day + loan_term_days；延长时改写） |
| penalty_started_day | 首次发现逾期时的 loanDay，0 = 未逾期 |
| created_at, updated_at | |

状态机（P0-1）：`active ⇄ overdue → repaid`；`overdue → written_off`（终态）。`overdue` 债务清零 → `repaid`；`written_off` 不接受任何还款（P1-9）。

### 4.3 token_loan_accounts 扩展

新增字段：`credit_score`（默认 50）、`blacklisted_until_day`（禁借截止 loanDay）、`lender_disclaimer_agreed_at`（放贷免责声明同意时间戳）。

### 4.4 token_loan_records 扩展

还款台账冗余 `funding_id` 与 `lender_id`（放贷人收益对账）；borrow 记录即 borrow_event。

### 4.5 账户与 funding 的关系

`TokenLoanAccount.DebtQuota`/`PrincipalQuota` 保持为借款人视图的总和口径，恒等于 Σ 该用户 active/overdue fundings 的 `debt_quota`/`principal_remaining`。计息与结算粒度下沉到 funding；账户字段只做投影与既有兼容（限额校验、状态展示）。不变式断言进测试。

## 5. 计息与结算

- 每个 funding 独立惰性结算，逻辑镜像现有 `settle()`：按 `rate` 日复利推进 `debt_quota`，`last_settled_day` 就地推进。
- **逾期判定时纯计算（P0-1）**：`today > due_day` 时按罚息利率计息，不依赖状态翻转。罚息利率 = `rate × overdue_penalty_multiplier`（全局固定，默认 2.0）。
- `repay_plan` 对结算的影响：
  - `full`：正常复利；逾期后按罚息利率计。
  - `no_penalty`：逾期后仍按 `rate` 计息，不产生罚息。
  - `interest_freeze`：`debt_quota` 冻结，不再增长。
  - `principal_only`：`debt_quota` 冻结且等于 `principal_remaining`（调整时一次性核销未付利息，见 §8）。
- **利息只在真实还款分配时计入放贷人余额**；结算（debt 增长）绝不动放贷人的账。
- `custom_daily_rate` 与 `interest_free_until` 只作用于 platform funding：platform funding 结算时用账户级有效利率与宽限期；P2P funding 永远用自己的 `rate`，不受平台宽限穿透。
- 整数舍入：funding 复利 `math.Round` 远离零取整；多 funding 汇总与拆分用最大余数法，断言 Σ 分配 ≡ 还款额。

## 6. 撮合引擎

AI 审批通过金额 X 后，两阶段撮资：

1. **定向挂单**：申请带意向 order 时，优先从该 order 出资（≤ `amount_available` 且 ≤ `per_loan_cap`，校验借款人信用分 ≥ `min_credit_score`），利率 = order 的 `rate_fixed`。
2. **统一市场**：剩余金额在所有 active offer 中吃量——固定利率单（pool/order）按 `rate_fixed` 升序；区间单（ai）仅当本次审批有 AI 出资方案时按其定价参与，无 AI 定价则跳过。每笔受 `per_loan_cap`、`amount_available`、信用分门槛约束。
3. **官方兜底**：仍不足的部分生成 platform funding，平台直接加余额（放款永不因来源不足失败）。

无可配撮合顺序、无 `official_use_ai_space` 开关。

AI 出资方案（审批输出 `fundings: [{offer_id, amount, rate}]`）必须在**锁定 offer 行的同一事务内**校验：`rate ∈ [rate_min, rate_max]`、`amount ≤ min(amount_available, per_loan_cap)`；越界项剔除并记录，缺额滑向官方兜底。

放款事务内：冻结资金 offer → funding 转移；借款人余额入账；台账写入；`lender_id != borrower_id` 硬校验。

## 7. 还款分配

还款事件（签到自动 / 手动提前）统一流程：

1. 结算借款人全部 active/overdue fundings（及账户投影）。
2. 还款额按各 funding **当前债务**（结算后 `debt_quota`）pro-rata 分配，最大余数法取整。
3. 每条 funding 内先息后本：`interest = debt_quota - principal_remaining`。
4. 分配到的利息计入对应放贷人余额（`cacheIncrUserQuota` 副作用对齐）；本金回补 offer 的 `amount_available`（offer 非 closed）或直接回放贷人余额（offer 已 closed）。platform funding 的本息归平台（债务销毁，无入账）。
5. 手动提前还款手续费照旧：按抵本部分 × 现有 `repay_fee_rate`，归平台，先于分配扣除。v1 不设 P2P 利息抽成键（P2-13，避免命名双轨）。
6. 逾期期间签到收入 100% 用于还款，分配沿用全量 pro-rata（P2-12：逾期 funding 因罚息债务膨胀自然多分，机制自洽）。

## 8. 还款计划与减免申诉

`repay_plan` 四档（§5 已定义结算语义）。**调整规则（P1-10）**：

- 调整必须 settle-first（先结算到当天再改档）；**已结算利息不回溯**（改档时点之前的利息/罚息保留）。
- `principal_only` 语义钉死：调整时**一次性**核销未付利息（`debt_quota := principal_remaining`），此后冻结——不是持续等式。

**调整权限（P0-2）**：

| 操作者 | platform funding | P2P funding |
|---|---|---|
| 放贷人本人 | — | 全四档随时可调（台账记录） |
| AI 审批员（减免申诉裁决） | 全四档 | 仅 `full → no_penalty → interest_freeze` 单向降档；`principal_only` 永远需放贷人本人操作 |

AI 裁决 JSON 走现有字段白名单 + 钳制模式，越权调整直接拒绝并记录（防 prompt 注入批量免息，AI 不得替放贷人放弃全部利息）。

借款人可发起新工单类型 **减免申诉**（利息/罚息滚到过高时说明理由）→ AI 审批员裁决。裁决沿用现有工单基础设施：think 剥离、结案日志记模型与结论。

## 9. 期限、逾期与违约处置

- 借款期限 `loan_term_days`（默认 30，可配）。
- **逾期触发（P0-1）**：罚息在结算中按 `today > due_day` 纯计算生效，任何读路径都可安全投影；状态翻转（`active → overdue`、`penalty_started_day` 落账）与副作用（AI 处置工单创建）仅在**写事务**（还款/借款/放贷人操作）内执行，以 funding_id 幂等防重（并发双还款不双翻状态、不双建工单）。
- 逾期债权处置（P0-5 资金后果钉死）：
  - **延长**：改写 `due_day`（已计罚息保留）。不限次数（风险自担）；按时还清加分的判定基准为**新 due_day**（否则没人敢延长）。
  - **核销**：funding → `written_off`（终态）。债权销毁分两部分：`principal_remaining` 从 offer 冻结池移出（`amount_total` 同步减，放贷人损失落地）+ 未付利息（借款人免除、放贷人从未收到，无账可冲）。借款人 `blacklisted_until_day = 当前 + blacklist_days_on_default`，每条被核销 funding 扣 `credit_default_penalty`（按下限 -50 截断，P1-9）。
  - **永续**：保持 overdue 继续计息，签到继续 100% 扣还。offer 此时可正常 close（停止新投放，后续本金回放贷人余额）。永续 funding 全部还清 → **黑名单立即解除**（还款激励；核销不可逆）。
- **platform funding**：逾期自动生成 AI 审批员工单（幂等，同上三选项）。**兜底**：`ai_enabled=false` 或 AI 调用失败时自动按"延长一个期限"处理并 SysError 告警（P0-5）。

## 10. 信用分

- 范围 -50 ~ 100，初始 `credit_initial`（默认 50），全部参数可配。
- 按时（当前 due_day 前）全额还清一笔借款事件：+`credit_repay_bonus`（封顶 100）。**金额门槛（P0-4）**：事件本金 < `credit_min_borrow_usd`（默认 1 USD，可配）不计分——堵死"借 0.10 USD 持有 3 天"的廉价刷分循环。
- **防刷**：持有不满 `credit_min_hold_days`（默认 3 天）即全额还清：不加分且扣 `credit_fast_repay_penalty`。
- 被核销：每条 funding 扣 `credit_default_penalty`（可至 -50）。
- 纵深防御（P1-7）：`lender_rate_min` 配置层校验 > 0（建议 ≥ 0.01%/天）——利率地板是刷分成本的定价器。
- 分数在借款申请、市场挑单、放贷人台账中可见；offer 的 `min_credit_score` 在撮合时过滤。

## 11. 合规文案

- 放贷侧**免责声明**：首次创建 offer 前弹窗（纯娱乐玩法、非真实金融、出借余额可能全部损失、平台不兜底不追偿），同意时间戳入 `lender_disclaimer_agreed_at`；放贷页面常驻提示。与 18+ 声明同机制但相互独立。
- 免责声明不替代违约机制。

## 12. 配置项（loan_setting 新增，全部可配）

| 键 | 默认 | 说明 |
|---|---|---|
| market_enabled | false | 市场总开关 |
| lender_min_amount | 0.10 USD | 最小入池金额 |
| lender_rate_min / lender_rate_max | 0.05% / 0.3% 每天 | 放贷利率区间；校验 `lender_rate_min > 0` 且 < 官方 daily_rate |
| per_loan_cap_default | 0（不限） | offer 单笔上限缺省值 |
| max_fundings_per_borrow | 5 | 单笔借款 funding 条数上限 |
| loan_term_days | 30 | 借款期限 |
| blacklist_days_on_default | 30 | 核销后禁借天数 |
| overdue_penalty_multiplier | 2.0 | 逾期罚息倍率（× funding 日利率） |
| credit_initial / credit_repay_bonus / credit_fast_repay_penalty / credit_default_penalty / credit_min_hold_days / credit_min_borrow_usd | 50 / 5 / 2 / 20 / 3 / 1.0 | 信用分参数 |

提前还款手续费沿用现有 `repay_fee_rate`，v1 不新增平台抽成键。

## 13. 用户端 UI（双主题 parity）

词元贷下新增"放贷市场"区（登录可见，沿用顶栏入口）：

- **我的供给**：offer 列表与 CRUD（创建/暂停/恢复/关闭）、闲置资金撤回、逾期债权决策按钮（延长/核销/永续）、`repay_plan` 调整。
- **收益台账**：funding 粒度明细（放出、已回本息、状态、借款人信用分）。
- **市场浏览**：order 挂单列表（金额、利率、放贷人匿名、信用分门槛）；借款人挑单发起申请入口。
- **信用分**：借款页与放贷页展示自己分数；挑单时可见对方分数。
- 免责声明弹窗 + 常驻文案；减免申诉工单入口。

## 14. 管理端与日志

- 管理员词元贷页新增：offers 列表、fundings 明细、市场总览（总供给/在贷/累计利息），只读。
- 新日志模板：`loan.offer_create` / `loan.offer_close` / `loan.offer_withdraw` / `loan.funding_matched` / `loan.interest_settled` / `loan.default_decision` / `loan.repay_plan_change` / `loan.credit_change` / `loan.disclaimer_agreed`。
- 复用现有管理端 API 风格（`/api/user/loan/admin/...`）。

## 15. 工程约束

- **锁序**：fundings（id 升序）→ offers（id 升序）→ users（id 升序），全部 `lockForUpdate`。SQLite 无事务分支镜像现有 checkin 手动回滚模式。
- 放贷人入账走 `users.quota` int32 上界校验，溢出告警不截断。
- 缓存副作用：每笔还款对 N 个放贷人 `cacheIncrUserQuota`，事务提交后异步执行。
- 对敲防线：`lender_id != borrower_id`；放贷关系纳入 `MultiAccountEvidence` 多账号风控证据链。
- 存量迁移：上线迁移先对全部存量贷款账户全量 settle，再为每个 `debt_quota > 0` 的账户生成一条 platform funding（amount=PrincipalQuota、debt_quota=DebtQuota、rate=当时有效利率、due_day=迁移日+loan_term_days）。`interest_free_until` 剩余宽限由 platform funding 继续承载，P2P 不继承。AutoMigrate 负责新表与新列。
- 放贷资金划转、撤回、撮资全部操作 `amount_available`，必须同锁 offer 行；§4.1 不变式落测试。

## 16. 动工前修复项（先于市场功能的首个 commit）

1. **BorrowLoan `amount<=0` 守卫恢复（P0-3）**：`model/loan.go:259` 注释"amount 必然 > 0"不成立——`common.QuotaPerUnit` 是运行时可改的 var（`model/option.go:696`），管理员调小后 0.01 USD 可换算出 0 quota。补守卫并删除错误注释。
2. **SQLite 签到回滚缓存补偿（P1-6）**：已确证 `model/checkin.go` 全文无 `cacheDecrUserQuota`，回滚路径存在缓存泄漏。市场还款会把缓存增量模式扩散到 N 个放贷人，先修。

## 17. 测试策略

- model 层：funding 结算（四档 repay_plan、罚息、宽限只作用 platform、settle-first 改档不回溯）、pro-rata 分配（最大余数、Σ≡还款额、高息收敛）、撮合（利率升序、cap、信用分过滤、AI 定价校验、兜底滑移）、撤回/关闭并发、核销与信用分（金额门槛、按 funding 扣分、下限截断）、迁移不变式（含存量宽限承载、批量同日到期无副作用，P2-14）。
- overdue 触发幂等：并发双还款不双翻状态/不双建工单（P2-15）。
- 借款闸门：overdue/黑名单用户借款被拒。
- service 层：AI 出资方案解析与越界剔除；AI repay_plan 权限边界（越权调整被拒，P2-15）；减免申诉裁决。
- 刷分成本断言：最小金额 + 利率地板下刷分循环不计分（P2-15）。
- 黑名单出口：永续全还清解除、written_off 拒绝还款。
- 共享 SQLite 测试的用户 id 会被回收复用，建行前按 user_id 清残留（既有约定）。
- controller 层：归属校验、免责声明门槛、i18n。
- 前端：双主题 typecheck/lint/build + `i18n:sync` + `bash scripts/check-i18n.sh`。
