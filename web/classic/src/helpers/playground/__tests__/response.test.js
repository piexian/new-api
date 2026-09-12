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

import { describe, expect, it } from 'bun:test';
import {
  NATIVE_STREAM_EVENTS,
  normalizeNativeResponse,
  parseNativeStreamResponse,
} from '../native-response';
import { getResponseFormat } from '../request';

describe('native protocol responses', () => {
  const responses = {
    openai: {
      choices: [{ message: { content: 'answer', reasoning_content: 'think' } }],
    },
    'openai-response': {
      output: [
        { type: 'reasoning', summary: [{ text: 'think' }] },
        { type: 'message', content: [{ type: 'output_text', text: 'answer' }] },
      ],
    },
    anthropic: {
      content: [
        { type: 'thinking', thinking: 'think' },
        { type: 'text', text: 'answer' },
      ],
    },
    gemini: {
      candidates: [
        {
          content: {
            parts: [{ thought: true, text: 'think' }, { text: 'answer' }],
          },
          finishReason: 'STOP',
        },
      ],
    },
  };
  for (const [format, response] of Object.entries(responses)) {
    it(`${format} non-stream response`, () => {
      const result = normalizeNativeResponse(response, format).choices[0]
        .message;
      expect(result.content).toBe('answer');
      expect(result.reasoning_content).toBe('think');
    });
  }
  const streams = {
    openai: [
      {
        choices: [{ delta: { reasoning_content: 'think', content: 'answer' } }],
      },
      { choices: [{ finish_reason: 'stop', delta: {} }] },
    ],
    'openai-response': [
      { type: 'response.reasoning_summary_text.delta', delta: 'think' },
      { type: 'response.output_text.delta', delta: 'answer' },
      { type: 'response.completed' },
    ],
    anthropic: [
      {
        type: 'content_block_start',
        content_block: { type: 'thinking', thinking: 'think' },
      },
      {
        type: 'content_block_delta',
        delta: { type: 'text_delta', text: 'answer' },
      },
      { type: 'message_stop' },
    ],
    gemini: [responses.gemini],
  };
  for (const [format, events] of Object.entries(streams)) {
    it(`${format} stream content/reasoning and terminal event`, () => {
      const parsed = events.map(parseNativeStreamResponse);
      expect(
        parsed
          .flatMap((part) => part.updates)
          .filter((part) => part.type === 'content')
          .map((part) => part.chunk)
          .join(''),
      ).toBe('answer');
      expect(
        parsed
          .flatMap((part) => part.updates)
          .filter((part) => part.type === 'reasoning')
          .map((part) => part.chunk)
          .join(''),
      ).toBe('think');
      expect(parsed.at(-1).done).toBe(true);
    });
  }
  it('recognizes protocol-specific endpoints and named SSE events', () => {
    expect(getResponseFormat('/pg/chat/completions')).toBe('openai');
    expect(getResponseFormat('/pg/responses')).toBe('openai-response');
    expect(getResponseFormat('/pg/messages')).toBe('anthropic');
    expect(getResponseFormat('/pg/v1beta/models/test:generateContent')).toBe(
      'gemini',
    );
    for (const event of streams.anthropic.concat(streams['openai-response']))
      expect(NATIVE_STREAM_EVENTS).toContain(event.type);
  });
  it('renders Gemini code/output and Responses refusals', () => {
    const response = {
      candidates: [
        {
          content: {
            parts: [
              { executableCode: { language: 'PYTHON', code: 'print(1)' } },
              { codeExecutionResult: { output: '1' } },
            ],
          },
        },
      ],
    };
    const result = normalizeNativeResponse(response, 'gemini').choices[0]
      .message.content;
    expect(result).toContain('```python\nprint(1)');
    expect(result).toContain('```text\n1');
    expect(
      parseNativeStreamResponse({ type: 'response.refusal.delta', delta: 'no' })
        .updates,
    ).toEqual([{ type: 'content', chunk: 'no' }]);
    expect(
      normalizeNativeResponse(
        { output: [{ type: 'message', content: [{ refusal: 'no' }] }] },
        'openai-response',
      ).choices[0].message.content,
    ).toBe('no');
  });
  it('propagates native errors and incomplete responses', () => {
    for (const response of [
      { error: { message: 'bad' } },
      { promptFeedback: { blockReason: 'SAFETY' } },
    ]) {
      expect(parseNativeStreamResponse(response).error).toBeTruthy();
      expect(() => normalizeNativeResponse(response, 'gemini')).toThrow();
    }
    expect(
      parseNativeStreamResponse({
        type: 'response.failed',
        response: { error: { message: 'failed' } },
      }).error,
    ).toBe('failed');
    expect(
      parseNativeStreamResponse({
        type: 'response.incomplete',
        response: { incomplete_details: { reason: 'max_output_tokens' } },
      }).error,
    ).toBe('max_output_tokens');
    expect(() =>
      normalizeNativeResponse({ status: 'incomplete' }, 'openai-response'),
    ).toThrow();
    expect(
      parseNativeStreamResponse({
        type: 'content_block_delta',
        delta: { partial_json: '{' },
      }).updates,
    ).toEqual([]);
  });
});
