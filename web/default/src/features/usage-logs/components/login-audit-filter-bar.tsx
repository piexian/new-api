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
import { useQueryClient } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import type { Table } from '@tanstack/react-table'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  LOGIN_AUDIT_RESULT_FILTERS,
  type LoginAuditResultFilter,
} from '../constants'
import { getDefaultTimeRange } from '../lib/utils'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'
import {
  LogsFilterField,
  LogsFilterInput,
  LogsFilterToolbar,
} from './logs-filter-toolbar'
import { useLogsViewScope } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

type LoginAuditDraft = {
  sourceKey: string
  result: LoginAuditResultFilter
  startTime?: Date
  endTime?: Date
  ip: string
  requestId: string
  userAgent: string
  username: string
}

function getResultFilter(value: unknown): LoginAuditResultFilter {
  if (Array.isArray(value) && value.length === 1) {
    if (String(value[0]) === '7') return 'success'
    if (String(value[0]) === '8') return 'security'
  }
  return 'all'
}

function getResultType(result: LoginAuditResultFilter): '7' | '8' | undefined {
  return LOGIN_AUDIT_RESULT_FILTERS.find((item) => item.value === result)?.type
}

/** Security events are administrator-only; "only mine" collapses them to all. */
function resolveResultFilter(
  value: unknown,
  isAdmin: boolean
): LoginAuditResultFilter {
  const result = getResultFilter(value)
  return !isAdmin && result === 'security' ? 'all' : result
}

function buildSourceKey(values: {
  result: LoginAuditResultFilter
  startTime?: Date
  endTime?: Date
  ip: string
  requestId: string
  userAgent: string
  username: string
}): string {
  return [
    values.result,
    values.startTime?.getTime(),
    values.endTime?.getTime(),
    values.ip,
    values.requestId,
    values.userAgent,
    values.username,
  ]
    .map((value) => String(value ?? ''))
    .join('\u001f')
}

interface LoginAuditFilterBarProps<TData> {
  table: Table<TData>
}

/** Filter bar for the login audit scope, without consumption statistics. */
export function LoginAuditFilterBar<TData>(
  props: LoginAuditFilterBarProps<TData>
) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const searchParams = route.useSearch()
  const { isAdminView: isAdmin } = useLogsViewScope()

  const searchState = useMemo<LoginAuditDraft>(() => {
    const { start, end } = getDefaultTimeRange()
    const startTime = searchParams.startTime
      ? new Date(searchParams.startTime)
      : start
    const endTime = searchParams.endTime ? new Date(searchParams.endTime) : end

    const result = resolveResultFilter(searchParams.type, isAdmin)
    const ip = searchParams.ip || ''
    const requestId = searchParams.requestId || ''
    const userAgent = searchParams.userAgent || ''
    const username = searchParams.username || ''

    return {
      sourceKey: buildSourceKey({
        result,
        startTime,
        endTime,
        ip,
        requestId,
        userAgent,
        username,
      }),
      result,
      startTime,
      endTime,
      ip,
      requestId,
      userAgent,
      username,
    }
  }, [
    searchParams.startTime,
    searchParams.endTime,
    searchParams.type,
    searchParams.ip,
    searchParams.requestId,
    searchParams.userAgent,
    searchParams.username,
    isAdmin,
  ])

  const [draft, setDraft] = useState<LoginAuditDraft>(() => searchState)
  const activeDraft =
    draft.sourceKey === searchState.sourceKey ? draft : searchState

  // A type=8 filter left over from the admin view is invalid under "only mine":
  // drop it so the switch lands on the own-login list instead of an empty one.
  const hasResidualSecurityType =
    !isAdmin && getResultFilter(searchParams.type) === 'security'

  useEffect(() => {
    if (!hasResidualSecurityType) return
    const { type: _type, ...rest } = searchParams
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: { ...rest, page: 1 },
      replace: true,
    })
  }, [hasResidualSecurityType, navigate, searchParams])
  const handleChange = useCallback(
    (patch: Partial<Omit<LoginAuditDraft, 'sourceKey'>>) => {
      setDraft((current) => {
        const base =
          current.sourceKey === searchState.sourceKey ? current : searchState
        return { ...base, ...patch, sourceKey: searchState.sourceKey }
      })
    },
    [searchState]
  )

  const handleApply = useCallback(() => {
    const resultType = getResultType(activeDraft.result)
    const ip = activeDraft.ip.trim()
    const requestId = activeDraft.requestId.trim()
    const userAgent = activeDraft.userAgent.trim()
    const username = isAdmin ? activeDraft.username.trim() : ''

    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        scope: 'login',
        page: 1,
        ...(resultType ? { type: [resultType] } : {}),
        ...(activeDraft.startTime
          ? { startTime: activeDraft.startTime.getTime() }
          : {}),
        ...(activeDraft.endTime
          ? { endTime: activeDraft.endTime.getTime() }
          : {}),
        ...(ip ? { ip } : {}),
        ...(requestId ? { requestId } : {}),
        ...(userAgent ? { userAgent } : {}),
        ...(username ? { username } : {}),
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
  }, [activeDraft, isAdmin, navigate, queryClient])

  const handleReset = useCallback(() => {
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: { scope: 'login', page: 1 },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
  }, [navigate, queryClient])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleApply()
    },
    [handleApply]
  )

  // Security events are admin-only, so "only mine" lists just the success filter.
  const resultItems = useMemo(
    () =>
      LOGIN_AUDIT_RESULT_FILTERS.filter(
        (item) => isAdmin || item.value !== 'security'
      ).map((item) => ({
        value: item.value,
        label: t(item.label),
      })),
    [isAdmin, t]
  )
  const resultLabel =
    resultItems.find((item) => item.value === activeDraft.result)?.label ??
    t('All Logins')

  const dateRangeFilter = (
    <LogsFilterField wide>
      <CompactDateTimeRangePicker
        start={activeDraft.startTime}
        end={activeDraft.endTime}
        onChange={({ start, end }) => {
          handleChange({ startTime: start, endTime: end })
        }}
      />
    </LogsFilterField>
  )

  const resultFilter = isAdmin ? (
    <LogsFilterField>
      <Select
        items={resultItems}
        value={activeDraft.result}
        onValueChange={(value) => {
          const next = LOGIN_AUDIT_RESULT_FILTERS.find(
            (item) => item.value === value
          )
          handleChange({ result: next ? next.value : 'all' })
        }}
      >
        <SelectTrigger>
          <SelectValue>{resultLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {resultItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </LogsFilterField>
  ) : null

  const ipFilter = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('IP Address')}
        value={activeDraft.ip}
        onChange={(e) => handleChange({ ip: e.target.value.trim() })}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )

  const requestIdFilter = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('Request ID')}
        value={activeDraft.requestId}
        onChange={(e) => handleChange({ requestId: e.target.value })}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )

  const usernameFilter = isAdmin ? (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('Username')}
        value={activeDraft.username}
        onChange={(e) => handleChange({ username: e.target.value })}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  ) : null

  const userAgentFilter = (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('Search User Agent (fuzzy)')}
        value={activeDraft.userAgent}
        onChange={(e) => handleChange({ userAgent: e.target.value.trim() })}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )

  const hasActiveFilters =
    activeDraft.result !== 'all' ||
    activeDraft.ip !== '' ||
    activeDraft.requestId !== '' ||
    activeDraft.userAgent !== '' ||
    (isAdmin && activeDraft.username !== '')

  return (
    <LogsFilterToolbar
      table={props.table}
      primaryFilters={
        <>
          {dateRangeFilter}
          {resultFilter}
          {ipFilter}
          {requestIdFilter}
        </>
      }
      advancedFilters={
        <>
          {usernameFilter}
          {userAgentFilter}
        </>
      }
      mobilePinnedFilters={dateRangeFilter}
      mobileFilters={
        <>
          {resultFilter}
          {ipFilter}
          {requestIdFilter}
          {usernameFilter}
          {userAgentFilter}
        </>
      }
      mobileFilterCount={
        [resultFilter, activeDraft.ip, activeDraft.requestId].filter(Boolean)
          .length
      }
      hasAdvancedActiveFilters={
        activeDraft.userAgent !== '' || (isAdmin && activeDraft.username !== '')
      }
      hasActiveFilters={hasActiveFilters}
      onSearch={handleApply}
      onReset={handleReset}
    />
  )
}
