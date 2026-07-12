# 需求文档：首页星空落地 + 友链悬浮球（Home Starfield v2.1）

## 1. 目标

1. **原生默认首页**，不新 embed。  
2. **密星空主题**：浅色蓝白、深色黑蓝紫。  
3. **主 CTA 唯一区**（Hero）：控制台 / 模型广场；禁止与之重复的快捷入口大卡。  
4. 用量排行 → `/rankings`；模型广场/健康 → `/pricing`（顶栏或文案链）。  
5. **接入方法**：Codex | Claude **互斥 Tab**。  
6. **双前端路径映射**。  
7. **友链**：系统设置维护；**悬浮球默认左下**展示。  
8. **中部服务说明**用旧版气质文案，**不用**「50+ 上游服务 / 100+ 模型计费」等通用指标。  

## 2. 服务说明文案（P0）

替换通用 Stats：

| 标题 | 副文案 |
|------|--------|
| 极速响应？ | 全球节点均未优化 |
| 稳定高可用？ | 私人自用服务 |
| 公益免费 | 用爱发电 随时跑路 |

正式实现：i18n 或可配置；默认上述中文。

## 3. 范围

### In Scope

- 星空背景 + Hero + 服务说明条 + Codex/Claude 接入区  
- 路径映射、友链 option/设置/左下悬浮球  
- 自定义 `home_page_content` 覆盖  

### Out of Scope

- embed、UserID 查询、tools 整包  
- 首页大表/健康矩阵/页内友链墙  
- 快捷入口三卡片  
- 通用 SaaS「50+/100+」指标条  

## 4. 功能需求

### FR-1 双前端路径映射 + 顶栏非冲突

| ID | 需求 |
|----|------|
| FR-1.1 | 逻辑 key + `resolveAppRoute`：default/classic 路径表分离（如 dashboard→`/dashboard` vs `/console`） |
| FR-1.2 | **双端首页均复用既有顶栏**（default `TopNav`/`useTopNavLinks`，classic `HeaderBar`/`useNavigation`），不新增首页专用顶栏 |
| FR-1.3 | 尊重 `HeaderNavModules`：`home/console/pricing/rankings/docs/about` 显示与 `pricing/rankings.requireAuth` |
| FR-1.4 | 首页内跳转链/主 CTA 用映射路径；模块关闭时不假装顶栏仍展示该入口 |
| FR-1.5 | 悬浮球以友链为主，避免复制一整套顶栏导航 |

### FR-2 首页结构

| ID | 需求 |
|----|------|
| FR-2.1 | Hero 唯一主 CTA：控制台 + 模型广场 |
| FR-2.2 | 禁止 Hero 下同义快捷入口大卡 |
| FR-2.3 | 排行/模型广场走**既有顶栏**（`HeaderNavModules`）；首页不重做导航条 |
| FR-2.4 | 公益气质三条**并排在最下方**（协议+接入区之后） |
| FR-2.5 | 密星空 + reduced-motion |
| FR-2.6 | **标题下展示 API 基址**（`server_address` / origin）+ 一键复制 |
| FR-2.7 | 接入示例网关地址与该基址一致 |
| FR-2.8 | **同一套 Tab 卡片**（HeroTerminal 样式）：Chat / Responses / Claude / Gemini / Codex / Claude Code |
| FR-2.9 | 协议 Tab 在前；第三方接入 Tab 接在 Gemini 后；自动轮播仅协议 Tab |
| FR-2.10 | Tab 卡片 **浅色/深色跟随主题**（浅色白底、深色暗面）；代码区用主题前景色，禁止固定黑底终端 |

### FR-3 接入方法（合并进统一 Tab）

| ID | 需求 |
|----|------|
| FR-3.1 | Codex / Claude Code 与协议演示同组件样式，互斥单面板 |
| FR-3.2 | Codex：`config.toml` `[model_providers.OpenAI]` + `auth.json`；`wire_api="responses"` |
| FR-3.3 | 禁止 `wire_api="chat"`；密钥不进 config.toml |
| FR-3.4 | Claude Code：BASE_URL + AUTH_TOKEN + 模型变量 + 第三方禁用变量 |
| FR-3.5 | 可复制区无注释；Codex config/auth 分块复制 |
| FR-3.6 | 网关域名与模型 ID 可配置为站点实际值 |
| FR-3.7 | 浅色星空降噪 |

### FR-4 友链 + 悬浮球

| ID | 需求 | 优先级 |
|----|------|--------|
| FR-4.1 | `console_setting.friend_links` + `friend_links_enabled` | P0 |
| FR-4.2 | 字段：name、url、icon URL、description、order、enabled | P0 |
| FR-4.3 | 设置：Dialog 增改、列表、排序（order + 上下移） | P0 |
| FR-4.4 | 悬浮球默认**左下**；展示图标/名称/描述 | P0 |
| FR-4.5 | 无页内友链墙；最多 30 条 | P0 |
| FR-4.6 | **可拖动**：指针/触摸拖拽改位置；拖拽阈值避免误触点击 | P0 |
| FR-4.7 | **浏览器记忆位置**：`localStorage` 持久化最后停放坐标；刷新后恢复 | P0 |
| FR-4.8 | 恢复时做 **视口夹紧**（resize/旋转后仍完整可见，不飞出屏外） | P0 |
| FR-4.9 | 面板开合方向随靠边自适应（靠左向右展、靠右向左展） | P1 |

```
[密星空背景]
[既有顶栏 · HeaderNavModules]
Hero：标题 → 基址复制 → 主 CTA
统一 Tab 卡片：Chat | Responses | Claude | Gemini | Codex | Claude Code
公益三条并排（最下方）
Footer
[悬浮球·左下] 友链
```

## 6. 验收

1. 星空够密  
2. 无重复快捷入口卡  
3. 公益三条**在最下方并排**  
4. Hero 有 BASE URL 复制  
5. **协议 + 接入合并同一 Tab 样式**（Gemini 后接 Codex / Claude Code）  
6. 不与双端既有顶栏冲突  
7. 悬浮球可拖动 + localStorage + 视口夹紧  
8. 友链可配置  
9. 双端路径正确  

## 7. 预览

`docs/specs/home-starfield-mockup.html`（v3.0）

## 8. 阶段

P0 需求 → P1 设计 → P2 友链/映射 → P3 default → P4 classic → P5 验证  
每阶段 `phase-N-summary.md`。
