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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import {
  REDEMPTION_VALIDATION,
  getRedemptionFormErrorMessages,
} from '../constants'
import {
  REDEMPTION_TYPE,
  type RedemptionFormData,
  type Redemption,
  type RedemptionType,
} from '../types'

// ============================================================================
// Form Schema (use getRedemptionFormSchema(t) in components for i18n messages)
// ============================================================================

export function getRedemptionFormSchema(t: TFunction) {
  const msg = getRedemptionFormErrorMessages(t)
  return z
    .object({
      name: z
        .string()
        .min(REDEMPTION_VALIDATION.NAME_MIN_LENGTH, msg.NAME_LENGTH_INVALID)
        .max(REDEMPTION_VALIDATION.NAME_MAX_LENGTH, msg.NAME_LENGTH_INVALID),
      type: z.enum([REDEMPTION_TYPE.QUOTA, REDEMPTION_TYPE.SUBSCRIPTION]),
      quota_dollars: z.number().min(0),
      subscription_plan_id: z.number().optional(),
      expired_time: z.date().optional(),
      max_redemptions: z
        .number()
        .min(
          REDEMPTION_VALIDATION.MAX_REDEMPTIONS_MIN,
          msg.MAX_REDEMPTIONS_INVALID
        ),
      count: z
        .number()
        .min(REDEMPTION_VALIDATION.COUNT_MIN, msg.COUNT_INVALID)
        .max(REDEMPTION_VALIDATION.COUNT_MAX, msg.COUNT_INVALID)
        .optional(),
    })
    .superRefine((data, ctx) => {
      if (data.type === REDEMPTION_TYPE.QUOTA && data.quota_dollars <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['quota_dollars'],
          message: t('Quota must be a positive number'),
        })
      }
      if (
        data.type === REDEMPTION_TYPE.SUBSCRIPTION &&
        !data.subscription_plan_id
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['subscription_plan_id'],
          message: t('Please select a subscription plan'),
        })
      }
    })
}

export type RedemptionFormValues = {
  name: string
  type: RedemptionType
  quota_dollars: number
  subscription_plan_id?: number
  expired_time?: Date
  max_redemptions: number
  count?: number
}

// ============================================================================
// Form Defaults
// ============================================================================

export const REDEMPTION_FORM_DEFAULT_VALUES: RedemptionFormValues = {
  name: '',
  type: REDEMPTION_TYPE.QUOTA,
  quota_dollars: 10,
  subscription_plan_id: undefined,
  expired_time: undefined,
  max_redemptions: 1,
  count: 1,
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: RedemptionFormValues
): RedemptionFormData {
  return {
    name: data.name,
    type: data.type,
    quota:
      data.type === REDEMPTION_TYPE.QUOTA
        ? parseQuotaFromDollars(data.quota_dollars)
        : 0,
    subscription_plan_id:
      data.type === REDEMPTION_TYPE.SUBSCRIPTION
        ? data.subscription_plan_id
        : 0,
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : 0,
    max_redemptions: data.max_redemptions,
    count: data.count || 1,
  }
}

/**
 * Transform redemption data to form defaults
 */
export function transformRedemptionToFormDefaults(
  redemption: Redemption
): RedemptionFormValues {
  const type = redemption.type || REDEMPTION_TYPE.QUOTA
  return {
    name: redemption.name,
    type,
    quota_dollars: quotaUnitsToDollars(redemption.quota),
    subscription_plan_id: redemption.subscription_plan_id || undefined,
    expired_time:
      redemption.expired_time > 0
        ? new Date(redemption.expired_time * 1000)
        : undefined,
    max_redemptions: redemption.max_redemptions ?? 1,
    count: 1,
  }
}
