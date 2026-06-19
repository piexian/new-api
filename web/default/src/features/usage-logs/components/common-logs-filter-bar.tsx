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
import { useState, useCallback } from 'react'
import { useQueryClient, useIsFetching } from '@tanstack/react-query'
import { useNavigate, getRouteApi } from '@tanstack/react-router'
import { type Table } from '@tanstack/react-table'
import { Eye, EyeOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useIsAdmin } from '@/hooks/use-admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableToolbar } from '@/components/data-table'
import { LOG_TYPES } from '../constants'
import { buildSearchParams } from '../lib/filter'
import { getDefaultTimeRange } from '../lib/utils'
import type { CommonLogFilters } from '../types'
import { CommonLogsStats } from './common-logs-stats'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')
const logTypeValues = ['0', '1', '2', '3', '4', '5', '6'] as const

type LogTypeValue = (typeof logTypeValues)[number]

function isLogTypeValue(value: string): value is LogTypeValue {
  return (logTypeValues as readonly string[]).includes(value)
}

function searchString(value: unknown) {
  if (value === undefined || value === null || value === '') return undefined
  return String(value)
}

function searchTimestamp(value: unknown) {
  if (value === undefined || value === null || value === '') return undefined
  const timestamp = Number(value)
  return Number.isFinite(timestamp) ? timestamp : undefined
}

function getInitialFilters(searchParams: Record<string, unknown>) {
  const { start, end } = getDefaultTimeRange()
  const filters: CommonLogFilters = { startTime: start, endTime: end }

  const startTime = searchTimestamp(searchParams.startTime)
  const endTime = searchTimestamp(searchParams.endTime)
  if (startTime !== undefined) filters.startTime = new Date(startTime)
  if (endTime !== undefined) filters.endTime = new Date(endTime)

  const channel = searchString(searchParams.channel)
  const model = searchString(searchParams.model)
  const token = searchString(searchParams.token)
  const group = searchString(searchParams.group)
  const username = searchString(searchParams.username)
  const requestId = searchString(searchParams.requestId)
  const upstreamRequestId = searchString(searchParams.upstreamRequestId)
  const ip = searchString(searchParams.ip)
  const userAgent = searchString(searchParams.userAgent)

  if (channel) filters.channel = channel
  if (model) filters.model = model
  if (token) filters.token = token
  if (group) filters.group = group
  if (username) filters.username = username
  if (requestId) filters.requestId = requestId
  if (upstreamRequestId) filters.upstreamRequestId = upstreamRequestId
  if (ip) filters.ip = ip
  if (userAgent) filters.userAgent = userAgent

  return filters
}

function getInitialLogType(searchParams: Record<string, unknown>) {
  const typeArr = searchParams.type
  const value =
    Array.isArray(typeArr) && typeArr.length === 1 ? String(typeArr[0]) : ''
  return isLogTypeValue(value) ? value : ''
}

function getFilterSearchKey(searchParams: Record<string, unknown>) {
  const fields = [
    'startTime',
    'endTime',
    'channel',
    'model',
    'token',
    'group',
    'username',
    'requestId',
    'upstreamRequestId',
    'ip',
    'userAgent',
  ]
  const typeValue = Array.isArray(searchParams.type)
    ? searchParams.type.join(',')
    : ''

  return [
    ...fields.map(
      (field) => `${field}:${searchString(searchParams[field]) ?? ''}`
    ),
    `type:${typeValue}`,
  ].join('|')
}

interface CommonLogsFilterBarProps<TData> {
  table: Table<TData>
}

export function CommonLogsFilterBar<TData>(
  props: CommonLogsFilterBarProps<TData>
) {
  const searchParams = route.useSearch()

  return (
    <CommonLogsFilterBarContent
      key={getFilterSearchKey(searchParams)}
      table={props.table}
      initialFilters={getInitialFilters(searchParams)}
      initialLogType={getInitialLogType(searchParams)}
    />
  )
}

interface CommonLogsFilterBarContentProps<
  TData,
> extends CommonLogsFilterBarProps<TData> {
  initialFilters: CommonLogFilters
  initialLogType: LogTypeValue | ''
}

function CommonLogsFilterBarContent<TData>(
  props: CommonLogsFilterBarContentProps<TData>
) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const isAdmin = useIsAdmin()
  const { sensitiveVisible, setSensitiveVisible } = useUsageLogsContext()
  const fetchingLogs = useIsFetching({ queryKey: ['logs'] })

  const [filters, setFilters] = useState<CommonLogFilters>(
    () => props.initialFilters
  )
  const [logType, setLogType] = useState<LogTypeValue | ''>(
    () => props.initialLogType
  )

  const handleChange = useCallback(
    (field: keyof CommonLogFilters, value: Date | string | undefined) => {
      setFilters((prev) => ({ ...prev, [field]: value }))
    },
    []
  )

  const handleApply = useCallback(() => {
    const filterParams = buildSearchParams(filters, 'common')
    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        ...filterParams,
        ...(logType ? { type: [logType] } : {}),
        page: 1,
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
    queryClient.invalidateQueries({ queryKey: ['usage-logs-stats'] })
  }, [filters, logType, navigate, queryClient])

  const handleReset = useCallback(() => {
    const { start, end } = getDefaultTimeRange()
    const resetFilters: CommonLogFilters = { startTime: start, endTime: end }
    setFilters(resetFilters)
    setLogType('')

    navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        page: 1,
        startTime: start.getTime(),
        endTime: end.getTime(),
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
    queryClient.invalidateQueries({ queryKey: ['usage-logs-stats'] })
  }, [navigate, queryClient])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleApply()
    },
    [handleApply]
  )

  const hasExpandedFilters =
    !!filters.token ||
    !!filters.username ||
    !!filters.channel ||
    !!filters.requestId ||
    !!filters.upstreamRequestId ||
    !!filters.ip ||
    !!filters.userAgent

  const hasAdditionalFilters =
    !!filters.model || !!filters.group || !!logType || hasExpandedFilters

  const inputClass = 'w-full sm:w-[140px] lg:w-[160px]'
  const sensitiveType = sensitiveVisible ? 'text' : 'password'

  const statsBar = (
    <div className='flex flex-wrap items-center gap-2'>
      <CommonLogsStats />
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon'
              onClick={() => setSensitiveVisible(!sensitiveVisible)}
              aria-label={sensitiveVisible ? t('Hide') : t('Show')}
              className='text-muted-foreground hover:text-foreground size-7'
            />
          }
        >
          {sensitiveVisible ? <Eye /> : <EyeOff />}
        </TooltipTrigger>
        <TooltipContent>
          {sensitiveVisible ? t('Hide') : t('Show')}
        </TooltipContent>
      </Tooltip>
    </div>
  )

  return (
    <DataTableToolbar
      table={props.table}
      leftActions={statsBar}
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
            placeholder={t('Model Name')}
            value={filters.model || ''}
            onChange={(e) => handleChange('model', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Input
            placeholder={t('Group')}
            type={sensitiveType}
            value={filters.group || ''}
            onChange={(e) => handleChange('group', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Select
            items={[
              { value: 'all', label: t('All Types') },
              ...LOG_TYPES.map((type) => ({
                value: String(type.value),
                label: t(type.label),
              })),
            ]}
            value={logType}
            onValueChange={(value) => {
              setLogType(value !== null && isLogTypeValue(value) ? value : '')
            }}
          >
            <SelectTrigger className={inputClass}>
              <SelectValue placeholder={t('All Types')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='all'>{t('All Types')}</SelectItem>
                {LOG_TYPES.map((type) => (
                  <SelectItem key={type.value} value={String(type.value)}>
                    {t(type.label)}
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
            placeholder={t('Token Name')}
            type={sensitiveType}
            value={filters.token || ''}
            onChange={(e) => handleChange('token', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          {isAdmin && (
            <Input
              placeholder={t('Username')}
              type={sensitiveType}
              value={filters.username || ''}
              onChange={(e) => handleChange('username', e.target.value)}
              onKeyDown={handleKeyDown}
              className={inputClass}
            />
          )}
          {isAdmin && (
            <Input
              placeholder={t('Channel ID')}
              value={filters.channel || ''}
              onChange={(e) => handleChange('channel', e.target.value)}
              onKeyDown={handleKeyDown}
              className={inputClass}
            />
          )}
          <Input
            placeholder={t('Request ID')}
            value={filters.requestId || ''}
            onChange={(e) => handleChange('requestId', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Input
            placeholder={t('Upstream Request ID')}
            value={filters.upstreamRequestId || ''}
            onChange={(e) => handleChange('upstreamRequestId', e.target.value)}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Input
            placeholder={t('IP Address')}
            value={filters.ip || ''}
            onChange={(e) => handleChange('ip', e.target.value.trim())}
            onKeyDown={handleKeyDown}
            className={inputClass}
          />
          <Input
            placeholder={t('Search User Agent (fuzzy)')}
            value={filters.userAgent || ''}
            onChange={(e) => handleChange('userAgent', e.target.value.trim())}
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
