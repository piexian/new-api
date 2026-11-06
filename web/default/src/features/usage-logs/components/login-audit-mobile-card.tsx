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
import type { Table } from '@tanstack/react-table'
import { Database, LogIn } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { UsageLog } from '../data/schema'
import { parseLogOther } from '../lib/format'
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
} from '../lib/login-audit'
import { getLogTypeConfig } from '../lib/utils'
import { LoginAuditDetailsDialog } from './dialogs/login-audit-details-dialog'

interface LoginAuditMobileListProps<TData> {
  table: Table<TData>
  isLoading?: boolean
  isAdmin: boolean
  emptyTitle?: string
  emptyDescription?: string
}

function LoginAuditField(props: {
  label: string
  value: string
  hint?: string
  mono?: boolean
  className?: string
}) {
  return (
    <div
      className={cn(
        'bg-muted/20 min-w-0 rounded-md px-2 py-1.5',
        props.className
      )}
    >
      <div className='text-muted-foreground mb-1 text-[11px] leading-none font-medium select-none'>
        {props.label}
      </div>
      <div
        className={cn(
          'min-w-0 truncate text-sm',
          props.mono && 'font-mono text-xs'
        )}
      >
        {props.value}
      </div>
      {props.hint && (
        <div className='text-muted-foreground/70 truncate text-[11px] leading-none'>
          {props.hint}
        </div>
      )}
    </div>
  )
}

function LoginAuditMobileRow(props: { log: UsageLog; isAdmin: boolean }) {
  const { t } = useTranslation()
  const [dialogOpen, setDialogOpen] = useState(false)

  const other = parseLogOther(props.log.other)
  const audit = getLoginAuthAudit(other)
  const outcome = resolveLoginAuditOutcome(props.log.type, audit)
  const typeConfig = getLogTypeConfig(props.log.type)
  const reasonLabel = getLoginAuditReasonLabel(audit?.reason)
  const stageLabel = getLoginAuditStageLabel(audit?.stage)
  const method = getLoginAuditMethod(other)
  const count = getLoginAuditCount(audit)
  const target = getLoginAuditTarget(other)
  const userAgent = getLoginAuditUserAgent(props.log, other)

  return (
    <div className='space-y-2.5'>
      <div className='flex min-w-0 items-center justify-between gap-2'>
        <div className='flex min-w-0 items-center gap-1.5'>
          <StatusBadge
            label={t(getLoginAuditOutcomeLabel(outcome))}
            variant={getLoginAuditOutcomeVariant(outcome)}
            size='sm'
            showDot={false}
            copyable={false}
          />
          <StatusBadge
            label={t(typeConfig.label)}
            variant={typeConfig.color as StatusBadgeProps['variant']}
            size='sm'
            showDot={false}
            copyable={false}
          />
        </div>
        <span className='text-muted-foreground shrink-0 font-mono text-xs tabular-nums'>
          {formatTimestampToDate(props.log.created_at)}
        </span>
      </div>

      <div className='grid grid-cols-2 gap-1.5'>
        <LoginAuditField
          label={t('Reason')}
          value={reasonLabel ? t(reasonLabel) : '-'}
          hint={stageLabel ? t(stageLabel) : undefined}
          className='col-span-2'
        />
        {props.isAdmin && (
          <LoginAuditField
            label={t('Target Account')}
            value={target.username || '-'}
            hint={target.userId ? `ID: ${target.userId}` : undefined}
          />
        )}
        {props.isAdmin && (
          <LoginAuditField
            label={t('Account')}
            value={props.log.username || '-'}
          />
        )}
        <LoginAuditField label={t('Login Method')} value={method || '-'} />
        <LoginAuditField label={t('Attempts')} value={`×${count}`} mono />
        <LoginAuditField
          label={t('IP Address')}
          value={props.log.ip || '-'}
          mono
          className='col-span-2'
        />
      </div>

      <div className='bg-muted/20 min-w-0 rounded-md px-2 py-1.5'>
        <div className='text-muted-foreground mb-1 text-[11px] leading-none font-medium select-none'>
          {t('User Agent')}
        </div>
        <p
          className='line-clamp-2 font-mono text-[11px] leading-relaxed break-all'
          title={userAgent}
        >
          {userAgent || '-'}
        </p>
      </div>

      <Button
        type='button'
        variant='outline'
        size='sm'
        className='w-full'
        onClick={() => setDialogOpen(true)}
      >
        <LogIn className='size-3.5' aria-hidden='true' />
        {t('Details')}
      </Button>

      <LoginAuditDetailsDialog
        log={props.log}
        isAdmin={props.isAdmin}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
      />
    </div>
  )
}

function LoginAuditMobileSkeleton() {
  return (
    <div className='border-border/50 bg-card overflow-hidden rounded-lg border'>
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className='border-border/40 space-y-2.5 border-b p-3 last:border-b-0'
        >
          <div className='flex items-center justify-between gap-3'>
            <Skeleton className='h-5 w-32 rounded-md' />
            <Skeleton className='h-5 w-24 rounded-md' />
          </div>
          <div className='grid grid-cols-2 gap-x-4 gap-y-2'>
            {[1, 2, 3, 4].map((j) => (
              <div key={j} className='min-w-0 space-y-1'>
                <Skeleton className='h-3 w-10 rounded' />
                <Skeleton className='h-4 w-full rounded' />
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

/** Mobile list for the login audit scope (type=7 success / type=8 events). */
export function LoginAuditMobileList<TData>({
  table,
  isLoading = false,
  isAdmin,
  emptyTitle,
  emptyDescription,
}: LoginAuditMobileListProps<TData>) {
  const { t } = useTranslation()

  if (isLoading) {
    return <LoginAuditMobileSkeleton />
  }

  const rows = table.getRowModel().rows

  if (!rows || rows.length === 0) {
    return (
      <div className='rounded-lg border p-6'>
        <Empty className='border-none p-0'>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <Database className='size-6' />
            </EmptyMedia>
            <EmptyTitle>{emptyTitle ?? t('No Logs Found')}</EmptyTitle>
            <EmptyDescription>
              {emptyDescription ??
                t('No login records match the current filters.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      </div>
    )
  }

  return (
    <div className='border-border/50 bg-card overflow-hidden rounded-lg border'>
      {rows.map((row) => (
        <div
          key={row.id}
          className='border-border/40 border-b p-3 transition-colors last:border-b-0'
        >
          <LoginAuditMobileRow
            log={row.original as UsageLog}
            isAdmin={isAdmin}
          />
        </div>
      ))}
    </div>
  )
}
