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
  LoanApplication,
  LoanApplicationDetail,
  LoanOfficerRoundResult,
  LoanRecord,
  LoanStatus,
  LoanTopic,
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
 * Borrow an amount in USD (string to preserve decimal precision).
 * Returns the refreshed loan status on success.
 */
export async function borrowLoan(
  amountUsd: string
): Promise<ApiResponse<LoanStatus>> {
  const res = await api.post('/api/user/loan/borrow', { amount_usd: amountUsd })
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
