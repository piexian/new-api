# Review

## Outcome

公网全局主题配置已成功保存为 `classic`。故障不是数据库未保存，而是 Cookie 优先级、CDN HTML 缓存和按 Cookie 分流静态资源叠加造成。

审查模式：根据仓库 opt-in override，使用单代理自审，未调用外部模型或子代理。

## Findings

### Critical: CDN 跨主题共享缓存 HTML

- `https://api.pie-xian.com/` 返回 `Cache-Control: max-age=2678400`、`CF-Cache-Status: HIT`，且没有 `Vary: Cookie`。
- 同一 URL 分别带 `new-api-frontend=default` 和 `classic`，均返回同一套经典前端 HTML。
- 唯一查询串首次回源时能按 Cookie 正确返回两套不同 HTML；随后换成相反 Cookie，仍命中首次缓存的主题。
- 仓库 `router/serveIndexPage` 明确设置 `Cache-Control: no-cache`，说明公网反向代理或 Cloudflare 规则覆盖了源站语义。

影响：首次访问、重新打开、刷新以及不同用户/主题之间会互相污染；缓存 TTL 为 31 天。

### High: HTML/Cookie 错配会稳定触发 ChunkLoadError

- 后端对 `/static/*` 也按当前 Cookie 选择 `web/default` 或 `web/classic` 文件系统。
- 经典前端 `/about` 的懒加载文件为 `/static/js/async/172.97471a0cc6.js`。
- 该文件带 `default` Cookie 回源返回 `404 application/json`，带 `classic` Cookie 返回 `200 text/javascript`。
- Chromium 现场复现：经典首页实际运行但 Cookie 为 `default`，点击 About 后该 chunk 404，浏览器报 `ChunkLoadError`，经典 `ErrorBoundary` 显示“页面渲染出错，请刷新页面重试”。

影响：这就是用户看到的 500/渲染错误直接原因；任何尚未被 CDN 预热的主题专属 chunk 都可能触发。

### High: 新版设置页没有同步优先级更高的主题 Cookie

- 服务端选择顺序是合法 Cookie 优先，全局 `theme.frontend` 仅作为无 Cookie 时的回退。
- 新版设置页保存 `classic` 成功后只执行 `window.location.replace('/')`，没有调用已有的 `switchToClassicFrontend()`，也没有写入 `new-api-frontend=classic`。
- 因此已有 `default` Cookie 会继续覆盖刚保存的全局 `classic`。
- 公网浏览器实测该矛盾状态为 `theme=default`、`system_theme=classic`。

影响：管理页显示全局值已是旧前端，但当前会话仍被 Cookie 固定到新前端。

### Warning: CSP nonce 被缓存复用

- `serveIndexPage` 每次生成 CSP nonce，本意是每个响应唯一。
- 公网 HIT 响应在不同 Cookie 请求中持续返回相同 nonce；观测时缓存年龄为 2734 秒。

影响：nonce 长期可预测，削弱 CSP 对脚本注入的防护。含 nonce 的 HTML 不应进入共享长期缓存。

### Warning: 回归测试缺少跨主题错配场景

- 当前测试只验证“主题 HTML + 同主题 Cookie + 同主题资源”成功。
- 没有断言 HTML 响应必须私有/不可缓存、必须按 Cookie 变体处理，也没有验证旧 HTML 在新 Cookie 下仍能加载其哈希资源。

## Recommended Order

1. 立即在 Cloudflare 停止缓存 `/` 和所有 SPA/无扩展名 HTML 路由，尊重源站 `no-store`，并清除已有 HTML 缓存；仅长期缓存内容哈希的 `/static/*`。
2. 源站 HTML 改为 `Cache-Control: private, no-store, max-age=0`，并添加 `Vary: Cookie`。
3. 新版系统设置保存主题后同步写入所选主题 Cookie，再跳转到对应路由；切换跳转增加一次性 cache-busting 查询参数可作为防御层。
4. 静态资源改为主题独立的命名空间，或对内容哈希资源在两个嵌入文件系统间回退查找，避免旧 HTML + 新 Cookie 时直接 404。
5. 增加 Cookie 变体、缓存头、跨主题旧 HTML chunk 和两向切换的集成测试。

## Evidence References

- `router/web-router.go`: `serveIndexPage`, `serveThemeStatic`, `selectThemeStatic`, `getRequestFrontendTheme`
- `controller/misc.go`: `/api/status` 的 `theme` 与 `system_theme`
- `web/default/src/features/system-settings/general/system-info-section.tsx`: 保存主题后的跳转
- `web/default/src/lib/frontend-theme.ts`: 已存在的经典主题 Cookie helper
- `web/classic/src/App.jsx`: `/about` 的 `React.lazy()`
- `web/classic/src/components/layout/PageLayout.jsx`: 全局 `ErrorBoundary`
- `web/classic/src/components/common/ErrorBoundary.jsx`: 渲染错误页面

## Scope

本任务只做诊断和审查，没有修改业务代码或公网配置。
