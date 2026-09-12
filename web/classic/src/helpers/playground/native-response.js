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

function blockUpdates(blocks) {
  return blocks.flatMap((block) => {
    if (block.thinking) return [{ type: 'reasoning', chunk: block.thinking }];
    if (block.text) {
      return [
        { type: block.thought ? 'reasoning' : 'content', chunk: block.text },
      ];
    }
    if (block.refusal) return [{ type: 'content', chunk: block.refusal }];
    if (block.executableCode?.code) {
      return [
        {
          type: 'content',
          chunk: `

\`\`\`${block.executableCode.language?.toLowerCase() ?? ''}
${block.executableCode.code}
\`\`\`
`,
        },
      ];
    }
    if (block.codeExecutionResult?.output) {
      return [
        {
          type: 'content',
          chunk: `

\`\`\`text
${block.codeExecutionResult.output}
\`\`\`
`,
        },
      ];
    }
    return [];
  });
}
export const NATIVE_STREAM_EVENTS = [
  'content_block_start',
  'content_block_delta',
  'message_stop',
  'response.output_text.delta',
  'response.reasoning_summary_text.delta',
  'response.refusal.delta',
  'response.completed',
  'response.failed',
  'response.incomplete',
];
export function parseNativeStreamResponse(response) {
  const error =
    response.error?.message ??
    response.response?.error?.message ??
    response.promptFeedback?.blockReason;
  if (error) return { updates: [], done: false, error };
  if (response.choices) {
    const choice = response.choices[0];
    const delta = choice?.delta;
    const updates = [];
    if (delta?.reasoning_content || delta?.reasoning)
      updates.push({
        type: 'reasoning',
        chunk: delta.reasoning_content || delta.reasoning,
      });
    if (delta?.content) updates.push({ type: 'content', chunk: delta.content });
    return { updates, done: Boolean(choice?.finish_reason) };
  }
  if (
    response.type === 'response.failed' ||
    response.type === 'response.incomplete'
  ) {
    return {
      updates: [],
      done: false,
      error: response.response?.incomplete_details?.reason ?? response.type,
    };
  }
  if (
    response.type === 'message_stop' ||
    response.type === 'response.completed'
  ) {
    return { updates: [], done: true };
  }
  if (response.candidates) {
    const candidate = response.candidates[0];
    return {
      updates: blockUpdates(candidate?.content?.parts ?? []),
      done: Boolean(candidate?.finishReason),
    };
  }
  if (typeof response.delta === 'string') {
    if (
      response.type === 'response.output_text.delta' ||
      response.type === 'response.refusal.delta'
    ) {
      return {
        updates: [{ type: 'content', chunk: response.delta }],
        done: false,
      };
    }
    if (response.type === 'response.reasoning_summary_text.delta') {
      return {
        updates: [{ type: 'reasoning', chunk: response.delta }],
        done: false,
      };
    }
  }
  if (response.type === 'content_block_start' && response.content_block) {
    return { updates: blockUpdates([response.content_block]), done: false };
  }
  if (
    response.type === 'content_block_delta' &&
    typeof response.delta === 'object'
  ) {
    return { updates: blockUpdates([response.delta]), done: false };
  }
  return { updates: [], done: false };
}
export function normalizeNativeResponse(response, format) {
  const error = response.error?.message ?? response.promptFeedback?.blockReason;
  if (error) throw new Error(error);
  if (response.status === 'failed' || response.status === 'incomplete') {
    throw new Error(response.incomplete_details?.reason ?? response.status);
  }
  if (format === 'openai') return response;
  let updates = [];
  if (format === 'anthropic') updates = blockUpdates(response.content ?? []);
  if (format === 'gemini') {
    updates = blockUpdates(response.candidates?.[0]?.content?.parts ?? []);
  }
  if (format === 'openai-response') {
    updates = (response.output ?? []).flatMap((item) => {
      if (item.type === 'reasoning') {
        return (item.summary ?? []).flatMap((part) =>
          part.text ? [{ type: 'reasoning', chunk: part.text }] : [],
        );
      }
      return blockUpdates(item.content ?? []);
    });
  }
  return {
    id: response.id ?? '',
    model: response.model ?? '',
    object: 'chat.completion',
    created: 0,
    choices: [
      {
        index: 0,
        finish_reason: 'stop',
        message: {
          role: 'assistant',
          content: updates
            .filter((update) => update.type === 'content')
            .map((update) => update.chunk)
            .join(''),
          reasoning_content: updates
            .filter((update) => update.type === 'reasoning')
            .map((update) => update.chunk)
            .join(''),
        },
      },
    ],
  };
}
