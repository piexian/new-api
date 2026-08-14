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
// Loan Type Definitions
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
 * Loan account status. All money fields are int64 quota ($1 = quotaPerUnit).
 * `interest` is an as-of-now projection; `interest_free_until` is a server
 * local day number (unix seconds / 86400), 0 = none.
 */
export interface LoanStatus {
  enabled: boolean
  principal: number
  interest: number
  debt: number
  available: number
  effective_max: number
  daily_rate: number
  interest_free_until: number
  total_borrowed: number
  total_repaid: number
  ai_enabled: boolean
  terms_enabled: boolean
  terms_agreed: boolean
  terms_text: string
}

/**
 * Loan ledger record (borrow / repay)
 */
export interface LoanRecord {
  id: number
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
 * AI officer application list item. `model_used` and `decision` are internal
 * audit fields and must never be rendered to users.
 */
export interface LoanApplication {
  id: number
  topic: 'credit' | 'rate' | 'grace' | 'other'
  status: 'open' | 'closed'
  rating: number
  rating_comment: string
  created_at: number
  updated_at: number
}

/**
 * AI officer conversation message
 */
export interface LoanApplicationMessage {
  id: number
  role: 'user' | 'assistant' | 'system'
  content: string
  created_at: number
}

/**
 * Application detail response
 */
export interface LoanApplicationDetail {
  application: LoanApplication
  messages: LoanApplicationMessage[]
}

/**
 * Result of creating an application / posting a reply
 */
export interface LoanOfficerRoundResult {
  application?: LoanApplication
  reply?: LoanApplicationMessage
  closed: boolean
}

export const LOAN_TOPIC_KEYS = ['credit', 'rate', 'grace', 'other'] as const

export type LoanTopic = (typeof LOAN_TOPIC_KEYS)[number]
