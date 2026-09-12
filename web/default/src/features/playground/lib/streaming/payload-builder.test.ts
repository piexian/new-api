import assert from 'node:assert/strict'
import { test } from 'node:test'

import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../../constants'
import type { ChatInterface, ParameterEnabled } from '../../types'
import { buildNativeRequest } from './native-request'
import { buildChatCompletionPayload } from './payload-builder'

const keys = Object.keys(
  DEFAULT_PARAMETER_ENABLED
) as (keyof ParameterEnabled)[]
const enabled = Object.fromEntries(
  keys.map((key) => [key, true])
) as unknown as ParameterEnabled
const formats: ChatInterface[] = [
  'openai',
  'openai-response',
  'anthropic',
  'gemini',
]
const nativeKeys = {
  temperature: 'temperature',
  top_p: 'topP',
  max_tokens: 'maxOutputTokens',
  frequency_penalty: 'frequencyPenalty',
  presence_penalty: 'presencePenalty',
  seed: 'seed',
}

test('all four protocols omit optional numbers on first use, when cleared, or when disabled', () => {
  for (const chatInterface of formats) {
    for (const parameterEnabled of [DEFAULT_PARAMETER_ENABLED, enabled]) {
      const config = { ...DEFAULT_CONFIG, chatInterface }
      const chat = buildChatCompletionPayload([], config, parameterEnabled)
      const { payload } = buildNativeRequest(chat, config)
      const parameters = (
        'generationConfig' in payload ? payload.generationConfig : payload
      ) as Record<string, unknown>
      for (const key of [
        ...keys,
        ...Object.values(nativeKeys),
        'max_output_tokens',
      ]) {
        assert.equal(
          Object.hasOwn(parameters, key),
          false,
          `${chatInterface}: ${key}`
        )
      }
    }
    const config = {
      ...DEFAULT_CONFIG,
      chatInterface,
      temperature: 0.7,
      max_tokens: 4096,
    }
    const chat = buildChatCompletionPayload(
      [],
      config,
      DEFAULT_PARAMETER_ENABLED
    )
    const { payload } = buildNativeRequest(chat, config)
    const parameters = (
      'generationConfig' in payload ? payload.generationConfig : payload
    ) as Record<string, unknown>
    for (const key of [
      'temperature',
      'max_tokens',
      'max_output_tokens',
      'maxOutputTokens',
    ]) {
      assert.equal(
        Object.hasOwn(parameters, key),
        false,
        `${chatInterface}: disabled ${key}`
      )
    }
  }
})

test('enabled explicit zero survives each protocol mapping without introducing unsupported fields', () => {
  for (const chatInterface of formats) {
    const config = {
      ...DEFAULT_CONFIG,
      chatInterface,
      temperature: 0,
      top_p: 0,
      max_tokens: 0,
      frequency_penalty: 0,
      presence_penalty: 0,
      seed: 0,
      stream: false,
    }
    const chat = buildChatCompletionPayload([], config, enabled)
    for (const key of keys) assert.equal(chat[key], 0)
    assert.equal(chat.stream, false)
    const { payload } = buildNativeRequest(chat, config)
    const parameters = (
      'generationConfig' in payload ? payload.generationConfig : payload
    ) as Record<string, unknown>
    if (chatInterface === 'gemini') {
      for (const key of Object.values(nativeKeys)) {
        assert.equal(parameters[key], 0)
      }
    } else if (chatInterface === 'openai') {
      for (const key of keys) assert.equal(parameters[key], 0)
    } else {
      assert.equal(parameters.temperature, 0)
      assert.equal(parameters.top_p, 0)
      assert.equal(
        parameters[
          chatInterface === 'anthropic' ? 'max_tokens' : 'max_output_tokens'
        ],
        0
      )
      for (const key of ['seed', 'presence_penalty', 'frequency_penalty']) {
        assert.equal(Object.hasOwn(parameters, key), false)
      }
    }
  }
})

test('non-finite enabled numbers are never included in request payloads', () => {
  for (const value of [Number.NaN, Infinity, -Infinity]) {
    const config = {
      ...DEFAULT_CONFIG,
      temperature: value,
      top_p: value,
      max_tokens: value,
      frequency_penalty: value,
      presence_penalty: value,
      seed: value,
    }
    const chat = buildChatCompletionPayload([], config, enabled)
    for (const key of keys) assert.equal(Object.hasOwn(chat, key), false)
  }
})
