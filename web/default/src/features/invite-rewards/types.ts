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
export interface ApiResponse<T = unknown> {
  success: boolean
  message: string
  data: T
}

export interface InviteRewardsUserData {
  id: number
  username: string
  quota: number
  aff_quota: number
  aff_history_quota: number
  aff_count: number
}

export interface InvitedUser {
  id: number
  username: string
  display_name?: string
  created_at?: number
}

export interface AffiliateTransferRequest {
  quota: number
}

export interface InviteTopupInfo {
  payment_compliance_confirmed?: boolean
}
