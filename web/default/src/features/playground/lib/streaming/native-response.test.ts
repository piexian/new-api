import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  normalizeNativeResponse,
  parseNativeStreamResponse,
  NATIVE_STREAM_EVENTS,
} from './native-response'

test('native streaming protocols display final text and terminate explicitly', () => {
  for (const response of [
    { type: 'response.output_text.delta', delta: 'answer' },
    {
      type: 'content_block_delta',
      delta: { type: 'text_delta', text: 'answer' },
    },
    { candidates: [{ content: { parts: [{ text: 'answer' }] } }] },
  ]) {
    assert.deepEqual(parseNativeStreamResponse(response).updates, [
      { type: 'content', chunk: 'answer' },
    ])
  }
  for (const type of ['message_stop', 'response.completed']) {
    assert.equal(parseNativeStreamResponse({ type }).done, true)
    assert.ok(NATIVE_STREAM_EVENTS.some((event) => event === type))
  }
  assert.equal(
    parseNativeStreamResponse({ candidates: [{ finishReason: 'STOP' }] }).done,
    true
  )
  assert.equal(
    parseNativeStreamResponse({
      type: 'response.failed',
      response: { error: { message: 'failed' } },
    }).error,
    'failed'
  )
})

test('non-streaming native text and Gemini executed code remain visible', () => {
  const responses = normalizeNativeResponse(
    {
      output: [
        { type: 'message', content: [{ type: 'output_text', text: 'answer' }] },
      ],
    },
    'openai-response'
  )
  const claude = normalizeNativeResponse(
    {
      content: [
        { type: 'thinking', thinking: 'reason' },
        { type: 'text', text: 'answer' },
      ],
    },
    'anthropic'
  )
  assert.equal(responses.choices[0].message.content, 'answer')
  assert.equal(claude.choices[0].message.content, 'answer')
  assert.equal(claude.choices[0].message.reasoning_content, 'reason')
  const gemini = normalizeNativeResponse(
    {
      candidates: [
        {
          content: {
            parts: [
              { executableCode: { code: 'print(2)', language: 'PYTHON' } },
              { codeExecutionResult: { output: '2' } },
              { text: 'answer' },
            ],
          },
        },
      ],
    },
    'gemini'
  )
  assert.match(gemini.choices[0].message.content, /print\(2\)/)
  assert.match(gemini.choices[0].message.content, /answer/)
})
