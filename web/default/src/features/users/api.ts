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
  User,
  GetUsersParams,
  GetUsersResponse,
  SearchUsersParams,
  UserFormData,
  UserTokenFormData,
  GetUserTokensParams,
  GetUserTokensResponse,
  ManageUserAction,
  ManageUserQuotaPayload,
  ManageUserPayload,
  ApiResponse,
  UserToken,
} from './types'

// ============================================================================
// User Management APIs
// ============================================================================

/**
 * Get paginated users list
 */
export async function getUsers(
  params: GetUsersParams = {}
): Promise<GetUsersResponse> {
  const {
    p = 1,
    page_size = 10,
    group = '',
    status = '',
    role = '',
    quota_order = '',
  } = params
  const res = await api.get('/api/user/', {
    params: {
      p,
      page_size,
      group: group || undefined,
      status: status || undefined,
      role: role || undefined,
      quota_order: quota_order || undefined,
    },
  })
  return res.data
}

/**
 * Search users by keyword or group
 */
export async function searchUsers(
  params: SearchUsersParams
): Promise<GetUsersResponse> {
  const {
    keyword = '',
    group = '',
    status = '',
    role = '',
    quota_order = '',
    p = 1,
    page_size = 10,
  } = params
  const res = await api.get('/api/user/search', {
    params: {
      keyword,
      group: group || undefined,
      status: status || undefined,
      role: role || undefined,
      quota_order: quota_order || undefined,
      p,
      page_size,
    },
  })
  return res.data
}

/**
 * Get single user by ID
 */
export async function getUser(id: number): Promise<ApiResponse<User>> {
  const res = await api.get(`/api/user/${id}`)
  return res.data
}

/**
 * Create a new user
 */
export async function createUser(
  data: UserFormData
): Promise<ApiResponse<User>> {
  const res = await api.post('/api/user/', data)
  return res.data
}

/**
 * Update an existing user
 */
export async function updateUser(
  data: UserFormData & { id: number }
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.put('/api/user/', data)
  return res.data
}

/**
 * Delete a single user (hard delete)
 */
export async function deleteUser(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/`)
  return res.data
}

/**
 * Manage user (promote, demote, enable, disable, delete)
 */
export async function manageUser(
  idOrPayload: number | ManageUserPayload,
  action?: ManageUserAction
): Promise<ApiResponse<Partial<User>>> {
  const payload =
    typeof idOrPayload === 'number' ? { id: idOrPayload, action } : idOrPayload
  const res = await api.post('/api/user/manage', payload)
  return res.data
}

/**
 * Adjust user quota atomically (add/subtract/override)
 */
export async function adjustUserQuota(
  payload: ManageUserQuotaPayload
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', payload)
  return res.data
}

/**
 * Reset user's Passkey registration
 */
export async function resetUserPasskey(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/reset_passkey`)
  return res.data
}

/**
 * Reset user's Two-Factor Authentication setup
 */
export async function resetUserTwoFA(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/2fa`)
  return res.data
}

/**
 * Get all available groups
 */
export async function getGroups(): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group/')
  return res.data
}

export async function getUserAvailableModels(
  userId: number
): Promise<ApiResponse<string[]>> {
  const res = await api.get(`/api/user/${userId}/models`)
  return res.data
}

export async function getUserAvailableGroups(
  userId: number
): Promise<
  ApiResponse<Record<string, { desc: string; ratio: number | string }>>
> {
  const res = await api.get(`/api/user/${userId}/groups`)
  return res.data
}

// ============================================================================
// Admin Binding Management APIs
// ============================================================================

export interface OAuthBinding {
  provider_id: string | number
  provider_name: string
  provider_slug?: string
  provider_icon?: string
  provider_user_id?: string
  user_id?: number
  external_id?: string
}

/**
 * Get user's custom OAuth bindings (admin)
 */
export async function getUserOAuthBindings(
  userId: number
): Promise<ApiResponse<OAuthBinding[]>> {
  const res = await api.get(`/api/user/${userId}/oauth/bindings`)
  return res.data
}

/**
 * Clear a user's built-in binding (admin)
 */
export async function adminClearUserBinding(
  userId: number,
  bindingType: string
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${userId}/bindings/${bindingType}`)
  return res.data
}

/**
 * Unbind custom OAuth for a user (admin)
 */
export async function adminUnbindCustomOAuth(
  userId: number,
  providerId: string
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${userId}/oauth/bindings/${providerId}`
  )
  return res.data
}

// ============================================================================
// Admin User Token Management APIs
// ============================================================================

export async function getUserTokens(
  userId: number,
  params: GetUserTokensParams = {}
): Promise<GetUserTokensResponse> {
  const { p = 1, size = 5 } = params
  const res = await api.get(`/api/user/${userId}/tokens`, {
    params: { p, size },
  })
  return res.data
}

export async function getUserToken(
  userId: number,
  tokenId: number
): Promise<ApiResponse<UserToken>> {
  const res = await api.get(`/api/user/${userId}/tokens/${tokenId}`)
  return res.data
}

export async function createUserToken(
  userId: number,
  data: UserTokenFormData
): Promise<ApiResponse<UserToken>> {
  const res = await api.post(`/api/user/${userId}/tokens`, data)
  return res.data
}

export async function updateUserToken(
  userId: number,
  tokenId: number,
  data: UserTokenFormData
): Promise<ApiResponse<UserToken>> {
  const res = await api.put(`/api/user/${userId}/tokens/${tokenId}`, data)
  return res.data
}

export async function updateUserTokenStatus(
  userId: number,
  tokenId: number,
  status: number
): Promise<ApiResponse<UserToken>> {
  const res = await api.put(
    `/api/user/${userId}/tokens/${tokenId}?status_only=true`,
    { status }
  )
  return res.data
}

export async function deleteUserToken(
  userId: number,
  tokenId: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${userId}/tokens/${tokenId}`)
  return res.data
}
