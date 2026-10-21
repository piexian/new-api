# Ollama 渠道

渠道类型 `Ollama`（type 4），同时支持本地服务和 Ollama Cloud。

- Base URL 填服务根地址：本地 `http://localhost:11434`，云端 `https://ollama.com`。末尾的 `/`、`/v1`、`/api` 会被自动去掉，云端文档里的 `https://ollama.com/v1`、`https://ollama.com/api` 可以直接粘贴。
- 鉴权固定使用 `Authorization: Bearer <key>`。云端必须填 API key（本地服务忽略该值，也不接受 `x-api-key`）。
- 每种入站协议只对应一条上游路径：
  - `POST /v1/chat/completions` → `/api/chat`（原生 Ollama chat 协议）
  - `POST /v1/completions` → `/api/generate`
  - `POST /v1/embeddings` → `/api/embed`（云端没有 `/v1/embeddings`，统一走原生端点）
  - `POST /v1/responses` → `/v1/responses` 原样透传
  - `POST /v1/messages` → `/v1/messages` 原样透传，网关补 `Authorization: Bearer` 和 `anthropic-version: 2023-06-01`
- Gemini 入站不支持；`/v1/messages/count_tokens`、`/v1/responses/compact`、`/v1/responses/input_tokens` 在云端不存在，会返回上游 404。
- 云端不支持 `tool_choice`、`metadata`、提示缓存、Batches、Citations 和 PDF 文档块，`budget_tokens` 会被接受但不生效。
- 模型列表读取 `/api/tags`；拉取、删除模型和版本查询只对本地服务有意义，云端创建或删除模型需要在本地 Ollama 服务器上操作。
