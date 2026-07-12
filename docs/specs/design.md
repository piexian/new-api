# 设计文档：首页星空 + 跳转入口 + 友链悬浮球

## Overview

默认首页从「塞满排行/监控」改为：

1. **星空主题落地页**（浅色蓝白 / 深色黑蓝紫）
2. **入口跳转**到既有页看数据（排行 → `/rankings`，模型健康 → `/pricing`）
3. **双前端路径映射**保证跳转不断链
4. **友链**在系统设置维护，**悬浮球**展示
5. 原生，无新 embed；无 UserID 查询

首页空出版面做星空排版，而不是再嵌 usage-dashboard 大表。

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  System Settings: friend_links JSON + enabled           │
│  status API 下发 → 前端 status store                     │
└───────────────────────────┬─────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────┐
│  Default / Classic Home (星空)                           │
│  - Hero + 入口卡（rankings / pricing / dashboard…）       │
│  - resolveRoute(key) 双端路径                            │
│  - 无大表、无健康矩阵、无友链墙                           │
└───────────────────────────┬─────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────┐
│  FloatingBall（全局或 PublicLayout）                     │
│  - 友链列表                                              │
│  - 可选快捷：排行、模型广场（映射跳转）                   │
└─────────────────────────────────────────────────────────┘

完整数据页（已有）：
  /rankings  用量/模型排行
  /pricing   模型广场（健康入口落点）
```

| 决策 | 选择 | 原因 |
|------|------|------|
| 首页是否内嵌用户/模型大表 | **否**（跳转） | 用户要求空出位置做星空；数据页已有 |
| 模型监控 | 跳转 **模型广场** `/pricing` | 不在首页做监控矩阵 |
| 用量排行 | 跳转 **`/rankings`** | 同上 |
| 友链位置 | **悬浮球**，非页内墙 | 用户明确 |
| 友链配置 | **系统设置** | 可运营、无需改代码 |
| 路径 | **逻辑 key + 双端映射** | classic/default 路由不同 |
| 主题 | **星空** + 浅蓝白 / 深黑蓝紫 | 用户明确 |
| usage-rankings API | **可选**：供 rankings 页扩展；首页可不调 | 避免首页请求负担 |
| **顶栏** | **双端均复用既有 Header**，不另做首页顶栏 | 避免与 `HeaderNavModules` / 登录态鉴权冲突 |

## Components and Interfaces

### A. 双前端路径映射 + 顶栏非冲突（P0）

**原则：导航只在现有顶栏；首页不重做一套。**

| 端 | 既有顶栏 | 配置源 | 首页职责 |
|----|----------|--------|----------|
| **default** | `PublicLayout` → `Header` / `TopNav` + `useTopNavLinks` | `status.HeaderNavModules` | 只改主内容；**不**再渲染第二套 nav |
| **classic** | `PageLayout` → `HeaderBar` + `useNavigation` / `useHeaderBar` | 同上 | 同上；**不**替换 `HeaderBar` |

**HeaderNavModules 必须尊重（双端一致）：**

- 模块：`home` / `console` / `pricing` / `rankings` / `docs` / `about`
- `pricing` / `rankings` 支持 `{ enabled, requireAuth }`
- 显示/隐藏与 requireAuth **只由既有顶栏逻辑处理**
- `docs`：有 `status.docs_link` 走外链（保持现网行为）

**路径映射（CTA / 悬浮球可选快捷，不是顶栏重实现）：**

**default** `web/default/src/lib/frontend-routes.ts`：

```ts
export type AppRouteKey =
  | 'home' | 'dashboard' | 'pricing' | 'rankings'
  | 'sign_in' | 'sign_up' | 'keys' | 'wallet' | 'docs'

const DEFAULT_ROUTES: Record<AppRouteKey, string> = {
  home: '/',
  dashboard: '/dashboard',
  pricing: '/pricing',
  rankings: '/rankings',
  sign_in: '/sign-in',
  sign_up: '/sign-up',
  keys: '/keys',
  wallet: '/wallet',
  docs: '', // 外链来自 status.docs_link
}

export function resolveAppRoute(key: AppRouteKey): string
```

**classic** 对齐 `helpers/frontendTheme.js` 路径表，导出 `resolveAppRoute`（dashboard→`/console` 等）。

**禁止：** 首页自建顶栏；default 写死 `/console`；classic 写死 default 路径；悬浮球复制整套顶栏导航。  
**允许：** Hero 主 CTA / 文案链用映射路径；模块关闭时隐藏或降级。  
**mockup** 顶栏仅为示意，正式实现删除。

### B. 星空首页（default）

```
features/home/
  components/
    starfield-background.tsx
    sections/
      hero.tsx                 # 基址 + 主 CTA + 统一 Tab 区
      service-strip.tsx        # 公益三条，页面最下方并排
    hero-capability-tabs.tsx   # Chat|Responses|Claude|Gemini|Codex|Claude Code（同样式）
  index.tsx                    # PublicLayout 包住；无自建顶栏
```

**反重复规则（P0）：**

- 主按钮只在 Hero（控制台 / 模型广场）。
- 不要快捷入口三卡片。
- **不与既有顶栏重复**：排行榜/模型广场/文档走 `HeaderNavModules` 顶栏。
- **标题下 API 基址展示**：`status.server_address` / origin + 一键复制；接入示例共用该基址。
- **统一能力/接入 Tab（HeroTerminal 样式，同一卡片）：**
  - 顺序：**Chat → Responses → Claude → Gemini | Codex → Claude Code**
  - 前 4 个：协议演示（method/path/request/response，自动轮播仅限协议）
  - 后 2 个：第三方接入配置（config/auth 或 settings.json，可复制、无注释）
  - **主题：与 default `HeroTerminalDemo` 一致**——浅色 `bg-white/95` 卡片 + 深色 `#0b0f17` 面；代码区跟随主题前景色，**禁止固定深色终端底**
  - 一次只显示一个面板；不再单独做第二块「接入方法」卡片
- **公益气质三条并排在页面最下方**（协议+接入区之后），不再插在中间。

**接入内容（合并进同一 Tab 区）：**

| Tab | 内容 |
|-----|------|
| Codex | `config.toml` `[model_providers.OpenAI]` + `auth.json` apikey；`wire_api="responses"` |
| Claude Code | `settings.json` env：BASE_URL/AUTH_TOKEN/模型变量 + 第三方禁用变量 |
| 禁止 | 可复制区写注释；`wire_api="chat"`；密钥写进 config.toml |

**浅色星空：** 更少、更淡、近静态。  
**深色星空：** 可更密。  
**预览：** `docs/specs/home-starfield-mockup.html` v3.0。  
**悬浮球：** 默认左下；可拖动 + localStorage。

### C. 友链数据与系统设置填写方式

**Option keys：**

- `console_setting.friend_links`：JSON 数组字符串  
- `console_setting.friend_links_enabled`：bool（总开关；关则悬浮球不展示友链区）

**条目字段（全部支持）：**

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是（系统生成） | 自增数字，前端添加时取 max+1 |
| `name` | **是** | **展示名称**（悬浮球主标题） |
| `url` | **是** | 目标链接，仅 `http://` / `https://` |
| `icon` | 否 | **自定义网站图标 URL**（png/jpg/svg/webp）；空则悬浮球用首字母或默认 link 图标 |
| `description` | 否 | **描述/副标题**（悬浮球次行文案） |
| `order` | 是 | **排序权重**，越小越靠前；保存时按 order 排序后写入 |
| `enabled` | 是 | 单条启用；总开关 + 单条双重控制 |

```ts
type FriendLink = {
  id: number
  name: string
  url: string
  icon?: string
  description?: string
  order: number
  enabled: boolean
}
```

**校验（保存时后端 + 前端表单）：**

- `name` 非空，长度 ≤ 64  
- `url` 合法 URL 且 scheme ∈ {http, https}  
- `icon` 若填：合法 URL，scheme ∈ {http, https}（**不上传文件到服务器**，只填图标地址；可用站点 favicon 或 CDN）  
- `description` 可选，长度 ≤ 200  
- 列表最多 **30** 条  

#### 系统设置 UI（怎么填）

交互对齐现有 **「控制台 API 信息」**（`ApiInfo` / `api-info-section`）模式，降低学习成本：

1. **总开关** `friend_links_enabled`：是否在悬浮球展示友链区  
2. **表格列表**列：拖拽/排序手柄 · 图标预览 · 名称 · 描述 · URL · 单条启用 · 操作（编辑/删除）  
3. **添加 / 编辑**：Dialog 表单字段：
   - 展示名称 `name` *  
   - 链接 `url` *  
   - 图标 URL `icon`（可选，旁路小预览）  
   - 描述 `description`（可选，textarea 一行～两行）  
   - 排序 `order`（数字，默认 = 当前最大 order+10 或列表末尾）  
   - 启用 `enabled`（默认 true）  
4. **排序支持（P0 至少一种，推荐两种都做）：**
   - **数字 order**：表单改数字后「保存设置」生效  
   - **列表上移/下移按钮**（或 dnd-kit 拖拽，P1）：调整后重写 `order = index * 10`  
5. **保存**：与 ApiInfo 相同——本地改列表 → 点 **保存设置** → `PUT /api/option/` 写 JSON；未保存有 `hasChanges` 提示  
6. **批量删除**（可选 P1，对齐 ApiInfo 勾选）  

**落点：**

- default：`system-settings` 下新 Section「友情链接 / Friend Links」（建议挂在 Dashboard/运营内容类分区旁，与 Announcements、ApiInfo 同级）  
- classic：`Setting/Dashboard` 增加 `SettingsFriendLinks.jsx`，交互同 Semi Table + Modal  

**status 下发：** 仅 `enabled===true` 且总开关开的条目，按 `order` 升序；悬浮球直接消费。

**不做：** 服务端图片上传、emoji 当唯一图标来源（可用 lucide 默认 fallback）。

### D. 悬浮球（可拖动 + 位置记忆）

```
components/floating-ball/
  floating-ball.tsx
  friend-links-panel.tsx
  use-floating-ball-position.ts   # 拖拽 + localStorage + 视口夹紧
```

- 挂载：`PublicLayout` / root layout（双端各自布局）
- 展开：友链列表（status 下发）；外链 `target=_blank` + `rel=noopener noreferrer`
- **默认位置：左下**（无记忆时）

#### 拖拽与记忆（生产必做，P0）

| 项 | 约定 |
|----|------|
| 交互 | pointer 事件（鼠标+触摸）拖动球体；移动距离 > 阈值（如 6px）视为拖拽，松手后**不触发** click 开合 |
| 存储 key | 建议 `newapi.floating_ball.position`（双端可共用字符串，或 default/classic 分 key） |
| 存储值 | `{ x: number, y: number, v: 1 }` 视口左上为原点的 **CSS px**（或存 left/top + 宽高比，实现选绝对 px + 夹紧更简单） |
| 写入时机 | `pointerup` 成功拖拽后 `localStorage.setItem` |
| 读取时机 | 挂载时读；非法 JSON / 缺字段 → 回落默认左下 |
| 视口夹紧 | 初始恢复、`resize`、`orientationchange` 时把球完全夹在 `padding` 安全区内（建议 ≥8px） |
| 面板方向 | P1：球中心在左半屏 → 面板向右；右半屏 → 向左，避免裁切 |
| 无障碍 | 键盘用户仍可 focus + Enter 开合；拖拽不替代可访问点击 |
| reduced-motion | 拖拽本身保留；不加强制吸附动画 |

**不做：** 把位置同步到服务端账号（仅浏览器本地）；跨设备同步非本需求。

### E. classic

- Home：星空简化版（CSS 星点即可）+ 同结构入口卡  
- FloatingBall 组件  
- 设置页友链  
- `resolveAppRoute` classic 表  

## Data Models

### Friend links option

存储与 `console_setting.api_info` 同模式（JSON 字符串 + enabled flag + ValidateConsoleSettings 分支）。

### 不强制新表

友链走 option 即可，无需新 DB 表。

## Error Handling

| 场景 | 行为 |
|------|------|
| 友链 JSON 非法 | 设置保存拒绝；status 返回 [] |
| 无友链 | 悬浮球仍可展开快捷跳转；友链区 empty |
| docs_link 空 | docs 入口隐藏 |
| rankings 导航模块关闭 | 入口卡可仍显示但跳转后 403/Forbidden 页；或入口随 HeaderNav 隐藏（实现选：随 `rankings` module enabled） |

## Testing Strategy

- 映射表单测：key → 两套 path  
- 友链校验单测：非法 URL、超限  
- 前端：入口点击目标 path；悬浮球渲染 enabled 列表  
- 视觉：light/dark 星空 + reduced-motion  
- 双端手工：从首页进 console/dashboard、pricing、rankings  

## Implementation Order

1. **路径映射 helper**（双端）  
2. **友链 option + 校验 + status**  
3. **default 星空背景 + 入口卡 + Hero 跳转修正**  
4. **default 悬浮球**  
5. **default 系统设置友链 UI**  
6. **classic 对齐**  
7. **验证 + phase summaries**  

## Out of Scope（再确认）

- 首页内嵌完整 usage 大表 / 健康热力图  
- UserID 查询  
- embed  
- 在 pricing 内新做完整健康监控产品（仅跳转；有则链锚点）  

## Risks

| 风险 | 缓解 |
|------|------|
| 「模型健康」在广场无独立 UI | 文案用「模型广场」；后续加 health tab |
| 星空喧宾夺主 | 背景弱化、正文对比度验收 |
| 悬浮球与其它 FAB 重叠 | 统一 z-index 规范 |

## 与前序设计的关系（防漂移）

| 前序 | 现状态 |
|------|--------|
| 首页内嵌用户/模型榜 Section | **撤销为跳转入口** |
| usage-rankings API 首页强依赖 | **降为可选**（排行页用） |
| 登录才见用户榜 | **仍适用于排行 API/页**，非首页大表 |
| 浅色蓝白深色黑蓝紫 | **保留并升级为星空主题** |
| 原生无 embed | **保留** |

## Phase Summary Rule

每阶段 `docs/specs/phase-N-summary.md`：做了什么 / 未做什么 / 偏差 / 下一入口。  
锚点：本 design + requirements。
