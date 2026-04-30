import {
  FRONTEND_THEME_COOKIE_MAX_AGE,
  FRONTEND_THEME_COOKIE_NAME,
} from '@/lib/constants'
import { setCookie } from '@/lib/cookies'

type ClassicRouteMap = {
  from: string
  to: string
  preserveSuffix?: boolean
}

const classicRouteMap: ClassicRouteMap[] = [
  { from: '/profile', to: '/console/personal' },
  { from: '/wallet', to: '/console/topup' },
  { from: '/keys', to: '/console/token' },
  { from: '/channels', to: '/console/channel' },
  { from: '/models', to: '/console/models' },
  { from: '/playground', to: '/console/playground' },
  { from: '/usage-logs', to: '/console/log' },
  { from: '/users', to: '/console/user' },
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
  const match = classicRouteMap.find(
    ({ from, preserveSuffix }) =>
      preserveSuffix ? pathname.startsWith(from) : pathname === from
  )
  if (!match) return '/console'
  return match.preserveSuffix
    ? `${match.to}${pathname.slice(match.from.length)}`
    : match.to
}

export function switchToClassicFrontend(pathname = window.location.pathname) {
  setCookie(FRONTEND_THEME_COOKIE_NAME, 'classic', FRONTEND_THEME_COOKIE_MAX_AGE)
  window.location.assign(getClassicFrontendPath(pathname))
}
