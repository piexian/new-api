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
import { formatTimestampToDate } from '@/lib/format'

import { EMAIL_STATUS_MAPPINGS } from '../../constants'
import type { EmailLog } from '../../types'
import { EmailLogPreviewAction } from '../dialogs/email-log-preview-dialog'
import { createFailReasonColumn } from './column-helpers'

function getEmailStatusConfig(status: string) {
  return (
    EMAIL_STATUS_MAPPINGS[status] ?? {
      label: status || 'Unknown',
      variant: 'neutral' as const,
    }
  )
}

export function useEmailLogsColumns(): ColumnDef<EmailLog>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'created_at',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Send Time')} />
      ),
      cell: ({ row }) => {
        const createdAt = row.getValue('created_at') as number
        return createdAt ? (
          <span className='font-mono text-xs tabular-nums'>
            {formatTimestampToDate(createdAt, 'seconds')}
          </span>
        ) : (
          <span className='text-muted-foreground/60 text-xs'>-</span>
        )
      },
      meta: { label: t('Send Time') },
    },
    {
      accessorKey: 'status',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Status')} />
      ),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        const config = getEmailStatusConfig(status)
        return (
          <StatusBadge
            label={t(config.label)}
            variant={config.variant}
            copyText={status}
            size='sm'
          />
        )
      },
      meta: { label: t('Status') },
    },
    {
      accessorKey: 'receiver',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Receiver')} />
      ),
      cell: ({ row }) => {
        const receiver = row.getValue('receiver') as string
        return receiver ? (
          <span className='font-mono text-xs break-all'>{receiver}</span>
        ) : (
          <span className='text-muted-foreground/60 text-xs'>-</span>
        )
      },
      meta: { label: t('Receiver') },
    },
    {
      accessorKey: 'subject',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Subject')} />
      ),
      cell: ({ row }) => {
        const subject = row.getValue('subject') as string
        return subject ? (
          <span
            className='block max-w-[22rem] truncate text-xs'
            title={subject}
          >
            {subject}
          </span>
        ) : (
          <span className='text-muted-foreground/60 text-xs'>-</span>
        )
      },
      meta: { label: t('Subject') },
    },
    {
      accessorKey: 'provider',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Provider')} />
      ),
      cell: ({ row }) => {
        const provider = row.getValue('provider') as string
        return provider ? (
          <StatusBadge
            label={provider}
            autoColor={provider}
            copyText={provider}
            size='sm'
          />
        ) : (
          <span className='text-muted-foreground/60 text-xs'>-</span>
        )
      },
      meta: { label: t('Provider') },
    },
    {
      accessorKey: 'duration_ms',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Duration')} />
      ),
      cell: ({ row }) => {
        const durationMs = row.getValue('duration_ms') as number
        return durationMs > 0 ? (
          <span className='font-mono text-xs tabular-nums'>{durationMs}ms</span>
        ) : (
          <span className='text-muted-foreground/60 text-xs'>-</span>
        )
      },
      meta: { label: t('Duration') },
    },
    createFailReasonColumn<EmailLog>({
      accessorKey: 'error_message',
      headerLabel: t('Error Message'),
      cellTitle: t('Fail Reason Details'),
    }),
    {
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => <EmailLogPreviewAction log={row.original} />,
      enableSorting: false,
      enableHiding: false,
      size: 72,
      meta: { label: t('Actions') },
    },
  ]
}
