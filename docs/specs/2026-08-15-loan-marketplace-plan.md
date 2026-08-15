# 词元贷第三方放贷市场 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 spec `docs/specs/2026-08-15-loan-marketplace-design.md`（v2）的第三方放贷市场（P2P 池 + AI 空间 + 挂单市场 + 信用分 + 违约处置 + repay_plan）并上线。

**Architecture:** 在既有词元贷（model/loan.go 账户+台账、model/checkin.go 签到钩子、service/loan_officer*.go AI 业务员）上叠加：新表 `token_loan_offers`/`token_loan_fundings`，计息粒度下沉到 funding（账户行保持投影兼容），撮合引擎在放款事务内两阶段撮资，还款 pro-rata 分配。前端双主题各加"放贷市场"区。

**Tech Stack:** Go/GORM（SQLite/MySQL/PG 三库兼容）、shopspring/decimal、React、bun。

## Global Constraints

- spec 唯一权威：`docs/specs/2026-08-15-loan-marketplace-design.md`（v2），每条任务隐含遵循
- 金额一律整数 quota；USD↔quota 换算走 `decimal` + `common.QuotaFromDecimalChecked`（common/quota_math.go:145）
- 行锁一律 `lockForUpdate(tx)`（model/locking.go:20）；**锁序：fundings(id 升序) → offers(id 升序) → users(id 升序)**
- 用户 quota 变动沿用既有模式：事务内 `gorm.Expr("quota ± ?")` 直写 + 提交后异步 `cacheIncrUserQuota/cacheDecrUserQuota`
- **利息只在真实还款分配时计入放贷人余额**；结算（debt 增长）绝不动放贷人的账
- 不直接调 `encoding/json`，用 `common.Marshal/Unmarshal/UnmarshalJsonStr`
- 新 controller 错误响应一律 `common.ApiErrorI18n(c, i18n.MsgXxx)`；key 加到 `i18n/keys.go` 并补齐三语言 yaml
- 数据库变更必须兼容 SQLite/MySQL>=5.7.8/PG>=9.6；新表新列走 AutoMigrate（model/main.go）
- 共享 SQLite 测试的用户 id 会被回收复用：测试建行前先按 user_id 删除残留 loan 数据（见 model/loan_admin_test.go 既有处理）
- Go 代码 `gofmt`；测试放同包 `*_test.go`
- 前端双主题 parity；default 主题 2 空格缩进、单引号、无分号、import 排序、无 console；i18n 走 patch 工具（`bun run i18n:apply -- <patch.json>` + `i18n:sync`），禁止直接改 locales/*.json
- 完成后必须跑：`go build ./...`、`go test ./...`、`bash scripts/check-i18n.sh`、两主题 build
- Conventional Commits；不提交 `.ccg/` 等工具文件；版本号不变（v1.0.0-rc.21）

## 关键设计落地说明（spec 未细化的解释，动工前已确认）

- **撮合触发点**：现有产品里借款是限额内即时放款（`BorrowLoan`），AI 业务员负责协商额度/利率/宽限。因此撮合在 `BorrowLoan` 内执行：定向挂单 → 固定利率单升序 → AI 区间单（同步调用一次 AI 定价，失败/超界则跳过该来源）→ 官方兜底。AI 定价调用在 DB 事务外发起，结果在锁定 offer 的事务内校验执行。
- **borrow_event**：复用 `token_loan_records` 的 borrow 行 id 作为 `borrow_event_id`，不新建事件表。

## 生产上线配置（Task 22 用）

- `market_enabled=true`、`lender_min_amount`=0.10 USD、`lender_rate_min`=0.0005（0.05%/天，< 官方 0.001）、`lender_rate_max`=0.003（0.3%/天）
- `loan_term_days`=30、`blacklist_days_on_default`=30、`overdue_penalty_multiplier`=2.0、`max_fundings_per_borrow`=5
- `credit_initial`=50、`credit_repay_bonus`=5、`credit_fast_repay_penalty`=2、`credit_default_penalty`=20、`credit_min_hold_days`=3、`credit_min_borrow_usd`=1.0

---

### Task 1: 动工前修复（spec §16）

**Files:**
- Modify: `model/loan.go:250-260`（BorrowLoan amount 守卫）、`model/loan.go:374-386`（RepayLoan 同样位置）
- Modify: `model/checkin.go:162-232`（SQLite 无事务分支缓存补偿）
- Test: `model/loan_test.go`、`model/checkin_loan_test.go`

**Interfaces:**
- Produces: `BorrowLoan`/`RepayLoan` 对换算后 amount<=0 返回 `ErrLoanInvalidAmount`

- [ ] **Step 1: 写失败测试**——`t.Setenv` 或直接改 `common.QuotaPerUnit = 1`（测试内 defer 恢复 500000），`BorrowLoan(uid, "0.01")` 断言返回 `ErrLoanInvalidAmount`；RepayLoan 同理
- [ ] **Step 2: 运行确认失败**——`go test ./model/ -run 'TestBorrowLoanZeroQuota|TestRepayLoanZeroQuota' -v`，当前会错误成功
- [ ] **Step 3: 实现**——两处 `QuotaFromDecimalChecked` 之后加 `if amount <= 0 { return nil, ErrLoanInvalidAmount }`；删除 `model/loan.go:259-260` 的错误注释（QuotaPerUnit 是运行时可改 var，model/option.go:696）
- [ ] **Step 4: SQLite 缓存补偿**——先读 `model/user.go` 的 `IncreaseUserQuota(id, quota, true)` 确认 db=true 分支是否触碰缓存；确认 `userCheckinWithoutTransaction` 在 `IncreaseUserQuota` 失败回滚后缓存与 DB 是否一致（若 IncreaseUserQuota 先写缓存后写库，则回滚路径需 `cacheDecrUserQuota(userId, netQuota)` 补偿），按实际读到的行为修复并补测试
- [ ] **Step 5: 全量 model 测试**——`go test ./model/ -count=1`
- [ ] **Step 6: Commit**——`fix(loan): guard zero-quota amounts and SQLite checkin cache compensation`

### Task 2: loan_setting 市场配置扩展

**Files:**
- Modify: `setting/operation_setting/loan_setting.go`
- Test: `setting/operation_setting/loan_setting_test.go`

**Interfaces:**
- Produces（LoanSetting 新字段，json key 即配置键）：
  `MarketEnabled bool \`json:"market_enabled"\``、`LenderMinAmount int64`（quota）`json:"lender_min_amount"`、`LenderRateMin float64 json:"lender_rate_min"`、`LenderRateMax float64 json:"lender_rate_max"`、`PerLoanCapDefault int64 json:"per_loan_cap_default"`、`MaxFundingsPerBorrow int json:"max_fundings_per_borrow"`、`LoanTermDays int json:"loan_term_days"`、`BlacklistDaysOnDefault int json:"blacklist_days_on_default"`、`OverduePenaltyMultiplier float64 json:"overdue_penalty_multiplier"`、`CreditInitial int json:"credit_initial"`、`CreditRepayBonus int json:"credit_repay_bonus"`、`CreditFastRepayPenalty int json:"credit_fast_repay_penalty"`、`CreditDefaultPenalty int json:"credit_default_penalty"`、`CreditMinHoldDays int json:"credit_min_hold_days"`、`CreditMinBorrowUsd float64 json:"credit_min_borrow_usd"`
- Produces: `ValidateLoanMarketSetting(s *LoanSetting) error`——校验 `LenderRateMin > 0`、`LenderRateMin < s.DailyRate`、`LenderRateMin <= LenderRateMax`、`MaxFundingsPerBorrow >= 1 && <= 10`；在配置保存路径调用（找到 loan_setting 的 update/save 入口挂载）

- [ ] **Step 1: 写失败测试**——默认值断言（market_enabled=false、lender_rate_min=0.0005、lender_rate_max=0.003、loan_term_days=30、blacklist_days_on_default=30、overdue_penalty_multiplier=2.0、max_fundings_per_borrow=5、credit_initial=50、credit_repay_bonus=5、credit_fast_repay_penalty=2、credit_default_penalty=20、credit_min_hold_days=3、credit_min_borrow_usd=1.0、lender_min_amount=50000 quota 即 0.10 USD）+ `ValidateLoanMarketSetting` 边界用例（rate_min=0 拒绝、rate_min>=daily_rate 拒绝、rate_min>rate_max 拒绝）
- [ ] **Step 2: 运行确认失败**——`go test ./setting/operation_setting/ -run TestLoanMarket -v`
- [ ] **Step 3: 实现**——结构体加字段 + 默认值 + Validate 函数；挂到配置保存入口
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): add marketplace settings with validation`

### Task 3: 数据模型——offers/fundings 表与既有表扩展

**Files:**
- Create: `model/loan_market.go`
- Modify: `model/loan.go`（TokenLoanAccount 加 `CreditScore int`、`BlacklistedUntilDay int`、`LenderDisclaimerAgreedAt int64`；TokenLoanRecord 加 `FundingId int64`、`LenderId int`）
- Modify: `model/main.go`（AutoMigrate 列表追加 `&TokenLoanOffer{}, &TokenLoanFunding{}`）
- Test: `model/loan_market_test.go`

**Interfaces:**
- Produces:
  - `type TokenLoanOffer struct { Id int; LenderId int; Mode string; Status string; AmountTotal int64; AmountAvailable int64; RateFixed float64; RateMin float64; RateMax float64; PerLoanCap int64; MinCreditScore int; TotalLent int64; TotalInterestEarned int64; CreatedAt int64; UpdatedAt int64 }`，`TableName()="token_loan_offers"`；Mode 常量 `LoanOfferModePool/LoanOfferModeAi/LoanOfferModeOrder` = "pool"/"ai"/"order"；Status 常量 `LoanOfferStatusActive/LoanOfferStatusPaused/LoanOfferStatusClosed`
  - `type TokenLoanFunding struct { Id int64; LoanUserId int; BorrowEventId int64; SourceType string; OfferId int; LenderId int; Amount int64; PrincipalRemaining int64; DebtQuota int64; LastSettledDay int; Rate float64; RepayPlan string; Status string; DueDay int; PenaltyStartedDay int; CreatedAt int64; UpdatedAt int64 }`，`TableName()="token_loan_fundings"`；SourceType 常量 `LoanFundingPlatform/LoanFundingPool/LoanFundingAi/LoanFundingOrder`；RepayPlan 常量 `LoanRepayFull/LoanRepayNoPenalty/LoanRepayInterestFreeze/LoanRepayPrincipalOnly`；Status 常量 `LoanFundingActive/LoanFundingOverdue/LoanFundingRepaid/LoanFundingWrittenOff`
  - `GetLoanAccountReadOnly` 不变；账户新列 AutoMigrate 自动补默认值（CreditScore 需在迁移任务中回填 credit_initial）

- [ ] **Step 1: 写失败测试**——建表后 CRUD 冒烟：创建 offer/funding 行并读回断言字段；`DB.Migrator().HasColumn(&TokenLoanAccount{}, "credit_score")` 为 true
- [ ] **Step 2: 运行确认失败**——`go test ./model/ -run TestLoanMarketModels -v`
- [ ] **Step 3: 实现**——按 Interfaces 定义结构体与常量；AutoMigrate 注册；gorm tag 风格对齐既有（bigint、varchar(16)、index）
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): add marketplace offer/funding models`

### Task 4: funding 结算引擎

**Files:**
- Create: `model/loan_funding.go`
- Test: `model/loan_funding_test.go`

**Interfaces:**
- Produces（后续所有资金任务依赖）：
  - `settleFunding(f *TokenLoanFunding, acc *TokenLoanAccount, now time.Time)`——单 funding 惰性结算，仅改内存：基础利率 `f.Rate`（platform 时走 `effectiveRate(acc)` 且尊重 `acc.InterestFreeUntil` 宽限）；`today > f.DueDay` 且 `f.RepayPlan == LoanRepayFull` 时利率 ×= `OverduePenaltyMultiplier`；`LoanRepayNoPenalty` 逾期不乘罚息倍率；`LoanRepayInterestFreeze`/`LoanRepayPrincipalOnly` 不增长。复利公式镜像 `settle()`（math.Pow + math.Round 远离零取整），`LastSettledDay` 就地推进
  - `ProjectFundingDebt(f *TokenLoanFunding, acc *TokenLoanAccount, now time.Time) int64`——只读投影
  - `syncAccountFromFundings(acc *TokenLoanAccount, fundings []TokenLoanFunding)`——把 Σ(debt_quota)/Σ(principal_remaining) 写回 acc.DebtQuota/PrincipalQuota（账户投影兼容，spec §4.5）

- [ ] **Step 1: 写失败测试**——表驱动用例：①正常复利与既有 settle 同值（单 platform funding 场景对拍）；②逾期后罚息倍率生效；③no_penalty 逾期不罚；④interest_freeze 冻结；⑤principal_only 冻结且 debt==principal_remaining；⑥platform funding 宽限期内不计息、P2P funding 同账户宽限期照常计息（spec §5 穿透防线）
- [ ] **Step 2: 运行确认失败**——`go test ./model/ -run TestSettleFunding -v`
- [ ] **Step 3: 实现**——按上接口
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): funding-granularity settlement engine`

### Task 5: 存量迁移（platform funding 生成 + credit_score 回填）

**Files:**
- Modify: `model/loan_market.go`
- Test: `model/loan_market_test.go`

**Interfaces:**
- Produces: `MigrateLoanToFundings() error`——幂等迁移：全量账户先 settle 落盘；`debt_quota > 0` 且无 platform funding 的账户生成一条 platform funding（Amount=PrincipalQuota、PrincipalRemaining=PrincipalQuota、DebtQuota=DebtQuota、Rate=当时 effectiveRate、DueDay=迁移日+loan_term_days、RepayPlan=full、Status=active、LastSettledDay=账户 LastSettledDay）；全部账户 `credit_score` 为 0 时回填 `credit_initial`。启动时调用一次（挂在既有迁移/初始化路径，需幂等可重入）

- [ ] **Step 1: 写失败测试**——预置两类账户（有宽限 interest_free_until 的、无宽限的）+ 零值 credit_score；跑迁移后断言：platform funding 字段正确、宽限账户 funding 结算仍尊重宽限、credit_score==50；二次执行不产生重复 funding（幂等）
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现 + 挂启动路径**
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): migrate legacy loans to platform fundings`

### Task 6: offer 生命周期（model 层）

**Files:**
- Modify: `model/loan_market.go`
- Test: `model/loan_market_test.go`

**Interfaces:**
- Produces（哨兵错误 + 函数，controller 直接消费）：
  - 错误：`ErrLoanMarketDisabled`、`ErrLoanDisclaimerRequired`、`ErrLoanOfferNotFound`、`ErrLoanOfferInvalidParams`、`ErrLoanOfferNotActive`、`ErrLoanNothingToWithdraw`
  - `AgreeLenderDisclaimer(userId int) error`——幂等写 `lender_disclaimer_agreed_at`
  - `CreateLoanOffer(lenderId int, mode string, amountUsd, rateFixed string, rateMin, rateMax float64, perLoanCap int64, minCreditScore int) (*TokenLoanOffer, error)`——事务内：market_enabled/免责声明/模式与利率区间校验（pool/order 需 rateFixed ∈ [LenderRateMin, LenderRateMax]；ai 需 [RateMin,RateMax] ⊆ 配置区间且 perLoanCap>0；amount ≥ LenderMinAmount）→ 锁 users 行扣 quota → 建 offer（AmountTotal=AmountAvailable=amount）。int32 上界校验
  - `SetLoanOfferStatus(lenderId int, offerId int, status string) error`——仅 active/paused 互转；关闭走 `CloseLoanOffer`
  - `CloseLoanOffer(lenderId int, offerId int) error`——事务内锁 offer：status→closed，`AmountAvailable` 退回用户余额（异步缓存增量），存续 funding 不受影响
  - `WithdrawLoanOffer(lenderId int, offerId int) (int64, error)`——退回全部 AmountAvailable（offer 保持 active 可再充值？否——v1 简化：撤回=退回闲置余额，offer 保留；返回退回额度）
  - `GetUserLoanOffers(lenderId int) ([]TokenLoanOffer, error)`、`GetLoanOfferById(id int) (*TokenLoanOffer, error)`

- [ ] **Step 1: 写失败测试**——免责声明未同意拒绝创建；rate 越界拒绝；创建扣款原子性（余额不足回滚）；撤回退回金额正确；关闭后不变式 `amount_total = amount_available + Σfunding 剩余本金` 成立
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**——金额解析复用 BorrowLoan 的 decimal 模式；锁 users 行用既有 `tx.Select(...).First(&user)` 模式
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): offer lifecycle (create/pause/close/withdraw)`

### Task 7: 撮合引擎

**Files:**
- Create: `model/loan_matcher.go`
- Test: `model/loan_matcher_test.go`

**Interfaces:**
- Produces:
  - `type FundingPlan struct { OfferId int; LenderId int; SourceType string; Amount int64; Rate float64 }`
  - `MatchLoanFundings(tx *gorm.DB, borrowerId int, creditScore int, amount int64, intendedOrderId int, aiPriced []FundingPlan) ([]FundingPlan, error)`——事务内调用（调用方已开事务并锁 borrower 行）：①intendedOrderId>0 时锁该 offer 校验（active、order 模式、available、cap、lender≠borrower、creditScore ≥ min_credit_score）吃量；②剩余按 rate_fixed 升序遍历 active 的 pool/order offer（跳过 lender==borrower、信用分不足、available=0；每笔 min(剩余, available, cap>0?cap:∞)）；③`aiPriced`（service 层 AI 定价结果，事务外预先产出）逐条校验：offer 存在且 active 且 ai 模式、rate ∈ [rate_min,rate_max]、amount ≤ min(available, cap)，越界剔除；④仍不足 → 调用方补 platform FundingPlan。返回计划总条数 > `MaxFundingsPerBorrow` 时截断（优先保留低利率与定向单）
  - 撮合只读+内存计算，**不改库**；改库在 Task 8 放款事务里按计划执行（锁 offer 行二次校验 available 后扣减）

- [ ] **Step 1: 写失败测试**——①利率升序吃量顺序；②cap 与 available 限制；③信用分门槛过滤；④lender==borrower 跳过；⑤AI 定价越界剔除（rate 超界、amount 超 cap）；⑥定向挂单优先；⑦全部来源不足时返回部分计划（调用方补 platform）；⑧条数截断
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): funding matcher engine`

### Task 8: BorrowLoan 改造——funding 放款

**Files:**
- Modify: `model/loan.go`（BorrowLoan）
- Modify: `model/loan_market.go`
- Test: `model/loan_test.go`、`model/loan_market_test.go`

**Interfaces:**
- Produces: `BorrowLoan` 签名扩展为 `BorrowLoan(userId int, amountUsd string, intendedOrderId int, aiPriced []FundingPlan) (*TokenLoanAccount, []TokenLoanFunding, error)`（controller/service 两处调用方同步改）；新增错误 `ErrLoanBlacklisted`、`ErrLoanHasOverdue`
- Consumes: `MatchLoanFundings`（Task 7）、`settleFunding/syncAccountFromFundings`（Task 4）

- [ ] **Step 1: 写失败测试**——①借款闸门：blacklisted_until_day 未过 / 有 overdue funding 拒绝；②纯官方路径生成一条 platform funding 且 BorrowEventId=台账 borrow 行 id；③混合投放：预置 pool offer，借款后 funding 两条（pool+platform）、offer.AmountAvailable 扣减、不变式成立；④超额来源时 platform 兜底金额正确
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**——事务内：锁用户行+账户（既有校验全部保留，含 Task 1 的 amount 守卫）→ 借款闸门 → settle → 额度校验 → `MatchLoanFundings` → 按计划锁 offer 行（id 升序）二次校验 available 并扣减 → 批量 Create fundings（DueDay=loanDay+LoanTermDays）→ 写台账 borrow 行 → `syncAccountFromFundings` → quota 入账。SQLite 分支镜像
- [ ] **Step 4: 测试通过 + 回归 `go test ./model/ -count=1`**
- [ ] **Step 5: Commit**——`feat(loan): disburse borrows as fundings with market matching`

### Task 9: 还款分配改造（RepayLoan + applyCheckinRepay）

**Files:**
- Modify: `model/loan.go`（RepayLoan、applyCheckinRepay 语义整体替换为 funding 分配）
- Create: `model/loan_repay.go`
- Test: `model/loan_repay_test.go`

**Interfaces:**
- Produces:
  - `distributeRepayment(tx *gorm.DB, acc *TokenLoanAccount, fundings []TokenLoanFunding, repay int64, now time.Time) (*LoanRepayInfo, []RepayAllocation, error)`——①逐 funding settleFunding；②按当前 debt_quota pro-rata 分配（最大余数法，Σ≡repay）；③每条内先息后本；④更新 funding 行（debt/principal_remaining 扣减、debt=0 → status=repaid）；⑤`syncAccountFromFundings`
  - `type RepayAllocation struct { FundingId int64; LenderId int; OfferId int; SourceType string; Amount int64; InterestPart int64; PrincipalPart int64 }`
  - `settleRepayAllocations(tx *gorm.DB, allocs []RepayAllocation) error`——对每条 alloc：利息部分 `users.quota += interest`（platform 跳过）累计 offer.TotalInterestEarned；本金部分 offer 非 closed → `amount_available += principal`，closed → `users.quota += principal`；台账写 repay 行（含 funding_id/lender_id 冗余）。返回各放贷人入账清单供事务外缓存增量

- [ ] **Step 1: 写失败测试**——①pro-rata 最大余数（3 条 funding 分 100 quota，Σ≡100）；②高息 funding 多分（当前债务比例）；③先息后本；④放贷人入账=利息、本金回补 offer；⑤offer 已关闭本金回放贷人余额；⑥platform funding 无入账（债务销毁）；⑦funding 全清 → status=repaid 且账户投影归零；⑧手续费仍按抵本部分计（既有 RepayFeeRate 行为回归）
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**——RepayLoan 事务重构：锁账户 → 锁 fundings（id 升序）→ 还款额/手续费计算（既有逻辑）→ distributeRepayment → settleRepayAllocations → 扣用户余额；提交后异步缓存：借款人 decr、各放贷人 incr
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): pro-rata repayment distribution across fundings`

### Task 10: 签到还款改造 + 逾期 100% 扣还

**Files:**
- Modify: `model/checkin.go`（userCheckinWithTransaction / userCheckinWithoutTransaction 的还款段）
- Modify: `model/checkin_loan_test.go`

**Interfaces:**
- Consumes: `distributeRepayment`（Task 9）；新增 `HasOverdueFundings(tx, userId) (bool, error)`
- Produces: 签到还款行为变更——有 overdue funding 时净额=0（全额还款），否则维持既有"签到自动还款"行为（spec §7.6）

- [ ] **Step 1: 写失败测试**——①有 overdue funding 时签到奖励全额抵债（netQuota=0）；②无逾期维持既有拆分；③双分支行为一致；④回滚路径（Task 1 修复后）funding 状态一致
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**——还款段替换为 distributeRepayment + settleRepayAllocations 调用
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): checkin repayment via fundings, 100% deduct when overdue`

### Task 11: 逾期状态机（写事务内幂等翻转）

**Files:**
- Modify: `model/loan_funding.go`
- Test: `model/loan_funding_test.go`

**Interfaces:**
- Produces: `flipOverdueFundingsTx(tx *gorm.DB, userId int, now time.Time) ([]TokenLoanFunding, error)`——在写事务内调用：对 `status=active 且 today > due_day` 的 funding 置 `status=overdue`、`penalty_started_day=today`（条件更新 `WHERE status='active'` 保证幂等，并发双还款不双翻）；返回新翻转的 funding 列表供信用分/AI 处置钩子使用。挂入 distributeRepayment 与 BorrowLoan 事务（结算后第一步）

- [ ] **Step 1: 写失败测试**——①到期未清翻转且 penalty_started_day 落账；②幂等（二次调用返回空）；③未到期不翻；④翻转后罚息按纯计算生效（settleFunding 不依赖 status）
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现 + 挂入 Task 9/8 事务**
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): overdue status machine with idempotent flip`

### Task 12: 违约处置（延长/核销/永续）+ 黑名单出口

**Files:**
- Modify: `model/loan_market.go`
- Test: `model/loan_market_test.go`

**Interfaces:**
- Produces:
  - `ResolveOverdueFunding(lenderId int, fundingId int64, action string, extendDays int) error`——action ∈ `extend`/`writeoff`/`perpetual`；仅 funding 的 lender 本人、仅 overdue 状态可调用：
    - `extend`：DueDay = loanDay(now) + extendDays（extendDays ∈ (0, LoanTermDays]），status→active（已计罚息保留）
    - `writeoff`：status→written_off；offer 侧 `amount_total -= principal_remaining`（若 offer 仍存续）；借款人 `blacklisted_until_day = today + BlacklistDaysOnDefault`、`credit_score -= CreditDefaultPenalty`（下限 -50）；`syncAccountFromFundings`（债务销毁）
    - `perpetual`：保持 overdue 继续计息（无状态变化，仅台账/日志记录决策）
  - 黑名单出口：funding 全部还清时若 `blacklisted_until_day > 0` 且无 overdue/written_off 关联活跃债务 → 清零解除（在 distributeRepayment 的 repaid 分支调用 `maybeLiftBlacklistTx`）
  - 错误：`ErrLoanFundingNotOverdue`、`ErrLoanInvalidDefaultAction`、`ErrLoanNotFundingOwner`

- [ ] **Step 1: 写失败测试**——①非 owner 拒绝；②非 overdue 拒绝；③核销后债务销毁+冻结池减量+黑名单+扣分（下限截断）；④延长后 due_day 更新、status→active；⑤永续全还清后黑名单解除；⑥written_off 后还款拒绝（分配跳过终态 funding）
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): overdue resolution (extend/writeoff/perpetual)`

### Task 13: 信用分引擎

**Files:**
- Create: `model/loan_credit.go`
- Test: `model/loan_credit_test.go`

**Interfaces:**
- Produces:
  - `scoreBorrowEventRepaidTx(tx *gorm.DB, borrowEventId int64, now time.Time) error`——在 funding 转 repaid 时检查其 borrow_event 全部 funding 均为 repaid 则评分：持有天数 = loanDay(now) - loanDay(borrow 行 CreatedAt)；持有 < CreditMinHoldDays → `credit_score -= CreditFastRepayPenalty`；否则按 due_day 基准（含延长后的新 due_day，取事件内 max(due_day)）：today ≤ max due_day 且事件本金 ≥ CreditMinBorrowUsd×QuotaPerUnit → `credit_score += CreditRepayBonus`；逾期后还清不加分不扣分。分数钳制 [-50,100]。写 `loan.credit_change` 日志数据
  - `GetCreditScore(userId int) (int, error)`——账户不存在返回 credit_initial

- [ ] **Step 1: 写失败测试**——①按时还清 +5；②持有 2 天还清 -2 不加分；③本金 < 1 USD 不计分（刷分堵死）；④逾期后还清不加；⑤核销扣分与下限；⑥分数上下限钳制
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现 + 挂入 distributeRepayment repaid 分支**
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): credit score engine`

### Task 14: repay_plan 调整（放贷人 + settle-first）

**Files:**
- Modify: `model/loan_market.go`
- Test: `model/loan_market_test.go`

**Interfaces:**
- Produces: `SetFundingRepayPlan(lenderId int, fundingId int64, plan string) error`——仅 lender 本人；plan ∈ 四档常量；事务内锁 funding → settleFunding 到当天（settle-first，已结算利息不回溯）→ 写新 plan；`principal_only` 一次性核销未付利息（`debt_quota = principal_remaining`）；`syncAccountFromFundings`；台账/日志记录
- 后续 Task 15 的 AI 裁决复用同函数但走独立入口 `SetFundingRepayPlanByOfficer(fundingId, plan)`（带权限边界校验：P2P funding 仅允许 full→no_penalty→interest_freeze 单向降档，principal_only 拒绝；platform funding 全档）

- [ ] **Step 1: 写失败测试**——①放贷人改 principal_only 后利息核销、debt 冻结；②settle-first：改档前未结算利息先落账再改；③AI 入口对 P2P 设 principal_only 拒绝；④AI 入口对 P2P 降档允许、升档（freeze→full）拒绝；⑤platform funding AI 全档允许
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): repay_plan adjustment with officer permission boundary`

### Task 15: AI 审批员扩展——区间单定价 + 减免申诉 + 官方逾期处置

**Files:**
- Modify: `service/loan_officer.go`、`service/loan_officer_decision.go`
- Create: `service/loan_officer_market.go`
- Test: `service/loan_officer_market_test.go`、`service/loan_officer_decision_test.go`

**Interfaces:**
- Produces:
  - `PriceAiSpaceFundings(borrowerId int, amount int64, candidates []model.TokenLoanOffer) ([]model.FundingPlan, error)`——同步一次 AI 调用：prompt 注入候选区间单（匿名 offer id、available、rate_min/max、cap）与借款人档案；输出 `{"fundings":[{"offer_id":1,"amount":0.0,"rate":0.0}]}`（USD 计价，换算 quota）；解析失败/超时/ai_enabled=false → 返回空（调用方跳过该来源）。复用 `PickLoanOfficerModel` 与现有调用链
  - 减免申诉：工单 topic 白名单加 `appeal`；该 topic 的工单上下文注入借款人 overdue/高息 funding 列表；结案 decision schema 扩展 `{"funding_id":0,"repay_plan":""}` 可选字段——解析后走 `SetFundingRepayPlanByOfficer`（Task 14 权限边界兜底），钳制逻辑同 ClampLoanDecision 模式
  - `DisposePlatformOverdueFunding(fundingId int64)`——官方逾期处置：一次性 AI 调用（非对话工单），输入 funding 概况，输出 extend/writeoff/perpetual；失败/ai_enabled=false → 自动 extend 一个 LoanTermDays 并 `common.SysError` 告警。由 Task 11 翻转钩子（platform funding 时）异步触发，funding_id 幂等防重
  - AI prompt 模板追加市场规则段（`{{placeholder}}` 注入区间，镜像现有模式）

- [ ] **Step 1: 写失败测试**——①定价输出解析（合法/多块/裸 JSON/越界）；②appeal decision 解析与权限边界（P2P principal_only 拒绝）；③官方处置解析与失败兜底（自动 extend + 告警）；④think 剥离对新 prompt 输出仍生效
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**——AI 调用走既有渠道调用封装（参考 RunLoanOfficerRound 的模型调用部分），mock 点与既有测试一致
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): officer pricing, appeal tickets, platform overdue disposal`

### Task 16: 用户端 API + controller

**Files:**
- Create: `controller/loan_market.go`
- Modify: `controller/loan.go`（BorrowLoan/RepayLoan 调用签名、respondLoanError 新错误映射、buildLoanStatusData 加 credit_score/market_enabled/blacklisted_until_day/overdue 信息）
- Modify: `router/api-router.go`（或 loan 路由所在文件，按现有 loan 路由注册位置追加）
- Modify: `i18n/keys.go` + `i18n/locales/{zh-CN,zh-TW,en}.yaml`
- Test: `controller/loan_market_test.go`（或并入既有 controller 测试风格）

**Interfaces:**
- Consumes: Task 6/8/9/12/13/14 的全部 model 函数
- Produces（路由，均挂在既有 `/api/user/loan/` 组、登录鉴权后）：
  - `POST /api/user/loan/market/disclaimer` → AgreeLenderDisclaimer
  - `GET /api/user/loan/market/offers`（自己的）/ `POST /api/user/loan/market/offers` / `POST /api/user/loan/market/offers/:id/pause` / `POST .../resume` / `POST .../close` / `POST .../withdraw`
  - `GET /api/user/loan/market/list`——市场浏览（全部 active order 单：金额、利率、信用分门槛、放贷人匿名 id）
  - `GET /api/user/loan/market/fundings`——放贷人收益台账（funding 粒度，含状态/已回本息/借款人信用分）
  - `POST /api/user/loan/market/fundings/:id/resolve`（body: action, extend_days）
  - `POST /api/user/loan/market/fundings/:id/repay_plan`（body: plan）
  - `POST /api/user/loan/borrow` 请求体加 `order_id`（0=不挑单）；借款前 service 层视情况先调 PriceAiSpaceFundings 传入
  - 新 i18n key：MsgLoanMarketDisabled、MsgLoanDisclaimerRequired、MsgLoanOfferNotFound、MsgLoanOfferInvalidParams、MsgLoanOfferNotActive、MsgLoanBlacklisted、MsgLoanHasOverdue、MsgLoanFundingNotOverdue、MsgLoanInvalidDefaultAction、MsgLoanNotFundingOwner、MsgLoanInvalidRepayPlan 等

- [ ] **Step 1: 写失败测试**——每个端点的鉴权/归属/参数校验/错误映射用例（对齐 controller/loan_test.go 风格）
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现 + 路由注册 + i18n 三语言**
- [ ] **Step 4: `go test ./controller/ -run TestLoanMarket -v` 通过 + `bash scripts/check-i18n.sh`**
- [ ] **Step 5: Commit**——`feat(loan): marketplace user APIs`

### Task 17: 管理端 API + 日志模板

**Files:**
- Modify: `model/loan_admin.go`、`controller/loan_admin.go`
- Modify: `model/log_templates.go`
- Test: `model/loan_admin_test.go`

**Interfaces:**
- Produces:
  - `GET /api/user/loan/admin/offers`（分页+keyword 按放贷人过滤）、`GET /api/user/loan/admin/fundings`（分页，可按 lender/借款人/status 过滤）、`GET /api/user/loan/admin/market_overview`（总供给/冻结中/在贷/累计利息/逾期笔数）
  - 日志模板（zh/en 双语，对齐既有模板格式）：`loan.offer_create`、`loan.offer_close`、`loan.offer_withdraw`、`loan.funding_matched`、`loan.interest_settled`、`loan.default_decision`、`loan.repay_plan_change`、`loan.credit_change`、`loan.disclaimer_agreed`——在 Task 6/8/9/12/13/14 的对应操作点埋 `recordUserSecurityAudit`（controller 层已埋的复用）

- [ ] **Step 1: 写失败测试**——admin 查询分页/过滤/非管理员拒绝（对齐既有 loan_admin 测试）
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现**
- [ ] **Step 4: 测试通过 + Commit**——`feat(loan): marketplace admin APIs and log templates`

### Task 18: web/default 前端——放贷市场

**Files:**
- Modify: `web/default/src/`（词元贷页所在目录新增 market 组件区；路由与顶栏入口复用既有词元贷模式）
- i18n: patch JSON（走 `bun run i18n:apply`）

**Interfaces:**
- Consumes: Task 16 全部端点
- Produces（页面区块，复用既有词元贷页布局与组件库）：
  - 我的供给：offer 列表（模式/总额/可用/已放出/累计利息/利率设置/状态）+ 创建表单（三模式切换，ai 模式显示利率上下限与单笔上限）+ 暂停/恢复/关闭/撤回按钮；首次创建前弹免责声明（勾选同意调 disclaimer 接口，未同意阻断）
  - 收益台账：funding 明细表（放出/已回本息/状态/借款人信用分）+ overdue 行的处置按钮（延长/核销/永续弹窗确认）+ repay_plan 调整下拉
  - 市场浏览：order 单列表（金额/利率/信用分门槛）+ "借这笔"按钮（带 order_id 跳借款流程）
  - 借款页：信用分展示、逾期/黑名单状态提示、减免申诉工单入口（topic=appeal）
  - 常驻免责声明文案条

- [ ] **Step 1: 实现页面与 API 对接**（组件粒度对齐现有词元贷页，状态管理复用页面既有模式）
- [ ] **Step 2: i18n patch + `bun run i18n:sync`，补齐全部 locale**
- [ ] **Step 3: `bun run typecheck` + `bun run lint` + `bun run build`（web/default）**
- [ ] **Step 4: Commit**——`feat(loan): marketplace UI for default theme`

### Task 19: web/classic 前端——parity

**Files:**
- Modify: `web/classic/src/`（对齐 classic 词元贷页模式，Semi UI 组件）
- 同 Task 18 的功能全集（classic i18n 用 `bun run i18n:extract`）

- [ ] **Step 1: 实现页面与 API 对接**
- [ ] **Step 2: i18n extract + 补齐翻译**
- [ ] **Step 3: classic build 通过**
- [ ] **Step 4: Commit**——`feat(loan): marketplace UI for classic theme`

### Task 20: 管理端前端（双主题）

**Files:**
- Modify: `web/default/src/`、`web/classic/src/` 的既有词元贷管理页
- Consumes: Task 17 端点

- [ ] **Step 1: offers 列表 + fundings 明细 + 市场总览卡片（只读）**
- [ ] **Step 2: i18n + typecheck/lint/build（双主题）**
- [ ] **Step 3: Commit**——`feat(loan): marketplace admin UI`

### Task 21: 全量验证

- [ ] **Step 1: `go build ./...` 通过**
- [ ] **Step 2: `go test ./... -count=1` 全绿**（已知基线失败除外：middleware 语言相关测试，与本功能无关——跑前记录基线对比）
- [ ] **Step 3: `bash scripts/check-i18n.sh` 通过**
- [ ] **Step 4: `make build-all-web` 双主题构建通过**
- [ ] **Step 5: 数据库三方言静态检查**——新 raw SQL（若有）按 common/ 方言助手分支复核；AutoMigrate 列类型核对
- [ ] **Step 6: Commit**——`chore(loan): marketplace final verification`

### Task 22: 构建、部署与上线配置

- [ ] **Step 1: 构建**——`go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api main.go`（版本号不变）
- [ ] **Step 2: 部署**——`systemctl restart newapi.service`（禁止直接 kill）；`systemctl status` + `journalctl -u newapi.service -n 50` 确认启动与迁移日志
- [ ] **Step 3: 冒烟**——DB 插测试用户（含 access_token），curl 走通：免责声明 → 创建 pool offer → 另一用户借款（撮合命中）→ 提前还款（放贷人入账）→ 逾期翻转 → 核销 → 信用分变化；管理员 overview 接口
- [ ] **Step 4: 按"生产上线配置"写入 loan_setting**
- [ ] **Step 5: 清理冒烟测试数据（测试用户及其 loan/offer/funding/logs 行）**
- [ ] **Step 6: 推送**——`git push`（用户确认后）

## Self-Review 记录

- spec §3-§15 每节均有对应 Task：§4 数据模型→T3/T5；§5 结算→T4；§6 撮合→T7/T8/T15；§7 还款→T9/T10；§8 repay_plan→T14/T15；§9 逾期违约→T11/T12/T15；§10 信用分→T13；§11 免责→T6/T16/T18；§12 配置→T2；§13 UI→T18/T19；§14 管理端→T17/T20；§15 工程约束→T8/T9 锁序与不变式测试；§16 修复项→T1
- 类型一致性：FundingPlan/RepayAllocation/settleFunding/distributeRepayment/SetFundingRepayPlanByOfficer 等跨任务签名已对齐
