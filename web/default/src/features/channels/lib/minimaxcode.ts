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
const RISK_DISCLAIMER_STORAGE_KEY = 'minimaxcode_risk_disclaimer_accepted'

export function hasAcceptedMiniMaxCodeRiskDisclaimer(): boolean {
  try {
    return (
      globalThis.localStorage?.getItem(RISK_DISCLAIMER_STORAGE_KEY) === 'true'
    )
  } catch {
    return false
  }
}

export function acceptMiniMaxCodeRiskDisclaimer(): void {
  try {
    globalThis.localStorage?.setItem(RISK_DISCLAIMER_STORAGE_KEY, 'true')
  } catch {
    // localStorage 不可用时跳过持久化
  }
}
