# 词元贷（Token Loan）+ AI 业务员 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 spec `docs/specs/2026-08-14-token-loan-design.md`（v4）的词元贷 + AI 业务员功能并上线。

**Architecture:** Go 后端（model 层账户/台账/工单 + service 层 AI 业务员 + controller 层 API），签到钩子改造 `model/checkin.go` 双分支；前端双主题（web/default React19+TS+TanStack Router，web/classic React18+Semi+react-router）各加一个 `/loan` 独立页面 + 顶栏入口 + 管理设置区。

**Tech Stack:** Go/GORM（SQLite/MySQL/PG 三库兼容）、shopspring/decimal、React、bun。

## Global Constraints

- spec 唯一权威：`docs/specs/2026-08-14-token-loan-design.md`（v4），每条任务隐含遵循其 Global 规则
- 金额一律整数 quota；USD↔quota 换算走 `decimal.NewFromFloat(x).Mul(decimal.NewFromFloat(common.QuotaPerUnit))` + `common.QuotaFromDecimalChecked`（common/quota_math.go:145）
- 行锁一律 `lockForUpdate(tx)`（model/locking.go:20），禁止裸写 `clause.Locking`
- 贷款 quota 变动一律 `IncreaseUserQuota/DecreaseUserQuota(id, quota, true)`（db=true 直写，model/user.go:1434/1466）
- 新 controller 错误响应一律 `common.ApiErrorI18n(c, i18n.MsgXxx)`；i18n key 加到 `i18n/keys.go` 并补齐 `i18n/locales/{zh-CN,zh-TW,en}.yaml`；禁止复制 controller/checkin.go 的硬编码中文模式
- 不直接调 `encoding/json`，用 `common.Marshal/Unmarshal/UnmarshalJsonStr`
- 上游 relay DTO 可选标量用指针 + `omitempty`
- Go 代码 `gofmt`；测试放同包 `*_test.go`
- 前端双主题 parity；default 主题 2 空格缩进、单引号、无分号、import 排序、无 console
- 完成后必须跑：`go build ./...`、`go test ./...`、`bash scripts/check-i18n.sh`、两主题 build
- 提交规范 Conventional Commits（`feat:` / `fix:` 等），不提交 `.ccg/` 等工具文件

## 生产上线配置（Task 15 用）

- `loan_setting.max_total` = $5 → 5 * QuotaPerUnit = 2500000 quota（签到 $0.4–$4/天，日均 ~$2.2，约 2-3 天还清）
- `loan_setting.ai_max_limit` = $20 → 10000000 quota
- `daily_rate` = 0.001、`ai_min_rate` = 0.0005、`ai_max_grace_days` = 30、`ai_max_rounds` = 10、`ai_daily_limit` = 3、`ai_max_active_applications` = 1、`terms_enabled` = true
- `ai_models`：上线时查生产 channels 表，排除 type 为 Codex/coding 类与 QwenTokenPlan(69) 的渠道后，从剩余可用模型中选 2-3 个快模型（如 glm-5.2-fast、deepseek-v4-flash），按模型卡标称填 context_window

---

### Task 1: loan_setting 配置组

**Files:**
- Create: `setting/operation_setting/loan_setting.go`
- Test: `setting/operation_setting/loan_setting_test.go`

**Interfaces:**
- Produces: `operation_setting.GetLoanSetting() *LoanSetting`，结构体字段：`Enabled bool`、`MaxTotal int64`（quota）、`DailyRate float64`、`MinRegisterDays int`、`MaxPerBorrow int64`（quota，0=跟随 MaxTotal）、`CheckinRepayEnabled bool`、`AiEnabled bool`、`AiModels []AiModelConfig`（`{Model string \`json:"model"\`; ContextWindow int \`json:"context_window"\`}`）、`AiMaxLimit int64`、`AiMinRate float64`、`AiMaxGraceDays int`、`AiMaxActiveApplications int`、`AiDailyLimit int`、`AiMaxRounds int`、`AiMaxOutput int`、`AiPrompt string`、`TermsEnabled bool`、`TermsText string`

- [ ] **Step 1: 写失败测试**（默认值断言 + 注册后可经 `config.GlobalConfig` 读取）

```go
func TestLoanSettingDefaults(t *testing.T) {
	s := GetLoanSetting()
	require.False(t, s.Enabled)
	require.Equal(t, int64(2500000), s.MaxTotal) // $5
	require.Equal(t, 0.001, s.DailyRate)
	require.Equal(t, int64(10000000), s.AiMaxLimit) // $20
	require.Equal(t, 0.0005, s.AiMinRate)
	require.Equal(t, 30, s.AiMaxGraceDays)
	require.Equal(t, 10, s.AiMaxRounds)
	require.Equal(t, 3, s.AiDailyLimit)
	require.Equal(t, 1, s.AiMaxActiveApplications)
	require.Equal(t, 2048, s.AiMaxOutput)
	require.True(t, s.CheckinRepayEnabled)
	require.True(t, s.TermsEnabled)
	require.NotEmpty(t, s.TermsText)
	require.NotEmpty(t, s.AiPrompt)
}
```

- [ ] **Step 2: 运行确认失败** — `go test ./setting/operation_setting/ -run TestLoanSettingDefaults -v`，Expected: undefined GetLoanSetting
- [ ] **Step 3: 实现**，镜像 `checkin_setting.go` 模式：`var loanSetting = LoanSetting{...}` + `init() { config.GlobalConfig.Register("loan_setting", &loanSetting) }` + `GetLoanSetting()`。`AiPrompt` 内置默认：业务员人格 + 只认 spec 5.3 的 fenced json 结案格式 + 硬边界数值（用 fmt 占位注入当前配置值的地方在 service 层拼接，这里只存模板）
- [ ] **Step 4: 测试通过**
- [ ] **Step 5: Commit** — `feat(loan): add loan_setting config group`

### Task 2: 贷款账户模型与惰性结算

**Files:**
- Create: `model/loan.go`
- Modify: `model/main.go:271`（AutoMigrate 列表追加）
- Test: `model/loan_test.go`

**Interfaces:**
- Produces（后续任务依赖的精确签名）：
  - `type TokenLoanAccount struct { UserId int; PrincipalQuota int64; DebtQuota int64; LastSettledDay int; CustomMaxTotal int64; CustomDailyRate float64; InterestFreeUntil int; TermsAgreedAt int64; TotalBorrowed int64; TotalRepaid int64; CreatedAt int64; UpdatedAt int64 }`，`TableName() = "token_loan_accounts"`
  - `type TokenLoanRecord struct { Id int; UserId int; Type string; Amount int64; InterestPart int64; PrincipalPart int64; DebtAfter int64; Source string; RefId int64; CreatedAt int64 }`，`TableName() = "token_loan_records"`
  - `func loanDay(t time.Time) int` — 服务器本地日：`int(time.Date(y, m, d, 0, 0, 0, 0, time.Local).Unix() / 86400)`
  - `func effectiveRate(acc *TokenLoanAccount) float64` — `custom>0 ? min(custom, 全局) : 全局`
  - `func settle(acc *TokenLoanAccount, now time.Time)` — spec 4.1 公式（max(0,...) 钳制、math.Pow+math.Round、推进 LastSettledDay）
  - `func getOrCreateLoanAccountTx(tx *gorm.DB, userId int) (*TokenLoanAccount, error)` — 事务内 `lockForUpdate(tx)` 读/建
  - `func ProjectLoanStatus(acc *TokenLoanAccount, now time.Time) (debt, interest int64)` — 只读投影（不落盘），供 GET status

- [ ] **Step 1: 失败测试** 覆盖：跨天复利（debt=1000000, rate=0.001, 3 天 → round(1000000*1.001^3)=1003003）；同日幂等；宽限跳段（interest_free_until 在未来 → days=0 且 LastSettledDay 推进）；宽限结束后的部分计息；effectiveRate 的 min 语义；最小债务（1 quota）跨天不报错
- [ ] **Step 2:** `go test ./model/ -run TestLoan -v` 确认失败
- [ ] **Step 3: 实现** `model/loan.go`；`model/main.go:271` 的 `DB.AutoMigrate(...)` 参数列表追加 `&TokenLoanAccount{}, &TokenLoanRecord{}, &TokenLoanApplication{}, &TokenLoanApplicationMessage{}`（后两个 struct 在 Task 5 创建，此处先在 loan.go 留空 struct 声明或 Task 5 再改 main.go——选择：本任务只加前两个，Task 5 加后两个）
- [ ] **Step 4: 测试通过**（model 包测试用 SQLite 内存库，参照现有 `model/*_test.go` 的 setup）
- [ ] **Step 5: Commit** — `feat(loan): add loan account model with lazy compound settlement`

### Task 3: 同意声明与借款

**Files:**
- Modify: `model/loan.go`
- Test: `model/loan_test.go`

**Interfaces:**
- Produces:
  - `var ErrLoanTermsNotAgreed = errors.New("loan terms not agreed")`（controller 映射为 i18n）
  - `var ErrLoanDisabled / ErrLoanLimitExceeded / ErrLoanInvalidAmount / ErrLoanRegisterTooNew`（同上）
  - `func AgreeLoanTerms(userId int) error` — 幂等写 TermsAgreedAt
  - `func BorrowLoan(userId int, amountUsd string) (*TokenLoanAccount, error)` — decimal 解析（两位小数）→ quota；校验 spec 4.3 全部条目（含 users.quota int32 上界）；事务内 lockForUpdate+settle+累加+`IncreaseUserQuota(userId, amount, true)`+台账

- [ ] **Step 1: 失败测试**：未同意声明拒绝；terms_enabled=false 放行（测试内临时改 loanSetting 并 defer 恢复）；超上限（debt+amount > max_total）；custom_max_total 覆盖；amount_usd 精度（"1.005" 拒绝或截断为两位——选定：拒绝非两位小数）；int32 上界；并发借款不超上限（两个 goroutine 各借 60% 上限，最终只有一笔成功或总额 ≤ 上限）
- [ ] **Step 2-4:** 失败 → 实现 → 通过
- [ ] **Step 5: Commit** — `feat(loan): borrow with terms gate and limit checks`

### Task 4: 签到还款钩子

**Files:**
- Modify: `model/checkin.go`（UserCheckin 及两个分支）
- Modify: `controller/checkin.go`（响应加 loan_repay）
- Test: `model/checkin_loan_test.go`

**Interfaces:**
- `UserCheckin` 签名改为 `func UserCheckin(userId int) (*Checkin, *LoanRepayInfo, error)`；`type LoanRepayInfo struct { Amount int64; InterestPart int64; PrincipalPart int64; DebtAfter int64 }`（nil = 无还款）
- controller `DoCheckin` 响应 data 增加 `loan_repay`（omitempty 语义：nil 时不输出该 key），`quota_awarded` 保持 gross

**实现要点（spec 4.4）：**
- MySQL/PG 分支 `userCheckinWithTransaction`：事务内创建签到记录后，若 `CheckinRepayEnabled && debt>0`：`getOrCreateLoanAccountTx` → settle → repay=min(award,debt) → 4.2 拆分 → 台账(source=checkin) → `Update("quota", gorm.Expr("quota + ?", award-repay))`；提交后缓存 `go cacheIncrUserQuota(userId, int64(award-repay))`
- SQLite 分支 `userCheckinWithoutTransaction`：顺序执行镜像——建签到记录 → 贷款扣减（账户更新+台账）→ `IncreaseUserQuota(userId, award-repay, true)`；任一步失败按既有手动回滚模式回滚（删签到记录、回滚账户变更、删台账）
- 缓存一致性：`cacheIncrUserQuota(userId, int64(award-repay))`（model/user_cache.go:168）

- [ ] **Step 1: 失败测试**：debt=0 时 loanRepay=nil 且行为不变；奖励<利息（全抵息）；奖励>债务（清账+净额入账，DB quota 增量=净额）；SQLite 路径回滚
- [ ] **Step 2-4:** 失败 → 实现 → 通过
- [ ] **Step 5: Commit** — `feat(loan): check-in auto repayment with net-amount crediting`

### Task 5: AI 工单数据模型

**Files:**
- Create: `model/loan_application.go`
- Modify: `model/main.go`（AutoMigrate 追加两个表）
- Test: `model/loan_application_test.go`

**Interfaces:**
- Produces:
  - `type TokenLoanApplication struct { Id int; UserId int; Topic string; Status string; ModelUsed string; Decision string; Rating int; RatingComment string; CreatedAt int64; UpdatedAt int64 }`，`TableName()="token_loan_applications"`
  - `type TokenLoanApplicationMessage struct { Id int; ApplicationId int; Role string; Content string; CreatedAt int64 }`，`TableName()="token_loan_application_messages"`
  - `func CreateLoanApplication(userId int, topic, modelUsed string) (*TokenLoanApplication, error)` — 事务内先插入后 Count（open 数超 `AiMaxActiveApplications` 回滚；当日数超 `AiDailyLimit` 回滚）
  - `func AddLoanApplicationMessage(appId int, role, content string) error`
  - `func RateLoanApplication(userId, appId int, rating int, comment string) error` — 条件更新 `WHERE id=? AND user_id=? AND status='closed' AND rating=0`，RowsAffected!=1 报错
  - `const LoanAppStatusOpen = "open" / LoanAppStatusClosed = "closed"`

- [ ] **Step 1: 失败测试**：建单上限（先插入后 Count 回滚）、每日上限、评分一次性（重复评分报错）、非 closed 不可评
- [ ] **Step 2-4:** 失败 → 实现 → 通过
- [ ] **Step 5: Commit** — `feat(loan): loan officer application model`

### Task 6: AI 业务员 service（决定解析 + 上下文裁剪 + 模型调用）

**Files:**
- Create: `service/loan_officer.go`（编排）、`service/loan_officer_decision.go`（纯函数：解析/截断/裁剪，便于测试）
- Test: `service/loan_officer_decision_test.go`

**Interfaces:**
- Produces:
  - `type LoanDecision struct { CreditLimit float64 \`json:"credit_limit"\`; DailyRate float64 \`json:"daily_rate"\`; InterestFreeDays int \`json:"interest_free_days"\` }`
  - `func ExtractLoanDecision(reply string) (displayText string, decision *LoanDecision, ok bool)` — spec 5.3：大小写不敏感取第一个 fenced json 块；action 白名单只认 "close"；剥离 json 块后的展示文本；多块/裸 JSON/非法 JSON → ok=false
  - `func ClampLoanDecision(d *LoanDecision, s *operation_setting.LoanSetting) *LoanDecision` — 三字段 <0 一律置 0；credit_limit>ai_max_limit 截断；daily_rate 先下限后上限钳制；interest_free_days>ai_max_grace_days 截断
  - `func TrimLoanMessages(msgs []TokenLoanApplicationMessage, budgetTokens int) []TokenLoanApplicationMessage` — 从最早开始丢弃直到 `CountTextToken` 估算塞进预算
  - `func RunLoanOfficerRound(userId int, app *model.TokenLoanApplication, userInput string) (reply string, closed bool, err error)` — 编排：档案注入 → 裁剪 → 调用模型 → 入库 → 解析 → 结案单事务执行（spec 5.3）；模型失败计数与 3 次重抽（spec 5.5）；强制结案轮失败自动关单（spec 5.1.4）
  - 模型调用：参照 `controller/channel-test.go:289-364` 模式——`gin.CreateTestContext(httptest.NewRecorder())` 造最小 ctx，`relaycommon.GenRelayInfo` + `info.IsChannelTest = true` + `relay.GetAdaptor(apiType)` 直调，非 stream；渠道选择 `service.CacheGetRandomSatisfiedChannel`（service/channel_select.go）；实现前排查 RelayInfo 空 TokenId/ApiKey 的 nil 引用

- [ ] **Step 1: 失败测试（纯函数，充分覆盖）**：正常提取+剥离展示；```JSON 大写；多 json 块取第一个；裸 JSON 失败；action!="close" 失败；负数三字段置 0；越界截断；ai_min_rate>daily_rate 误配钳制顺序；长历史滑动窗口裁剪；单条超预算
- [ ] **Step 2-4:** 失败 → 实现纯函数 → 通过 → 实现 RunLoanOfficerRound 编排（模型调用部分用 httptest mock 上游或注入函数变量便于测试）→ 通过
- [ ] **Step 5: Commit** — `feat(loan): AI loan officer service with decision parsing and context trim`

### Task 7: controller + 路由 + i18n

**Files:**
- Create: `controller/loan.go`
- Modify: `router/api-router.go`（selfRoute 组内追加）
- Modify: `i18n/keys.go`、`i18n/locales/zh-CN.yaml`、`zh-TW.yaml`、`en.yaml`
- Test: `controller/loan_test.go`（薄：参数校验与错误映射）

**Interfaces:**
- 路由（全部挂 `selfRoute`，即 `/api/user/` 下、已含 `middleware.UserAuth()`，router/api-router.go:80-81）：
  - `GET loan/status` → `controller.GetLoanStatus`（spec 7 字段，只读投影）
  - `POST loan/agree` → `controller.AgreeLoanTerms`
  - `POST loan/borrow` `{amount_usd string}` → `controller.BorrowLoan`
  - `GET loan/records?p=&page_size=` → `controller.GetLoanRecords`
  - `POST loan/applications` `{topic, content}` → `controller.CreateLoanApplication`（建单+首轮 AI 回复）
  - `GET loan/applications?p=` / `GET loan/applications/:id` / `POST loan/applications/:id/reply` / `POST loan/applications/:id/rate`
- i18n key 命名：`loan.disabled`、`loan.terms_required`、`loan.limit_exceeded`、`loan.invalid_amount`、`loan.register_too_new`、`loan.officer_disabled`、`loan.application_limit`、`loan.reply_in_progress`、`loan.content_too_long`、`loan.officer_unavailable`、`loan.already_rated`、`loan.not_found` 等，全部入 keys.go + 三 yaml
- 错误映射：model 层哨兵错误 → `common.ApiErrorI18n`；进行中轮次互斥用 per-application 进程内 `sync.Mutex` map（或消息数条件更新）

- [ ] **Step 1: 失败测试**（httptest + gin，鉴权中间件用测试替身注入 user id）
- [ ] **Step 2-4:** 失败 → 实现 → 通过
- [ ] **Step 5:** 跑 `bash scripts/check-i18n.sh` 全绿
- [ ] **Step 6: Commit** — `feat(loan): loan API endpoints with i18n`

### Task 8: 后端总闸门

- [ ] `go build ./... && go test ./...` 全绿；`gofmt -l` 无输出；Commit 收尾

---

### Task 9: web/default 顶栏入口 + 路由骨架

**Files:**
- Modify: `web/default/src/lib/nav-modules.ts`（`HeaderNavModule` 联合类型加 `'loan'`；`HeaderNavModules` 加 `loan: boolean`；默认值 `loan: false`；`isHeaderRouteEnabledFromStatus` 加 `matchesPrefix(path, '/loan')` 分支）
- Modify: `web/default/src/hooks/use-top-nav-links.ts`（isAuthed 时 push `{ href: '/loan', label: t('Token Loan') }`，未登录不 push——与 pricing/rankings 的 requiresAuth 跳转不同，loan 直接不渲染）
- Create: `web/default/src/routes/_authenticated/loan/index.tsx`（`createFileRoute('/_authenticated/loan/')`，先渲染占位 `<LoanPage />`）

参照：`lib/nav-modules.ts:23-43,355-380`、`hooks/use-top-nav-links.ts:46-104`。

- [ ] **Step 1:** 实现上述修改（占位页面即可）
- [ ] **Step 2:** `cd web/default && bun run typecheck && bun run build` 通过；手工验证 `/loan` 未登录 302 到 sign-in（`_authenticated/route.tsx` beforeLoad 已有）
- [ ] **Step 3: Commit** — `feat(loan): default theme nav entry and route skeleton`

### Task 10: web/default 词元贷页面

**Files:**
- Create: `web/default/src/features/loan/api.ts`（`import { api } from '@/lib/api'`，对齐 `features/profile/api.ts:186-204` 模式）
- Create: `web/default/src/features/loan/types.ts`
- Create: `web/default/src/features/loan/components/terms-dialog.tsx`（18+ 声明，同意调 `POST /api/user/loan/agree`）
- Create: `web/default/src/features/loan/components/loan-status-card.tsx`（本金/利息"截至当前"/合计/可借/利率/宽限状态）
- Create: `web/default/src/features/loan/components/borrow-form.tsx`（金额输入，react-hook-form+zod）
- Create: `web/default/src/features/loan/components/loan-records-table.tsx`
- Create: `web/default/src/features/loan/components/officer-applications.tsx`（工单列表+对话串+新建表单+评分组件，参照 `components/ui/dialog`、`Table` 既有用法）
- Create: `web/default/src/features/loan/index.tsx`（LoanPage 组装）
- Modify: `web/default/src/routes/_authenticated/loan/index.tsx`（接 LoanPage）

**契约（consumes Task 7 API）：**
- `GET /api/user/loan/status` → `{enabled, principal, interest, debt, available, effective_max, daily_rate, interest_free_until, total_borrowed, total_repaid, ai_enabled, terms_enabled, terms_agreed, terms_text}`
- `POST /api/user/loan/agree`、`POST /api/user/loan/borrow {amount_usd}`、`GET /api/user/loan/records?p=&page_size=`
- `POST /api/user/loan/applications {topic, content}`、`GET .../applications?p=`、`GET .../applications/:id`、`POST .../applications/:id/reply {content}`、`POST .../applications/:id/rate {rating, comment}`
- 金额展示用既有 quota→USD 格式化 helper（参照 profile/topup 页面的做法），禁止硬编码签到金额

- [ ] **Step 1:** api.ts + types.ts
- [ ] **Step 2:** terms-dialog（terms_enabled && !terms_agreed 时强制打开，不可关闭跳过）
- [ ] **Step 3:** status card + borrow form + records table
- [ ] **Step 4:** officer 工单 UI（open 工单可继续 reply；closed 显示 decision.reply 与评分星；ai_enabled=false 时隐藏入口）
- [ ] **Step 5:** `bun run typecheck && bun run lint && bun run build` 通过
- [ ] **Step 6: Commit** — `feat(loan): default theme loan page`

### Task 11: web/default 管理设置区

**Files:**
- Create: `web/default/src/features/system-settings/general/loan-settings-section.tsx`（RHF+Zod，镜像 `checkin-settings-section.tsx`；`useUpdateOption` 写 `loan_setting.*` 扁平键；ai_models 为动态列表编辑器：每行 model 文本 + context_window 数字，可加删行，保存时序列化为 JSON 字符串）
- Modify: `web/default/src/features/system-settings/billing/section-registry.tsx`（注册 `id: 'loan'`，紧跟 checkin section 之后，L190-202 区域）
- Modify: `web/default/src/features/system-settings/maintenance/header-navigation-section.tsx`（simpleModules 加 loan 开关，schema L51-60 同步加字段）

- [ ] **Step 1-3:** 实现 → typecheck/lint/build 通过 → Commit `feat(loan): default theme loan admin settings`

### Task 12: web/classic 顶栏入口 + 路由

**Files:**
- Modify: `web/classic/src/helpers/navModules.js`（`parseHeaderNavModules` 默认加 `loan: false`；`isHeaderRouteEnabled` 加 `/loan` 分支，L187-205）
- Modify: `web/classic/src/hooks/common/useNavigation.js`（L76-93 的模块过滤对未知 key 已通用，确认 `loan:true` 生效即可；在 mainNavLinks 定义处加 loan 链接，仅登录渲染）
- Modify: `web/classic/src/App.jsx`（`<Route path='/loan' element={routeGuard('/loan', <PrivateRoute><LoanPage /></PrivateRoute>)} />`，参照 L434-461 的 console 路由模式）

- [ ] **Step 1-3:** 实现（占位页）→ `cd web/classic && bun run build` 通过 → Commit `feat(loan): classic theme nav entry and route`

### Task 13: web/classic 词元贷页面

**Files:**
- Create: `web/classic/src/pages/Loan/index.jsx`（+ 必要时拆 `components/`）
- HTTP 用 `API`（`src/helpers` 导出），UI 用 Semi 组件（Card/Form/Table/Modal/Toast），参照 `src/components/settings/personal/cards/CheckinCalendar.jsx` 与 `src/pages/Setting/Operation/SettingsCheckin.jsx` 的既有模式

**功能与 Task 10 对齐（parity）**：18+ 声明 Modal、状态卡片、借款表单、台账 Table、工单对话、评分。文案 key 用中文（classic 惯例），数值展示与 default 一致。

- [ ] **Step 1-4:** 实现 → build 通过 → Commit `feat(loan): classic theme loan page`

### Task 14: web/classic 管理设置

**Files:**
- Create: `web/classic/src/pages/Setting/Operation/SettingsLoan.jsx`（Semi Form 写 `loan_setting.*` 键，镜像 `SettingsCheckin.jsx`；ai_models 动态行编辑器）
- Modify: `web/classic/src/components/settings/OperationSetting.jsx`（挂载 SettingsLoan，参照 L154-157）
- Modify: `web/classic/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx`（模块卡片列表加 loan，L36-49 默认 map 同步）

- [ ] **Step 1-3:** 实现 → build 通过 → Commit `feat(loan): classic theme loan admin settings`

### Task 15: i18n 同步 + 前端总闸门

- [ ] **Step 1:** `cd web/classic && bun run i18n:extract`，补齐所有 locale 翻译
- [ ] **Step 2:** `cd web/default && bun run i18n:sync`，补齐所有 locale 翻译
- [ ] **Step 3:** `bash scripts/check-i18n.sh` 全绿
- [ ] **Step 4:** 两主题 `bun run typecheck`（default）+ `bun run build` 全绿
- [ ] **Step 5: Commit** — `feat(loan): i18n translations for loan feature`

### Task 16: 构建上线

- [ ] **Step 1:** `make build-all-web`
- [ ] **Step 2:** `go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api main.go`（版本号不变）
- [ ] **Step 3:** 本机 `systemctl restart newapi.service`，验证 AutoMigrate 建表、`/api/user/loan/status` 200
- [ ] **Step 4:** 管理后台写入生产配置（值见"生产上线配置"节）：enabled=true、ai_enabled=true、terms_enabled=true、HeaderNavModules 加 loan；`ai_models` 从生产渠道挑（排除 Codex/coding 与 QwenTokenPlan(69) 渠道）
- [ ] **Step 5:** scp 二进制到 netcup2（md5 校验→原子替换→restart new-api.service），写同样配置
- [ ] **Step 6:** 端到端冒烟：同意声明 → 借 $1 → 签到看自动还款 → 开工单收到 AI 回复

---

## Self-Review 记录

- Spec 覆盖：1-11 节全部有对应任务（配置→T1/T11/T14；数据模型→T2/T5；计息→T2；借款→T3；签到钩子→T4；AI 业务员→T5/T6；API→T7；前端→T9-T14；风控→各任务内；测试→各任务 Step；部署→T16）
- 类型一致性：LoanRepayInfo、LoanDecision、AiModelConfig、GetLoanSetting、lockForUpdate、cacheIncrUserQuota 签名跨任务一致
- T4 改 `UserCheckin` 签名，需在 T4 同步改 `controller/checkin.go` 调用点（已含在 T4 Files）
