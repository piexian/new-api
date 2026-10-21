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

export const CHANNEL_PROTOCOL_ITEMS = [
  {
    key: 'chat',
    label: 'Chat',
    endpoint: '/v1/chat/completions',
    color: 'green',
  },
  {
    key: 'responses',
    label: 'Responses',
    endpoint: '/v1/responses',
    color: 'blue',
  },
  {
    key: 'messages',
    label: 'Messages',
    endpoint: '/v1/messages',
    color: 'purple',
  },
  {
    key: 'gemini',
    label: 'Gemini',
    endpoint: ':generateContent',
    color: 'amber',
  },
];

const ZCODE_BASE_URLS = new Set([
  'glm-coding-plan',
  'glm-coding-plan-international',
  'zcode-start-plan',
]);

// 与 relay/common/protocol_support.go 的 DefaultChannelProtocolSupport 保持一致
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
]);

function isGeminiFamilyChannel(channelType) {
  return Number(channelType) === 24 || Number(channelType) === 72;
}

function isZhipuZCodeBase(baseUrl) {
  return ZCODE_BASE_URLS.has(
    String(baseUrl || '')
      .trim()
      .toLowerCase(),
  );
}

function parseSettingObject(setting) {
  if (!setting) return {};
  if (typeof setting === 'string') {
    const trimmed = setting.trim();
    if (!trimmed) return {};
    try {
      const parsed = JSON.parse(trimmed);
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        return { ...parsed };
      }
    } catch {
      return {};
    }
    return {};
  }
  if (typeof setting === 'object' && !Array.isArray(setting)) {
    return { ...setting };
  }
  return {};
}

export function getDefaultProtocolSupport(channelType, baseUrl, setting) {
  const parsed = parseSettingObject(setting);
  switch (Number(channelType)) {
    case 0:
    case 64:
    case 65:
    case 67:
    case 73:
    case 77:
    case 78:
      return {
        chat: true,
        responses: true,
        messages: true,
        gemini: true,
      };
    case 26:
      if (parsed.zcode_mode_enabled === true && isZhipuZCodeBase(baseUrl)) {
        return {
          chat: false,
          responses: false,
          messages: true,
          gemini: false,
        };
      }
      return {
        chat: true,
        responses: true,
        messages: true,
        gemini: false,
      };
    case 4:
    case 17:
    case 25:
    case 35:
    case 43:
    case 59:
    case 62:
    case 63:
    case 66:
    case 69:
    case 75:
      return {
        chat: true,
        responses: true,
        messages: true,
        gemini: false,
      };
    case 1:
    case 3:
    case 27:
    case 45:
    case 46:
    case 48:
      return {
        chat: true,
        responses: true,
        messages: false,
        gemini: false,
      };
    case 16:
    case 71:
      return {
        chat: true,
        responses: false,
        messages: true,
        gemini: false,
      };
    case 57:
      return {
        chat: false,
        responses: true,
        messages: false,
        gemini: false,
      };
    case 14:
    case 33:
      return {
        chat: false,
        responses: false,
        messages: true,
        gemini: false,
      };
    case 24:
    case 72:
      return {
        chat: parsed.upstream_openai_compat_enabled === true,
        responses: false,
        messages: false,
        gemini: true,
      };
    case 41:
      return {
        chat: false,
        responses: false,
        messages: false,
        gemini: true,
      };
    default:
      // OpenAI 兼容上游原样转发 /v1/responses；无原生 Responses 端点的自有 adaptor 仅支持 Chat
      return {
        chat: true,
        responses: !NO_NATIVE_RESPONSES_CHANNEL_TYPES.has(Number(channelType)),
        messages: false,
        gemini: false,
      };
  }
}

export function resolveChannelProtocolSupport(channelType, baseUrl, setting) {
  const parsed = parseSettingObject(setting);
  const resolved = getDefaultProtocolSupport(channelType, baseUrl, parsed);
  const ps =
    parsed.protocol_support &&
    typeof parsed.protocol_support === 'object' &&
    !Array.isArray(parsed.protocol_support)
      ? parsed.protocol_support
      : undefined;

  if (typeof ps?.chat === 'boolean') {
    resolved.chat = ps.chat;
  }
  if (typeof ps?.responses === 'boolean') {
    resolved.responses = ps.responses;
  }
  if (typeof ps?.messages === 'boolean') {
    resolved.messages = ps.messages;
  }
  if (typeof ps?.gemini === 'boolean') {
    resolved.gemini = ps.gemini;
  }

  return resolved;
}

// 只保留与渠道类型默认值不同的协议，未改动的协议沿用 adaptor 内置处理；
// Gemini 渠道的 Chat 由 upstream_openai_compat_enabled 承载，不写入 protocol_support。
export function buildProtocolSupportOverrides(
  channelType,
  baseUrl,
  setting,
  protocolSupport,
) {
  if (!protocolSupport || typeof protocolSupport !== 'object') return undefined;
  const defaults = getDefaultProtocolSupport(channelType, baseUrl, setting);
  const overrides = {};
  CHANNEL_PROTOCOL_ITEMS.forEach(({ key }) => {
    if (key === 'chat' && isGeminiFamilyChannel(channelType)) return;
    const value = protocolSupport[key];
    if (typeof value === 'boolean' && value !== defaults[key]) {
      overrides[key] = value;
    }
  });
  return Object.keys(overrides).length > 0 ? overrides : undefined;
}

// 协议开关的表单值：Gemini 渠道的 Chat 只由兼容开关驱动
export function toProtocolSupportFormValue(channelType, value) {
  if (!isGeminiFamilyChannel(channelType)) return { ...value };
  return {
    responses: value.responses,
    messages: value.messages,
    gemini: value.gemini,
  };
}

// 在表单已有的显式覆盖上记录本次开关，避免之后切换类型/base_url 时按旧快照 diff；
// Gemini 渠道的 Chat 由兼容开关承载
export function withProtocolOverride(channelType, current, key, value) {
  const next = { ...(current || {}) };
  if (key === 'chat' && isGeminiFamilyChannel(channelType)) {
    delete next.chat;
    return next;
  }
  next[key] = value;
  return next;
}

export function buildUpdatedProtocolSupportSetting(
  channelType,
  baseUrl,
  rawSetting,
  protocolKey,
  nextValue,
) {
  const parsed = parseSettingObject(rawSetting);
  const nextProtocolSupport = {
    ...resolveChannelProtocolSupport(channelType, baseUrl, parsed),
    [protocolKey]: nextValue,
  };
  if (isGeminiFamilyChannel(channelType)) {
    parsed.upstream_openai_compat_enabled = nextProtocolSupport.chat;
  }
  const overrides = buildProtocolSupportOverrides(
    channelType,
    baseUrl,
    parsed,
    nextProtocolSupport,
  );
  if (overrides) {
    parsed.protocol_support = overrides;
  } else {
    delete parsed.protocol_support;
  }
  delete parsed.chat_completions_to_responses_enabled;
  delete parsed.chat_completions_to_responses_models;
  return JSON.stringify(parsed);
}
