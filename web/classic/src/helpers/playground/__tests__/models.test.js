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
import { normalizeModelOptions, filterChatModels } from '../models';

describe('model capability filtering', () => {
  const models = normalizeModelOptions([
    'unknown',
    { id: 'empty', supported_endpoint_types: [] },
    { id: 'chat', supported_endpoint_types: ['openai'] },
    { id: 'native', supported_endpoint_types: ['gemini'] },
    { id: 'image', supported_endpoint_types: ['image-generation'] },
  ]);
  const values = (items) => items.map((item) => item.value);
  it('accepts object and legacy string model lists', () => {
    expect(values(models)).toEqual([
      'unknown',
      'empty',
      'chat',
      'native',
      'image',
    ]);
    expect(normalizeModelOptions(null)).toEqual([]);
    expect(normalizeModelOptions([null, {}])).toEqual([]);
  });
  it('filters non-chat endpoints but retains unknown capability without tools', () => {
    expect(
      values(filterChatModels(models, { chatInterface: 'gemini' })),
    ).toEqual(['unknown', 'empty', 'chat', 'native']);
  });
  it('requires native endpoint only when an effective built-in tool is enabled', () => {
    expect(
      values(
        filterChatModels(models, {
          chatInterface: 'gemini',
          codeInterpreterEnabled: true,
        }),
      ),
    ).toEqual(['unknown', 'empty', 'native']);
    expect(
      values(
        filterChatModels(models, {
          chatInterface: 'openai',
          codeInterpreterEnabled: true,
        }),
      ),
    ).toEqual(['unknown', 'empty', 'chat', 'native']);
    expect(
      values(
        filterChatModels(models, {
          chatInterface: 'openai',
          webSearchEnabled: true,
        }),
      ),
    ).toEqual(['unknown', 'empty', 'chat']);
  });
  it('uses catalog text modalities only when endpoint capabilities are absent', () => {
    const catalog = {
      unknown: { modalities: { input: ['audio'], output: ['audio'] } },
      chat: { modalities: { output: ['image'] } },
    };
    expect(values(filterChatModels(models, {}, catalog))).toEqual([
      'empty',
      'chat',
      'native',
    ]);
  });
});
