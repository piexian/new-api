# 会话标识风控

在风控中心「会话风控」页签开启「记录会话标识」后，网关会从原始请求中记录两类标识到日志：

- 会话 ID：`x-opencode-session-id`、`x-claude-code-session-id`、`x-astrbot-conversation-id`、`session-id` 请求头；Kimi CLI（`KimiCLI/` UA 或 `X-Msh-Platform: kimi_cli`）的 `metadata.user_id`。
- 缓存关联 ID：`x-session-affinity` 请求头与 `prompt_cache_key` 字段。

标识同时保存原文与 SHA-256 哈希（哈希用于大小写不敏感 collation 下的聚合与筛选）。仅采集消费/错误日志，multipart 与 WebSocket 请求不采集；存量日志不回填，关闭开关即停止采集。

「会话风控」页签聚合时间窗内被 ≥2 个用户共用的标识，展示关联账号、令牌、IP 与请求量，支持勾选批量封禁；封禁前会在所选时间窗内复核关联关系，防止凭过期数据误封。

日志页新增「Session ID」「Cache Key」精确筛选，对应 `GET /api/log` 的 `session_id`、`cache_key` 参数。

接口（均需 Root）：`GET/PUT /api/risk/session/config`、`GET /api/risk/session/shared`、`POST /api/risk/session/shared/ban-users`。

生产注意：升级会对 logs 表新增 7 列与 2 个复合索引（`idx_logs_session_time`、`idx_logs_cache_time`），大表建议在低峰期执行首次迁移。
