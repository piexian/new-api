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
 * Pure helpers for the login audit view (type=7 success / type=8 security
 * events). Every snake_case value from the API is mapped to a human readable
 * i18n key here so components never render raw identifiers.
 */
import type { StatusBadgeProps } from '@/components/status-badge'

import {
  LOG_TYPE_ENUM,
  LOGIN_AUDIT_LOG_TYPES,
  LOGIN_AUDIT_OUTCOME_MAPPINGS,
  LOGIN_AUDIT_REASON_LABELS,
  LOGIN_AUDIT_SECURITY_EVENT_OUTCOMES,
  LOGIN_AUDIT_STAGE_LABELS,
  LOGIN_AUDIT_UNKNOWN_OUTCOME,
  LOGIN_AUDIT_UNKNOWN_OUTCOME_LABEL,
  LOGIN_AUDIT_UNKNOWN_REASON_LABEL,
  LOGIN_AUDIT_UNKNOWN_STAGE_LABEL,
} from '../constants'
import type { UsageLog } from '../data/schema'
import type { LoginAuthAudit, LogOtherData } from '../types'

export function isLoginAuditLogType(type: number): boolean {
  return (LOGIN_AUDIT_LOG_TYPES as readonly number[]).includes(type)
}

export function getLoginAuthAudit(
  other: LogOtherData | null | undefined
): LoginAuthAudit | null {
  const audit = other?.auth_audit
  return audit && typeof audit === 'object' ? audit : null
}

/** Resolve the outcome: type=7 is always success, type=8 is a known security-event outcome or unknown. */
export function resolveLoginAuditOutcome(
  logType: number,
  audit: LoginAuthAudit | null | undefined
): string {
  if (logType === LOG_TYPE_ENUM.LOGIN) return 'success'

  const raw =
    typeof audit?.outcome === 'string' ? audit.outcome.trim().toLowerCase() : ''
  return (LOGIN_AUDIT_SECURITY_EVENT_OUTCOMES as readonly string[]).includes(
    raw
  )
    ? raw
    : LOGIN_AUDIT_UNKNOWN_OUTCOME
}

export function getLoginAuditOutcomeLabel(outcome: string): string {
  return (
    LOGIN_AUDIT_OUTCOME_MAPPINGS[outcome]?.label ??
    LOGIN_AUDIT_UNKNOWN_OUTCOME_LABEL
  )
}

export function getLoginAuditOutcomeVariant(
  outcome: string
): StatusBadgeProps['variant'] {
  return LOGIN_AUDIT_OUTCOME_MAPPINGS[outcome]?.variant ?? 'neutral'
}

export function getLoginAuditStageLabel(stage?: string): string | null {
  const raw = typeof stage === 'string' ? stage.trim() : ''
  if (!raw) return null
  return LOGIN_AUDIT_STAGE_LABELS[raw] ?? LOGIN_AUDIT_UNKNOWN_STAGE_LABEL
}

export function getLoginAuditReasonLabel(reason?: string): string | null {
  const raw = typeof reason === 'string' ? reason.trim() : ''
  if (!raw) return null
  return LOGIN_AUDIT_REASON_LABELS[raw] ?? LOGIN_AUDIT_UNKNOWN_REASON_LABEL
}

/** Aggregated attempt count; a single event is the default. */
export function getLoginAuditCount(
  audit: LoginAuthAudit | null | undefined
): number {
  const count = Number(audit?.count)
  if (!Number.isFinite(count) || count < 1) return 1
  return Math.floor(count)
}

/** Login method recorded in the log's `other` payload. */
export function getLoginAuditMethod(
  other: LogOtherData | null | undefined
): string {
  const method = other?.login_method
  return typeof method === 'string' ? method.trim() : ''
}

/** User agent for a login record: the log column first, then the `other` payload. */
export function getLoginAuditUserAgent(
  log: UsageLog,
  other?: LogOtherData | null
): string {
  const topLevel = (log.user_agent || '').trim()
  if (topLevel) return topLevel

  const fromOther = other?.user_agent
  return typeof fromOther === 'string' ? fromOther.trim() : ''
}

export interface LoginAuditTarget {
  username: string
  userId: string
}

/** Audited account of a security event, read from the admin-only `admin_info`. */
export function getLoginAuditTarget(
  other: LogOtherData | null | undefined
): LoginAuditTarget {
  const adminInfo = other?.admin_info
  const username =
    typeof adminInfo?.target_username === 'string'
      ? adminInfo.target_username.trim()
      : ''
  const attempted =
    typeof adminInfo?.attempted_identifier === 'string'
      ? adminInfo.attempted_identifier.trim()
      : ''
  const rawId = adminInfo?.target_user_id
  const userId =
    rawId != null && String(rawId).trim() !== '' ? String(rawId).trim() : ''

  return { username: username || attempted, userId }
}

/** Aggregation window of a blocked/rate-limited event, in seconds. */
export function getLoginAuditWindow(audit: LoginAuthAudit | null | undefined): {
  start?: number
  end?: number
} {
  const toTimestamp = (value: unknown) => {
    const num = Number(value)
    return Number.isFinite(num) && num > 0 ? num : undefined
  }
  return {
    start: toTimestamp(audit?.window_start),
    end: toTimestamp(audit?.window_end),
  }
}
