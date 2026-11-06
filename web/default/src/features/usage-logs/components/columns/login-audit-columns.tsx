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
import { LogIn } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { formatTimestampToDate } from '@/lib/format'

import { LOG_TYPE_ALL_VALUE } from '../../constants'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'
import {
  getLoginAuditCount,
  getLoginAuditMethod,
  getLoginAuditOutcomeLabel,
  getLoginAuditOutcomeVariant,
  getLoginAuditReasonLabel,
  getLoginAuditStageLabel,
  getLoginAuditTarget,
  getLoginAuditUserAgent,
  getLoginAuthAudit,
  resolveLoginAuditOutcome,
} from '../../lib/login-audit'
import { getLogTypeConfig } from '../../lib/utils'
import { LoginAuditDetailsDialog } from '../dialogs/login-audit-details-dialog'

/** Desktop columns for the login audit scope: audit fields only, no consumption stats. */
export function useLoginAuditColumns(isAdmin: boolean): ColumnDef<UsageLog>[] {
  const { t } = useTranslation()

  const columns: ColumnDef<UsageLog>[] = [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      cell: ({ row }) => {
        const log = row.original
        const config = getLogTypeConfig(log.type)

        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <span className='truncate font-mono text-xs tabular-nums'>
              {formatTimestampToDate(log.created_at)}
            </span>
            <StatusBadge
              label={t(config.label)}
              variant={config.color as StatusBadgeProps['variant']}
              size='sm'
              copyable={false}
              className='-ml-1.5 !text-xs [&_span]:!text-xs'
            />
          </div>
        )
      },
      filterFn: (row, _id, value) => {
        if (!Array.isArray(value) || value.length === 0) return true
        if (value.includes(LOG_TYPE_ALL_VALUE)) return true
        return value.includes(String(row.original.type))
      },
      enableHiding: false,
      size: 170,
    },
    {
      id: 'auth_outcome',
      header: t('Result'),
      cell: ({ row }) => {
        const other = parseLogOther(row.original.other)
        const outcome = resolveLoginAuditOutcome(
          row.original.type,
          getLoginAuthAudit(other)
        )

        return (
          <StatusBadge
            label={t(getLoginAuditOutcomeLabel(outcome))}
            variant={getLoginAuditOutcomeVariant(outcome)}
            size='sm'
            showDot={false}
            copyable={false}
          />
        )
      },
      enableSorting: false,
      size: 130,
    },
  ]

  if (isAdmin) {
    columns.push(
      {
        id: 'user',
        header: t('Account'),
        cell: ({ row }) => {
          const log = row.original
          if (!log.username) {
            return <span className='text-muted-foreground'>-</span>
          }
          return (
            <span className='max-w-[140px] truncate text-sm'>
              {log.username}
            </span>
          )
        },
        enableSorting: false,
        size: 140,
      },
      {
        id: 'target_account',
        header: t('Target Account'),
        cell: ({ row }) => {
          const other = parseLogOther(row.original.other)
          const target = getLoginAuditTarget(other)
          if (!target.username && !target.userId) {
            return <span className='text-muted-foreground'>-</span>
          }

          return (
            <span className='flex max-w-[160px] flex-col gap-0.5'>
              <span className='truncate text-sm'>{target.username || '-'}</span>
              {target.userId && (
                <span className='text-muted-foreground font-mono text-[11px] leading-none'>
                  ID: {target.userId}
                </span>
              )}
            </span>
          )
        },
        enableSorting: false,
        size: 160,
      }
    )
  }

  columns.push(
    {
      id: 'login_method',
      header: t('Login Method'),
      cell: ({ row }) => {
        const method = getLoginAuditMethod(parseLogOther(row.original.other))
        return method ? (
          <span className='truncate text-sm'>{method}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
      enableSorting: false,
      size: 110,
    },
    {
      id: 'auth_reason',
      header: t('Reason'),
      cell: ({ row }) => {
        const other = parseLogOther(row.original.other)
        const audit = getLoginAuthAudit(other)
        const reasonLabel = getLoginAuditReasonLabel(audit?.reason)
        const stageLabel = getLoginAuditStageLabel(audit?.stage)

        if (!reasonLabel && !stageLabel) {
          return <span className='text-muted-foreground'>-</span>
        }

        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            {reasonLabel && (
              <span className='truncate text-xs'>{t(reasonLabel)}</span>
            )}
            {stageLabel && (
              <span className='text-muted-foreground/70 truncate text-[11px] leading-none'>
                {t(stageLabel)}
              </span>
            )}
          </div>
        )
      },
      enableSorting: false,
      size: 170,
    },
    {
      id: 'auth_count',
      header: t('Attempts'),
      cell: ({ row }) => {
        const count = getLoginAuditCount(
          getLoginAuthAudit(parseLogOther(row.original.other))
        )

        return (
          <span
            className={
              count > 1
                ? 'font-mono text-xs font-semibold text-amber-600 tabular-nums dark:text-amber-400'
                : 'text-muted-foreground font-mono text-xs tabular-nums'
            }
          >
            ×{count}
          </span>
        )
      },
      enableSorting: false,
      meta: { label: t('Attempts'), mobileHidden: true },
      size: 80,
    },
    {
      accessorKey: 'ip',
      header: t('IP Address'),
      cell: ({ row }) => {
        const ip = row.original.ip
        return ip ? (
          <span className='truncate font-mono text-xs'>{ip}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
      enableSorting: false,
      meta: { label: t('IP Address') },
      size: 120,
    },
    {
      accessorKey: 'user_agent',
      header: t('User Agent'),
      cell: ({ row }) => {
        const ua = getLoginAuditUserAgent(
          row.original,
          parseLogOther(row.original.other)
        )
        return ua ? (
          <div className='max-w-xs truncate font-mono text-xs' title={ua}>
            {ua}
          </div>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
      enableSorting: false,
      meta: { label: t('User Agent'), mobileHidden: true },
      size: 220,
    },
    {
      id: 'content',
      header: t('Details'),
      cell: function LoginAuditDetailsCell({ row }) {
        const [dialogOpen, setDialogOpen] = useState(false)
        const log = row.original
        const other = parseLogOther(log.other)
        const audit = getLoginAuthAudit(other)
        const outcome = resolveLoginAuditOutcome(log.type, audit)
        const reasonLabel = getLoginAuditReasonLabel(audit?.reason)

        return (
          <>
            <button
              type='button'
              className='text-muted-foreground group flex max-w-[160px] items-center gap-1 text-left text-xs hover:underline'
              onClick={() => setDialogOpen(true)}
              title={t('Click to view full details')}
            >
              <LogIn className='size-3.5 shrink-0' aria-hidden='true' />
              <span className='truncate'>
                {reasonLabel
                  ? t(reasonLabel)
                  : t(getLoginAuditOutcomeLabel(outcome))}
              </span>
            </button>
            <LoginAuditDetailsDialog
              log={log}
              isAdmin={isAdmin}
              open={dialogOpen}
              onOpenChange={setDialogOpen}
            />
          </>
        )
      },
      size: 160,
    }
  )

  return columns
}
