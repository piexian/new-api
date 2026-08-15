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
  /** Early repayment fee rate applied to the principal part (0 = no fee) */
  repay_fee_rate: number
  /** Current credit score (0 is a valid penalized value) */
  credit_score: number
  /** Whether the lending market is enabled */
  market_enabled: boolean
  /** Server-local day number the user is blacklisted until, 0 = not blacklisted */
  blacklisted_until_day: number
  /** Whether the user currently has any overdue funding */
  has_overdue: boolean
  /** Whether the user agreed to the lender disclaimer */
  lender_disclaimer_agreed: boolean
  /** Estimated fast-settle penalty if the whole debt were manually settled now (0 = none) */
  fast_repay_penalty_estimate?: number
  /** Repay breakdown returned by a successful manual repay (absent otherwise) */
  repay?: LoanRepayResult
  /** Funding breakdown returned by a successful borrow (absent otherwise) */
  fundings?: LoanBorrowFunding[]
}

/**
 * Per-funding breakdown returned in the borrow response (`data.fundings`).
 * Money fields are int64 quota; `rate` is a daily rate fraction.
 */
export interface LoanBorrowFunding {
  source_type: LoanFundingSource
  amount: number
  rate: number
  /** Fast-settle penalty charged if this funding is manually settled inside the window (0 = none) */
  fast_repay_penalty_quota: number
  /** Fast-settle window in days (0 = same day only) */
  fast_repay_window_days: number
}

/**
 * Repay result breakdown returned in the repay response (`data.repay`).
 * All money fields are int64 quota; `penalty_part` is the fast-settle
 * penalty total charged for fundings repaid within the lender window.
 */
export interface LoanRepayResult {
  amount: number
  interest_part: number
  principal_part: number
  /** Early repayment fee (manual repay only) */
  fee_part: number
  /** Fast-settle penalty total (0 when no funding was penalized) */
  penalty_part: number
  debt_after: number
}

/**
 * Loan ledger record (borrow / repay / credit score change)
 */
export interface LoanRecord {
  id: number
  type: 'borrow' | 'repay' | 'credit'
  amount: number
  interest_part: number
  principal_part: number
  /** Early repayment fee (manual repay only; 0 otherwise) */
  fee_part: number
  /** Fast-settle penalty (manual repay within the lender window only; 0 otherwise) */
  penalty_part: number
  /** Debt after change; for type=credit this is the credit score after the change */
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
  topic: 'credit' | 'rate' | 'grace' | 'other' | 'appeal'
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
  reply?: string
  closed: boolean
}

export const LOAN_TOPIC_KEYS = [
  'credit',
  'rate',
  'grace',
  'other',
  'appeal',
] as const

export type LoanTopic = (typeof LOAN_TOPIC_KEYS)[number]

// ============================================================================
// Lending Market Type Definitions
// ============================================================================

/**
 * 撮合模式：pool 资金池自动撮合 / ai AI 审核撮合 / order 订单式公开撮合
 */
export const LOAN_OFFER_MODE_KEYS = ['pool', 'ai', 'order'] as const
export type LoanOfferMode = (typeof LOAN_OFFER_MODE_KEYS)[number]

export const LOAN_OFFER_STATUS_KEYS = ['active', 'paused', 'closed'] as const
export type LoanOfferStatus = (typeof LOAN_OFFER_STATUS_KEYS)[number]

/**
 * Lender's own offer. Money fields are int64 quota ($1 = quotaPerUnit);
 * rate fields are daily rate fractions (0.001 = 0.1%/day);
 * min_credit_score -50 = no limit.
 */
export interface LoanOffer {
  id: number
  mode: LoanOfferMode
  status: LoanOfferStatus
  amount_total: number
  amount_available: number
  rate_fixed: number
  rate_min: number
  rate_max: number
  per_loan_cap: number
  min_credit_score: number
  /** Fast-settle penalty quota charged per fully-repaid funding inside the window (0 = none) */
  fast_repay_penalty_quota: number
  /** Fast-settle window in days (0 = same day only) */
  fast_repay_window_days: number
  total_lent: number
  total_interest_earned: number
  created_at: number
}

/**
 * Anonymized order offer shown on the market browse list.
 */
export interface MarketOffer {
  id: number
  amount_available: number
  rate_fixed: number
  min_credit_score: number
  lender_credit_score: number
  /** Fast-settle penalty quota charged per fully-repaid funding inside the window (0 = none) */
  fast_repay_penalty_quota: number
  /** Fast-settle window in days (0 = same day only) */
  fast_repay_window_days: number
}

export const LOAN_FUNDING_SOURCE_KEYS = [
  'platform',
  'pool',
  'ai',
  'order',
] as const
export type LoanFundingSource = (typeof LOAN_FUNDING_SOURCE_KEYS)[number]

export const LOAN_FUNDING_STATUS_KEYS = [
  'active',
  'overdue',
  'repaid',
  'written_off',
] as const
export type LoanFundingStatus = (typeof LOAN_FUNDING_STATUS_KEYS)[number]

export const LOAN_REPAY_PLAN_KEYS = [
  'full',
  'no_penalty',
  'interest_freeze',
  'principal_only',
] as const
export type LoanRepayPlan = (typeof LOAN_REPAY_PLAN_KEYS)[number]

/**
 * A lender-ledger funding entry. Money fields are int64 quota;
 * `debt` is a projected amount for active/overdue rows.
 */
export interface LoanFunding {
  id: number
  loan_user_id: number
  source_type: LoanFundingSource
  offer_id: number
  amount: number
  principal_remaining: number
  repaid_principal: number
  debt: number
  rate: number
  repay_plan: LoanRepayPlan
  status: LoanFundingStatus
  due_day: number
  created_at: number
  borrower_credit_score: number
  /** Fast-settle penalty quota charged if fully repaid inside the window (0 = none) */
  fast_repay_penalty_quota: number
  /** Fast-settle window in days (0 = same day only) */
  fast_repay_window_days: number
}

/** Payload for creating a loan market offer */
export interface CreateLoanOfferPayload {
  mode: LoanOfferMode
  amount_usd: string
  rate_fixed: string
  rate_min: number
  rate_max: number
  per_loan_cap: number
  min_credit_score: number
  /** Fast-settle penalty in USD (empty string = 0, no penalty) */
  fast_repay_penalty_usd: string
  /** Fast-settle window in days, 0 = same day only */
  fast_repay_window_days: number
}

/** Overdue resolution actions available to the lender */
export const LOAN_FUNDING_RESOLVE_ACTIONS = [
  'extend',
  'writeoff',
  'perpetual',
] as const
export type LoanFundingResolveAction =
  (typeof LOAN_FUNDING_RESOLVE_ACTIONS)[number]
