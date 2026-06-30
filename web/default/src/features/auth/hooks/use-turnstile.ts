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
import { useState } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { useStatus } from '@/hooks/use-status'
import type { SystemStatus } from '@/features/auth/types'

export type TurnstileScope =
  | 'login'
  | 'register'
  | 'register_email_verification'
  | 'email_binding_verification'
  | 'password_reset'
  | 'checkin'
  | 'sensitive_update'

const turnstileScopeStatusKeys: Record<TurnstileScope, string> = {
  login: 'turnstile_login',
  register: 'turnstile_register',
  register_email_verification: 'turnstile_register_email_verification',
  email_binding_verification: 'turnstile_email_binding_verification',
  password_reset: 'turnstile_password_reset',
  checkin: 'turnstile_checkin',
  sensitive_update: 'turnstile_sensitive_update',
}

function statusValue(status: SystemStatus | null | undefined, key: string) {
  return status?.[key] ?? status?.data?.[key]
}

export function isTurnstileScopeEnabled(
  status: SystemStatus | null | undefined,
  scope: TurnstileScope
) {
  return Boolean(
    statusValue(status, turnstileScopeStatusKeys[scope]) &&
    statusValue(status, 'turnstile_site_key')
  )
}

export function getTurnstileSiteKey(status: SystemStatus | null | undefined) {
  return String(statusValue(status, 'turnstile_site_key') || '')
}

/**
 * Hook for managing Turnstile verification
 */
export function useTurnstile(scope: TurnstileScope) {
  const { status } = useStatus()
  const [turnstileToken, setTurnstileToken] = useState('')
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0)

  const isTurnstileEnabled = isTurnstileScopeEnabled(status, scope)
  const turnstileSiteKey = getTurnstileSiteKey(status)

  /**
   * Validate if turnstile is ready when required
   */
  const validateTurnstile = (): boolean => {
    if (isTurnstileEnabled && !turnstileToken) {
      toast.info(
        i18next.t('Please wait a moment, human check is initializing...')
      )
      return false
    }
    return true
  }

  const resetTurnstile = () => {
    setTurnstileToken('')
    setTurnstileWidgetKey((value) => value + 1)
  }

  return {
    isTurnstileEnabled,
    turnstileSiteKey,
    turnstileWidgetKey,
    turnstileToken,
    setTurnstileToken,
    validateTurnstile,
    resetTurnstile,
  }
}
