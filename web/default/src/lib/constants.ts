/**
 * Application-wide constants
 */

// System Configuration Defaults
export const DEFAULT_SYSTEM_NAME = 'New API'
export const DEFAULT_LOGO = '/logo.png'

// LocalStorage Keys
export const STORAGE_KEYS = {
  SYSTEM_NAME: 'system_name',
  LOGO: 'logo',
  FOOTER_HTML: 'footer_html',
} as const

// Frontend Theme
export const FRONTEND_THEME_COOKIE_NAME = 'new-api-frontend'
export const FRONTEND_THEME_COOKIE_MAX_AGE = 60 * 60 * 24 * 365
export const FRONTEND_RETURN_TIP_PENDING_KEY =
  'new-api-default-frontend-return-tip-pending'
