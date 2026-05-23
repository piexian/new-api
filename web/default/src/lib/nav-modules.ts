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
import { getStatus } from '@/lib/api'

export type ModuleAccess = { enabled: boolean; requireAuth: boolean }

export type HeaderNavModule = 'rankings' | 'pricing'
export type HeaderNavBooleanModule = 'home' | 'console' | 'docs' | 'about'

export type HeaderNavModules = {
  home: boolean
  console: boolean
  pricing: ModuleAccess
  rankings: ModuleAccess
  docs: boolean
  about: boolean
  [key: string]: boolean | ModuleAccess
}

const DEFAULT_HEADER_NAV_MODULES: HeaderNavModules = {
  home: true,
  console: true,
  pricing: { enabled: true, requireAuth: false },
  rankings: { enabled: true, requireAuth: false },
  docs: true,
  about: true,
}

const DEFAULTS: Record<HeaderNavModule, ModuleAccess> = {
  pricing: DEFAULT_HEADER_NAV_MODULES.pricing,
  rankings: DEFAULT_HEADER_NAV_MODULES.rankings,
}

function cloneHeaderNavDefaults(): HeaderNavModules {
  return {
    ...DEFAULT_HEADER_NAV_MODULES,
    pricing: { ...DEFAULT_HEADER_NAV_MODULES.pricing },
    rankings: { ...DEFAULT_HEADER_NAV_MODULES.rankings },
  }
}

export function parseHeaderNavBoolean(
  raw: unknown,
  fallback: boolean
): boolean {
  if (typeof raw === 'boolean') return raw
  if (typeof raw === 'number') {
    if (raw === 1) return true
    if (raw === 0) return false
    return fallback
  }
  if (typeof raw === 'string') {
    const normalized = raw.trim().toLowerCase()
    if (normalized === 'true' || normalized === '1') return true
    if (normalized === 'false' || normalized === '0') return false
  }
  return fallback
}

function parseAccess(raw: unknown, fallback: ModuleAccess): ModuleAccess {
  if (
    typeof raw === 'boolean' ||
    typeof raw === 'number' ||
    typeof raw === 'string'
  ) {
    return {
      enabled: parseHeaderNavBoolean(raw, fallback.enabled),
      requireAuth: fallback.requireAuth,
    }
  }
  if (raw && typeof raw === 'object') {
    const r = raw as Record<string, unknown>
    return {
      enabled: parseHeaderNavBoolean(r.enabled, fallback.enabled),
      requireAuth: parseHeaderNavBoolean(r.requireAuth, fallback.requireAuth),
    }
  }
  return { ...fallback }
}

function parseHeaderNavRecord(raw: unknown): Record<string, unknown> | null {
  if (!raw || String(raw).trim() === '') return null
  if (raw && typeof raw === 'object') return raw as Record<string, unknown>

  try {
    return JSON.parse(String(raw)) as Record<string, unknown>
  } catch {
    return null
  }
}

export function parseHeaderNavModules(raw: unknown): HeaderNavModules {
  const result = cloneHeaderNavDefaults()
  const parsed = parseHeaderNavRecord(raw)
  if (!parsed) return result

  Object.entries(parsed).forEach(([key, value]) => {
    if (key === 'pricing') {
      result.pricing = parseAccess(value, result.pricing)
      return
    }
    if (key === 'rankings') {
      result.rankings = parseAccess(value, result.rankings)
      return
    }

    const fallback = result[key]
    if (
      typeof fallback === 'boolean' ||
      typeof value === 'boolean' ||
      typeof value === 'number' ||
      typeof value === 'string'
    ) {
      result[key] = parseHeaderNavBoolean(
        value,
        typeof fallback === 'boolean' ? fallback : true
      )
    }
  })

  return result
}

export function parseHeaderNavModulesFromStatus(
  status: Record<string, unknown> | null
): HeaderNavModules {
  return parseHeaderNavModules(status?.HeaderNavModules)
}

function getCachedStatus(): Record<string, unknown> | null {
  try {
    if (typeof window === 'undefined') return null
    const raw = window.localStorage.getItem('status')
    return raw ? (JSON.parse(raw) as Record<string, unknown>) : null
  } catch {
    return null
  }
}

function cacheStatus(status: Record<string, unknown> | null): void {
  try {
    if (typeof window !== 'undefined' && status) {
      window.localStorage.setItem('status', JSON.stringify(status))
    }
  } catch {
    /* empty */
  }
}

export function getModuleAccessFromStatus(
  status: Record<string, unknown> | null,
  module: HeaderNavModule
): ModuleAccess {
  return parseHeaderNavModulesFromStatus(status)[module] ?? DEFAULTS[module]
}

export function getHeaderModuleEnabledFromStatus(
  status: Record<string, unknown> | null,
  module: HeaderNavModule | HeaderNavBooleanModule
): boolean {
  const modules = parseHeaderNavModulesFromStatus(status)
  const value = modules[module]
  if (typeof value === 'object') return value.enabled
  if (typeof value === 'boolean') return value
  return true
}

export function getModuleAccess(module: HeaderNavModule): ModuleAccess {
  return getModuleAccessFromStatus(getCachedStatus(), module)
}

async function getFreshStatus(): Promise<Record<string, unknown> | null> {
  try {
    const status = (await getStatus()) as Record<string, unknown> | null
    cacheStatus(status)
    return status
  } catch {
    return getCachedStatus()
  }
}

export async function getFreshModuleAccess(
  module: HeaderNavModule
): Promise<ModuleAccess> {
  return getModuleAccessFromStatus(await getFreshStatus(), module)
}

export async function getFreshHeaderModuleEnabled(
  module: HeaderNavModule | HeaderNavBooleanModule
): Promise<boolean> {
  return getHeaderModuleEnabledFromStatus(await getFreshStatus(), module)
}

type SidebarSectionConfig = {
  enabled?: boolean
  [key: string]: boolean | undefined
}

type SidebarModulesAdminConfig = Record<string, SidebarSectionConfig>

const DEFAULT_SIDEBAR_MODULES: SidebarModulesAdminConfig = {
  chat: {
    enabled: true,
    playground: true,
    chat: true,
  },
  console: {
    enabled: true,
    detail: true,
    token: true,
    log: true,
    midjourney: true,
    task: true,
  },
  personal: {
    enabled: true,
    topup: true,
    personal: true,
  },
  admin: {
    enabled: true,
    channel: true,
    models: true,
    redemption: true,
    user: true,
    setting: true,
    subscription: true,
  },
}

function cloneSidebarDefaults(): SidebarModulesAdminConfig {
  return Object.entries(DEFAULT_SIDEBAR_MODULES).reduce(
    (acc, [section, config]) => {
      acc[section] = { ...config }
      return acc
    },
    {} as SidebarModulesAdminConfig
  )
}

function parseSidebarAdminConfig(raw: unknown): SidebarModulesAdminConfig {
  const defaults = cloneSidebarDefaults()
  if (!raw || String(raw).trim() === '') return defaults

  try {
    const parsed =
      typeof raw === 'string'
        ? (JSON.parse(raw) as SidebarModulesAdminConfig)
        : (raw as SidebarModulesAdminConfig)
    if (!parsed || typeof parsed !== 'object') return defaults

    const result = cloneSidebarDefaults()
    Object.entries(parsed).forEach(([sectionKey, sectionValue]) => {
      if (!sectionValue || typeof sectionValue !== 'object') return
      const base = result[sectionKey] ?? { enabled: true }
      result[sectionKey] = { ...base }
      Object.entries(sectionValue).forEach(([moduleKey, moduleValue]) => {
        result[sectionKey][moduleKey] = parseHeaderNavBoolean(
          moduleValue,
          base[moduleKey] ?? true
        )
      })
    })
    return result
  } catch {
    return defaults
  }
}

export function isSidebarModuleEnabledFromStatus(
  status: Record<string, unknown> | null,
  section: string,
  module: string
): boolean {
  const config = parseSidebarAdminConfig(status?.SidebarModulesAdmin)
  const sectionConfig = config[section]
  if (!sectionConfig) return true
  if (sectionConfig.enabled === false) return false
  if (sectionConfig[module] === false) return false
  return true
}

export function isSidebarModuleEnabled(
  section: string,
  module: string
): boolean {
  return isSidebarModuleEnabledFromStatus(getCachedStatus(), section, module)
}

type SidebarRouteRule = {
  prefix: string
  section: string
  module: string
}

const SIDEBAR_ROUTE_RULES: SidebarRouteRule[] = [
  { prefix: '/usage-logs/drawing', section: 'console', module: 'midjourney' },
  { prefix: '/usage-logs/task', section: 'console', module: 'task' },
  { prefix: '/usage-logs', section: 'console', module: 'log' },
  { prefix: '/console/log', section: 'console', module: 'log' },
  { prefix: '/console/topup', section: 'personal', module: 'topup' },
  { prefix: '/system-settings', section: 'admin', module: 'setting' },
  { prefix: '/redemption-codes', section: 'admin', module: 'redemption' },
  { prefix: '/subscriptions', section: 'admin', module: 'subscription' },
  { prefix: '/dashboard', section: 'console', module: 'detail' },
  { prefix: '/keys', section: 'console', module: 'token' },
  { prefix: '/wallet', section: 'personal', module: 'topup' },
  { prefix: '/profile', section: 'personal', module: 'personal' },
  { prefix: '/channels', section: 'admin', module: 'channel' },
  { prefix: '/models', section: 'admin', module: 'models' },
  { prefix: '/users', section: 'admin', module: 'user' },
  { prefix: '/playground', section: 'chat', module: 'playground' },
  { prefix: '/chat2link', section: 'chat', module: 'chat' },
  { prefix: '/chat', section: 'chat', module: 'chat' },
]

function normalizePathname(pathname: string): string {
  if (!pathname || pathname === '/') return '/'
  return pathname.replace(/\/+$/, '')
}

function matchesPrefix(pathname: string, prefix: string): boolean {
  const normalized = normalizePathname(pathname)
  const normalizedPrefix = normalizePathname(prefix)
  return (
    normalized === normalizedPrefix ||
    normalized.startsWith(`${normalizedPrefix}/`)
  )
}

export function isHeaderRouteEnabledFromStatus(
  status: Record<string, unknown> | null,
  pathname: string
): boolean {
  const path = normalizePathname(pathname)
  if (path === '/') return getHeaderModuleEnabledFromStatus(status, 'home')
  if (
    matchesPrefix(path, '/user-agreement') ||
    matchesPrefix(path, '/privacy-policy')
  ) {
    return getHeaderModuleEnabledFromStatus(status, 'docs')
  }
  if (matchesPrefix(path, '/about')) {
    return getHeaderModuleEnabledFromStatus(status, 'about')
  }
  if (matchesPrefix(path, '/pricing')) {
    return getHeaderModuleEnabledFromStatus(status, 'pricing')
  }
  if (matchesPrefix(path, '/rankings')) {
    return getHeaderModuleEnabledFromStatus(status, 'rankings')
  }
  if (matchesPrefix(path, '/dashboard') || matchesPrefix(path, '/console')) {
    return getHeaderModuleEnabledFromStatus(status, 'console')
  }
  return true
}

export function isSidebarRouteEnabledFromStatus(
  status: Record<string, unknown> | null,
  pathname: string
): boolean {
  const rule = SIDEBAR_ROUTE_RULES.find(({ prefix }) =>
    matchesPrefix(pathname, prefix)
  )
  if (!rule) return true
  return isSidebarModuleEnabledFromStatus(status, rule.section, rule.module)
}

export async function getFreshRouteEnabled(pathname: string): Promise<boolean> {
  const status = await getFreshStatus()
  if (!isHeaderRouteEnabledFromStatus(status, pathname)) {
    return false
  }
  if (!isSidebarRouteEnabledFromStatus(status, pathname)) {
    return false
  }
  return true
}

export async function ensureFreshRouteEnabled(pathname: string): Promise<void> {
  if (!(await getFreshRouteEnabled(pathname))) {
    throw new Error('route_disabled')
  }
}
