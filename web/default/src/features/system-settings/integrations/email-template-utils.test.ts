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
import { describe, test } from 'node:test'

import {
  buildAITemplatePrompt,
  resolveEmailPreviewLink,
} from './email-template-utils'

describe('buildAITemplatePrompt', () => {
  test('requests directly pasteable HTML without JSON wrapping', () => {
    const prompt = buildAITemplatePrompt('auth.verify_code', 'zh-CN', [
      'logo_url',
      'verify_url',
    ])

    assert.match(prompt, /return only the complete HTML document/i)
    assert.match(prompt, /Output HTML only/i)
    assert.match(prompt, /\{\{ logo_url \}\}, \{\{ verify_url \}\}/)
    assert.doesNotMatch(prompt, /valid JSON object/i)
    assert.doesNotMatch(prompt, /\{"subject":"\.\.\.","html":"\.\.\."\}/)
  })
})

describe('resolveEmailPreviewLink', () => {
  test('accepts absolute and relative HTTP links', () => {
    assert.equal(
      resolveEmailPreviewLink('https://example.com/reset', 'https://app.test'),
      'https://example.com/reset'
    )
    assert.equal(
      resolveEmailPreviewLink('/account', 'https://app.test/settings'),
      'https://app.test/account'
    )
  })

  test('rejects non-HTTP links and malformed URLs', () => {
    for (const href of [
      'javascript:alert(1)',
      'data:text/html,unsafe',
      'mailto:user@example.com',
      'ftp://example.com/file',
      'https://[invalid',
    ]) {
      assert.equal(resolveEmailPreviewLink(href, 'https://app.test'), null)
    }
  })
})
