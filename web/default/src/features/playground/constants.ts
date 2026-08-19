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
import type { PlaygroundConfig, ParameterEnabled } from './types'

// Message constants
export const MESSAGE_ROLES = {
  USER: 'user',
  ASSISTANT: 'assistant',
  SYSTEM: 'system',
} as const

export const MESSAGE_STATUS = {
  LOADING: 'loading',
  STREAMING: 'streaming',
  COMPLETE: 'complete',
  ERROR: 'error',
} as const

// API endpoints
export const API_ENDPOINTS = {
  CHAT_COMPLETIONS: '/pg/chat/completions',
  RESPONSES: '/pg/responses',
  MESSAGES: '/pg/messages',
  IMAGE_GENERATIONS: '/pg/images/generations',
  IMAGE_EDITS: '/pg/images/edits',
  VIDEO_GENERATIONS: '/pg/videos/generations',
  VIDEO_EDITS: '/pg/videos/edits',
  VIDEO_EXTENSIONS: '/pg/videos/extensions',
  AUDIO_SPEECH: '/pg/audio/speech',
  AUDIO_TRANSCRIPTIONS: '/pg/audio/transcriptions',
  USER_MODELS: '/api/user/models',
  USER_GROUPS: '/api/user/self/groups',
  MODELS_DEV_CATALOG: '/api/user/models-dev/catalog',
} as const

// Playground 模式 Tab 配置
export const PLAYGROUND_MODES = [
  { mode: 'chat' as const, labelKey: 'playground.mode.chat' },
  { mode: 'image' as const, labelKey: 'playground.mode.image' },
  { mode: 'video' as const, labelKey: 'playground.mode.video' },
  { mode: 'audio' as const, labelKey: 'playground.mode.audio' },
]

// Chat 接口选项
export const CHAT_INTERFACE_OPTIONS = [
  { value: 'openai' as const, endpoint: '/pg/chat/completions', labelKey: 'OpenAI' },
  { value: 'openai-response' as const, endpoint: '/pg/responses', labelKey: 'Responses API' },
  { value: 'anthropic' as const, endpoint: '/pg/messages', labelKey: 'Anthropic' },
  { value: 'gemini' as const, endpoint: '/pg/v1beta/models/gemini-pro:generateContent', labelKey: 'Gemini' },
]

// 图片尺寸选项
export const IMAGE_SIZE_OPTIONS = ['256x256', '512x512', '1024x1024', '1792x1024', '1024x1792'] as const

// 图片质量选项
export const IMAGE_QUALITY_OPTIONS = ['standard', 'hd'] as const

// 图片风格选项
export const IMAGE_STYLE_OPTIONS = ['vivid', 'natural'] as const

// 音频 TTS voice 选项
export const AUDIO_VOICE_OPTIONS = ['alloy', 'echo', 'fable', 'onyx', 'nova', 'shimmer'] as const

// Default group — uses 'default' as the safe fallback; auto-group is
// only selected when the backend confirms it is available for the user.
export const DEFAULT_GROUP = 'default' as const

// Default configuration
export const DEFAULT_CONFIG: PlaygroundConfig = {
  model: 'gpt-4o',
  group: DEFAULT_GROUP,
  temperature: 0.7,
  top_p: 1,
  max_tokens: 4096,
  frequency_penalty: 0,
  presence_penalty: 0,
  seed: null,
  stream: true,
  reasoningEffort: 'none',
  toolsEnabled: false,
}

export const DEFAULT_PARAMETER_ENABLED: ParameterEnabled = {
  temperature: true,
  top_p: true,
  max_tokens: false,
  frequency_penalty: true,
  presence_penalty: true,
  seed: false,
}

// Storage keys
export const STORAGE_KEYS = {
  CONFIG: 'playground_config',
  MESSAGES: 'playground_messages',
  PARAMETER_ENABLED: 'playground_parameter_enabled',
} as const

// Error messages
export const ERROR_MESSAGES = {
  API_REQUEST_ERROR: 'Request error occurred',
  NETWORK_ERROR: 'Network connection failed or server not responding',
  PARSE_ERROR: 'Error parsing response data',
  STREAM_START_ERROR: 'Error establishing connection',
  CONNECTION_CLOSED: 'Connection closed',
  INTERRUPTED: 'Generation was interrupted',
} as const

// Message action button styles
export const MESSAGE_ACTION_BUTTON_STYLES = {
  BASE: 'size-7 text-muted-foreground hover:text-foreground',
  DELETE: 'size-7 text-muted-foreground hover:text-destructive',
  ICON: 'size-4',
} as const

// Message action labels
export const MESSAGE_ACTION_LABELS = {
  COPY: 'Copy',
  COPIED: 'Copied!',
  REGENERATE: 'Regenerate',
  SHOW_PREVIEW: 'Show preview',
  SHOW_SOURCE: 'Show source',
  EDIT: 'Edit',
  DELETE: 'Delete',
  NO_CONTENT: 'No content to copy',
  WAIT_GENERATION: 'Please wait for the current generation to complete',
} as const
