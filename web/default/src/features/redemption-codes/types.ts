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

// ============================================================================
// Redemption Schema & Types
// ============================================================================

export const REDEMPTION_TYPE = {
  QUOTA: 'quota',
  SUBSCRIPTION: 'subscription',
  REGISTRATION: 'registration',
} as const

export type RedemptionType =
  (typeof REDEMPTION_TYPE)[keyof typeof REDEMPTION_TYPE]

export const redemptionSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  name: z.string(),
  key: z.string(),
  status: z.number(), // 1: enabled, 2: disabled, 3: used
  type: z.preprocess(
    (value) => value || REDEMPTION_TYPE.QUOTA,
    z.enum([
      REDEMPTION_TYPE.QUOTA,
      REDEMPTION_TYPE.SUBSCRIPTION,
      REDEMPTION_TYPE.REGISTRATION,
    ])
  ),
  quota: z.number(),
  subscription_plan_id: z.preprocess((value) => value || 0, z.number()),
  subscription_plan_title: z.string().optional(),
  created_time: z.number(),
  redeemed_time: z.number(),
  expired_time: z.number(), // 0 for never expires
  max_redemptions: z.preprocess((value) => value ?? 1, z.number()),
  redeemed_count: z.preprocess((value) => value ?? 0, z.number()),
  used_user_id: z.number(),
})

export type Redemption = z.infer<typeof redemptionSchema>

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetRedemptionsParams {
  p?: number
  page_size?: number
  type?: RedemptionType
}

export interface GetRedemptionsResponse {
  success: boolean
  message?: string
  data?: {
    items: Redemption[]
    total: number
    page: number
    page_size: number
  }
}

export interface SearchRedemptionsParams {
  keyword?: string
  status?: string
  p?: number
  page_size?: number
  type?: RedemptionType
}

export interface RedemptionFormData {
  id?: number
  name: string
  type: RedemptionType
  quota: number
  subscription_plan_id?: number
  expired_time: number
  max_redemptions?: number
  count?: number // Only for create
  status?: number // Only for status update
}

// ============================================================================
// Dialog Types
// ============================================================================

export type RedemptionsDialogType = 'create' | 'update' | 'delete' | 'view'
