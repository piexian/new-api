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

export const ipBanTypeValues = ['permanent', 'temporary'] as const
export type IPBanType = (typeof ipBanTypeValues)[number]

export const ipBanSchema = z.object({
  id: z.number(),
  target: z.string(),
  reason: z.string(),
  expires_at: z.number(),
  created_at: z.number().optional(),
  updated_at: z.number().optional(),
  created_by: z.number().optional(),
})

export type IPBan = z.infer<typeof ipBanSchema>

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetIPBansParams {
  p?: number
  page_size?: number
  type?: IPBanType | ''
}

export interface SearchIPBansParams extends GetIPBansParams {
  keyword?: string
}

export interface GetIPBansResponse {
  success: boolean
  message?: string
  data?: {
    items: IPBan[]
    total: number
    page: number
    page_size: number
  }
}

export interface IPBanFormData {
  id?: number
  target: string
  reason: string
  expires_at: number
  confirm_self_lock?: boolean
}

export interface IPBanBatchFormData {
  lines: string
  default_reason: string
  expires_at: number
  confirm_self_lock?: boolean
}

export interface IPBanBatchEntry {
  line_number: number
  target: string
  reason: string
}

export interface IPBanBatchInvalidLine {
  line_number: number
  content: string
  message: string
}

export interface IPBanBatchResult {
  created: number
  skipped: number
  invalid: IPBanBatchInvalidLine[]
  created_items?: IPBan[]
  skipped_items?: IPBanBatchEntry[]
}

export type IPBanConfirmationData = {
  requires_confirmation?: boolean
  target?: string
  client_ip?: string
}

export type IPBansDialogType = 'create' | 'update' | 'delete' | 'batch'
