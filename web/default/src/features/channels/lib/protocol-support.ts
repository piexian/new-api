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

/**
 * Protocol view computed by the backend (relay/common/protocol_support.go), the
 * single source of the per-type defaults and of which channels are locked.
 */
export interface ChannelProtocolSupportInfo {
  defaults: ResolvedProtocolSupport
  resolved: ResolvedProtocolSupport
  locked: boolean
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

export function isGeminiFamilyChannel(channelType: number): boolean {
  return channelType === 24 || channelType === 72
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

/** Apply the explicit form overrides on top of the backend defaults. */
export function applyProtocolOverrides(
  defaults: ResolvedProtocolSupport,
  overrides: ChannelProtocolSupport | undefined
): ResolvedProtocolSupport {
  const resolved = { ...defaults }
  for (const { key } of CHANNEL_PROTOCOL_ITEMS) {
    const value = overrides?.[key]
    if (typeof value === 'boolean') {
      resolved[key] = value
    }
  }
  return resolved
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

/**
 * Setting string for one list-row toggle. The full target state is submitted;
 * the backend drops values equal to the defaults and moves Gemini Chat into the
 * compat flag, so no defaults matrix is needed here.
 */
export function buildUpdatedProtocolSupportSetting(
  rawSetting: string | null | undefined,
  current: ResolvedProtocolSupport,
  protocolKey: ChannelProtocolKey,
  nextValue: boolean
): string {
  const parsed = parseSettingObject(rawSetting)
  parsed.protocol_support = { ...current, [protocolKey]: nextValue }
  delete parsed.chat_completions_to_responses_enabled
  delete parsed.chat_completions_to_responses_models
  return JSON.stringify(parsed)
}
