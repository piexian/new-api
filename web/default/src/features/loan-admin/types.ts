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
// ============================================================================
// Loan Admin Type Definitions
// ============================================================================

/**
 * Generic API response
 */
export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

/**
 * Paginated response (backend common.PageInfo)
 */
export interface PageInfo<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}

/**
 * Loan account view for admins. Money fields are int64 quota ($1 = quotaPerUnit);
 * `debt_now` / `interest_now` are the live projected values. 0 values mean
 * "use the global config" for `custom_max_total` / `custom_daily_rate` and
 * "not agreed / none" for `terms_agreed_at` / `interest_free_until`.
 */
export interface LoanAdminAccount {
  user_id: number
  username: string
  principal_quota: number
  debt_quota: number
  debt_now: number
  interest_now: number
  custom_max_total: number
  custom_daily_rate: number
  interest_free_until: number
  terms_agreed_at: number
  total_borrowed: number
  total_repaid: number
  last_settled_day: number
  created_at: number
  updated_at: number
}

/**
 * Loan ledger record with username (admin cross-user view)
 */
export interface LoanAdminRecord {
  id: number
  user_id: number
  username: string
  type: 'borrow' | 'repay'
  amount: number
  interest_part: number
  principal_part: number
  debt_after: number
  source: 'manual' | 'checkin' | 'ai' | string
  ref_id: number
  created_at: number
}

/**
 * Loan application with username (admin cross-user view). `model_used` and
 * `decision` are internal audit fields and are not exposed by the API.
 */
export interface LoanAdminApplication {
  id: number
  user_id: number
  username: string
  topic: 'credit' | 'rate' | 'grace' | 'other'
  status: 'open' | 'closed'
  rating: number
  rating_comment: string
  created_at: number
  updated_at: number
}