import assert from 'node:assert/strict'
import { test } from 'node:test'

import type { ModelOption, PlaygroundMode } from '../../types'
import { filterModelsForMode } from './model-capabilities'
import {
  getModelFallback,
  shouldClearModelForGroup,
} from './playground-option-utils'

const endpoints = [
  'openai',
  'anthropic',
  'gemini',
  'image-generation',
  'image-edit',
  'openai-video',
  'video-edit',
  'video-extension',
  'audio-speech',
  'audio-transcription',
  'embedding',
]
const models: ModelOption[] = endpoints.map((endpoint) => ({
  value: endpoint,
  label: endpoint,
  supportedEndpointTypes: [endpoint],
}))

test('native tools keep models that advertise the selected chat endpoint', () => {
  const filtered = filterModelsForMode(models, 'chat', '', undefined, 'gemini')
  assert.deepEqual(
    filtered.map((model) => model.value),
    ['gemini']
  )
})

test('endpoint classification works without the optional catalog', () => {
  assert.deepEqual(
    filterModelsForMode(models, 'chat', '').map((m) => m.value),
    ['openai', 'anthropic', 'gemini']
  )
  const cases: [PlaygroundMode, string][] = [
    ['image', 'image-generation'],
    ['image', 'image-edit'],
    ['video', 'openai-video'],
    ['video', 'video-edit'],
    ['video', 'video-extension'],
    ['audio', 'audio-speech'],
    ['audio', 'audio-transcription'],
  ]
  for (const [mode, endpoint] of cases) {
    const filtered = filterModelsForMode(models, mode, endpoint)
    assert.deepEqual(
      filtered.map((m) => m.value),
      [endpoint]
    )
    assert.equal(getModelFallback(filtered, 'openai'), endpoint)
    assert.equal(getModelFallback(filtered, endpoint), null)
  }
})

test('empty endpoint lists clear stale selections and unknown models remain selectable', () => {
  const filtered = filterModelsForMode(
    models.slice(0, 3),
    'image',
    'image-edit',
    {}
  )
  assert.deepEqual(filtered, [])
  assert.equal(shouldClearModelForGroup(filtered, 'openai'), true)
  const unknown = { value: 'custom', label: 'custom' }
  assert.deepEqual(filterModelsForMode([unknown], 'image', 'image-edit'), [
    unknown,
  ])
})
