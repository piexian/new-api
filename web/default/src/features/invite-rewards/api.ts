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
  AffiliateTransferRequest,
  ApiResponse,
  InvitedUser,
  InviteTopupInfo,
} from './types'

export async function getAffiliateCode(): Promise<ApiResponse<string>> {
  const res = await api.get('/api/user/aff')
  return res.data
}

export async function resetAffiliateCode(): Promise<ApiResponse<string>> {
  const res = await api.post('/api/user/aff/reset')
  return res.data
}

export async function getInvitedUsers(): Promise<ApiResponse<InvitedUser[]>> {
  const res = await api.get('/api/user/aff/invited')
  return res.data
}

export async function transferAffiliateQuota(
  request: AffiliateTransferRequest
): Promise<ApiResponse> {
  const res = await api.post('/api/user/aff_transfer', request)
  return res.data
}

export async function getInviteTopupInfo(): Promise<
  ApiResponse<InviteTopupInfo>
> {
  const res = await api.get('/api/user/topup/info')
  return res.data
}
