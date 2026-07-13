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
  GetIPBansParams,
  GetIPBansResponse,
  IPBan,
  IPBanBatchFormData,
  IPBanBatchResult,
  IPBanFormData,
  SearchIPBansParams,
} from './types'

const skipBusinessErrorConfig = {
  skipBusinessError: true,
} as Record<string, unknown>

export async function getIPBans(
  params: GetIPBansParams = {}
): Promise<GetIPBansResponse> {
  const { p = 1, page_size = 10, type = '' } = params
  const res = await api.get('/api/ip_ban/', {
    params: { p, page_size, type: type || undefined },
  })
  return res.data
}

export async function searchIPBans(
  params: SearchIPBansParams
): Promise<GetIPBansResponse> {
  const { keyword = '', p = 1, page_size = 10, type = '' } = params
  const res = await api.get('/api/ip_ban/search', {
    params: { keyword, p, page_size, type: type || undefined },
  })
  return res.data
}

export async function getIPBan(id: number): Promise<ApiResponse<IPBan>> {
  const res = await api.get(`/api/ip_ban/${id}`)
  return res.data
}

export async function createIPBan(
  data: IPBanFormData
): Promise<ApiResponse<IPBan>> {
  const res = await api.post('/api/ip_ban/', data, skipBusinessErrorConfig)
  return res.data
}

export async function updateIPBan(
  data: IPBanFormData & { id: number }
): Promise<ApiResponse<IPBan>> {
  const res = await api.put('/api/ip_ban/', data, skipBusinessErrorConfig)
  return res.data
}

export async function deleteIPBan(
  id: number
): Promise<ApiResponse<{ id: number }>> {
  const res = await api.delete(`/api/ip_ban/${id}`)
  return res.data
}

export async function batchCreateIPBans(
  data: IPBanBatchFormData
): Promise<ApiResponse<IPBanBatchResult>> {
  const res = await api.post('/api/ip_ban/batch', data, skipBusinessErrorConfig)
  return res.data
}
