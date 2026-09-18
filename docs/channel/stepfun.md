# StepFun（阶跃星辰）渠道

- 渠道类型：`StepFun`（type 75），默认 Base URL：`https://api.stepfun.com`，Bearer 鉴权。
- Base 别名白名单：`stepfun`（国内开放平台）、`stepfun-intl`（国际站 api.stepfun.ai）、`stepfun-step-plan`、`stepfun-intl-step-plan`；也可直接填官方域名（容忍尾部 `/`、`/v1`、`/step_plan/v1`）。
- 文本三端点原生直传（命中白名单才启用，否则回落通用协议转换）：`/v1/chat/completions`、`/v1/messages`（Anthropic 兼容）、`/v1/responses`。
- Messages 入站按官方字段裁剪：剥离 `metadata`/`thinking`/`tool_choice`/`cache_control` 等 Anthropic 私有字段，`thinking` 预算映射为官方 `output_config.effort`；工具只保留 `name`/`description`/`input_schema`。
- 模型↔端点校验依据上游 `GET /v1/models/{id}` 的 `supported_protocols`（快照在 `constant/stepfun.go`）：如 `step-3.5-flash` 不支持 Responses，本地直接 400。
- TTS `/v1/audio/speech` 保留 StepFun 扩展字段（`volume`/`text_normalization`/`voice_label`/`instruction`/`sample_rate`/`pronunciation_map`/`markdown_filter`/`return_url`/`timestamp`），OpenAI 的 `instructions` 自动映射为 `instruction`；`return_url=true` 的 JSON 响应与 `stream_format=sse` 均原样透传。
- 原生端点透传（入站路径与上游 1:1）：`/v1/audio/generate`、`/v1/audio/music/submit|query`、`/v1/audio/asr/sse`、`/v1/audio/asr/file/submit|query`、`/v1/audio/voices`(GET/POST)、`/v1/audio/voices/preview`、`/v1/audio/system_voices`、`/v1/files*`。
- 路由用的 `model` 仅在渠道路由阶段使用：上游 schema 无该字段的端点（音乐、ASR 文件、SSE 等）会在转发前剥离；`?model=` 仅 `system_voices` 属上游真实参数。
- 提交类端点按模型价格计费，查询/列表类不重复计费。
- 无 `model` 字段的端点（`/v1/files*`、音色列表等）使用伪模型 `stepfun-native` 路由：需加入渠道模型清单（"填入所有模型"已包含），若令牌开启了模型白名单也需一并放行；查询类端点也可由客户端在请求体或 `?model=` 里带上模型名替代。
- WebSocket 原生端点原始双向透传（帧级转发，不改写事件）：`/v1/realtime/audio`（流式 TTS）、`/v1/audio/asr/stream`（双向流式识别，仅开放平台）、`/v1/realtime`（双向实时语音，按模型名 `stepaudio-*-realtime` 与 OpenAI Realtime 共用路径分派）。查询串原样转发（`?model=` 是上游必需参数）；上游 WS 不回用量，按请求体估算 prompt token 计费，建议为 WS 模型配置固定价格。
- 通道能力差异（实测）：Step Plan 通道仅提供文本三端点 + `audio/speech` + `audio/asr/sse` + `audio/voices` + `realtime/audio`；音乐、音频生成、ASR 文件异步、`system_voices`、`files`、`voices/preview` 仅开放平台提供，网关侧直接 400。
- 待实测项（账号额度限制）：`return_url` 音频下载域名、音乐/音频生成成功态载荷、`tool_choice` 是否可放宽。
