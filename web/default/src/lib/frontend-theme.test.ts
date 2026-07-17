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
  FRONTEND_THEME_PREFERENCE_KEY,
} from '@/lib/constants'

import {
  restoreFrontendThemePreference,
  setFrontendThemeCookie,
} from './frontend-theme'

const originalDocumentDescriptor = Object.getOwnPropertyDescriptor(
  globalThis,
  'document'
)
const originalWindowDescriptor = Object.getOwnPropertyDescriptor(
  globalThis,
  'window'
)

afterEach(() => {
  if (originalDocumentDescriptor) {
    Object.defineProperty(globalThis, 'document', originalDocumentDescriptor)
  } else {
    Reflect.deleteProperty(globalThis, 'document')
  }
  if (originalWindowDescriptor) {
    Object.defineProperty(globalThis, 'window', originalWindowDescriptor)
  } else {
    Reflect.deleteProperty(globalThis, 'window')
  }
})

type MemoryStorage = {
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => void
  removeItem: (key: string) => void
}

function createStorage(): MemoryStorage {
  const data = new Map<string, string>()
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => {
      data.set(key, String(value))
    },
    removeItem: (key: string) => {
      data.delete(key)
    },
  }
}

type BrowserMock = {
  cookies: Map<string, string>
  localStorage: MemoryStorage
  sessionStorage: MemoryStorage
  replacedUrls: string[]
}

// 模拟浏览器的 Cookie 语义：setter 解析首个 `name=value`，getter 拼接现存项
function installBrowserMock(pathname = '/dashboard'): BrowserMock {
  const cookies = new Map<string, string>()
  const localStorage = createStorage()
  const sessionStorage = createStorage()
  const replacedUrls: string[] = []

  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: {
      get cookie() {
        return [...cookies.entries()]
          .map(([name, value]) => `${name}=${value}`)
          .join('; ')
      },
      set cookie(value: string) {
        const [pair] = value.split(';')
        const eq = pair.indexOf('=')
        cookies.set(pair.slice(0, eq), pair.slice(eq + 1))
      },
    },
  })
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      localStorage,
      sessionStorage,
      location: {
        pathname,
        replace: (url: string) => {
          replacedUrls.push(url)
        },
      },
    },
  })
  return { cookies, localStorage, sessionStorage, replacedUrls }
}

describe('restoreFrontendThemePreference', () => {
  test('does nothing when no preference is mirrored', () => {
    const browser = installBrowserMock()

    restoreFrontendThemePreference()

    assert.equal(browser.cookies.size, 0)
    assert.deepEqual(browser.replacedUrls, [])
  })

  test('does nothing when the cookie already matches the preference', () => {
    const browser = installBrowserMock()
    browser.cookies.set(FRONTEND_THEME_COOKIE_NAME, 'classic')
    browser.localStorage.setItem(FRONTEND_THEME_PREFERENCE_KEY, 'classic')

    restoreFrontendThemePreference()

    assert.deepEqual(browser.replacedUrls, [])
  })

  test('trusts a surviving cookie and syncs the mirror to it', () => {
    const browser = installBrowserMock()
    browser.cookies.set(FRONTEND_THEME_COOKIE_NAME, 'default')
    browser.localStorage.setItem(FRONTEND_THEME_PREFERENCE_KEY, 'classic')

    restoreFrontendThemePreference()

    assert.equal(
      browser.localStorage.getItem(FRONTEND_THEME_PREFERENCE_KEY),
      'default'
    )
    assert.deepEqual(browser.replacedUrls, [])
  })

  test('rewrites a lost cookie from the mirror without redirecting', () => {
    const browser = installBrowserMock()
    browser.localStorage.setItem(FRONTEND_THEME_PREFERENCE_KEY, 'default')

    restoreFrontendThemePreference()

    assert.equal(browser.cookies.get(FRONTEND_THEME_COOKIE_NAME), 'default')
    assert.deepEqual(browser.replacedUrls, [])
  })

  test('restores the classic frontend once when its cookie was lost', () => {
    const browser = installBrowserMock('/keys')
    browser.localStorage.setItem(FRONTEND_THEME_PREFERENCE_KEY, 'classic')

    restoreFrontendThemePreference()

    assert.equal(browser.cookies.get(FRONTEND_THEME_COOKIE_NAME), 'classic')
    assert.deepEqual(browser.replacedUrls, ['/console/token'])
  })

  test('does not redirect again within the same session', () => {
    const browser = installBrowserMock()
    browser.localStorage.setItem(FRONTEND_THEME_PREFERENCE_KEY, 'classic')
    browser.sessionStorage.setItem('new-api-frontend-restore-attempted', '1')

    restoreFrontendThemePreference()

    assert.equal(browser.cookies.get(FRONTEND_THEME_COOKIE_NAME), 'classic')
    assert.deepEqual(browser.replacedUrls, [])
  })
})

describe('setFrontendThemeCookie mirror', () => {
  test('mirrors the selection into localStorage for boot-time restore', () => {
    const browser = installBrowserMock()

    setFrontendThemeCookie('classic')

    assert.equal(
      browser.localStorage.getItem(FRONTEND_THEME_PREFERENCE_KEY),
      'classic'
    )
  })
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
