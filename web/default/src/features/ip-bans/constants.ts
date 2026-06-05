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
import type { IPBanType } from './types'

export const IP_BAN_TYPES = {
  PERMANENT: 'permanent',
  TEMPORARY: 'temporary',
} as const satisfies Record<string, IPBanType>

export const IP_BAN_TYPE_VALUES = [
  IP_BAN_TYPES.PERMANENT,
  IP_BAN_TYPES.TEMPORARY,
] as const

export const getIPBanTypeOptions = (t: (key: string) => string) => [
  { label: t('Permanent'), value: IP_BAN_TYPES.PERMANENT },
  { label: t('Temporary'), value: IP_BAN_TYPES.TEMPORARY },
]

export const SUCCESS_MESSAGES = {
  IP_BAN_CREATED: 'IP ban rule created successfully',
  IP_BAN_UPDATED: 'IP ban rule updated successfully',
  IP_BAN_DELETED: 'IP ban rule deleted successfully',
  IP_BAN_BATCH_CREATED: 'IP ban batch import completed',
} as const
