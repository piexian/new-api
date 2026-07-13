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
/**
 * Logical app routes for the default frontend.
 * Keep CTA / floating-ball links free of classic paths (e.g. never hardcode `/console`).
 */
export type AppRouteKey =
  | 'home'
  | 'dashboard'
  | 'pricing'
  | 'rankings'
  | 'sign_in'
  | 'sign_up'
  | 'keys'
  | 'wallet'
  | 'docs'

const DEFAULT_ROUTES: Record<AppRouteKey, string> = {
  home: '/',
  dashboard: '/dashboard',
  pricing: '/pricing',
  rankings: '/rankings',
  sign_in: '/sign-in',
  sign_up: '/sign-up',
  keys: '/keys',
  wallet: '/wallet',
  docs: '', // external docs come from status.docs_link
}

export type ResolveAppRouteOptions = {
  /** Prefer status.docs_link for docs; falls back to `/docs` when empty */
  docsLink?: string | null
}

/**
 * Resolve a logical route key to a default-frontend path.
 */
export function resolveAppRoute(
  key: AppRouteKey,
  options?: ResolveAppRouteOptions
): string {
  if (key === 'docs') {
    const external = options?.docsLink?.trim()
    if (external) return external
    return '/docs'
  }
  return DEFAULT_ROUTES[key]
}

export function getDefaultAppRoutes(): Readonly<Record<AppRouteKey, string>> {
  return DEFAULT_ROUTES
}
