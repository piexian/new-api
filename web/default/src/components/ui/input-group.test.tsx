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

import type { MouseEvent } from 'react'

import { InputGroup, InputGroupAddon } from './input-group'
import { getInputGroupFocusTarget } from './input-group-focus'

const groupSelector = '[data-slot="input-group"]'

// Model only the DOM query results; browser interaction coverage lives separately.
function createFixture() {
  const controls: HTMLInputElement[] = []
  const group = {
    querySelectorAll(selector: string) {
      assert.equal(
        selector,
        'input[data-slot="input-group-control"], textarea[data-slot="input-group-control"]'
      )
      return controls
    },
  }
  const target = {
    nodeType: 1,
    closest(selector: string): unknown {
      return selector === groupSelector ? group : null
    },
  }
  const addon = {
    contains(node: unknown) {
      return node === target
    },
    closest(selector: string) {
      assert.equal(selector, groupSelector)
      return group
    },
  }

  function addControl({
    type = 'text',
    disabled = false,
    nested = false,
    hidden = false,
  } = {}) {
    let focusCount = 0
    const control = {
      type,
      closest(selector: string) {
        if (selector === groupSelector) {
          return nested ? {} : group
        }
        assert.equal(selector, '[hidden], [inert], [aria-hidden="true"]')
        return hidden ? {} : null
      },
      matches(selector: string) {
        assert.equal(selector, ':disabled')
        return disabled
      },
      focus() {
        focusCount += 1
      },
      get focusCount() {
        return focusCount
      },
    }
    controls.push(control as unknown as HTMLInputElement)
    return control
  }

  return {
    addControl,
    target,
    addon,
    getFocusTarget() {
      return getInputGroupFocusTarget(
        addon as unknown as HTMLElement,
        target as unknown as Node
      )
    },
    click(defaultPrevented = false) {
      InputGroupAddon({}).props.onClick({
        currentTarget: addon,
        target,
        defaultPrevented,
      } as unknown as MouseEvent<HTMLDivElement>)
    },
  }
}

test('addon icon and whitespace clicks focus the available input or textarea', () => {
  for (const type of ['text', 'textarea']) {
    const fixture = createFixture()
    const control = fixture.addControl({ type })
    fixture.click()
    assert.equal(control.focusCount, 1)
  }
})

test('portal clicks never query or focus the ancestor input group', () => {
  const fixture = createFixture()
  const control = fixture.addControl()
  fixture.addon.contains = () => false
  fixture.target.closest = () => {
    assert.fail('portal content must be rejected before DOM target queries')
  }
  fixture.click()
  assert.equal(control.focusCount, 0)
})

test('clicking interactive content does not redirect its focus', () => {
  for (const selector of [
    'button',
    'a[href]',
    'input',
    'textarea',
    'select',
    'label',
    'summary',
    '[tabindex]',
    '[contenteditable]:not([contenteditable="false"])',
    '[role="button"]',
    '[role="combobox"]',
  ]) {
    const fixture = createFixture()
    const control = fixture.addControl()
    const closest = fixture.target.closest
    fixture.target.closest = (query) =>
      query.split(', ').includes(selector) ? fixture.target : closest(query)
    fixture.click()
    assert.equal(control.focusCount, 0, selector)
  }
})

test('a focusable command ancestor does not block addon icon focus', () => {
  const fixture = createFixture()
  const control = fixture.addControl()
  const closest = fixture.target.closest
  fixture.target.closest = (query) =>
    query.includes('[tabindex]') ? {} : closest(query)
  fixture.click()
  assert.equal(control.focusCount, 1)
})

test('nested input group addon clicks cannot focus the outer group', () => {
  const fixture = createFixture()
  const control = fixture.addControl()
  fixture.target.closest = () => ({})
  fixture.click()
  assert.equal(control.focusCount, 0)
})

test('focus skips hidden, disabled, inert and nested controls', () => {
  const fixture = createFixture()
  const unavailable = [
    fixture.addControl({ type: 'hidden' }),
    fixture.addControl({ disabled: true }),
    fixture.addControl({ hidden: true }),
    fixture.addControl({ nested: true }),
  ]
  const textarea = fixture.addControl({ type: 'textarea' })
  assert.equal(fixture.getFocusTarget(), textarea)
  fixture.click()
  assert.equal(textarea.focusCount, 1)
  for (const control of unavailable) {
    assert.equal(control.focusCount, 0)
  }
})

test('groups without an available control do not redirect focus', () => {
  const fixture = createFixture()
  assert.equal(fixture.getFocusTarget(), undefined)
  const control = fixture.addControl({ disabled: true })
  fixture.click()
  assert.equal(control.focusCount, 0)
})

test('prevented clicks retain focus and custom addon handlers retain precedence', () => {
  const fixture = createFixture()
  const control = fixture.addControl()
  fixture.click(true)
  assert.equal(control.focusCount, 0)
  const onClick = () => undefined
  assert.equal(InputGroupAddon({ onClick }).props.onClick, onClick)
})

test('disabled group styling is scoped to input controls, not footer buttons', () => {
  const classes: string[] = InputGroup({}).props.className.split(' ')
  assert.ok(!classes.some((value) => value.includes('has-disabled:')))
  for (const style of ['bg-input/50', 'opacity-50']) {
    assert.ok(
      classes.includes(
        `has-[[data-slot=input-group-control]:disabled]:${style}`
      )
    )
  }
  assert.ok(
    classes.includes(
      'dark:has-[[data-slot=input-group-control]:disabled]:bg-input/80'
    )
  )
})
