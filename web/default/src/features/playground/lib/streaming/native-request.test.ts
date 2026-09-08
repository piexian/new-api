import assert from 'node:assert/strict'
import { test } from 'node:test'

import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../../constants'
import type { ChatInterface } from '../../types'
import { buildNativeRequest, supportsCodeInterpreter } from './native-request'
import { buildChatCompletionPayload } from './payload-builder'

test('native tool switches emit independent protocol fields without client functions', () => {
  const formats: ChatInterface[] = [
    'openai',
    'openai-response',
    'anthropic',
    'gemini',
  ]
  for (const chatInterface of formats) {
    for (const webSearchEnabled of [false, true]) {
      for (const codeInterpreterEnabled of [false, true]) {
        const config = {
          ...DEFAULT_CONFIG,
          model: 'native-model',
          group: 'test-group',
          chatInterface,
          webSearchEnabled,
          codeInterpreterEnabled,
        }
        const chat = buildChatCompletionPayload(
          [],
          config,
          DEFAULT_PARAMETER_ENABLED
        )
        const { payload, endpoint } = buildNativeRequest(chat, config)
        const wire = JSON.parse(JSON.stringify(payload))
        assert.equal(wire.group, 'test-group')
        assert.equal(JSON.stringify(wire).includes('"function"'), false)
        if (chatInterface === 'openai') {
          assert.equal(endpoint, '/pg/chat/completions')
          assert.equal(Boolean(wire.web_search_options), webSearchEnabled)
          assert.equal(wire.tools, undefined)
        } else {
          assert.equal(
            wire.tools?.length ?? 0,
            Number(webSearchEnabled) + Number(codeInterpreterEnabled)
          )
          if (chatInterface === 'openai-response' && codeInterpreterEnabled) {
            assert.deepEqual(
              wire.tools.find(
                (tool: { type: string }) => tool.type === 'code_interpreter'
              ),
              { type: 'code_interpreter', container: { type: 'auto' } }
            )
          }
          if (chatInterface === 'anthropic' && webSearchEnabled) {
            assert.deepEqual(wire.tools[0], {
              type: 'web_search_20250305',
              name: 'web_search',
            })
          }
          if (chatInterface === 'gemini' && codeInterpreterEnabled) {
            assert.deepEqual(wire.tools.at(-1), { codeExecution: {} })
          }
        }
      }
    }
  }
  assert.equal(supportsCodeInterpreter('openai'), false)
  assert.equal(supportsCodeInterpreter('openai-response'), true)
})

test('native requests match their routes and preserve system instructions and explicit zero controls', () => {
  const config = {
    ...DEFAULT_CONFIG,
    model: 'gemini-test',
    group: 'selected',
    temperature: 0,
    top_p: 0,
    seed: 0,
  }
  const chat = {
    model: config.model,
    group: config.group,
    stream: true,
    messages: [
      { role: 'system' as const, content: 'Be concise' },
      { role: 'user' as const, content: 'hello' },
    ],
    temperature: 0,
    top_p: 0,
    seed: 0,
  }
  const gemini = buildNativeRequest(chat, {
    ...config,
    chatInterface: 'gemini',
  })
  assert.equal(
    gemini.endpoint,
    '/pg/v1beta/models/gemini-test:streamGenerateContent?alt=sse'
  )
  assert.deepEqual(gemini.payload, {
    model: config.model,
    group: config.group,
    contents: [{ role: 'user', parts: [{ text: 'hello' }] }],
    systemInstruction: { parts: [{ text: 'Be concise' }] },
    generationConfig: {
      temperature: 0,
      topP: 0,
      seed: 0,
      maxOutputTokens: undefined,
      presencePenalty: undefined,
      frequencyPenalty: undefined,
    },
    tools: undefined,
  })
  const responses = buildNativeRequest(chat, {
    ...config,
    chatInterface: 'openai-response',
  })
  assert.equal(responses.endpoint, '/pg/responses')
  assert.equal('messages' in responses.payload, false)
  assert.deepEqual(
    (responses.payload as Record<string, unknown>).input,
    chat.messages
  )
  const claude = buildNativeRequest(chat, {
    ...config,
    chatInterface: 'anthropic',
  })
  assert.equal(claude.endpoint, '/pg/messages')
  assert.equal((claude.payload as Record<string, unknown>).system, 'Be concise')
  assert.equal(
    (claude.payload as Record<string, unknown>).max_tokens,
    config.max_tokens
  )
})
