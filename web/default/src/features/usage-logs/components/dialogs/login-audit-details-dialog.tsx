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
import { Globe, LogIn, ShieldAlert, UserCog } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { formatTimestampToDate } from '@/lib/format'

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
  getLoginAuditWindow,
  getLoginAuthAudit,
  resolveLoginAuditOutcome,
} from '../../lib/login-audit'
import { getLogTypeConfig } from '../../lib/utils'
import { DetailRow, DetailSection } from './details-dialog'

interface LoginAuditDetailsDialogProps {
  log: UsageLog
  isAdmin: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}

function formatWindowLabel(start?: number, end?: number): string {
  if (!start) return ''
  const startText = formatTimestampToDate(start)
  if (!end) return startText
  return `${startText} - ${formatTimestampToDate(end)}`
}

function formatAccountLabel(username: string, userId: string): string {
  if (!username) return userId ? `ID: ${userId}` : '-'
  return userId ? `${username} (ID: ${userId})` : username
}

/** Login audit details: whitelisted fields only, admin diagnostics gated by role. */
export function LoginAuditDetailsDialog(props: LoginAuditDetailsDialogProps) {
  const { t } = useTranslation()
  const other = parseLogOther(props.log.other)
  const adminInfo = other?.admin_info
  const audit = getLoginAuthAudit(other)
  const outcome = resolveLoginAuditOutcome(props.log.type, audit)
  const outcomeVariant = getLoginAuditOutcomeVariant(outcome)
  const typeConfig = getLogTypeConfig(props.log.type)

  const stageLabel = getLoginAuditStageLabel(audit?.stage)
  const reasonLabel = getLoginAuditReasonLabel(audit?.reason)
  const method = getLoginAuditMethod(other)
  const count = getLoginAuditCount(audit)
  const window = getLoginAuditWindow(audit)
  const userAgent = getLoginAuditUserAgent(props.log, other)
  const target = getLoginAuditTarget(other)

  const windowText = formatWindowLabel(window.start, window.end)
  const accountId = props.log.user_id > 0 ? String(props.log.user_id) : ''

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <>
          {t('Login Audit')}
          <StatusBadge
            label={t(typeConfig.label)}
            variant={typeConfig.color as StatusBadgeProps['variant']}
            size='sm'
            copyable={false}
          />
        </>
      }
      description={t('View the complete details for this log entry')}
      contentClassName='min-w-0 overflow-hidden'
      descriptionClassName='sr-only'
      titleClassName='flex items-center gap-2 text-base'
      contentHeight='min(72dvh, 720px)'
      bodyClassName='pr-2 sm:pr-4'
    >
      <div className='w-full max-w-full min-w-0 space-y-2.5 overflow-x-hidden py-1 sm:space-y-3'>
        <DetailSection
          icon={<LogIn className='size-3.5' aria-hidden='true' />}
          iconTone='info'
          label={t('Result')}
        >
          <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
            <StatusBadge
              label={t(getLoginAuditOutcomeLabel(outcome))}
              variant={outcomeVariant}
              size='sm'
              showDot={false}
              copyable={false}
            />
            {stageLabel && (
              <StatusBadge
                label={t(stageLabel)}
                variant='neutral'
                size='sm'
                showDot={false}
                copyable={false}
              />
            )}
          </div>
          {reasonLabel && (
            <DetailRow label={t('Reason')} value={t(reasonLabel)} />
          )}
        </DetailSection>

        {(method || count > 1 || windowText) && (
          <DetailSection label={t('Login Info')}>
            {method && <DetailRow label={t('Login Method')} value={method} />}
            {count > 1 && (
              <DetailRow label={t('Attempts')} value={`×${count}`} mono />
            )}
            {windowText && (
              <DetailRow label={t('Attempt window')} value={windowText} mono />
            )}
          </DetailSection>
        )}

        {props.isAdmin && (target.username || target.userId) && (
          <DetailSection
            icon={<UserCog className='size-3.5' aria-hidden='true' />}
            iconTone='info'
            label={t('Target Account')}
          >
            {target.username && (
              <DetailRow label={t('Account')} value={target.username} mono />
            )}
            {target.userId && (
              <DetailRow label={t('User ID')} value={target.userId} mono />
            )}
            {adminInfo?.attempted_identifier && (
              <DetailRow
                label={t('Attempted Identifier')}
                value={String(adminInfo.attempted_identifier)}
                mono
              />
            )}
          </DetailSection>
        )}

        <DetailSection
          icon={<Globe className='size-3.5' aria-hidden='true' />}
          label={t('Network')}
        >
          {other?.request_path && (
            <DetailRow
              label={t('Path')}
              value={String(other.request_path)}
              mono
            />
          )}
          {props.log.ip && (
            <DetailRow label={t('IP Address')} value={props.log.ip} mono />
          )}
          {props.isAdmin && adminInfo?.peer_ip && (
            <DetailRow
              label={t('Peer IP')}
              value={String(adminInfo.peer_ip)}
              mono
            />
          )}
          {props.isAdmin && adminInfo?.cf_ray && (
            <DetailRow
              label={t('CF-Ray')}
              value={String(adminInfo.cf_ray)}
              mono
            />
          )}
          {props.log.request_id && (
            <DetailRow
              label={t('Request ID')}
              value={props.log.request_id}
              mono
            />
          )}
        </DetailSection>

        <DetailRow
          label={t('Account')}
          value={formatAccountLabel(props.log.username, accountId)}
          mono
        />

        <DetailRow
          label={t('Time')}
          value={formatTimestampToDate(props.log.created_at)}
          mono
        />

        <DetailSection
          icon={<ShieldAlert className='size-3.5' aria-hidden='true' />}
          label={t('User Agent')}
        >
          {userAgent ? (
            <div className='bg-background/60 max-h-40 min-w-0 overflow-x-hidden overflow-y-auto rounded border p-2'>
              <p className='font-mono text-[11px] leading-relaxed break-all whitespace-pre-wrap'>
                {userAgent}
              </p>
            </div>
          ) : (
            <span className='text-muted-foreground text-xs'>-</span>
          )}
        </DetailSection>
      </div>
    </Dialog>
  )
}
