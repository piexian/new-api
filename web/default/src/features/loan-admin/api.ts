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
  LoanAdminAccount,
  LoanAdminApplication,
  LoanAdminFunding,
  LoanAdminOffer,
  LoanAdminRecord,
  LoanMarketOverview,
  PageInfo,
} from './types'

// ============================================================================
// Loan Admin APIs (admin-only)
// ============================================================================

/**
 * Get paginated token loan accounts. keyword fuzzy-matches username; a purely
 * numeric keyword also matches user_id.
 */
export async function getAdminLoanAccounts(
  page: number,
  pageSize: number,
  keyword: string
): Promise<ApiResponse<PageInfo<LoanAdminAccount>>> {
  const res = await api.get('/api/user/loan/admin/accounts', {
    params: { p: page, page_size: pageSize, keyword },
  })
  return res.data
}

/**
 * Get paginated loan ledger records (user_id > 0 filters by user)
 */
export async function getAdminLoanRecords(
  page: number,
  pageSize: number,
  userId: string
): Promise<ApiResponse<PageInfo<LoanAdminRecord>>> {
  const res = await api.get('/api/user/loan/admin/records', {
    params: { p: page, page_size: pageSize, user_id: userId },
  })
  return res.data
}

/**
 * Get paginated loan applications (user_id > 0 filters by user; status is
 * 'open' | 'closed' or empty for all)
 */
export async function getAdminLoanApplications(
  page: number,
  pageSize: number,
  userId: string,
  status: string
): Promise<ApiResponse<PageInfo<LoanAdminApplication>>> {
  const res = await api.get('/api/user/loan/admin/applications', {
    params: { p: page, page_size: pageSize, user_id: userId, status },
  })
  return res.data
}

/**
 * Get paginated lending market offers (keyword filters lender; a purely
 * numeric keyword matches lender user id, otherwise fuzzy-matches username)
 */
export async function getAdminLoanOffers(
  page: number,
  pageSize: number,
  keyword: string
): Promise<ApiResponse<PageInfo<LoanAdminOffer>>> {
  const res = await api.get('/api/user/loan/admin/offers', {
    params: { p: page, page_size: pageSize, keyword },
  })
  return res.data
}

/**
 * Get paginated lending market fundings (lender_id / loan_user_id > 0 filter
 * by user; status is 'active' | 'overdue' | 'repaid' | 'written_off' or empty)
 */
export async function getAdminLoanFundings(
  page: number,
  pageSize: number,
  lenderId: string,
  loanUserId: string,
  status: string
): Promise<ApiResponse<PageInfo<LoanAdminFunding>>> {
  const res = await api.get('/api/user/loan/admin/fundings', {
    params: {
      p: page,
      page_size: pageSize,
      lender_id: lenderId,
      loan_user_id: loanUserId,
      status,
    },
  })
  return res.data
}

/**
 * Get the lending market aggregate overview (read-only)
 */
export async function getAdminLoanMarketOverview(): Promise<
  ApiResponse<LoanMarketOverview>
> {
  const res = await api.get('/api/user/loan/admin/market_overview')
  return res.data
}