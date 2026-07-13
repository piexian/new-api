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
import { useIsFetching, useQueryClient } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import type { Table } from '@tanstack/react-table'
import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTableToolbar } from '@/components/data-table'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { EMAIL_STATUS, EMAIL_STATUS_MAPPINGS } from '../constants'
import { buildSearchParams } from '../lib/filter'
import { getDefaultTimeRange } from '../lib/utils'
import type { EmailLogFilters } from '../types'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'

const route = getRouteApi('/_authenticated/usage-logs/$section')
const emailStatusValues = Object.values(EMAIL_STATUS) as string[]

function searchString(value: unknown) {
  if (value === undefined || value === null || value === '') return undefined
  return String(value)
}

function searchTimestamp(value: unknown) {
  if (value === undefined || value === null || value === '') return undefined
  const timestamp = Number(value)
  return Number.isFinite(timestamp) ? timestamp : undefined
}

function getInitialFilters(
  searchParams: Record<string, unknown>
): EmailLogFilters {
  const { start, end } = getDefaultTimeRange()
  const filters: EmailLogFilters = { startTime: start, endTime: end }

  const startTime = searchTimestamp(searchParams.startTime)
  const endTime = searchTimestamp(searchParams.endTime)
  if (startTime !== undefined) filters.startTime = new Date(startTime)
  if (endTime !== undefined) filters.endTime = new Date(endTime)

  const receiver = searchString(searchParams.receiver)
  const subject = searchString(searchParams.subject)
  const status = searchString(searchParams.status)
  const provider = searchString(searchParams.provider)

  if (receiver) filters.receiver = receiver
  if (subject) filters.subject = subject
  if (status && emailStatusValues.includes(status)) filters.status = status
  if (provider) filters.provider = provider

  return filters
}

interface EmailLogsFilterBarProps<TData> {
  table: Table<TData>
}

export function EmailLogsFilterBar<TData>(
  props: EmailLogsFilterBarProps<TData>
) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { startTime, endTime, receiver, subject, status, provider } =
    route.useSearch()
  const fetchingLogs = useIsFetching({ queryKey: ['logs'] })

  const [filters, setFilters] = useState<EmailLogFilters>(() =>
    getInitialFilters({
      startTime,
      endTime,
      receiver,
      subject,
      status,
      provider,
    })
  )

  useEffect(() => {
    setFilters(
      getInitialFilters({
        startTime,
        endTime,
        receiver,
        subject,
        status,
        provider,
      })
    )
  }, [startTime, endTime, receiver, subject, status, provider])

  const handleChange = useCallback(
    (field: keyof EmailLogFilters, value: Date | string | undefined) => {
      setFilters((prev) => ({ ...prev, [field]: value }))
    },
    []
  )

  const handleApply = useCallback(() => {
    const filterParams = buildSearchParams(filters, 'email')
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'email' },
      search: {
        ...filterParams,
        page: 1,
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
  }, [filters, navigate, queryClient])

  const handleReset = useCallback(() => {
    const { start, end } = getDefaultTimeRange()
    const resetFilters: EmailLogFilters = { startTime: start, endTime: end }
    setFilters(resetFilters)

    navigate({
      to: '/usage-logs/$section',
      params: { section: 'email' },
      search: {
        page: 1,
        startTime: start.getTime(),
        endTime: end.getTime(),
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
  }, [navigate, queryClient])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleApply()
    },
    [handleApply]
  )

  const inputClass = 'w-full sm:w-[150px] lg:w-[180px]'
  const hasExpandedFilters = !!filters.subject || !!filters.provider
  const hasAdditionalFilters =
    !!filters.receiver || !!filters.status || hasExpandedFilters

  return (
    <DataTableToolbar
      table={props.table}
      customSearch={
        <CompactDateTimeRangePicker
          start={filters.startTime}
          end={filters.endTime}
          onChange={({ start, end }) => {
            handleChange('startTime', start)
            handleChange('endTime', end)
          }}
          className='w-full sm:w-[340px]'
        />
      }
      additionalSearch={
        <>
          <Input
            placeholder={t('Receiver')}
            value={filters.receiver || ''}
            onChange={(e) => handleChange('receiver', e.target.value.trim())}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Select
            items={[
              { value: 'all', label: t('All Status') },
              ...emailStatusValues.map((status) => ({
                value: status,
                label: t(EMAIL_STATUS_MAPPINGS[status].label),
              })),
            ]}
            value={filters.status || 'all'}
            onValueChange={(value) => {
              handleChange(
                'status',
                value && value !== 'all' ? value : undefined
              )
            }}
          >
            <SelectTrigger className={inputClass}>
              <SelectValue placeholder={t('All Status')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='all'>{t('All Status')}</SelectItem>
                {emailStatusValues.map((status) => (
                  <SelectItem key={status} value={status}>
                    {t(EMAIL_STATUS_MAPPINGS[status].label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </>
      }
      expandable={
        <>
          <Input
            placeholder={t('Subject')}
            value={filters.subject || ''}
            onChange={(e) => handleChange('subject', e.target.value.trim())}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Input
            placeholder={t('Provider')}
            value={filters.provider || ''}
            onChange={(e) => handleChange('provider', e.target.value.trim())}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
        </>
      }
      hasExpandedActiveFilters={hasExpandedFilters}
      hasAdditionalFilters={hasAdditionalFilters}
      onSearch={handleApply}
      searchLoading={fetchingLogs > 0}
      onReset={handleReset}
    />
  )
}
