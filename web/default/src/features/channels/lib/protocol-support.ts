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
import type { ChannelProtocolSupport, ChannelSettings } from '../types'

export type ChannelProtocolKey = 'chat' | 'responses' | 'messages' | 'gemini'

export interface ResolvedProtocolSupport {
  chat: boolean
  responses: boolean
  messages: boolean
  gemini: boolean
}

export const CHANNEL_PROTOCOL_ITEMS: readonly {
  key: ChannelProtocolKey
  label: string
  endpoint: string
}[] = [
  { key: 'chat', label: 'Chat', endpoint: '/v1/chat/completions' },
  { key: 'responses', label: 'Responses', endpoint: '/v1/responses' },
  { key: 'messages', label: 'Messages', endpoint: '/v1/messages' },
  { key: 'gemini', label: 'Gemini', endpoint: ':generateContent' },
]

const ZCODE_BASE_URLS = new Set([
  'glm-coding-plan',
  'glm-coding-plan-international',
  'zcode-start-plan',
])

// Keep in sync with DefaultChannelProtocolSupport in relay/common/protocol_support.go
const NO_NATIVE_RESPONSES_CHANNEL_TYPES = new Set([
  11, // PaLM
  15, // Baidu
  18, // Xunfei
  23, // Tencent
  34, // Cohere
  37, // Dify
  38, // Jina
  40, // SiliconFlow
  42, // Mistral
  44, // MokaAI
  49, // Coze
  51, // Jimeng
  53, // Submodel
  56, // Replicate
  61, // XunfeiMaaSImage
  68, // Cerebras
  70, // MistralConsole
  76, // TypeSafe
])

function isGeminiFamilyChannel(channelType: number): boolean {
  return channelType === 24 || channelType === 72
}

function isZhipuZCodeBase(baseUrl?: string | null): boolean {
  return ZCODE_BASE_URLS.has(
    String(baseUrl || '')
      .trim()
      .toLowerCase()
  )
}

function parseSettingObject(
  setting?: Partial<ChannelSettings> | string | null
): Record<string, unknown> {
  if (!setting) return {}
  if (typeof setting === 'string') {
    const trimmed = setting.trim()
    if (!trimmed) return {}
    try {
      const parsed = JSON.parse(trimmed)
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        return parsed as Record<string, unknown>
      }
    } catch {
      return {}
    }
    return {}
  }
  if (typeof setting === 'object' && !Array.isArray(setting)) {
    return setting as Record<string, unknown>
  }
  return {}
}

export function getDefaultProtocolSupport(
  channelType: number,
  baseUrl?: string | null,
  setting?: Partial<ChannelSettings> | string | null
): ResolvedProtocolSupport {
  const parsed = parseSettingObject(setting)
  switch (channelType) {
    case 0:
    case 64: // ZenMux
    case 65: // OpenCode
    case 67: // AdvancedCustom
    case 73: // CLIProxyAPI
    case 77: // Sub2API
    case 78: // NewAPI
      return {
        chat: true,
        responses: true,
        messages: true,
        gemini: true,
      }
    case 26: // Zhipu_v4
      if (parsed.zcode_mode_enabled === true && isZhipuZCodeBase(baseUrl)) {
        return {
          chat: false,
          responses: false,
          messages: true,
          gemini: false,
        }
      }
      return {
        chat: true,
        responses: true,
        messages: true,
        gemini: false,
      }
    case 4: // Ollama
    case 17: // Ali
    case 25: // Moonshot
    case 35: // MiniMax
    case 43: // DeepSeek
    case 59: // Poe
    case 62: // XiaomiMiMo
    case 63: // AgnesAI
    case 66: // Moark
    case 69: // QwenTokenPlan
    case 75: // StepFun
      return {
        chat: true,
        responses: true,
        messages: true,
        gemini: false,
      }
    case 1: // OpenAI
    case 3: // Azure
    case 27: // Perplexity
    case 45: // VolcEngine
    case 46: // BaiduV2
    case 48: // xAI
      return {
        chat: true,
        responses: true,
        messages: false,
        gemini: false,
      }
    case 16: // Zhipu
    case 71: // GMICloud
      return {
        chat: true,
        responses: false,
        messages: true,
        gemini: false,
      }
    case 57: // Codex
      return {
        chat: false,
        responses: true,
        messages: false,
        gemini: false,
      }
    case 14: // Anthropic
    case 33: // AWS
    case 81: // MiniMax Code
      return {
        chat: false,
        responses: false,
        messages: true,
        gemini: false,
      }
    case 24: // Gemini
    case 72: // GeminiInteractions
      return {
        chat: parsed.upstream_openai_compat_enabled === true,
        responses: false,
        messages: false,
        gemini: true,
      }
    case 41: // VertexAi
      return {
        chat: false,
        responses: false,
        messages: false,
        gemini: true,
      }
    default:
      // OpenAI-compatible upstreams relay /v1/responses natively; own adaptors
      // without a native Responses endpoint only support Chat.
      return {
        chat: true,
        responses: !NO_NATIVE_RESPONSES_CHANNEL_TYPES.has(channelType),
        messages: false,
        gemini: false,
      }
  }
}

export function resolveChannelProtocolSupport(
  channelType: number,
  baseUrl?: string | null,
  setting?: Partial<ChannelSettings> | string | null
): ResolvedProtocolSupport {
  const parsed = parseSettingObject(setting)
  const resolved = getDefaultProtocolSupport(channelType, baseUrl, parsed)
  const ps =
    parsed.protocol_support &&
    typeof parsed.protocol_support === 'object' &&
    !Array.isArray(parsed.protocol_support)
      ? (parsed.protocol_support as ChannelProtocolSupport)
      : undefined

  if (typeof ps?.chat === 'boolean') {
    resolved.chat = ps.chat
  }
  if (typeof ps?.responses === 'boolean') {
    resolved.responses = ps.responses
  }
  if (typeof ps?.messages === 'boolean') {
    resolved.messages = ps.messages
  }
  if (typeof ps?.gemini === 'boolean') {
    resolved.gemini = ps.gemini
  }

  return resolved
}

/**
 * Keep only the protocols that differ from the channel-type defaults, so
 * protocols left at their default keep the adaptor's built-in handling. Gemini
 * channels store Chat as upstream_openai_compat_enabled instead.
 */
export function buildProtocolSupportOverrides(
  channelType: number,
  baseUrl: string | null | undefined,
  setting: Partial<ChannelSettings> | Record<string, unknown>,
  protocolSupport: ChannelProtocolSupport | undefined
): ChannelProtocolSupport | undefined {
  if (!protocolSupport) return undefined
  const defaults = getDefaultProtocolSupport(channelType, baseUrl, setting)
  const overrides: ChannelProtocolSupport = {}
  for (const { key } of CHANNEL_PROTOCOL_ITEMS) {
    if (key === 'chat' && isGeminiFamilyChannel(channelType)) continue
    const value = protocolSupport[key]
    if (typeof value === 'boolean' && value !== defaults[key]) {
      overrides[key] = value
    }
  }
  return Object.keys(overrides).length > 0 ? overrides : undefined
}

/**
 * Form value for the protocol switches: Gemini channels drive Chat through
 * upstream_openai_compat_enabled, so it is never kept in protocol_support.
 */
export function toProtocolSupportFormValue(
  channelType: number,
  value: ChannelProtocolSupport
): ChannelProtocolSupport {
  if (!isGeminiFamilyChannel(channelType)) return { ...value }
  return {
    responses: value.responses,
    messages: value.messages,
    gemini: value.gemini,
  }
}

/**
 * Record one switch change on top of the explicit overrides already in the
 * form, so later type or base URL edits are diffed against the new defaults
 * instead of a stale snapshot. Gemini Chat lives in the compat flag instead.
 */
export function withProtocolOverride(
  channelType: number,
  current: ChannelProtocolSupport | undefined,
  key: ChannelProtocolKey,
  value: boolean
): ChannelProtocolSupport {
  const next: ChannelProtocolSupport = { ...current }
  if (key === 'chat' && isGeminiFamilyChannel(channelType)) {
    delete next.chat
    return next
  }
  next[key] = value
  return next
}

export function buildUpdatedProtocolSupportSetting(
  channelType: number,
  baseUrl: string | null | undefined,
  rawSetting: string | null | undefined,
  protocolKey: ChannelProtocolKey,
  nextValue: boolean
): string {
  const parsed = parseSettingObject(rawSetting)
  const nextProtocolSupport: ResolvedProtocolSupport = {
    ...resolveChannelProtocolSupport(channelType, baseUrl, parsed),
    [protocolKey]: nextValue,
  }
  if (isGeminiFamilyChannel(channelType)) {
    parsed.upstream_openai_compat_enabled = nextProtocolSupport.chat
  }
  const overrides = buildProtocolSupportOverrides(
    channelType,
    baseUrl,
    parsed,
    nextProtocolSupport
  )
  if (overrides) {
    parsed.protocol_support = overrides
  } else {
    delete parsed.protocol_support
  }
  delete parsed.chat_completions_to_responses_enabled
  delete parsed.chat_completions_to_responses_models
  return JSON.stringify(parsed)
}
