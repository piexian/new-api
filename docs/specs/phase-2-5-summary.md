# Phase 2–5 总结：首页星空 + 友链悬浮球落地

## 做了什么

### Foundation
- 双端 `resolveAppRoute`：
  - `web/default/src/lib/frontend-routes.ts`
  - `web/classic/src/helpers/frontend-routes.js`（并导出到 helpers）
- 友链后端：
  - `console_setting.friend_links` / `friend_links_enabled`
  - 校验（name/url 必填、最多 30、URL 安全）
  - status 仅下发 enabled 列表（order 升序）
  - Go 测试：`setting/console_setting/validation_friend_links_test.go`

### default 前端
- 星空背景 `StarfieldBackground`（浅色降噪 / 深色更密 / reduced-motion / 主题切换重绘）
- Hero：BASE URL 复制 + 主 CTA（`resolveAppRoute`）+ 统一能力 Tab
- 统一 Tab：`Chat | Responses | Claude | Gemini | Codex | Claude Code`
  - 浅/深跟随 `HeroTerminalDemo` 风格，禁止固定黑终端
  - Codex：`config.toml` + `auth.json` + `wire_api=responses`
  - Claude Code：settings env + 禁用变量 + 可指定网关模型
- 公益三条并排在底部 `ServiceStrip`
- 可拖动友链悬浮球 `FloatingFriendLinks`（localStorage + 视口夹紧）
- 系统设置：`FriendLinksSection` 接入 content 设置

### classic 前端
- Home：控制台路径走 `resolveAppRoute('dashboard')`
- `HomeCapabilityTabs` + 公益三条
- `FloatingFriendLinks` 挂到 `PageLayout`
- 设置：`SettingsFriendLinks` 挂到 `DashboardSetting`

## 验证
- `go test ./setting/console_setting/ -count=1` ✅
- `bun run typecheck`（web/default）✅
- classic JSX：`esbuild --bundle --packages=external` 解析 ✅
- 路径冒烟：default `/dashboard`；classic `/console` `/login` ✅

## 未推送
- 仅本地提交；**禁止 push 远程**（按用户要求）

## 风险 / 后续
- i18n 新增中文文案未全量 `i18n:sync`（可显示 key 原文/中文直接传 `t()`）
- classic Home 仍保留供应商图标区（在 Tab/公益条之后），未强删
- 悬浮球仅在有 enabled 友链时显示
