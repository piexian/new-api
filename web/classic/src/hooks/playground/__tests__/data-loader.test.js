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
import { readFileSync } from 'node:fs';
import { API_ENDPOINTS } from '../../../constants/playground.constants';
import {
  filterChatModels,
  normalizeModelOptions,
} from '../../../helpers/playground/models';

// Evaluate the real hook with a scoped hook harness, without global React mocks
// that would leak into the other playground hook tests.
const hookSource = readFileSync(
  new URL('../useDataLoader.js', import.meta.url),
  'utf8',
)
  .replace(/import[\s\S]*?from ['"][^'"]+['"];?/g, '')
  .replace('export const useDataLoader', 'const useDataLoader');
const pageSource = readFileSync(
  new URL('../../../pages/Playground/index.jsx', import.meta.url),
  'utf8',
);
const correctionStart = pageSource.indexOf(
  '  useEffect(() => {',
  pageSource.indexOf('const chatModels'),
);
const correctionEnd = pageSource.indexOf('\n\n  // 消息编辑', correctionStart);
const runCorrection = new Function(
  'useEffect',
  'customRequestMode',
  'modelsReady',
  'models',
  'mode',
  'chatModels',
  'inputs',
  'handleInputChange',
  pageSource.slice(correctionStart, correctionEnd),
);

function harness() {
  const slots = [];
  let cursor = 0;
  let effects = [];
  const requests = [];
  const errors = [];
  let models = [];
  const changes = [];
  let inputs = { group: 'A', model: 'B-only', chatInterface: 'openai' };
  const userState = { user: { group: 'A' } };
  const sameDeps = (a, b) =>
    a && a.length === b.length && a.every((item, i) => Object.is(item, b[i]));
  const useState = (initial) => {
    const i = cursor++;
    if (slots[i] === undefined) slots[i] = { value: initial };
    return [
      slots[i].value,
      (value) => {
        slots[i].value = value;
      },
    ];
  };
  const useRef = (initial) => useState({ current: initial })[0];
  const useCallback = (fn, deps) => {
    const i = cursor++;
    if (!sameDeps(slots[i]?.deps, deps)) slots[i] = { value: fn, deps };
    return slots[i].value;
  };
  const useEffect = (fn, deps) => {
    const i = cursor++;
    if (!sameDeps(slots[i]?.deps, deps)) {
      const previous = slots[i];
      const next = { deps };
      slots[i] = next;
      effects.push(() => {
        previous?.cleanup?.();
        next.cleanup = fn();
      });
    }
  };
  const t = (key) => key;
  const API = {
    get: (endpoint, options) => {
      if (endpoint !== API_ENDPOINTS.USER_MODELS) {
        return Promise.resolve({ data: { success: true, data: {} } });
      }
      return new Promise((resolve, reject) =>
        requests.push({ options, resolve, reject }),
      );
    },
  };
  const useDataLoader = new Function(
    'useState',
    'useRef',
    'useCallback',
    'useEffect',
    'useTranslation',
    'API',
    'API_ENDPOINTS',
    'processModelsData',
    'processGroupsData',
    'showError',
    `${hookSource}\nreturn useDataLoader;`,
  )(
    useState,
    useRef,
    useCallback,
    useEffect,
    () => ({ t }),
    API,
    API_ENDPOINTS,
    (data) => ({ modelOptions: normalizeModelOptions(data) }),
    () => [{ value: 'A' }, { value: 'B' }],
    (error) => errors.push(error),
  );
  const handleInputChange = (key, value) => {
    changes.push([key, value]);
    inputs = { ...inputs, [key]: value };
  };
  const setModels = (value) => {
    models = value;
  };
  const setGroups = () => {};
  return {
    requests,
    errors,
    changes,
    models: () => models,
    inputs: () => inputs,
    switchGroup: (group) => {
      inputs = { ...inputs, group };
    },
    render: () => {
      cursor = 0;
      const result = useDataLoader(
        userState,
        inputs,
        handleInputChange,
        setModels,
        setGroups,
      );
      // The page effect captures models from this render, before the loader effect
      // clears them. This reproduces the otherwise easy-to-miss switch instant.
      runCorrection(
        useEffect,
        false,
        result.modelsReady,
        models,
        'chat',
        filterChatModels(models, inputs),
        inputs,
        handleInputChange,
      );
      return result;
    },
    flush: () => {
      const pending = effects;
      effects = [];
      pending.forEach((effect) => effect());
    },
    unmount: () => {
      slots.forEach((slot) => slot?.cleanup?.());
    },
  };
}

async function respond(request, data) {
  request.resolve({ data: { success: true, data } });
  await Promise.resolve();
}

function mount() {
  const current = harness();
  current.render();
  current.flush();
  return current;
}

describe('group-scoped playground models', () => {
  it('B first then A cannot overwrite models or correct the selection to A-only', async () => {
    const current = mount();
    current.switchGroup('B');
    expect(current.render().modelsReady).toBe(false);
    current.flush();
    expect(current.requests.map((request) => request.options.params)).toEqual([
      { group: 'A', with_capabilities: 'true' },
      { group: 'B', with_capabilities: 'true' },
    ]);
    await respond(current.requests[1], [
      { id: 'B-only', supported_endpoint_types: ['openai'] },
    ]);
    expect(current.render().modelsReady).toBe(true);
    current.flush();
    await respond(current.requests[0], ['A-only']);
    current.render();
    current.flush();
    expect(current.models()).toEqual([
      { label: 'B-only', value: 'B-only', supportedEndpointTypes: ['openai'] },
    ]);
    expect(current.inputs().model).toBe('B-only');
    expect(current.changes).toEqual([]);
  });

  it('does not correct from the previous group list on the first new-group render', async () => {
    const current = mount();
    await respond(current.requests[0], ['A-only']);
    // A has loaded, but before another render/correction the user switches to B.
    current.switchGroup('B');
    expect(current.models()[0].value).toBe('A-only');
    expect(current.render().modelsReady).toBe(false);
    current.flush();
    expect(current.changes).toEqual([]);
    expect(current.models()).toEqual([]);
    await respond(current.requests[1], ['B-only']);
    expect(current.render().modelsReady).toBe(true);
    current.flush();
    expect(current.models()).toEqual([{ label: 'B-only', value: 'B-only' }]);
    expect(current.inputs().model).toBe('B-only');
  });

  for (const failure of ['response', 'network']) {
    it(`ignores stale ${failure} errors but still reports current errors`, async () => {
      const current = mount();
      current.switchGroup('B');
      current.render();
      current.flush();
      expect(current.requests[0].options.skipErrorHandler).toBe(true);
      if (failure === 'response')
        current.requests[0].resolve({
          data: { success: false, message: 'stale' },
        });
      else current.requests[0].reject(new Error('stale'));
      await Promise.resolve();
      expect(current.errors).toEqual([]);
      if (failure === 'response')
        current.requests[1].resolve({
          data: { success: false, message: 'current' },
        });
      else current.requests[1].reject(new Error('current'));
      await Promise.resolve();
      expect(current.errors).toEqual([
        failure === 'response' ? 'current' : '加载模型失败',
      ]);
      expect(current.render().modelsReady).toBe(false);
    });
  }

  it('invalidates same-group refreshes and requests pending at unmount', async () => {
    const current = mount();
    const refresh = current.render().loadModels();
    await respond(current.requests[1], ['fresh']);
    await refresh;
    await respond(current.requests[0], ['stale']);
    expect(current.models()[0].value).toBe('fresh');
    const pending = current.render().loadModels();
    current.unmount();
    await respond(current.requests[2], ['after-unmount']);
    await pending;
    expect(current.models()).toEqual([]);
    expect(current.errors).toEqual([]);
  });
});
