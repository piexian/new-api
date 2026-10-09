/*
Copyright (C) 2025 QuantumNous

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

// 保留提供商返回的错误参数，由后端先校验 state 再处理错误。
export function getOAuthCallbackParams(searchParams) {
  const params = new URLSearchParams();
  for (const key of ['code', 'state', 'error', 'error_description']) {
    const value = searchParams.get(key);
    if (value !== null) params.set(key, value);
  }
  return params;
}
