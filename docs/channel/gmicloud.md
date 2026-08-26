# GMI Cloud 渠道

- 渠道类型：`GMI Cloud`，默认 Base URL：`https://api.gmi-serving.com`，密钥使用 Bearer 认证。
- 文本模型 `MiniMaxAI/MiniMax-M2.7` 支持 `/v1/chat/completions`、`/v1/messages` 和 `/v1/responses`；Responses 在网关内转换为 Chat Completions。
- 音频请求自动使用 GMI requestqueue：`minimax-tts-speech-2.8-turbo`、`minimax-tts-speech-2.8-hd`、两个 `minimax-audio-voice-clone-speech-*` 模型。
- 语音克隆调用 `/v1/audio/speech` 时，`metadata` 必须包含 HTTP(S) `source_audio` URL。
- 音乐仅支持 `minimax-music-3.0` 的 MiniMax 原生 `/v1/music_generation` 格式；不支持流式生成，`output_format` 仅支持 `url` 或 `hex`。
- 音频和音乐任务完成前由网关轮询 GMI 状态接口；终态失败不会切换密钥重试，以免重复创建或扣费。
