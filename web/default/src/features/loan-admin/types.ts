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
  fee_part: number
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

/**
 * Lending market offer with lender username (admin cross-user view).
 * Money fields are int64 quota ($1 = quotaPerUnit); rate fields are daily
 * rate fractions (0.001 = 0.1%/day); min_credit_score -50 = no limit.
 */
export interface LoanAdminOffer {
  id: number
  lender_id: number
  username: string
  mode: 'pool' | 'ai' | 'order'
  status: 'active' | 'paused' | 'closed'
  amount_total: number
  amount_available: number
  rate_fixed: number
  rate_min: number
  rate_max: number
  per_loan_cap: number
  min_credit_score: number
  total_lent: number
  total_interest_earned: number
  created_at: number
  updated_at: number
}

/**
 * Lending market funding record with lender/borrower usernames (admin
 * cross-user view). Money fields are int64 quota.
 */
export interface LoanAdminFunding {
  id: number
  loan_user_id: number
  borrow_event_id: number
  source_type: 'platform' | 'pool' | 'ai' | 'order'
  offer_id: number
  lender_id: number
  amount: number
  principal_remaining: number
  debt_quota: number
  last_settled_day: number
  rate: number
  repay_plan: 'full' | 'no_penalty' | 'interest_freeze' | 'principal_only'
  status: 'active' | 'overdue' | 'repaid' | 'written_off'
  due_day: number
  penalty_started_day: number
  created_at: number
  updated_at: number
  lender_username: string
  borrower_username: string
}

/**
 * Lending market aggregate overview (admin read-only)
 */
export interface LoanMarketOverview {
  offers_by_status: Record<string, number>
  frozen_idle: number
  in_loan_principal: number
  total_interest_earned: number
  overdue_fundings: number
  active_offers: number
}