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
import { z } from 'zod'
import type { TFunction } from 'i18next'
import type { IPBan, IPBanBatchFormData, IPBanFormData } from '../types'

const MAX_REASON_LENGTH = 255

export const IP_BAN_FORM_DEFAULT_VALUES = {
  target: '',
  reason: '',
  expires_at: 0,
} satisfies IPBanFormData

export const IP_BAN_BATCH_FORM_DEFAULT_VALUES = {
  lines: '',
  default_reason: '',
  expires_at: 0,
} satisfies IPBanBatchFormData

export function getIPBanFormSchema(t: TFunction) {
  return z.object({
    target: z.string().trim().min(1, t('IP address or CIDR is required')),
    reason: z
      .string()
      .trim()
      .min(1, t('Ban reason is required'))
      .max(
        MAX_REASON_LENGTH,
        t('Ban reason must be at most {{count}} characters', {
          count: MAX_REASON_LENGTH,
        })
      ),
    expires_at: z.number().min(0),
  })
}

export function getIPBanBatchFormSchema(t: TFunction) {
  return z.object({
    lines: z.string().trim().min(1, t('Batch IP list is required')),
    default_reason: z
      .string()
      .trim()
      .max(
        MAX_REASON_LENGTH,
        t('Ban reason must be at most {{count}} characters', {
          count: MAX_REASON_LENGTH,
        })
      ),
    expires_at: z.number().min(0),
  })
}

export type IPBanFormValues = z.infer<ReturnType<typeof getIPBanFormSchema>>
export type IPBanBatchFormValues = z.infer<
  ReturnType<typeof getIPBanBatchFormSchema>
>

export function transformIPBanToFormDefaults(ban: IPBan): IPBanFormValues {
  return {
    target: ban.target,
    reason: ban.reason,
    expires_at: ban.expires_at || 0,
  }
}

export function transformIPBanFormToPayload(
  values: IPBanFormValues,
  confirmed = false
): IPBanFormData {
  return {
    target: values.target.trim(),
    reason: values.reason.trim(),
    expires_at: values.expires_at || 0,
    confirm_self_lock: confirmed,
  }
}

export function transformIPBanBatchFormToPayload(
  values: IPBanBatchFormValues,
  confirmed = false
): IPBanBatchFormData {
  return {
    lines: values.lines,
    default_reason: values.default_reason.trim(),
    expires_at: values.expires_at || 0,
    confirm_self_lock: confirmed,
  }
}
