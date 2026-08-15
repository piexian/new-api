# 词元贷第三方放贷市场设计（v1）

日期：2026-08-15
状态：已确认（用户 + pi/qwen3.8-max 交叉核定）
前置：`docs/specs/2026-08-14-token-loan-design.md`（词元贷 v4）

## 1. 背景与目标

现有词元贷只有官方池：平台直接给借款人加余额。本设计引入第三方放贷市场——余额充足的用户可以把余额出借给其他用户赚利息，平台只做撮合与记账。纯娱乐玩法，平台不兜底、不抽成（费率默认 0）。

## 2. 已确认的核心决策

| 决策点 | 结论 |
|---|---|
| 违约风险 | 放贷人自担，平台不兜底 |
| 撮合模式 | 混合投放：P2P 池 + AI 空间 + 挂单市场，一次全做 |
| 利率 | 放贷人在平台区间 `[lender_rate_min, lender_rate_max]` 内自定；区间下限必须低于官方日利率，否则 P2P 无竞争力 |
| 流动性 | 闲置资金随时撤回余额；已放出部分锁定至该笔债权终结 |
| 实施范围 | 一次交付全部三种形态 |
| 违约处置 | 放贷人对每笔逾期债权三选一：延长 / 核销拉黑 / 永续；官方债权默认交 AI 审批员处置 |
| 信用分 | -50 ~ 100，初始 50，公开可查；offer 可设最低分门槛 |
| 还款计划 | funding 粒度四档 `repay_plan`，放贷人可调，借款人可向 AI 申诉减免 |

## 3. 概念与角色

### 3.1 供给侧：loan_offers（放贷供给单）

放贷人余额划入即冻结（`amount_available`），三种模式：

- **pool（P2P 池）**：`rate_fixed` 固定利率。自动撮合时按利率升序吃量。
- **ai（AI 空间）**：`rate_min`/`rate_max` 区间 + `per_loan_cap`。勾选即授权 AI 审批员在边界内定价并决定投向。区间单在没有 AI 定价时**跳过不成交**（不按下限兜底成交）。
- **order（挂单）**：`rate_fixed` + 公开展示。借款人浏览市场挑单发起申请，该单作为意向资金源优先撮资；挑中后仍走 AI 审批工单。

`ai` 与 `pool/order` 的统一视角：区间单就是"利率待 AI 定价的 offer"，撮合引擎只有一条路径（见 §6），`mode` 只做展示与校验分类。

### 3.2 需求侧

借款人流程不变：申请 → AI 审批工单 → 放款。新增可选入口"市场挑单"。一笔借款可由多个来源混合出资（1..N 条 funding，N ≤ 5）。

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
| disclaimer_agreed_at | 冗余在账户表，见 §10 |
| created_at, updated_at | 秒级时间戳 |

不变式：`amount_total = amount_available + Σ(active funding 的剩余本金)`。

### 4.2 token_loan_fundings（借款资金构成）

| 字段 | 说明 |
|---|---|
| id, loan_user_id | 借款人（一笔借款 = 同一用户的一次放款事件，1..N 条 funding 共享 borrow_event_id） |
| borrow_event_id | 放款事件 id（台账 borrow 记录 id） |
| source_type | `platform` / `pool` / `ai` / `order` |
| offer_id, lender_id | platform 时为 0 |
| amount | 原始本金（quota） |
| principal_remaining | 剩余本金（quota） |
| debt_quota | 当前债务（本金+应计利息），惰性结算承载字段，镜像 TokenLoanAccount |
| last_settled_day | 该 funding 的利息时钟 |
| rate | 执行日利率（ai 模式为 AI 定价结果） |
| repay_plan | `full` / `no_penalty` / `interest_freeze` / `principal_only`，见 §8 |
| status | `active` / `repaid` / `overdue` / `written_off` |
| due_day | 到期 loanDay（borrow day + loan_term_days） |
| penalty_started_day | 进入逾期时的 loanDay，0 = 未逾期 |
| created_at, updated_at | |

### 4.3 token_loan_accounts 扩展

新增字段：`credit_score`（默认 50）、`blacklisted_until_day`（禁借截止 loanDay）、`lender_disclaimer_agreed_at`（放贷免责声明同意时间戳）。

### 4.4 token_loan_records 扩展

还款台账冗余 `funding_id` 与 `lender_id`（放贷人收益对账，pi #21）；borrow 记录即 borrow_event。

### 4.5 账户与 funding 的关系

`TokenLoanAccount.DebtQuota`/`PrincipalQuota` 保持为借款人视图的总和口径，其值必须恒等于 Σ 该用户 active/overdue fundings 的 `debt_quota`/`principal_remaining`。计息与结算粒度下沉到 funding；账户字段只做投影与既有兼容（限额校验、状态展示）。不变式断言进测试。

## 5. 计息与结算

- 每个 funding 独立惰性结算，逻辑镜像现有 `settle()`：按 `rate` 日复利推进 `debt_quota`，`last_settled_day` 就地推进。
- `repay_plan` 对结算的影响：
  - `full`：正常复利；逾期后按罚息利率（全局固定 = 2 × 该 funding 日利率）计。
  - `no_penalty`：逾期后仍按 `rate` 计息，不产生罚息。
  - `interest_freeze`：`debt_quota` 冻结，不再增长。
  - `principal_only`：`debt_quota` 冻结且恒等于 `principal_remaining`（利息清零，调整时一次性核销未付利息）。
- **利息只在真实还款分配时计入放贷人余额**；结算（debt 增长）绝不动放贷人的账（pi #2，凭空印钞防线）。
- `custom_daily_rate` 与 `interest_free_until` 只作用于 platform funding（pi #7）：platform funding 结算时用账户级有效利率与宽限期；P2P funding 永远用自己的 `rate`，不受平台宽限穿透。
- 整数舍入：funding 复利 `math.Round` 远离零取整；多 funding 汇总与拆分用最大余数法，断言 Σ 分配 ≡ 还款额（pi #5）。

## 6. 撮合引擎

AI 审批通过金额 X 后，两阶段撮资（pi #13 简化）：

1. **定向挂单**：申请带意向 order 时，优先从该 order 出资（≤ `amount_available` 且 ≤ `per_loan_cap`，校验借款人信用分 ≥ `min_credit_score`），利率 = order 的 `rate_fixed`。
2. **统一市场**：剩余金额在所有 active offer 中吃量——固定利率单（pool/order）按 `rate_fixed` 升序；区间单（ai）仅当本次审批有 AI 出资方案时按其定价参与，无 AI 定价则跳过。每笔受 `per_loan_cap`、`amount_available`、信用分门槛约束。
3. **官方兜底**：仍不足的部分生成 platform funding，平台直接加余额（现状不变，放款永不因来源不足失败）。

无 `official_use_ai_space` 开关、无可配撮合顺序（pi #15、#17）。

AI 出资方案（审批输出 `fundings: [{offer_id, amount, rate}]`）必须在**锁定 offer 行的同一事务内**校验：`rate ∈ [rate_min, rate_max]`、`amount ≤ min(amount_available, per_loan_cap)`；越界项剔除并记录，缺额滑向官方兜底（pi #10）。

放款事务内：冻结资金 offer → funding 转移；借款人余额入账；台账写入；`lender_id != borrower_id` 硬校验（pi #18）。

## 7. 还款分配

还款事件（签到自动 / 手动提前）统一流程：

1. 结算借款人全部 active/overdue fundings（及账户投影）。
2. 还款额按各 funding **当前债务**（结算后 `debt_quota`）pro-rata 分配，最大余数法取整（pi #3：按原始本金分配会让高息 funding 永久膨胀）。
3. 每条 funding 内先息后本：`interest = debt_quota - principal_remaining`。
4. 分配到的利息计入对应放贷人余额（`cacheIncrUserQuota` 副作用对齐）；本金回补 offer 的 `amount_available`（offer 非 closed）或直接回放贷人余额（offer 已 closed）。platform funding 的本息归平台（即债务销毁，无入账）。
5. 手动提前还款手续费照旧：按抵本部分 × `repay_fee_rate`，归平台，先于分配扣除（pi #6）。
6. 违约（overdue）期间签到收入 100% 用于还款（现有签到还款比例的违约态覆盖）。

## 8. 还款计划与减免申诉

`repay_plan` 四档（§5 已定义结算语义），调整路径：

- 放贷人对自有 funding 随时可调（台账记录）。
- 借款人可发起新工单类型 **减免申诉**（利息/罚息滚到过高时说明理由）→ AI 审批员裁决，可调整任意 funding（含 platform）的 `repay_plan`。
- 裁决沿用现有工单基础设施：think 剥离、结案日志记模型与结论。

## 9. 期限、逾期与违约处置

- 借款期限 `loan_term_days`（默认 30，可配）。到期日未清的 funding 进入 `overdue`，`penalty_started_day` 落账，之后按 §5 罚息规则计息（`repay_plan` 决定是否有罚息）。
- 逾期债权处置（`overdue` → 终态）：
  - **P2P funding**：放贷人在收益台账对每笔逾期债权三选一：
    - **延长**：设新 `due_day`（罚息期间已计部分保留）。
    - **核销**：funding → `written_off`，剩余债权销毁（放贷人损失落地）；借款人 `blacklisted_until_day = 当前 + blacklist_days_on_default`、信用分扣 `credit_default_penalty`。
    - **永续**：保持 overdue 继续计息，签到继续 100% 扣还。
  - **platform funding**：逾期自动生成 AI 审批员工单，由 AI 在同样三选项中处置。
- 核销是 quota 通缩（债务销毁无对应入账），游戏机制上可接受（pi #4 方案 A）。

## 10. 信用分

- 范围 -50 ~ 100，初始 `credit_initial`（默认 50），全部参数可配。
- 按时（到期日前）全额还清一笔借款事件：+`credit_repay_bonus`（封顶 100）。
- **防刷**：借款事件持有不满 `credit_min_hold_days`（默认 3 天）即全额还清：不加分且扣 `credit_fast_repay_penalty`。
- 被核销：扣 `credit_default_penalty`（可至 -50）。
- 分数在借款申请、市场挑单、放贷人台账中可见；offer 的 `min_credit_score` 在撮合时过滤。

## 11. 合规文案

- 放贷侧**免责声明**：首次创建 offer 前弹窗（纯娱乐玩法、非真实金融、出借余额可能全部损失、平台不兜底不追偿），同意时间戳入 `lender_disclaimer_agreed_at`；放贷页面常驻提示。与 18+ 声明同机制但相互独立。
- 免责声明不替代违约机制（pi #22）。

## 12. 配置项（loan_setting 新增，全部可配）

| 键 | 默认 | 说明 |
|---|---|---|
| market_enabled | false | 市场总开关 |
| lender_min_amount | 0.10 USD | 最小入池金额 |
| lender_rate_min / lender_rate_max | 0.05% / 0.3% 每天 | 放贷利率平台区间（min 必须 < 官方 daily_rate） |
| per_loan_cap_default | 0（不限） | offer 单笔上限缺省值 |
| max_fundings_per_borrow | 5 | 单笔借款 funding 条数上限 |
| loan_term_days | 30 | 借款期限 |
| blacklist_days_on_default | 30 | 核销后禁借天数 |
| overdue_penalty_multiplier | 2.0 | 逾期罚息倍率（× funding 日利率） |
| credit_initial / credit_repay_bonus / credit_fast_repay_penalty / credit_default_penalty / credit_min_hold_days | 50 / 5 / 2 / 20 / 3 | 信用分参数 |
| market_platform_fee_rate | 0 | 平台抽成（公益站默认 0，预留） |

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

- **锁序**：fundings（id 升序）→ offers（id 升序）→ users（id 升序），全部 `lockForUpdate`（pi #9）。SQLite 无事务分支镜像现有 checkin 手动回滚模式。
- 放贷人入账走 `users.quota` int32 上界校验，溢出告警不截断（pi #8）。
- 缓存副作用：每笔还款对 N 个放贷人 `cacheIncrUserQuota`，事务提交后异步执行，与现有 BorrowLoan/RepayLoan 模式一致。
- 对敲防线：`lender_id != borrower_id`；放贷关系纳入 `MultiAccountEvidence` 多账号风控证据链（pi #18）。
- 存量迁移：上线迁移脚本先对全部存量贷款账户全量 settle，再为每个 `debt_quota > 0` 的账户生成一条 platform funding（amount=PrincipalQuota、debt_quota=DebtQuota、rate=当时有效利率、due_day 宽限一个完整期限）。AutoMigrate 负责新表与新列。
- 放贷资金划转、撤回、撮资全部操作 `amount_available`，必须同锁 offer 行；不变式 §4.1 落监控/测试（pi #11）。

## 16. 动工前核实项（pi 声称的 main 上既有阻断）

1. ~~BorrowLoan 缺 `amount<=0` 守卫~~：已核实不存在——`loan.go:259` 注释说明 usd>0 且 QuotaPerUnit=500000，最小 0.01 USD=5000 quota，amount 必然 >0。
2. SQLite 签到回滚路径的缓存补偿（`checkin.go:216` `IncreaseUserQuota(userId, netQuota, true)` 失败回滚时是否有缓存侧泄漏）：动工时先读 `IncreaseUserQuota` 确认 db=true 分支的缓存行为，有问题先单独修。

## 17. 测试策略

- model 层：funding 结算（四档 repay_plan、罚息、宽限只作用 platform）、pro-rata 分配（最大余数、Σ≡还款额、高息收敛）、撮合（利率升序、cap、信用分过滤、AI 定价校验、兜底滑移）、撤回/关闭并发、核销与信用分、迁移不变式。
- 共享 SQLite 测试的用户 id 会被回收复用，建行前按 user_id 清残留（既有约定）。
- service 层：AI 出资方案解析与越界剔除、减免申诉裁决。
- controller 层：归属校验、免责声明门槛、i18n。
- 前端：双主题 typecheck/lint/build + `i18n:sync` + `bash scripts/check-i18n.sh`。
