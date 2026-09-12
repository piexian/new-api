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

import type { Popover } from '@base-ui/react/popover'

import { getModelSearchInitialFocus } from './model-group-selector-focus'

test('Base UI initial focus chooses model search for mouse and keyboard opening', () => {
  const input = {} as HTMLInputElement
  const initialFocus = ((interactionType) => {
    return getModelSearchInitialFocus(interactionType, input)
  }) satisfies Popover.Popup.Props['initialFocus']

  for (const interactionType of ['mouse', 'keyboard', 'pen'] as const) {
    assert.equal(initialFocus(interactionType), input)
  }
})

test('touch opening does not force focus or summon the soft keyboard', () => {
  const input = {} as HTMLInputElement
  assert.equal(getModelSearchInitialFocus('touch', input), false)
  assert.equal(getModelSearchInitialFocus('touch', null), false)
})

test('focus is deferred to Base UI instead of imperatively focusing a stale ref', () => {
  const input = {
    focus() {
      assert.fail('initial focus resolution must not call focus itself')
    },
  } as unknown as HTMLInputElement
  assert.equal(getModelSearchInitialFocus('keyboard', null), null)
  assert.equal(getModelSearchInitialFocus('keyboard', input), input)
})
