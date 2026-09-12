/*
Copyright (C) 2023-2026 QuantumNous

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
import assert from 'node:assert/strict'
import { test } from 'node:test'

import { initializeFrontendCache } from './frontend-cache'

test('frontend cache initialization preserves playground history for migration', () => {
  const preserved = {
    user: '{"id":1}',
    playground_config: '{"inputs":{"temperature":0}}',
    playground_config_default: '{"version":1,"data":{"temperature":null}}',
    playground_config_classic: '{"inputs":{"webSearchEnabled":false}}',
    playground_parameter_enabled: '{"temperature":false}',
    playground_messages: '[{"content":"saved conversation"}]',
  }
  const values = new Map(
    Object.entries({ ...preserved, stale_ui_cache: 'old' })
  )
  const localStorage = {
    get length() {
      return values.size
    },
    key(index: number) {
      return [...values.keys()][index] ?? null
    },
    getItem(key: string) {
      return values.get(key) ?? null
    },
    setItem(key: string, value: string) {
      values.set(key, value)
    },
    removeItem(key: string) {
      values.delete(key)
    },
  }
  const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window')
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: { localStorage },
  })
  try {
    initializeFrontendCache()
    for (const [key, value] of Object.entries(preserved)) {
      assert.equal(values.get(key), value)
    }
    assert.equal(values.has('stale_ui_cache'), false)
    assert.equal(values.get('newapi:default:cache-version'), 'default-v1')
    values.set('new_ui_cache', 'current')
    initializeFrontendCache()
    assert.equal(values.get('new_ui_cache'), 'current')
  } finally {
    if (originalWindow) {
      Object.defineProperty(globalThis, 'window', originalWindow)
    } else {
      Reflect.deleteProperty(globalThis, 'window')
    }
  }
})
