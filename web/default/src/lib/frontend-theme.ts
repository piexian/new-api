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
import {
  FRONTEND_THEME_COOKIE_MAX_AGE,
  FRONTEND_THEME_COOKIE_NAME,
  FRONTEND_THEME_PREFERENCE_KEY,
} from '@/lib/constants'
import { getCookie, setCookie } from '@/lib/cookies'

export type FrontendTheme = 'default' | 'classic'

type ClassicRouteMap = {
  from: string
  to: string
  preserveSuffix?: boolean
}

const classicRouteMap: ClassicRouteMap[] = [
  { from: '/profile', to: '/console/personal' },
  { from: '/wallet', to: '/console/topup' },
  { from: '/invite-rewards', to: '/console/invite' },
  { from: '/keys', to: '/console/token' },
  { from: '/channels', to: '/console/channel' },
  { from: '/models', to: '/console/models' },
  { from: '/playground', to: '/console/playground' },
  { from: '/usage-logs', to: '/console/log' },
  { from: '/users', to: '/console/user' },
  { from: '/ip-bans', to: '/console/ip_ban' },
  { from: '/redemption-codes', to: '/console/redemption' },
  { from: '/subscriptions', to: '/console/subscription' },
  { from: '/system-settings', to: '/console/setting' },
  { from: '/chat/', to: '/console/chat/', preserveSuffix: true },
  { from: '/dashboard', to: '/console' },
  { from: '/sign-in', to: '/login' },
  { from: '/sign-up', to: '/register' },
  { from: '/forgot-password', to: '/reset' },
]

export function getClassicFrontendPath(pathname: string): string {
  const match = classicRouteMap.find(({ from, preserveSuffix }) =>
    preserveSuffix ? pathname.startsWith(from) : pathname === from
  )
  if (!match) return '/console'
  return match.preserveSuffix
    ? `${match.to}${pathname.slice(match.from.length)}`
    : match.to
}

export function setFrontendThemeCookie(theme: FrontendTheme): void {
  setCookie(FRONTEND_THEME_COOKIE_NAME, theme, FRONTEND_THEME_COOKIE_MAX_AGE)
  saveFrontendThemePreference(theme)
  // 显式切换后允许本会话内再次自动恢复
  try {
    window.sessionStorage.removeItem(FRONTEND_THEME_RESTORE_ATTEMPTED_KEY)
  } catch {
    /* empty */
  }
}

export function switchToClassicFrontend(pathname = window.location.pathname) {
  setFrontendThemeCookie('classic')
  window.location.assign(getClassicFrontendPath(pathname))
}

// 跳转死循环保护：Cookie 被完全禁用时，每次会话只尝试一次恢复跳转
const FRONTEND_THEME_RESTORE_ATTEMPTED_KEY =
  'new-api-frontend-restore-attempted'

function saveFrontendThemePreference(theme: FrontendTheme): void {
  try {
    window.localStorage.setItem(FRONTEND_THEME_PREFERENCE_KEY, theme)
  } catch {
    /* empty */
  }
}

/**
 * 启动时恢复前端主题偏好。
 *
 * Cookie 是服务端选择首屏 HTML 的唯一依据，但部分浏览器会在退出时清除
 * Cookie（登录态存在 localStorage 中不受影响），导致偏好丢失、回退到系统
 * 默认主题。这里用 localStorage 镜像兜底：Cookie 缺失而镜像存在时重写
 * Cookie；当前运行的默认前端与镜像不一致时，一次性跳转到经典前端。
 */
export function restoreFrontendThemePreference(): void {
  try {
    const stored = window.localStorage.getItem(FRONTEND_THEME_PREFERENCE_KEY)
    const preference =
      stored === 'default' || stored === 'classic' ? stored : undefined
    if (!preference) return
    const cookie = getCookie(FRONTEND_THEME_COOKIE_NAME)
    if (cookie === preference) return
    if (cookie === 'default' || cookie === 'classic') {
      // Cookie 仍然有效但与镜像不一致：以 Cookie 为准并同步镜像
      saveFrontendThemePreference(cookie)
      return
    }
    // Cookie 已丢失：按镜像重写
    setCookie(
      FRONTEND_THEME_COOKIE_NAME,
      preference,
      FRONTEND_THEME_COOKIE_MAX_AGE
    )
    if (preference !== 'classic') return
    if (window.sessionStorage.getItem(FRONTEND_THEME_RESTORE_ATTEMPTED_KEY)) {
      return
    }
    window.sessionStorage.setItem(FRONTEND_THEME_RESTORE_ATTEMPTED_KEY, '1')
    window.location.replace(getClassicFrontendPath(window.location.pathname))
  } catch {
    /* empty */
  }
}
