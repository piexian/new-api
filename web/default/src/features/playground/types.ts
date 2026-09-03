/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// Message types
export type MessageRole = 'user' | 'assistant' | 'system'

export type MessageStatus = 'loading' | 'streaming' | 'complete' | 'error'

export type PlaygroundMessageLayoutMode = 'alternating' | 'left'

// Playground 模式
export type PlaygroundMode = 'chat' | 'image' | 'video' | 'audio'

// Chat 接口类型（多 chat 接口切换）
export type ChatInterface =
  | 'openai'
  | 'openai-response'
  | 'anthropic'
  | 'gemini'

// 图片接口类型
export type ImageInterface = 'generations' | 'edits'

// 视频接口类型
export type VideoInterface = 'generations' | 'edits' | 'extensions'

// 音频接口类型
export type AudioInterface = 'speech' | 'transcriptions'

export interface MessageVersion {
  id: string
  content: string
}

export interface Message {
  key: string
  from: MessageRole
  versions: MessageVersion[]
  createdAt?: number
  startedAt?: number
  completedAt?: number
  durationMs?: number
  sources?: { href: string; title: string }[]
  reasoning?: {
    content: string
    duration: number
    startedAt?: number
    completedAt?: number
    durationMs?: number
  }
  isReasoningStreaming?: boolean
  toolCalls?: ToolCallInfo[]
  isReasoningComplete?: boolean
  isContentComplete?: boolean
  status?: MessageStatus
  errorCode?: string | null
}

// API payload types
export interface ChatCompletionMessage {
  role: MessageRole
  content: string | ContentPart[]
}

export interface ToolCallInfo {
  id: string
  name: string
  arguments: string
}

export interface ContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: {
    url: string
  }
}

export interface PlaygroundTool {
  type: 'function'
  function: {
    name: string
    description?: string
    parameters?: Record<string, unknown>
  }
}

export interface ChatCompletionRequest {
  model: string
  group?: string
  messages: ChatCompletionMessage[]
  stream: boolean
  temperature?: number
  top_p?: number
  max_tokens?: number
  frequency_penalty?: number
  presence_penalty?: number
  seed?: number
  reasoning_effort?: 'low' | 'medium' | 'high' | 'max'
  tools?: PlaygroundTool[]
}

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    delta: {
      role?: MessageRole
      content?: string
      reasoning_content?: string
      tool_calls?: Array<
        | {
            index: number
            id?: string
            function: { name?: string; arguments?: string }
            type?: string
          }
        | {
            index: number
            id: string
            function: { name: string; arguments: string }
            type: 'function'
          }
      >
    }
    finish_reason: string | null
  }>
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    message: {
      role: MessageRole
      content: string
      reasoning_content?: string
    }
    finish_reason: string
  }>
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

// Configuration types
export interface PlaygroundConfig {
  model: string
  group: string
  temperature: number
  top_p: number
  max_tokens: number
  frequency_penalty: number
  presence_penalty: number
  seed: number | null
  stream: boolean
  reasoningEffort: 'none' | 'low' | 'medium' | 'high' | 'max'
  webSearchEnabled: boolean
  codeInterpreterEnabled: boolean
  chatInterface: ChatInterface
}

export interface ParameterEnabled {
  temperature: boolean
  top_p: boolean
  max_tokens: boolean
  frequency_penalty: boolean
  presence_penalty: boolean
  seed: boolean
}

// Model and group options
export interface ModelOption {
  label: string
  value: string
  supportedEndpointTypes?: string[]
  reasoning?: boolean
  toolCall?: boolean
}

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}

// models.dev catalog entry (保底分类)
export interface ModelsDevEntry {
  id: string
  name?: string
  family?: string
  reasoning: boolean
  reasoning_options?: { type: string; values?: string[] }[]
  tool_call: boolean
  attachment: boolean
  modalities: { input: string[]; output: string[] }
}

// 对话类 endpoint types（非 embed/rerank/audio-only 等）
export const CHAT_CAPABLE_ENDPOINTS = new Set([
  'openai',
  'openai-response',
  'anthropic',
  'gemini',
  'cohere-chat',
])

// 图片生成请求
export interface ImageGenerationRequest {
  model: string
  group?: string
  prompt: string
  n?: number
  size?: string
  quality?: string
  style?: string
  response_format?: 'url' | 'b64_json'
}

// 图片编辑请求
export interface ImageEditRequest {
  model: string
  group?: string
  prompt: string
  image: string | string[]
  mask?: string
  n?: number
  size?: string
  response_format?: 'url' | 'b64_json'
}

export interface ImageData {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface ImageResponse {
  created: number
  data: ImageData[]
}

// 视频生成请求
export interface VideoGenerationRequest {
  model: string
  group?: string
  prompt: string
  image?: string
  size?: string
  duration?: number
  fps?: number
}

export interface VideoTask {
  id: string
  status: 'queued' | 'processing' | 'succeeded' | 'failed'
  video?: { url: string; duration_seconds?: number }
  error?: { code: string; message: string }
}

export interface VideoTaskResponse {
  id: string
  status: 'queued' | 'processing' | 'succeeded' | 'failed'
  video?: { url: string; duration_seconds?: number }
  error?: { code: string; message: string }
}

// 音频 TTS 请求
export interface AudioSpeechRequest {
  model: string
  group?: string
  input: string
  voice?: string
  response_format?: string
  speed?: number
}

// 音频转写请求
export interface AudioTranscriptionRequest {
  model: string
  group?: string
  file: File
  language?: string
  prompt?: string
  response_format?: string
}

export interface AudioTranscriptionResponse {
  text: string
}
