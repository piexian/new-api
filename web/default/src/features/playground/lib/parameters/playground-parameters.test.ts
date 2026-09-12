import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  getParameterControlValueText,
  normalizeParameterNumberValue,
  PLAYGROUND_PARAMETER_CONTROLS,
} from './playground-parameters'

test('clearing or invalid input leaves every optional numeric control unset', () => {
  for (const { key } of PLAYGROUND_PARAMETER_CONTROLS) {
    for (const input of [
      '',
      ' ',
      null,
      'invalid',
      '1oops',
      Infinity,
      Number.NaN,
    ]) {
      assert.equal(
        normalizeParameterNumberValue(key, input),
        null,
        `${key}: ${input}`
      )
    }
    assert.equal(normalizeParameterNumberValue(key, 0), 0, key)
    assert.equal(normalizeParameterNumberValue(key, '0'), 0, key)
  }
  assert.equal(getParameterControlValueText(null), 'Not set')
  assert.equal(getParameterControlValueText(0), '0')
})

test('numeric controls still normalize valid user changes to their supported range and step', () => {
  assert.equal(normalizeParameterNumberValue('temperature', '0.7'), 0.7)
  assert.equal(normalizeParameterNumberValue('frequency_penalty', '-1.2'), -1.2)
  assert.equal(normalizeParameterNumberValue('top_p', 5), 1)
  assert.equal(normalizeParameterNumberValue('max_tokens', '1024.9'), 1024)
  assert.equal(normalizeParameterNumberValue('seed', '-1'), 0)
})
