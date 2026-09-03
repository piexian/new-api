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
/**
 * Username + user id cell shared by loan admin tables
 */
export function UserCell({
  username,
  userId,
}: {
  username: string
  userId: number
}) {
  return (
    <div className='flex min-w-0 flex-col'>
      <span className='truncate font-medium'>{username || '-'}</span>
      <span className='text-muted-foreground text-xs'>#{userId}</span>
    </div>
  )
}
