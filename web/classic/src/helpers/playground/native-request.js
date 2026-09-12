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

function numberParameter(key, value) {
  return typeof value === 'number' && Number.isFinite(value)
    ? { [key]: value }
    : {};
}
export function supportsCodeInterpreter(chatInterface) {
  return chatInterface !== 'openai';
}
export function buildNativeRequest(chat, config) {
  const base = { model: chat.model, group: chat.group, stream: chat.stream };
  const messages = chat.messages.filter((message) => message.role !== 'system');
  const system = chat.messages
    .filter((message) => message.role === 'system')
    .map((message) => message.content).join(`
`);
  if (config.chatInterface === 'openai-response') {
    const tools = [];
    if (config.webSearchEnabled) tools.push({ type: 'web_search' });
    if (config.codeInterpreterEnabled) {
      tools.push({ type: 'code_interpreter', container: { type: 'auto' } });
    }
    return {
      endpoint: '/pg/responses',
      payload: {
        ...base,
        input: chat.messages.map((message) => ({
          role: message.role,
          content:
            typeof message.content === 'string'
              ? message.content
              : message.content.map((part) =>
                  part.type === 'text'
                    ? {
                        type:
                          message.role === 'assistant'
                            ? 'output_text'
                            : 'input_text',
                        text: part.text,
                      }
                    : { type: 'input_image', image_url: part.image_url?.url },
                ),
        })),
        ...numberParameter('temperature', chat.temperature),
        ...numberParameter('top_p', chat.top_p),
        ...numberParameter('max_output_tokens', chat.max_tokens),
        reasoning: chat.reasoning_effort
          ? { effort: chat.reasoning_effort }
          : undefined,
        tools: tools.length ? tools : undefined,
      },
    };
  }
  if (config.chatInterface === 'anthropic') {
    const tools = [];
    if (config.webSearchEnabled) {
      tools.push({ type: 'web_search_20250305', name: 'web_search' });
    }
    if (config.codeInterpreterEnabled) {
      tools.push({ type: 'code_execution_20250825', name: 'code_execution' });
    }
    return {
      endpoint: '/pg/messages',
      payload: {
        ...base,
        system: system || undefined,
        messages: messages.map((message) => ({
          ...message,
          content:
            typeof message.content === 'string'
              ? message.content
              : message.content.map((part) => {
                  if (part.type === 'text') {
                    return { type: 'text', text: part.text };
                  }
                  const url = part.image_url?.url ?? '';
                  const inline = /^data:([^;]+);base64,(.*)$/.exec(url);
                  return {
                    type: 'image',
                    source: inline
                      ? {
                          type: 'base64',
                          media_type: inline[1],
                          data: inline[2],
                        }
                      : { type: 'url', url },
                  };
                }),
        })),
        ...numberParameter('max_tokens', chat.max_tokens),
        ...numberParameter('temperature', chat.temperature),
        ...numberParameter('top_p', chat.top_p),
        output_config: chat.reasoning_effort
          ? { effort: chat.reasoning_effort }
          : undefined,
        tools: tools.length ? tools : undefined,
      },
    };
  }
  if (config.chatInterface === 'gemini') {
    const tools = [];
    if (config.webSearchEnabled) tools.push({ googleSearch: {} });
    if (config.codeInterpreterEnabled) tools.push({ codeExecution: {} });
    const action = chat.stream
      ? 'streamGenerateContent?alt=sse'
      : 'generateContent';
    return {
      endpoint: `/pg/v1beta/models/${encodeURIComponent(chat.model)}:${action}`,
      payload: {
        model: chat.model,
        group: chat.group,
        contents: messages.map((message) => ({
          role: message.role === 'assistant' ? 'model' : 'user',
          parts:
            typeof message.content === 'string'
              ? [{ text: message.content }]
              : message.content.map((part) => {
                  if (part.type === 'text') return { text: part.text };
                  const url = part.image_url?.url ?? '';
                  const inline = /^data:([^;]+);base64,(.*)$/.exec(url);
                  return inline
                    ? { inlineData: { mimeType: inline[1], data: inline[2] } }
                    : { fileData: { fileUri: url } };
                }),
        })),
        systemInstruction: system ? { parts: [{ text: system }] } : undefined,
        generationConfig: {
          ...numberParameter('temperature', chat.temperature),
          ...numberParameter('topP', chat.top_p),
          ...numberParameter('maxOutputTokens', chat.max_tokens),
          ...numberParameter('presencePenalty', chat.presence_penalty),
          ...numberParameter('frequencyPenalty', chat.frequency_penalty),
          ...numberParameter('seed', chat.seed),
        },
        tools: tools.length ? tools : undefined,
      },
    };
  }
  return { endpoint: '/pg/chat/completions', payload: chat };
}
