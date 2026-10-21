# OpenCode Zen / Go

选择 OpenCode 渠道（type 65）：`opencode-zen` 对应 `https://opencode.ai/inference`，`opencode-go` 对应 `https://opencode.ai/inference/go`（上游已迁移至 `/inference`，旧 `/zen` 路径仍可用但待下线；存量渠道 base 以 `/zen/go` 结尾仍识别为 Go）。

## 请求头

自动补齐 `User-Agent`、`x-opencode-client`、`x-opencode-project`、`x-opencode-session` 与 `x-opencode-session-id`，指纹对齐 OpenCode CLI 2.x（`opencode/latest/<version>/cli`）。默认客户端为 `cli`，项目为 `global`。

- 保留官方客户端铸造的 `ses_...`、项目、客户端标识及 `opencode/latest/<version>/cli` 形式的 User-Agent。
- 官方 session 编码为 12 位十六进制倒序时间戳（低 12 位 `0xfc0`–`0xfff`，即计数 1–64）加 14 位 Base62。形状正确但编码不在窗口内的客户端 ID 会被确定性重编码（同一信号 → 同一 ID），避免 403 且不破坏会话亲和。
- 无可用 session 信号时按官方算法生成：时间戳与共享计数编码为 12 位十六进制（倒序），再拼接 14 位加密随机 Base62；生成的 ID 在本次请求及其重试中复用。
- 网关自行推导的 session 按「渠道 + 令牌」隔离作用域，不同用户开场白相同也不会共用上游会话、缓存或限流桶。
- 不再生成或转发 `x-opencode-request`：2026-10-14 起上游对任何非空值返回 403。
- `x-opencode-project` 为规范 `prj_*` 形式时会被上游拒绝，网关将其稳定哈希为官方客户端使用的 40 位十六进制形式，同一会话固定到同一上游项目。
- `/inference`（v2）指纹补充 `x-session-affinity`、`X-Session-Id`、`Accept: application/json, text/event-stream`；子会话的 `x-opencode-session-id` 保留子 ID，`x-opencode-session` 与亲和头使用父会话，并补 `x-opencode-parent-session-id` / `x-parent-session-id`。旧 `/zen`（v1）两者携带同一 ID，不发亲和头。
- v2 的 Messages 请求固定 `?beta=true`、`anthropic-version: 2023-06-01` 与 `anthropic-beta: interleaved-thinking-2025-05-14`；v1 保持 1.x 形状。
- 渠道请求头覆盖仍具有最终优先级；固定或无效的 ID 覆盖值可能导致上游拒绝。
- Go 渠道默认透传客户端请求头（排除 hop-by-hop、鉴权、Content-Length 等），以保留上游会话与提示缓存亲和；Zen 渠道不透传，强制使用官方 CLI 指纹，因为 Zen 的 free 模型仅限 OpenCode 客户端使用。
- 匿名免费模型要求 `Authorization: Bearer public`（字面量，区分大小写）：用免费模型时把渠道密钥填 `public`；2026-10-14 起缺失该头直接 403。

## TLS 指纹与出口

`/zen`、`/inference` 上的免费模型流量通过 uTLS 复刻 Bun 运行时的 ClientHello（无 GREASE、仅 `http/1.1` ALPN、首组密钥共享 `X25519MLKEM768`，JA4 `t13d1713h1`），HTTP 走 1.1 且每次请求新建连接；`/zen/go` 与付费模型仍用标准 Go 传输与连接复用。代理沿用渠道 Proxy 设置，未配置时遵循进程 `HTTP(S)_PROXY` 环境；支持 `http`、`https`、`socks5`、`socks5h` 及 `#pool`/`pool+` 标记（标记仅表示轮换出口，本实现本就逐请求建连）。

免费通道的出口状态按「渠道 + 代理」记录，进程内生效：

- 上游 403 命中 IP 滥用拦截（`abusive traffic`、`abuse of traffic`、`Free models aren't available ... from your current network`）时，该出口冷却 15 分钟；同一 IP 换请求形状无法绕过，继续重试只会延长封禁。
- 传输失败（拨号、连接重置、超时）按 15 秒起指数冷却，上限 120 秒；`Retry-After` 只会延长该冷却。
- 连续 6 次失败后该渠道免费通道暂停，1 小时起每次再触发翻倍，上限 24 小时；任一成功请求清零冷却、失败计数与暂停。
- 冷却或暂停期间免费请求快速失败，由重试循环切换到其他渠道；多出口通过为同一模型配置多个不同代理的渠道实现。

## 模型与端点

| Zen 模型 | 上游端点 |
| --- | --- |
| MiMo Free、Ling Free、Nemotron Free、Big Pickle | `/openai/v1/chat/completions` |
| Muse Spark Contributor Free | `/openai/v1/responses` |
| `jev-1.13`、`jev-1.13-free` | `/systemone/v1/systemone` |

近期新增模型按以下端点路由：

| 渠道 | 上游端点 | 模型 ID |
| --- | --- | --- |
| Zen | `/openai/v1/responses` | `gpt-6-astra`、`gpt-6.1-sol`、`gpt-6-sol`、`gpt-6-luna`、`grok-4.7`、`muse-spark-1.3`、`muse-wonder-1.0` |
| Zen | `/anthropic/v1/messages` | `claude-fable-5-1`、`claude-opus-5-5-backup`、`claude-opus-5-5`、`claude-sonnet-5-5`、`qwen3.8-flash` |
| Zen | `/google/v1beta/models/{model}:generateContent` | `gemini-4-argon`、`gemini-3.8-flash` |
| Zen | `/openai/v1/chat/completions` | `deepseek-v4.1-flash-free`、`deepseek-v4.1-flash`、`glm-5.3-flash-free`、`glm-5.3-flash`、`glm-5.3`、`mistral-large-4`、`cat-jam-free`、`qwen3.8-max`、`space-bunny-free`、`longcat-2.5-preview-free`、`nemotron-3.5-super-vl-free`、`ll-alpha-free` |
| Go | `/openai/v1/responses` | `grok-4.7`、`grok-4.6`、`gpt-6-luna`、`muse-spark-1.3-contributor`、`muse-spark-1.2-contributor` |
| Go | `/anthropic/v1/messages` | `qwen3.8-flash` |
| Go | `/google/v1beta/models/{model}:generateContent` | `gemini-4-argon-contributor` |
| Go | `/openai/v1/chat/completions` | `glm-5.3-flash`、`longcat-2.0`、`longcat-2.5-preview-free`、`deepseek-v4.1-flash`、`mimo-v2.6-flash`、`mimo-v2.6-pro`、`cat-jam-free`、`hy4-preview`、`omen-alpha`、`space-bunny`、`step-5-preview`、`beam-free`、`minimax-m3.1-flash-free` |

`/inference` 与 `/inference/go` 网关均按协议分前缀（`/openai`、`/anthropic`、`/google`，System One 为 `/systemone`），Zen（console/models）与 Go（console/go）文档的 Endpoints 一节以此为准；Gemini 路径版本段为 `v1beta`；`/v1/models` 仍在网关根下，`/openai/v1/models` 为 404。旧的扁平 `/inference/v1/*`、`/inference/go/v1/*` 路由上游暂未下线（不带 key 探测与对照路径的 404 比较），本站已切到带前缀的新路径。遗留的 `/zen`、`/zen/go` 基址没有前缀路由（实测 404），继续使用扁平 `/v1/*`。粘贴带前缀的端点 URL 作为 Base URL 时会被归一化回网关根。

Zen 文档新增但未登记的 `minimax-m3.1-flash-free`、`beam-free`、`flint-40b-free` 未加入 Zen 路由表：`-free` 后缀的 Zen Chat 模型会触发 Free 请求整形，行为未经验证。它们仍可按未登记模型透传使用，Go 渠道的 `beam-free`、`minimax-m3.1-flash-free` 不受影响。

Zen 已下架 `qwen3.7-max`、`qwen3.7-plus`、`hy3-free`、`x-preview-f-free`、`claude-sonnet-4`、`exo-free`、`fledge-alpha-free`、`mimo-v2.5-free`、`ling-3.0-flash-fin-free`、`ling-3.1-flash-free`、`nemotron-3-ultra-free`、`deepseek-v4-flash-free`、`deepseek-v4-flash-vision-exp`、`step-5-preview-free`（上游返回 Model is unavailable、ModelDeprecated 或从模型目录移除），本站路由表同步移除；Go 仍保留 qwen3.7 系列与 step-5-preview-free。
Go 已下架 `glm-5`、`kimi-k2.5`、`grok-4.5`、`hy3-preview`、`mimo-v2-pro`、`mimo-v2-omni`、`qwen3.5-plus`、`deepseek-v4-flash`、`deepseek-v4-flash-vision-exp`、`deepseek-flash`，本站路由表同步移除。
Go 渠道的 `space-bunny-free` 经渠道实测仅支持 Chat；作为 RouteOnly 例外保留 Chat 路由，不加入 Go 静态模型回退列表。
`gemini-4-argon-contributor` 走原生 Gemini 端点（generateContent），该路由只在 `/inference/go` 网关上存在，旧 `/zen/go` base 未挂载（实测 404）；存量 `/zen/go` 渠道需把 base 换到 `/inference/go`。Zen 的 Gemini 端点要求 `x-goog-api-key` 鉴权，仅 Bearer 会被拒绝。

Chat、Responses、Claude Messages 和 Gemini generateContent 入站均按 Zen/Go 的模型协议表选上游端点：同协议保持原生格式，异协议双向转换，优先于全局或渠道请求体透传；未登记模型保留原透传规则。模型映射后按实际上游模型名匹配。count_tokens、Responses compact 等非文本生成端点不自动转换。

动态模型列表来自上游 `/v1/models`；`/inference` 后端无模型路由（404）时回退到旧 `/zen/v1/models` 目录。未登记的 Alpha 模型可手动配置并使用其实际支持的端点，不按名称猜测协议。模型列表可能与文档不同，隐藏模型和账号权限仍由上游决定。

上游地区、账号权限和限流规则仍然生效；请求头兼容不代表所有模型都可使用。

`-free` 表示上游产品名称，不自动修改本站计费倍率。免费模型可能用于提供方的数据收集，不要发送敏感内容。

## Free 文本请求

既有 Free 兼容模型及 Big Pickle 自动使用上游流式并补齐 `shell/edit/glob/grep/read`；`space-bunny-free` 只走 Chat 端点，不启用该请求整形。客户端仍按原协议接收 JSON 或 SSE，保留实际 usage。

- 2026-10-14 起上游逐个校验可识别工具的描述与参数 schema（与官方 CLI 定义逐字节一致），因此补入的工具使用内嵌的 CLI 2.0.32 官方定义（含 `patch`，`shell` 为 Linux/bash 描述）；调用方声明的同名可识别工具（如 `read`、`write`、`patch`）也会被替换为官方定义，未识别的调用方工具原样保留。2.x 使用 `shell`，不再注入旧拼写 `bash`。
- 保留调用方已有工具及选择；无工具的 Chat 请求默认禁用工具调用，Responses 保持上游支持的自动选择。补入工具的调用会被拒绝，不交给客户端执行。
- 非流式聚合上限 32 MiB，单个 SSE 事件上限 8 MiB；遵守客户端取消及流式空闲超时，缺少结束事件、上游报错或截断均按失败处理。
- 未登记模型的请求体透传跳过兼容处理；JEV、付费模型和 `space-bunny-free` 不套用。补入声明会计入上游输入用量，不改变本站倍率或渠道认证。

## JEV

JEV 只接受原生 `POST /v1/systemone`，不支持 Chat/Responses 转换或流式。OpenCode Go 不支持该端点。渠道测试自动选择 System One 并使用非流式请求。

```json
{
  "model": "jev-1.13-free",
  "state": "The customer reports a failed payment.",
  "questions": {
    "is_urgent": { "type": "noul", "instructions": "Does this require urgent attention?" }
  }
}
```

`state`、`questions`、附加字段及上游响应原样传递；模型映射只改写 `model`，渠道参数覆盖按原有规则应用。用量从 `usage.input_tokens/output_tokens` 提取。

参考：[Zen 文档](https://opencode.ai/docs/zen/) · [Go 文档](https://opencode.ai/docs/go/) · [ID 算法](https://github.com/anomalyco/opencode/blob/v1.19.0/packages/schema/src/identifier.ts)（CLI 2.x 指纹与 2026-10-14 门禁行为以实测为准）
