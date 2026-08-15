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
import { api } from '@/lib/api'

import type {
  ApiResponse,
  CreateLoanOfferPayload,
  LoanApplication,
  LoanApplicationDetail,
  LoanFunding,
  LoanFundingResolveAction,
  LoanOffer,
  LoanOfficerRoundResult,
  LoanRecord,
  LoanRepayPlan,
  LoanStatus,
  LoanTopic,
  MarketOffer,
  PageInfo,
} from './types'

// ============================================================================
// Loan Account APIs
// ============================================================================

/**
 * Get current user's loan status
 */
export async function getLoanStatus(): Promise<ApiResponse<LoanStatus>> {
  const res = await api.get('/api/user/loan/status')
  return res.data
}

/**
 * Agree to the loan terms (idempotent)
 */
export async function agreeLoanTerms(): Promise<ApiResponse> {
  const res = await api.post('/api/user/loan/agree')
  return res.data
}

/**
 * Borrow an amount in USD (string to preserve decimal precision),
 * optionally targeting a specific market order offer. Pass the amount only;
 * order_id 0 means no specific order.
 * Returns the refreshed loan status on success.
 */
export async function borrowLoan(
  amountUsd: string,
  orderId?: number
): Promise<ApiResponse<LoanStatus>> {
  const res = await api.post('/api/user/loan/borrow', {
    amount_usd: amountUsd,
    order_id: orderId ?? 0,
  })
  return res.data
}

/**
 * Repay an amount in USD, or "all" to repay as much as the wallet covers.
 * Returns the refreshed loan status (plus a repay breakdown) on success.
 */
export async function repayLoan(
  amountUsd: string
): Promise<ApiResponse<LoanStatus>> {
  const res = await api.post('/api/user/loan/repay', { amount_usd: amountUsd })
  return res.data
}

/**
 * Get paginated loan ledger records
 */
export async function getLoanRecords(
  page: number,
  pageSize: number
): Promise<ApiResponse<PageInfo<LoanRecord>>> {
  const res = await api.get('/api/user/loan/records', {
    params: { p: page, page_size: pageSize },
  })
  return res.data
}

// ============================================================================
// AI Officer Application APIs
// ============================================================================

/**
 * Create an AI officer application and run the first conversation round.
 * Note: on officer failure the application may still exist — callers should
 * refresh the list and guide the user to continue in the detail view.
 */
export async function createLoanApplication(
  topic: LoanTopic,
  content: string
): Promise<ApiResponse<LoanOfficerRoundResult>> {
  const res = await api.post('/api/user/loan/applications', { topic, content })
  return res.data
}

/**
 * Get paginated AI officer applications
 */
export async function getLoanApplications(
  page: number,
  pageSize: number
): Promise<ApiResponse<PageInfo<LoanApplication>>> {
  const res = await api.get('/api/user/loan/applications', {
    params: { p: page, page_size: pageSize },
  })
  return res.data
}

/**
 * Get an application's detail with the full message thread
 */
export async function getLoanApplicationDetail(
  id: number
): Promise<ApiResponse<LoanApplicationDetail>> {
  const res = await api.get(`/api/user/loan/applications/${id}`)
  return res.data
}

/**
 * Post a reply in an open application (runs one AI round)
 */
export async function replyLoanApplication(
  id: number,
  content: string
): Promise<ApiResponse<LoanOfficerRoundResult>> {
  const res = await api.post(`/api/user/loan/applications/${id}/reply`, {
    content,
  })
  return res.data
}

/**
 * Rate a closed application (1-5, one-time)
 */
export async function rateLoanApplication(
  id: number,
  rating: number,
  comment: string
): Promise<ApiResponse> {
  const res = await api.post(`/api/user/loan/applications/${id}/rate`, {
    rating,
    comment,
  })
  return res.data
}

// ============================================================================
// Lending Market APIs
// ============================================================================

/**
 * Agree to the lender disclaimer (idempotent)
 */
export async function agreeLenderDisclaimer(): Promise<ApiResponse> {
  const res = await api.post('/api/user/loan/market/disclaimer')
  return res.data
}

/**
 * Get the current user's own loan offers
 */
export async function getLoanMarketOffers(): Promise<
  ApiResponse<{ offers: LoanOffer[] }>
> {
  const res = await api.get('/api/user/loan/market/offers')
  return res.data
}

/**
 * Create a loan offer. Returns the created offer on success.
 */
export async function createLoanMarketOffer(
  payload: CreateLoanOfferPayload
): Promise<ApiResponse<LoanOffer>> {
  const res = await api.post('/api/user/loan/market/offers', payload)
  return res.data
}

/**
 * Pause an offer so it no longer participates in matching
 */
export async function pauseLoanMarketOffer(id: number): Promise<ApiResponse> {
  const res = await api.post(`/api/user/loan/market/offers/${id}/pause`)
  return res.data
}

/**
 * Resume a paused offer
 */
export async function resumeLoanMarketOffer(id: number): Promise<ApiResponse> {
  const res = await api.post(`/api/user/loan/market/offers/${id}/resume`)
  return res.data
}

/**
 * Close an offer (terminal state); idle quota is refunded to balance
 */
export async function closeLoanMarketOffer(id: number): Promise<ApiResponse> {
  const res = await api.post(`/api/user/loan/market/offers/${id}/close`)
  return res.data
}

/**
 * Withdraw all idle quota of an offer back to the balance; offer keeps its
 * current status. Returns the refunded quota amount.
 */
export async function withdrawLoanMarketOffer(
  id: number
): Promise<ApiResponse<{ refunded: number }>> {
  const res = await api.post(`/api/user/loan/market/offers/${id}/withdraw`)
  return res.data
}

/**
 * Get the anonymized order-offer list for market browsing
 */
export async function getLoanMarketList(): Promise<
  ApiResponse<{ offers: MarketOffer[] }>
> {
  const res = await api.get('/api/user/loan/market/list')
  return res.data
}

/**
 * Get the paginated lender funding ledger
 */
export async function getLoanMarketFundings(
  page: number,
  pageSize: number
): Promise<ApiResponse<PageInfo<LoanFunding>>> {
  const res = await api.get('/api/user/loan/market/fundings', {
    params: { p: page, page_size: pageSize },
  })
  return res.data
}

/**
 * Resolve an overdue funding (extend / writeoff / perpetual)
 */
export async function resolveLoanMarketFunding(
  id: number,
  action: LoanFundingResolveAction,
  extendDays: number
): Promise<ApiResponse> {
  const res = await api.post(`/api/user/loan/market/fundings/${id}/resolve`, {
    action,
    extend_days: extendDays,
  })
  return res.data
}

/**
 * Adjust the repay plan of a funding
 */
export async function setLoanMarketFundingRepayPlan(
  id: number,
  plan: LoanRepayPlan
): Promise<ApiResponse> {
  const res = await api.post(
    `/api/user/loan/market/fundings/${id}/repay_plan`,
    { plan }
  )
  return res.data
}
