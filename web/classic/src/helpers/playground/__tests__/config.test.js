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

import { afterEach, beforeEach, describe, expect, it } from 'bun:test';
import {
  DEFAULT_CONFIG,
  STORAGE_KEYS,
} from '../../../constants/playground.constants';
import {
  clearConfig,
  importConfig,
  loadConfig,
  saveConfig,
} from '../../../components/playground/configStorage';
import { normalizeConfig } from '../config';
import { OPTIONAL_PARAMETERS, normalizeOptionalNumber } from '../parameters';

let storage;
const previousStorage = globalThis.localStorage;
const previousReader = globalThis.FileReader;
beforeEach(() => {
  storage = new Map();
  globalThis.localStorage = {
    getItem: (key) => storage.get(key) ?? null,
    setItem: (key, value) => storage.set(key, value),
    removeItem: (key) => storage.delete(key),
  };
});
afterEach(() => {
  globalThis.localStorage = previousStorage;
  globalThis.FileReader = previousReader;
});
const put = (key, value) => storage.set(key, JSON.stringify(value));

describe('classic configuration compatibility', () => {
  it('no history and reset defaults leave all optional parameters null and disabled', () => {
    expect(loadConfig()).toEqual(DEFAULT_CONFIG);
    for (const key of OPTIONAL_PARAMETERS) {
      expect(DEFAULT_CONFIG.inputs[key]).toBeNull();
      expect(DEFAULT_CONFIG.parameterEnabled[key]).toBe(false);
    }
  });
  it('new theme key wins and saving does not mutate shared config or messages', () => {
    put(STORAGE_KEYS.LEGACY_CONFIG, { inputs: { model: 'shared' } });
    put(STORAGE_KEYS.MESSAGES, { messages: ['untouched'] });
    put(STORAGE_KEYS.CONFIG, { inputs: { model: 'classic' } });
    expect(loadConfig().inputs.model).toBe('classic');
    saveConfig(loadConfig());
    expect(
      JSON.parse(storage.get(STORAGE_KEYS.LEGACY_CONFIG)).inputs.model,
    ).toBe('shared');
    clearConfig();
    expect(storage.has(STORAGE_KEYS.CONFIG)).toBe(false);
    expect(storage.has(STORAGE_KEYS.LEGACY_CONFIG)).toBe(true);
    expect(JSON.parse(storage.get(STORAGE_KEYS.MESSAGES)).messages).toEqual([
      'untouched',
    ]);
  });
  it('preserves classic zero values and explicit false preferences', () => {
    const inputs = {
      toolsEnabled: true,
      webSearchEnabled: false,
      stream: false,
      ...Object.fromEntries(OPTIONAL_PARAMETERS.map((key) => [key, 0])),
    };
    const parameterEnabled = {
      ...DEFAULT_CONFIG.parameterEnabled,
      temperature: true,
    };
    put(STORAGE_KEYS.LEGACY_CONFIG, { inputs, parameterEnabled });
    const config = loadConfig();
    expect(config.parameterEnabled).toEqual(parameterEnabled);
    expect(config.inputs.webSearchEnabled).toBe(false);
    expect(config.inputs.codeInterpreterEnabled).toBe(true);
    expect(config.inputs.stream).toBe(false);
    for (const key of OPTIONAL_PARAMETERS) expect(config.inputs[key]).toBe(0);
    saveConfig(config);
    expect(loadConfig()).toEqual(config);
  });
  it('loads old default envelopes and the independent parameter-enabled envelope', () => {
    put(STORAGE_KEYS.LEGACY_CONFIG, {
      version: 1,
      data: {
        model: 'default-model',
        temperature: 0,
        top_p: 0,
        seed: 0,
        toolsEnabled: true,
        codeInterpreterEnabled: false,
      },
    });
    put(STORAGE_KEYS.LEGACY_PARAMETER_ENABLED, {
      version: 1,
      data: { temperature: false, top_p: true, seed: true },
    });
    const config = loadConfig();
    expect(config.inputs.model).toBe('default-model');
    expect(config.inputs.temperature).toBe(0);
    expect(config.parameterEnabled.temperature).toBe(false);
    expect(config.parameterEnabled.seed).toBe(true);
    expect(config.inputs.webSearchEnabled).toBe(true);
    expect(config.inputs.codeInterpreterEnabled).toBe(false);
  });
  it('damaged new storage falls back to legacy; damaged legacy/storage access uses defaults', () => {
    storage.set(STORAGE_KEYS.CONFIG, '{');
    put(STORAGE_KEYS.LEGACY_CONFIG, { inputs: { model: 'legacy' } });
    expect(loadConfig().inputs.model).toBe('legacy');
    storage.set(STORAGE_KEYS.LEGACY_CONFIG, 'null');
    expect(loadConfig()).toEqual(DEFAULT_CONFIG);
    globalThis.localStorage.getItem = () => {
      throw new Error('unavailable');
    };
    expect(loadConfig()).toEqual(DEFAULT_CONFIG);
  });
  it('empty/null values remain empty, numeric historical strings including seed zero remain numeric', () => {
    for (const value of ['', ' ', null, undefined, false, NaN, Infinity, 'bad'])
      expect(normalizeOptionalNumber(value)).toBeNull();
    expect(normalizeOptionalNumber('0')).toBe(0);
    const normalized = normalizeConfig({
      inputs: { temperature: '', top_p: null, max_tokens: '', seed: '0' },
      customRequestMode: false,
      customRequestBody: '',
    });
    expect(normalized.inputs.temperature).toBeNull();
    expect(normalized.inputs.top_p).toBeNull();
    expect(normalized.inputs.max_tokens).toBeNull();
    expect(normalized.inputs.seed).toBe(0);
    saveConfig(normalized);
    expect(loadConfig()).toEqual(normalized);
  });
  it('invalid stored input shapes fall back without breaking controls', () => {
    const config = normalizeConfig({
      inputs: {
        imageUrls: 1,
        stream: null,
        model: [],
        group: false,
        temperature: {},
      },
    });
    expect(config.inputs).toEqual(DEFAULT_CONFIG.inputs);
  });
  it('import accepts both classic and default formats with the same migration', async () => {
    globalThis.FileReader = class {
      readAsText(file) {
        this.onload({ target: { result: file } });
      }
    };
    const inputs = {
      temperature: 0,
      toolsEnabled: true,
      webSearchEnabled: false,
    };
    for (const data of [
      { inputs, parameterEnabled: { temperature: false } },
      { version: 1, data: inputs },
    ]) {
      expect(await importConfig(JSON.stringify(data))).toEqual(
        normalizeConfig(data),
      );
    }
    await expect(importConfig('{}')).rejects.toThrow('配置文件格式无效');
    await expect(importConfig('{')).rejects.toThrow('解析配置文件失败');
  });
});
