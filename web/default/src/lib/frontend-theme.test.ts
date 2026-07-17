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
import { afterEach, describe, test } from 'node:test'

import {
  FRONTEND_THEME_COOKIE_MAX_AGE,
  FRONTEND_THEME_COOKIE_NAME,
} from '@/lib/constants'

import { setFrontendThemeCookie } from './frontend-theme'

const originalDocumentDescriptor = Object.getOwnPropertyDescriptor(
  globalThis,
  'document'
)

afterEach(() => {
  if (originalDocumentDescriptor) {
    Object.defineProperty(globalThis, 'document', originalDocumentDescriptor)
  } else {
    Reflect.deleteProperty(globalThis, 'document')
  }
})

describe('setFrontendThemeCookie', () => {
  for (const theme of ['default', 'classic'] as const) {
    test(`persists the ${theme} frontend selection`, () => {
      let cookie = ''
      Object.defineProperty(globalThis, 'document', {
        configurable: true,
        value: {
          get cookie() {
            return cookie
          },
          set cookie(value: string) {
            cookie = value
          },
        },
      })

      setFrontendThemeCookie(theme)

      assert.equal(
        cookie,
        `${FRONTEND_THEME_COOKIE_NAME}=${theme}; path=/; max-age=${FRONTEND_THEME_COOKIE_MAX_AGE}`
      )
    })
  }
})
