/*
Copyright (C) 2025 QuantumNous

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

import { buildNativeRequest } from './native-request';
import { OPTIONAL_PARAMETERS, isValidParameter } from './parameters';

export function buildApiPayload(
  messages,
  systemPrompt,
  inputs,
  parameterEnabled = {},
) {
  const processedMessages = messages
    .filter(
      (message) => message?.role && (message.content || message.content === ''),
    )
    .map(({ role, content }) => ({ role, content }));
  if (systemPrompt?.trim()) {
    processedMessages.unshift({ role: 'system', content: systemPrompt.trim() });
  }
  const payload = {
    model: inputs.model,
    group: inputs.group,
    messages: processedMessages,
    stream: inputs.stream,
  };
  for (const key of OPTIONAL_PARAMETERS) {
    if (parameterEnabled[key] && isValidParameter(key, inputs[key])) {
      payload[key] = inputs[key];
    }
  }
  if (inputs.reasoningEffort && inputs.reasoningEffort !== 'none') {
    payload.reasoning_effort = inputs.reasoningEffort;
  }
  if (inputs.webSearchEnabled && inputs.chatInterface === 'openai') {
    payload.web_search_options = {};
  }
  return payload;
}

export function getChatEndpoint(chatInterface, model, stream) {
  switch (chatInterface) {
    case 'openai-response':
      return '/pg/responses';
    case 'anthropic':
      return '/pg/messages';
    case 'gemini':
      return `/pg/v1beta/models/${encodeURIComponent(model)}:${stream ? 'streamGenerateContent?alt=sse' : 'generateContent'}`;
    default:
      return '/pg/chat/completions';
  }
}

export function getResponseFormat(endpoint = '') {
  if (endpoint.startsWith('/pg/v1beta/models/')) return 'gemini';
  if (endpoint === '/pg/messages') return 'anthropic';
  if (endpoint === '/pg/responses') return 'openai-response';
  return 'openai';
}

// One request boundary for sending, regenerating, editing and previewing.
// Custom JSON is authoritative: never inject model, messages, tools or parameters.
export function buildPlaygroundRequest(
  messages,
  inputs,
  parameterEnabled,
  customRequestMode = false,
  customRequestBody = '',
) {
  if (customRequestMode) {
    const payload = JSON.parse(customRequestBody);
    if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
      throw new Error('自定义请求体格式错误，请检查JSON格式');
    }
    const isStream = payload.stream !== false;
    return {
      payload,
      isStream,
      endpoint: getChatEndpoint(
        inputs.chatInterface,
        payload.model ?? inputs.model,
        isStream,
      ),
    };
  }
  const chat = buildApiPayload(messages, null, inputs, parameterEnabled);
  return { ...buildNativeRequest(chat, inputs), isStream: inputs.stream };
}
