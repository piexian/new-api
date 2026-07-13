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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { DataTableColumnHeader } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Checkbox } from '@/components/ui/checkbox'
import { formatTimestampToDate } from '@/lib/format'

import { IP_BAN_TYPES } from '../constants'
import type { IPBan } from '../types'
import { DataTableRowActions } from './data-table-row-actions'

function getIPBanType(ban: IPBan) {
  return ban.expires_at === 0 ? IP_BAN_TYPES.PERMANENT : IP_BAN_TYPES.TEMPORARY
}

function isExpired(ban: IPBan) {
  return ban.expires_at > 0 && ban.expires_at <= Math.floor(Date.now() / 1000)
}

export function useIPBansColumns(): ColumnDef<IPBan>[] {
  const { t } = useTranslation()

  return [
    {
      id: 'select',
      meta: { label: t('Select') },
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('Select all')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('Select row')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    },
    {
      accessorKey: 'id',
      meta: { label: t('ID'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('ID')} />
      ),
      cell: ({ row }) => <div className='w-[60px]'>{row.getValue('id')}</div>,
    },
    {
      accessorKey: 'target',
      meta: { label: t('IP / CIDR'), mobileTitle: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('IP / CIDR')} />
      ),
      cell: ({ row }) => (
        <div className='max-w-[220px] truncate font-mono text-sm font-medium'>
          {row.getValue('target')}
        </div>
      ),
    },
    {
      id: 'type',
      meta: { label: t('Type'), mobileBadge: true },
      accessorFn: getIPBanType,
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Type')} />
      ),
      cell: ({ row }) => {
        const ban = row.original
        if (isExpired(ban)) {
          return (
            <StatusBadge
              label={t('Expired')}
              variant='warning'
              copyable={false}
            />
          )
        }
        const type = getIPBanType(ban)
        return (
          <StatusBadge
            label={
              type === IP_BAN_TYPES.PERMANENT ? t('Permanent') : t('Temporary')
            }
            variant={type === IP_BAN_TYPES.PERMANENT ? 'danger' : 'warning'}
            copyable={false}
          />
        )
      },
      filterFn: (row, _id, value) => value.includes(getIPBanType(row.original)),
    },
    {
      accessorKey: 'reason',
      meta: { label: t('Reason') },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Reason')} />
      ),
      cell: ({ row }) => (
        <div className='max-w-[360px] truncate text-sm'>
          {row.getValue('reason')}
        </div>
      ),
      enableSorting: false,
    },
    {
      accessorKey: 'expires_at',
      meta: { label: t('Expires'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Expires')} />
      ),
      cell: ({ row }) => {
        const value = row.getValue('expires_at') as number
        if (!value) {
          return (
            <StatusBadge
              label={t('Never')}
              variant='neutral'
              copyable={false}
            />
          )
        }
        return (
          <div className='min-w-[140px] font-mono text-sm'>
            {formatTimestampToDate(value)}
          </div>
        )
      },
    },
    {
      accessorKey: 'created_at',
      meta: { label: t('Created'), mobileHidden: true },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Created')} />
      ),
      cell: ({ row }) => (
        <div className='min-w-[140px] font-mono text-sm'>
          {formatTimestampToDate(row.getValue('created_at'))}
        </div>
      ),
    },
    {
      id: 'actions',
      cell: ({ row }) => <DataTableRowActions row={row} />,
    },
  ]
}
