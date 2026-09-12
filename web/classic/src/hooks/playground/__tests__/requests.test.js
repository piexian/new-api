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

import {
  afterAll,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  mock,
} from 'bun:test';
import { readFileSync } from 'node:fs';
import { DEFAULT_CONFIG } from '../../../constants/playground.constants';
import { buildPlaygroundRequest } from '../../../helpers/playground/request';

// A small hook harness exercises transport/callback wiring without a browser.
let slots = [];
let cursor = 0;
const useState = (initial) => {
  const index = cursor++;
  if (!(index in slots))
    slots[index] = typeof initial === 'function' ? initial() : initial;
  return [
    slots[index],
    (next) => {
      slots[index] = typeof next === 'function' ? next(slots[index]) : next;
    },
  ];
};
const useRef = (initial) => useState(() => ({ current: initial }))[0];
mock.module('react', () => ({
  useCallback: (fn) => fn,
  useState,
  useRef,
  useEffect: () => {},
}));
mock.module('react-i18next', () => ({
  useTranslation: () => ({ t: (key) => key }),
}));
let modal;
mock.module('@douyinfe/semi-ui', () => ({
  Toast: { error: () => {}, success: () => {} },
  Modal: {
    confirm: (options) => {
      modal = options;
    },
  },
}));
mock.module('../../../helpers', () => ({
  getUserIdFromLocalStorage: () => 'user-id',
  handleApiError: (error) => ({ error: error.message }),
  processThinkTags: (content, reasoningContent) => ({
    content,
    reasoningContent,
  }),
  processIncompleteThinkTags: (content, reasoningContent) => ({
    content,
    reasoningContent,
  }),
  getTextContent: (message) =>
    typeof message.content === 'string'
      ? message.content
      : message.content.find((part) => part.type === 'text')?.text || '',
}));
let source;
class FakeSSE {
  constructor(url, options) {
    this.url = url;
    this.options = options;
    this.listeners = {};
    this.readyState = 1;
    this.status = 200;
    source = this;
  }
  addEventListener(name, handler) {
    this.listeners[name] = handler;
  }
  stream() {}
  close() {
    this.readyState = 2;
    this.emit('readystatechange', undefined, { readyState: 2 });
  }
  emit(type, data, rest = {}) {
    this.listeners[type]?.({
      type,
      data: typeof data === 'object' ? JSON.stringify(data) : data,
      ...rest,
    });
  }
}
mock.module('sse.js', () => ({ SSE: FakeSSE }));
let useApiRequest, useMessageActions, useMessageEdit, usePlaygroundState;
beforeAll(async () => {
  ({ useApiRequest } = await import('../useApiRequest'));
  ({ useMessageActions } = await import('../useMessageActions'));
  ({ useMessageEdit } = await import('../useMessageEdit'));
  ({ usePlaygroundState } = await import('../usePlaygroundState'));
});
const originalFetch = globalThis.fetch;
const originalStorage = globalThis.localStorage;
afterAll(() => {
  globalThis.fetch = originalFetch;
  globalThis.localStorage = originalStorage;
  mock.restore();
});
beforeEach(() => {
  cursor = 0;
  slots = [];
  modal = null;
  globalThis.localStorage = {
    getItem: () => null,
    setItem: () => {},
    removeItem: () => {},
  };
});

function transport() {
  let messages = [
    { role: 'assistant', status: 'loading', content: '', reasoningContent: '' },
  ];
  let debug = {};
  const ref = { current: null };
  const hook = useApiRequest(
    (update) => {
      messages = typeof update === 'function' ? update(messages) : update;
    },
    (update) => {
      debug = update(debug);
    },
    () => {},
    ref,
    () => {},
  );
  return {
    hook,
    ref,
    message: () => messages.at(-1),
    debug: () => debug,
    replaceMessages: (next) => {
      messages = next;
    },
  };
}
const formats = ['openai', 'openai-response', 'anthropic', 'gemini'];
const responses = {
  openai: {
    choices: [{ message: { content: 'answer', reasoning_content: 'think' } }],
  },
  'openai-response': {
    output: [
      { type: 'reasoning', summary: [{ text: 'think' }] },
      { type: 'message', content: [{ text: 'answer' }] },
    ],
  },
  anthropic: { content: [{ thinking: 'think' }, { text: 'answer' }] },
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

describe('native transport integration', () => {
  for (const chatInterface of formats) {
    it(`${chatInterface} sends native JSON and parses non-stream response`, async () => {
      const request = buildPlaygroundRequest(
        [{ role: 'user', content: 'hi' }],
        { ...DEFAULT_CONFIG.inputs, chatInterface, stream: false },
        {},
      );
      let sent;
      globalThis.fetch = async (url, options) => {
        sent = { url, payload: JSON.parse(options.body) };
        return { ok: true, json: async () => responses[chatInterface] };
      };
      const current = transport();
      await current.hook.sendRequest(
        request.payload,
        request.isStream,
        request.endpoint,
      );
      expect(sent.url).toBe(request.endpoint);
      expect(sent.payload).toEqual(JSON.parse(JSON.stringify(request.payload)));
      expect(current.message()).toMatchObject({
        content: 'answer',
        reasoningContent: 'think',
        status: 'complete',
      });
      expect(JSON.parse(current.debug().response)).toEqual(
        responses[chatInterface],
      );
    });
    it(`${chatInterface} handles named SSE events and terminates without a trailing DONE`, () => {
      const request = buildPlaygroundRequest(
        [],
        { ...DEFAULT_CONFIG.inputs, chatInterface },
        {},
      );
      const current = transport();
      current.hook.sendRequest(request.payload, true, request.endpoint);
      expect(source.url).toBe(request.endpoint);
      expect(source.options.start).toBe(false);
      if (chatInterface === 'openai') {
        source.emit('message', {
          choices: [
            { delta: { content: 'answer', reasoning_content: 'think' } },
          ],
        });
        source.emit('message', { choices: [{ finish_reason: 'stop' }] });
      } else if (chatInterface === 'openai-response') {
        source.emit('response.reasoning_summary_text.delta', {
          delta: 'think',
        });
        source.emit('response.output_text.delta', { delta: 'answer' });
        source.emit('response.completed', {});
      } else if (chatInterface === 'anthropic') {
        source.emit('content_block_start', {
          content_block: { thinking: 'think' },
        });
        source.emit('content_block_delta', { delta: { text: 'answer' } });
        source.emit('message_stop', {});
      } else source.emit('message', responses.gemini);
      expect(current.message()).toMatchObject({
        content: 'answer',
        reasoningContent: 'think',
        status: 'complete',
      });
      expect(current.ref.current).toBeNull();
      expect(current.debug().isStreaming).toBe(false);
    });
  }
  it('marks error frames and premature EOF as errors, but not an intentional stop', () => {
    const failed = transport();
    failed.hook.sendRequest({}, true, '/pg/responses');
    source.emit('response.failed', {
      response: { error: { message: 'native failure' } },
    });
    expect(failed.message().status).toBe('error');
    expect(failed.ref.current).toBeNull();
    const interrupted = transport();
    interrupted.hook.sendRequest({}, true, '/pg/messages');
    source.emit('readystatechange', undefined, { readyState: 2 });
    expect(interrupted.message().status).toBe('error');
    const stopped = transport();
    stopped.hook.sendRequest({}, true, '/pg/messages');
    stopped.hook.onStopGenerator();
    expect(stopped.message().status).toBe('complete');
    expect(stopped.message().content).toBe('');
    expect(stopped.debug().isStreaming).toBe(false);
  });
});

describe('stopped non-stream requests', () => {
  for (const outcome of ['response', 'error']) {
    it(`ignores a late ${outcome} after stop and a new request`, async () => {
      const pending = [];
      globalThis.fetch = mock(
        (url, options) =>
          new Promise((resolve, reject) => {
            pending.push({ resolve, reject, signal: options.signal });
          }),
      );
      const current = transport();
      const first = current.hook.sendRequest({ model: 'first' }, false);
      current.hook.onStopGenerator();
      expect(pending[0].signal.aborted).toBe(true);
      current.replaceMessages([
        { role: 'assistant', status: 'loading', content: '' },
      ]);
      const second = current.hook.sendRequest({ model: 'second' }, false);
      if (outcome === 'error') pending[0].reject(new Error('late failure'));
      else
        pending[0].resolve({
          ok: true,
          json: async () => ({ choices: [{ message: { content: 'FIRST' } }] }),
        });
      await first;
      expect(current.message().content).toBe('');
      expect(current.message().status).toBe('loading');
      pending[1].resolve({
        ok: true,
        json: async () => ({ choices: [{ message: { content: 'SECOND' } }] }),
      });
      await second;
      expect(current.message().content).toBe('SECOND');
      expect(current.message().status).toBe('complete');
    });
  }
});

describe('generation entry points and optional state', () => {
  for (const chatInterface of formats) {
    for (const customMode of [false, true]) {
      it(`${chatInterface} regeneration and edit share parameters and custom priority=${customMode}`, () => {
        const original = [
          { id: 'u', role: 'user', content: 'old' },
          { id: 'a', role: 'assistant', content: 'reply' },
        ];
        let messages = original;
        const inputs = {
          ...DEFAULT_CONFIG.inputs,
          chatInterface,
          temperature: 0,
          top_p: null,
          stream: false,
          webSearchEnabled: true,
          codeInterpreterEnabled: true,
        };
        const customBody = JSON.stringify({
          model: 'custom',
          stream: false,
          extension: 'untouched',
        });
        let actual;
        const generate = (history) => {
          actual = buildPlaygroundRequest(
            history,
            inputs,
            { temperature: true, top_p: true },
            customMode,
            customBody,
          );
        };
        const setMessage = (update) => {
          messages = typeof update === 'function' ? update(messages) : update;
        };
        const expected = buildPlaygroundRequest(
          original.slice(0, 1),
          inputs,
          { temperature: true, top_p: true },
          customMode,
          customBody,
        );
        useMessageActions(
          messages,
          setMessage,
          generate,
          () => {},
        ).handleMessageReset(original[1]);
        expect(actual).toEqual(expected);
        const renderEdit = () => {
          cursor = 0;
          return useMessageEdit(setMessage, generate, () => {});
        };
        let edit = renderEdit();
        edit.handleMessageEdit(original[0]);
        edit.setEditValue('edited');
        edit = renderEdit();
        edit.handleEditSave();
        expect(modal).not.toBeNull();
        modal.onOk();
        const edited = [{ ...original[0], content: 'edited' }];
        expect(actual).toEqual(
          buildPlaygroundRequest(
            edited,
            inputs,
            { temperature: true, top_p: true },
            customMode,
            customBody,
          ),
        );
      });
    }
  }
  it('editing one duplicate historical id only changes that object, including deferred updates', () => {
    const original = [
      { id: '4', role: 'user', content: 'older history' },
      { id: '4', role: 'user', content: 'newer message' },
    ];
    let queuedUpdate;
    const render = () => {
      cursor = 0;
      return useMessageEdit(
        (update) => {
          queuedUpdate = update;
        },
        () => {},
        () => {},
      );
    };
    let edit = render();
    edit.handleMessageEdit(original[1]);
    edit.setEditValue('edited latest');
    edit = render();
    edit.handleEditSave();
    const updated = queuedUpdate(original);
    expect(updated[0]).toBe(original[0]);
    expect(updated[1].content).toBe('edited latest');
  });

  it('clearing parameters stores null, seed zero survives, import false and reset are effective', () => {
    const renderState = () => {
      cursor = 0;
      return usePlaygroundState();
    };
    let state = renderState();
    expect(state.inputs.temperature).toBeNull();
    expect(state.parameterEnabled.temperature).toBe(false);
    state.handleInputChange('temperature', '');
    state.handleInputChange('seed', '0');
    state = renderState();
    expect(state.inputs.temperature).toBeNull();
    expect(state.inputs.seed).toBe(0);
    state.handleConfigImport({
      inputs: { temperature: 0 },
      customRequestMode: true,
      customRequestBody: '{}',
    });
    state = renderState();
    expect(state.inputs.temperature).toBe(0);
    state.handleConfigImport({
      inputs: { temperature: null },
      customRequestMode: false,
      customRequestBody: '',
    });
    state = renderState();
    expect(state.customRequestMode).toBe(false);
    expect(state.customRequestBody).toBe('');
    state.handleConfigReset();
    state = renderState();
    expect(state.inputs).toEqual(DEFAULT_CONFIG.inputs);
    expect(state.parameterEnabled).toEqual(DEFAULT_CONFIG.parameterEnabled);
  });
  it('ordinary send and preview use the same request builder; api export is the tested implementation', () => {
    const page = readFileSync(
      new URL('../../../pages/Playground/index.jsx', import.meta.url),
      'utf8',
    );
    expect(page).toContain('generateResponse([...message, userMessage])');
    expect(page).toContain(
      'buildPlaygroundRequest(messages, inputs, parameterEnabled).payload',
    );
    expect(page).not.toContain('syncMessageToCustomBody');
    expect(page).not.toContain('format=gemini');
    const optimized = readFileSync(
      new URL(
        '../../../components/playground/OptimizedComponents.js',
        import.meta.url,
      ),
      'utf8',
    );
    expect(optimized).toContain(
      'prevProps.onEditSave === nextProps.onEditSave',
    );
    const api = readFileSync(
      new URL('../../../helpers/api.js', import.meta.url),
      'utf8',
    );
    expect(api).toContain(
      "export { buildApiPayload } from './playground/request'",
    );
  });
});
