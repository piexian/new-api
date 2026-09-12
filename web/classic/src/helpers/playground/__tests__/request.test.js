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
import { DEFAULT_CONFIG } from '../../../constants/playground.constants';
import { buildApiPayload, buildPlaygroundRequest } from '../request';
import { OPTIONAL_PARAMETERS } from '../parameters';

const formats = ['openai', 'openai-response', 'anthropic', 'gemini'];
const messages = [
  { role: 'system', content: 'system' },
  { role: 'user', content: 'hello' },
];
const enabled = Object.fromEntries(
  OPTIONAL_PARAMETERS.map((key) => [key, true]),
);
const wire = (value) => JSON.parse(JSON.stringify(value));

describe('native playground requests', () => {
  for (const chatInterface of formats) {
    for (const webSearchEnabled of [false, true]) {
      for (const codeInterpreterEnabled of [false, true]) {
        it(`${chatInterface}: search=${webSearchEnabled}, code=${codeInterpreterEnabled}`, () => {
          const inputs = {
            ...DEFAULT_CONFIG.inputs,
            model: 'model/name',
            chatInterface,
            webSearchEnabled,
            codeInterpreterEnabled,
          };
          const { payload, endpoint } = buildPlaygroundRequest(
            messages,
            inputs,
            enabled,
          );
          expect(payload).toBeDefined();
          expect(JSON.stringify(payload)).not.toContain('"function"');
          if (chatInterface === 'openai') {
            expect(payload.web_search_options).toEqual(
              webSearchEnabled ? {} : undefined,
            );
            expect(payload.tools).toBeUndefined();
            expect(endpoint).toBe('/pg/chat/completions');
          } else {
            expect(payload.tools?.length ?? 0).toBe(
              Number(webSearchEnabled) + Number(codeInterpreterEnabled),
            );
            if (chatInterface === 'openai-response') {
              expect(endpoint).toBe('/pg/responses');
              expect(payload.input).toEqual(messages);
              if (webSearchEnabled)
                expect(payload.tools).toContainEqual({ type: 'web_search' });
              if (codeInterpreterEnabled)
                expect(payload.tools).toContainEqual({
                  type: 'code_interpreter',
                  container: { type: 'auto' },
                });
            }
            if (chatInterface === 'anthropic') {
              expect(endpoint).toBe('/pg/messages');
              expect(payload.system).toBe('system');
              expect(payload.messages).toEqual([messages[1]]);
              if (webSearchEnabled)
                expect(payload.tools).toContainEqual({
                  type: 'web_search_20250305',
                  name: 'web_search',
                });
              if (codeInterpreterEnabled)
                expect(payload.tools).toContainEqual({
                  type: 'code_execution_20250825',
                  name: 'code_execution',
                });
              expect('max_tokens' in payload).toBe(false);
            }
            if (chatInterface === 'gemini') {
              expect(endpoint).toBe(
                '/pg/v1beta/models/model%2Fname:streamGenerateContent?alt=sse',
              );
              expect(payload.contents).toEqual([
                { role: 'user', parts: [{ text: 'hello' }] },
              ]);
              expect(payload.systemInstruction).toEqual({
                parts: [{ text: 'system' }],
              });
              if (webSearchEnabled)
                expect(payload.tools).toContainEqual({ googleSearch: {} });
              if (codeInterpreterEnabled)
                expect(payload.tools).toContainEqual({ codeExecution: {} });
            }
          }
          expect(JSON.stringify(payload)).not.toContain(':null');
        });
      }
    }
    it(`${chatInterface}: custom body has priority without rewriting`, () => {
      const custom = {
        model: 'custom/model',
        stream: false,
        tools: [{ type: 'my-native-tool' }],
        contents: ['untouched'],
        temperature: null,
        extension: 0,
      };
      const inputs = {
        ...DEFAULT_CONFIG.inputs,
        chatInterface,
        webSearchEnabled: true,
        codeInterpreterEnabled: true,
      };
      const body = JSON.stringify(custom, null, 3);
      const result = buildPlaygroundRequest(
        messages,
        inputs,
        enabled,
        true,
        body,
      );
      expect(result.payload).toEqual(custom);
      expect(result.isStream).toBe(false);
      if (chatInterface === 'gemini')
        expect(result.endpoint).toBe(
          '/pg/v1beta/models/custom%2Fmodel:generateContent',
        );
      const streaming = buildPlaygroundRequest(
        [],
        inputs,
        {},
        true,
        JSON.stringify({ ...custom, stream: true }),
      );
      expect(streaming.isStream).toBe(true);
      if (chatInterface === 'gemini')
        expect(streaming.endpoint).toEndWith(':streamGenerateContent?alt=sse');
      expect(body).toBe(JSON.stringify(custom, null, 3));
    });
  }

  it('buildApiPayload returns valid zero values, omits invalid/disabled fields', () => {
    const inputs = {
      ...DEFAULT_CONFIG.inputs,
      ...Object.fromEntries(OPTIONAL_PARAMETERS.map((key) => [key, 0])),
    };
    const payload = buildApiPayload(messages, 'extra', inputs, enabled);
    expect(payload.messages[0]).toEqual({ role: 'system', content: 'extra' });
    for (const key of OPTIONAL_PARAMETERS) {
      expect(payload[key]).toBe(0);
      for (const invalid of [null, '', '0', undefined, NaN, Infinity, false]) {
        expect(
          buildApiPayload(
            messages,
            null,
            { ...inputs, [key]: invalid },
            enabled,
          )[key],
        ).toBeUndefined();
      }
      expect(
        buildApiPayload(messages, null, inputs, { ...enabled, [key]: false })[
          key
        ],
      ).toBeUndefined();
    }
  });

  it('all protocols map enabled numeric fields and omit empty values', () => {
    for (const chatInterface of formats) {
      const inputs = {
        ...DEFAULT_CONFIG.inputs,
        chatInterface,
        ...Object.fromEntries(OPTIONAL_PARAMETERS.map((key) => [key, 0])),
      };
      const payload = buildPlaygroundRequest(messages, inputs, enabled).payload;
      const parameters =
        chatInterface === 'gemini' ? payload.generationConfig : payload;
      expect(parameters[chatInterface === 'gemini' ? 'topP' : 'top_p']).toBe(0);
      expect(
        parameters[
          chatInterface === 'gemini'
            ? 'maxOutputTokens'
            : chatInterface === 'openai-response'
              ? 'max_output_tokens'
              : 'max_tokens'
        ],
      ).toBe(0);
      const empty = wire(
        buildPlaygroundRequest(
          messages,
          { ...DEFAULT_CONFIG.inputs, chatInterface },
          enabled,
        ).payload,
      );
      expect(JSON.stringify(empty)).not.toContain('temperature');
      expect(JSON.stringify(empty)).not.toContain('max_tokens');
    }
  });

  it('converts multimodal messages and assistant roles natively', () => {
    const imageMessages = [
      {
        role: 'user',
        content: [
          { type: 'text', text: 'look' },
          {
            type: 'image_url',
            image_url: { url: 'data:image/png;base64,AAA' },
          },
        ],
      },
      { role: 'assistant', content: 'answer' },
    ];
    const request = (chatInterface) =>
      buildPlaygroundRequest(
        imageMessages,
        { ...DEFAULT_CONFIG.inputs, chatInterface },
        {},
      ).payload;
    expect(request('anthropic').messages[0].content[1].source).toEqual({
      type: 'base64',
      media_type: 'image/png',
      data: 'AAA',
    });
    expect(request('gemini').contents[0].parts[1]).toEqual({
      inlineData: { mimeType: 'image/png', data: 'AAA' },
    });
    expect(request('gemini').contents[1].role).toBe('model');
    expect(request('openai-response').input[0].content[1].type).toBe(
      'input_image',
    );
  });

  it('rejects damaged/empty custom JSON instead of silently sending a default body', () => {
    for (const body of ['', '{', 'null', '[]', '42']) {
      expect(() =>
        buildPlaygroundRequest(messages, DEFAULT_CONFIG.inputs, {}, true, body),
      ).toThrow();
    }
  });
});
