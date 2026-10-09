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

// 与后端 common.ValidateLoginPassword 保持一致
export const PASSWORD_POLICY_MESSAGE =
  'Password must contain at least 8 characters, at most 72 UTF-8 bytes, and include a digit, an uppercase letter, a lowercase letter and an ASCII punctuation symbol.'

export function getPasswordChecks(password = '') {
  return {
    length: [...password].length >= 8,
    bytes: new TextEncoder().encode(password).length <= 72,
    digit: /[0-9]/.test(password),
    upper: /[A-Z]/.test(password),
    lower: /[a-z]/.test(password),
    symbol: /[\x21-\x2f\x3a-\x40\x5b-\x60\x7b-\x7e]/.test(password),
  }
}

export function isValidLoginPassword(password: string) {
  return Object.values(getPasswordChecks(password)).every(Boolean)
}
