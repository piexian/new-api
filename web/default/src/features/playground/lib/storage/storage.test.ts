import assert from 'node:assert/strict'
import { after, beforeEach, test } from 'node:test'

import {
  DEFAULT_CONFIG,
  DEFAULT_PARAMETER_ENABLED,
  STORAGE_KEYS,
} from '../../constants'
import {
  getInitialParameterEnabled,
  getInitialPlaygroundConfig,
} from '../state/playground-state-utils'
import {
  loadConfig,
  loadParameterEnabled,
  saveConfig,
  saveParameterEnabled,
} from './storage'

const originalStorage = Object.getOwnPropertyDescriptor(
  globalThis,
  'localStorage'
)
const entries = new Map<string, string>()
Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: {
    getItem: (key: string) => entries.get(key) ?? null,
    setItem: (key: string, value: string) => entries.set(key, value),
    removeItem: (key: string) => entries.delete(key),
  },
})
beforeEach(() => entries.clear())
after(() => {
  if (originalStorage) {
    Object.defineProperty(globalThis, 'localStorage', originalStorage)
  } else {
    Reflect.deleteProperty(globalThis, 'localStorage')
  }
})

function store(key: string, value: unknown): void {
  entries.set(key, JSON.stringify(value))
}

function readStored(key: string) {
  const value = entries.get(key)
  assert.ok(value)
  return JSON.parse(value)
}

test('first visit leaves every optional parameter unset and disabled', () => {
  assert.deepEqual(getInitialPlaygroundConfig(), DEFAULT_CONFIG)
  assert.deepEqual(getInitialParameterEnabled(), DEFAULT_PARAMETER_ENABLED)
  for (const key of Object.keys(DEFAULT_PARAMETER_ENABLED)) {
    assert.equal(
      DEFAULT_CONFIG[key as keyof typeof DEFAULT_PARAMETER_ENABLED],
      null
    )
    assert.equal(
      DEFAULT_PARAMETER_ENABLED[key as keyof typeof DEFAULT_PARAMETER_ENABLED],
      false
    )
  }
  assert.equal(DEFAULT_CONFIG.stream, true)
  assert.equal(DEFAULT_CONFIG.chatInterface, 'openai')
  assert.equal(entries.has(STORAGE_KEYS.PARAMETER_ENABLED), false)
})

test('legacy default values infer historical flags once and persist through later config edits', () => {
  const legacy = {
    version: 1,
    data: {
      temperature: 0.25,
      top_p: 0,
      frequency_penalty: 0,
      presence_penalty: 0.5,
      max_tokens: 2048,
      seed: 0,
    },
  }
  store(STORAGE_KEYS.LEGACY_CONFIG, legacy)
  const expected = {
    ...DEFAULT_PARAMETER_ENABLED,
    temperature: true,
    top_p: true,
    frequency_penalty: true,
    presence_penalty: true,
  }
  assert.deepEqual(getInitialParameterEnabled(), expected)
  assert.equal(getInitialPlaygroundConfig().temperature, 0.25)
  assert.equal(getInitialPlaygroundConfig().top_p, 0)
  assert.deepEqual(readStored(STORAGE_KEYS.PARAMETER_ENABLED).data, {
    temperature: true,
    top_p: true,
    frequency_penalty: true,
    presence_penalty: true,
  })
  saveConfig({ ...getInitialPlaygroundConfig(), model: 'edited-model' })
  assert.deepEqual(getInitialParameterEnabled(), expected)
  assert.deepEqual(readStored(STORAGE_KEYS.LEGACY_CONFIG), legacy)
})

test('legacy inference only enables present fields and preserves null instead of inventing defaults', () => {
  store(STORAGE_KEYS.LEGACY_CONFIG, {
    version: 1,
    data: { temperature: 0.25, frequency_penalty: null, seed: 0 },
  })
  assert.deepEqual(getInitialParameterEnabled(), {
    ...DEFAULT_PARAMETER_ENABLED,
    temperature: true,
    frequency_penalty: true,
  })
  assert.equal(getInitialPlaygroundConfig().frequency_penalty, null)
  assert.equal(getInitialPlaygroundConfig().top_p, null)
  assert.equal(getInitialPlaygroundConfig().presence_penalty, null)
})

test('classic embedded false and separate saved false override historical defaults', () => {
  store(STORAGE_KEYS.LEGACY_CONFIG, {
    inputs: { temperature: '0', top_p: '0.25', presence_penalty: null },
    parameterEnabled: { temperature: false, top_p: true },
  })
  store(STORAGE_KEYS.PARAMETER_ENABLED, {
    version: 1,
    data: { top_p: false },
  })
  assert.deepEqual(getInitialParameterEnabled(), {
    ...DEFAULT_PARAMETER_ENABLED,
    presence_penalty: true,
  })
  assert.equal(loadParameterEnabled().temperature, false)
  assert.equal(loadParameterEnabled().top_p, false)
})

test('new config without flags never reimports old shared flags or historical defaults', () => {
  store(STORAGE_KEYS.CONFIG, { version: 1, data: { temperature: 0.1 } })
  store(STORAGE_KEYS.LEGACY_CONFIG, {
    inputs: { temperature: 0.7, top_p: 0.9 },
    parameterEnabled: { temperature: true, seed: true },
  })
  assert.deepEqual(getInitialParameterEnabled(), DEFAULT_PARAMETER_ENABLED)
  assert.equal(entries.has(STORAGE_KEYS.PARAMETER_ENABLED), false)
  store(STORAGE_KEYS.PARAMETER_ENABLED, {
    version: 1,
    data: { temperature: false },
  })
  assert.deepEqual(loadParameterEnabled(), { temperature: false })
  assert.equal(getInitialPlaygroundConfig().temperature, 0.1)
})

test('damaged new keys fall back to legacy inference and persist the repaired flags', () => {
  entries.set(STORAGE_KEYS.CONFIG, '{broken')
  entries.set(STORAGE_KEYS.PARAMETER_ENABLED, '{broken')
  store(STORAGE_KEYS.LEGACY_CONFIG, { version: 1, data: { temperature: 0.25 } })
  assert.deepEqual(getInitialParameterEnabled(), {
    ...DEFAULT_PARAMETER_ENABLED,
    temperature: true,
  })
  assert.equal(getInitialPlaygroundConfig().temperature, 0.25)
  assert.equal(
    readStored(STORAGE_KEYS.PARAMETER_ENABLED).data.temperature,
    true
  )
  saveConfig(getInitialPlaygroundConfig())
  assert.equal(getInitialParameterEnabled().temperature, true)
})

test('unrecognized legacy records cannot migrate embedded enabled flags', () => {
  store(STORAGE_KEYS.LEGACY_CONFIG, {
    version: 1,
    data: { parameterEnabled: { temperature: true }, unrelated: 'value' },
  })
  assert.deepEqual(getInitialParameterEnabled(), DEFAULT_PARAMETER_ENABLED)
  assert.equal(entries.has(STORAGE_KEYS.PARAMETER_ENABLED), false)
})

test('versioned default history preserves zero, disabled values and explicit false', () => {
  store(STORAGE_KEYS.LEGACY_CONFIG, {
    version: 1,
    data: {
      model: 'saved-model',
      temperature: 0,
      top_p: 0.5,
      seed: 0,
      stream: false,
    },
  })
  store(STORAGE_KEYS.PARAMETER_ENABLED, {
    version: 1,
    data: { temperature: true, top_p: false, seed: true },
  })
  const config = getInitialPlaygroundConfig()
  assert.equal(config.model, 'saved-model')
  assert.equal(config.temperature, 0)
  assert.equal(config.top_p, 0.5)
  assert.equal(config.seed, 0)
  assert.equal(config.stream, false)
  assert.equal(config.max_tokens, null)
  assert.deepEqual(getInitialParameterEnabled(), {
    ...DEFAULT_PARAMETER_ENABLED,
    temperature: true,
    seed: true,
  })
})

test('classic history migrates numeric strings and only missing independent tool flags', () => {
  store(STORAGE_KEYS.LEGACY_CONFIG, {
    inputs: {
      model: 'classic-model',
      max_tokens: '4096',
      temperature: '0',
      top_p: '',
      seed: null,
      toolsEnabled: true,
      webSearchEnabled: false,
      stream: false,
      chatInterface: 'gemini',
      reasoningEffort: 'high',
    },
    parameterEnabled: { temperature: true, max_tokens: true, seed: false },
  })
  store(STORAGE_KEYS.PARAMETER_ENABLED, {
    version: 1,
    data: { temperature: false },
  })
  assert.deepEqual(getInitialPlaygroundConfig(), {
    ...DEFAULT_CONFIG,
    model: 'classic-model',
    max_tokens: 4096,
    temperature: 0,
    stream: false,
    chatInterface: 'gemini',
    reasoningEffort: 'high',
    webSearchEnabled: false,
    codeInterpreterEnabled: true,
  })
  assert.deepEqual(getInitialParameterEnabled(), {
    ...DEFAULT_PARAMETER_ENABLED,
    max_tokens: true,
    top_p: true,
  })
})

test('new default key is authoritative and saves all four previously dropped fields', () => {
  store(STORAGE_KEYS.LEGACY_CONFIG, {
    inputs: { temperature: 0.7, toolsEnabled: true },
  })
  const config = {
    ...DEFAULT_CONFIG,
    temperature: 0,
    stream: false,
    webSearchEnabled: true,
    codeInterpreterEnabled: false,
    chatInterface: 'anthropic' as const,
    reasoningEffort: 'max' as const,
  }
  saveConfig(config)
  saveParameterEnabled({ ...DEFAULT_PARAMETER_ENABLED, temperature: true })
  assert.deepEqual(getInitialPlaygroundConfig(), config)
  assert.equal(getInitialParameterEnabled().temperature, true)
  const legacy = entries.get(STORAGE_KEYS.LEGACY_CONFIG)
  saveConfig({ ...config, temperature: null, webSearchEnabled: false })
  assert.equal(getInitialPlaygroundConfig().temperature, null)
  assert.equal(getInitialPlaygroundConfig().webSearchEnabled, false)
  assert.equal(entries.get(STORAGE_KEYS.LEGACY_CONFIG), legacy)
  store(STORAGE_KEYS.CONFIG, { version: 1, data: { stream: false } })
  assert.equal(getInitialPlaygroundConfig().temperature, null)
  assert.equal(getInitialPlaygroundConfig().codeInterpreterEnabled, false)
})

test('empty, missing and individually corrupt fields never become zero or erase other preferences', () => {
  store(STORAGE_KEYS.CONFIG, {
    version: 1,
    data: {
      temperature: '',
      top_p: '  ',
      max_tokens: null,
      seed: 'not-a-number',
      frequency_penalty: false,
      stream: false,
      chatInterface: 'invalid',
      presence_penalty: 0,
      webSearchEnabled: false,
    },
  })
  assert.deepEqual(getInitialPlaygroundConfig(), {
    ...DEFAULT_CONFIG,
    stream: false,
    presence_penalty: 0,
  })
  store(STORAGE_KEYS.PARAMETER_ENABLED, {
    temperature: false,
    top_p: 'false',
    seed: true,
  })
  assert.deepEqual(loadParameterEnabled(), { temperature: false, seed: true })
})

test('corrupt new entries fall back to legacy while fully corrupt storage uses defaults', () => {
  store(STORAGE_KEYS.LEGACY_CONFIG, { version: 1, data: { seed: 0 } })
  entries.set(STORAGE_KEYS.CONFIG, '{broken')
  assert.equal(getInitialPlaygroundConfig().seed, 0)
  store(STORAGE_KEYS.CONFIG, ['wrong shape'])
  assert.equal(getInitialPlaygroundConfig().seed, 0)
  entries.set(STORAGE_KEYS.LEGACY_CONFIG, '{broken')
  entries.set(STORAGE_KEYS.PARAMETER_ENABLED, '{broken')
  assert.deepEqual(loadConfig(), {})
  assert.deepEqual(getInitialPlaygroundConfig(), DEFAULT_CONFIG)
  assert.deepEqual(getInitialParameterEnabled(), DEFAULT_PARAMETER_ENABLED)
})

test('reset persists empty disabled controls without deleting legacy configuration or messages', () => {
  store(STORAGE_KEYS.LEGACY_CONFIG, {
    inputs: { temperature: 1 },
    parameterEnabled: { temperature: true },
  })
  entries.set(STORAGE_KEYS.MESSAGES, 'existing message history')
  saveConfig({ ...DEFAULT_CONFIG, temperature: 0, seed: 0 })
  saveParameterEnabled({ ...DEFAULT_PARAMETER_ENABLED, temperature: true })
  saveConfig(DEFAULT_CONFIG)
  saveParameterEnabled(DEFAULT_PARAMETER_ENABLED)
  assert.deepEqual(getInitialPlaygroundConfig(), DEFAULT_CONFIG)
  assert.deepEqual(getInitialParameterEnabled(), DEFAULT_PARAMETER_ENABLED)
  assert.equal(entries.has(STORAGE_KEYS.LEGACY_CONFIG), true)
  assert.equal(entries.get(STORAGE_KEYS.MESSAGES), 'existing message history')
})
