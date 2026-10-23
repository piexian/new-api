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
// 协议默认值与锁定规则只在后端 relay/common/protocol_support.go 维护，
// 渠道列表/详情返回 protocol_support_info，编辑表单通过 resolve 接口获取。
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

export function isGeminiFamilyChannel(channelType) {
  return Number(channelType) === 24 || Number(channelType) === 72;
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

// 在后端默认值上叠加表单里的显式覆盖
export function applyProtocolOverrides(defaults, overrides) {
  const resolved = { ...defaults };
  CHANNEL_PROTOCOL_ITEMS.forEach(({ key }) => {
    if (typeof overrides?.[key] === 'boolean') {
      resolved[key] = overrides[key];
    }
  });
  return resolved;
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

// 列表页单个协议开关：提交完整目标状态，由后端丢弃与默认值相同的项，
// 并把 Gemini 的 Chat 归入兼容开关
export function buildUpdatedProtocolSupportSetting(
  rawSetting,
  current,
  protocolKey,
  nextValue,
) {
  const parsed = parseSettingObject(rawSetting);
  parsed.protocol_support = { ...current, [protocolKey]: nextValue };
  delete parsed.chat_completions_to_responses_enabled;
  delete parsed.chat_completions_to_responses_models;
  return JSON.stringify(parsed);
}
