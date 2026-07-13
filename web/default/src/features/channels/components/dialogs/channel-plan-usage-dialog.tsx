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
import {
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Copy,
  Loader2,
  RefreshCw,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import dayjs from '@/lib/dayjs'

import type { ChannelPlanUsageResponse } from '../../api'
import type { Channel } from '../../types'

export type ChannelPlanUsageKind = 'minimax' | 'zhipu' | 'kimi'

type ChannelPlanUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  kind: ChannelPlanUsageKind
  channel?: Pick<Channel, 'id' | 'name' | 'base_url'> | null
  response: ChannelPlanUsageResponse | null
  currentKeyIndex: number
  onKeyIndexChange: (keyIndex: number) => void
  onRefresh: (keyIndex: number) => void
  isRefreshing?: boolean
}

type UsageWindow = {
  key: string
  label: string
  total: number | null
  used: number | null
  remaining: number | null
  isUnlimited?: boolean
  percent: number
  remainingPercent: number | null
  status: number | null
  remainsTime: number | null
  startTime: number | null
  endTime: number | null
}

type MiniMaxModelRemain = Record<string, unknown>

const INFINITE_QUOTA_LABEL = '∞'

type ZhipuLimitCard = {
  key: string
  title: string
  percentage: number
  usageLabel: string | null
  nextResetTime: string
  details: { key: string; name: string; usage: number | null }[]
}

type KimiUsageRow = {
  key: string
  label: string
  used: number
  limit: number
  remaining: number
  percent: number
  resetHint: string | null
}

const TOOL_NAME_MAP: Record<string, string> = {
  'search-prime': 'Web Search',
  'web-reader': 'Web Reader',
  zread: 'Open Repository',
}

function toRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function isRecordResult(
  value: Record<string, unknown> | null
): value is Record<string, unknown> {
  return value !== null
}

function toNumber(value: unknown): number | null {
  const numericValue = Number(value)
  return Number.isFinite(numericValue) ? numericValue : null
}

function toOptionalNumber(value: unknown): number | null {
  if (value == null || value === '') return null
  return toNumber(value)
}

function clampPercent(value: unknown): number {
  const numericValue = Number(value)
  if (!Number.isFinite(numericValue)) return 0
  return Math.max(0, Math.min(100, numericValue))
}

function formatCount(value: unknown): string {
  if (value == null || value === '') return '-'
  const numericValue = toNumber(value)
  if (numericValue == null) return '-'
  return numericValue.toLocaleString()
}

function formatWindowCount(value: unknown, isUnlimited?: boolean): string {
  if (isUnlimited) return INFINITE_QUOTA_LABEL
  return formatCount(value)
}

function formatPercent(value: unknown): string {
  if (value == null || value === '') return '-'
  const numericValue = toNumber(value)
  if (numericValue == null) return '-'
  return `${Math.floor(clampPercent(numericValue))}%`
}

function normalizeEpochMs(value: unknown): number | null {
  const numericValue = toNumber(value)
  if (numericValue == null || numericValue <= 0) return null
  const absValue = Math.abs(numericValue)
  if (absValue >= 1e18) return Math.floor(numericValue / 1e6)
  if (absValue >= 1e15) return Math.floor(numericValue / 1e3)
  if (absValue >= 1e12) return Math.floor(numericValue)
  return Math.floor(numericValue * 1000)
}

function parseResetTime(value: unknown): number | null {
  if (value == null || value === '') return null
  if (typeof value === 'number') return normalizeEpochMs(value)

  const text = String(value).trim()
  if (!text) return null
  if (/^-?\d+$/.test(text)) return normalizeEpochMs(text)

  const parsed = Date.parse(text)
  return Number.isNaN(parsed) ? null : parsed
}

function formatDateTime(value: unknown): string {
  const epochMs = normalizeEpochMs(value)
  if (epochMs == null) return value ? String(value) : '-'
  return dayjs(epochMs).format('YYYY-MM-DD HH:mm:ss')
}

function formatResetTime(value: unknown, t: (key: string) => string): string {
  const epochMs = parseResetTime(value)
  if (epochMs == null) return value ? String(value) : t('Unknown')
  return dayjs(epochMs).format('YYYY-MM-DD HH:mm:ss')
}

function formatDurationMs(value: unknown, t: (key: string) => string): string {
  const numericValue = toNumber(value)
  if (numericValue == null || numericValue <= 0) return '-'

  const totalSeconds = Math.floor(numericValue / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  if (hours > 0) return `${hours}${t('h')} ${minutes}${t('m')}`
  if (minutes > 0) return `${minutes}${t('m')} ${seconds}${t('s')}`
  return `${seconds}${t('s')}`
}

function getProgressVariant(percent: number): StatusBadgeProps['variant'] {
  if (percent >= 95) return 'danger'
  if (percent >= 80) return 'warning'
  return 'info'
}

function getRemainingVariant(
  percent: number | null
): StatusBadgeProps['variant'] {
  if (percent == null) return 'neutral'
  if (percent <= 5) return 'danger'
  if (percent <= 20) return 'warning'
  return 'success'
}

function getWindowStatusVariant(
  status: number | null
): StatusBadgeProps['variant'] {
  if (status == null) return 'neutral'
  if (status === 3) return 'purple'
  return status === 1 ? 'success' : 'warning'
}

function formatWindowStatus(
  status: number | null,
  t: (key: string) => string
): string {
  if (status == null) return '-'
  if (status === 3) return `${status} (${INFINITE_QUOTA_LABEL})`
  return status === 1 ? `${status} (${t('Normal')})` : String(status)
}

function isMiniMaxUnlimitedWeeklyLimit(item: MiniMaxModelRemain): boolean {
  const weeklyStatus = toNumber(item.current_weekly_status)
  if (weeklyStatus === 3) return true

  const total = toNumber(item.current_weekly_total_count)
  const used = toNumber(item.current_weekly_usage_count)
  const remaining = toNumber(item.current_weekly_remaining_count)
  const remainingPercent = toNumber(item.current_weekly_remaining_percent)
  const hasWeeklyWindow =
    toNumber(item.weekly_remains_time) != null ||
    normalizeEpochMs(item.weekly_start_time) != null ||
    normalizeEpochMs(item.weekly_end_time) != null

  return (
    hasWeeklyWindow &&
    total === 0 &&
    (used == null || used === 0) &&
    (remaining == null || remaining === 0) &&
    remainingPercent === 100
  )
}

function resolveWindowQuota(input: {
  total: unknown
  remaining: unknown
  upstreamUsageCount: unknown
}): { total: number | null; used: number | null; remaining: number | null } {
  const total = toNumber(input.total)
  const usedCount = toNumber(input.upstreamUsageCount)
  const remainingCount = toNumber(input.remaining)
  const used =
    usedCount ??
    (total != null && remainingCount != null
      ? Math.max(total - remainingCount, 0)
      : null)
  const remaining =
    remainingCount ??
    (total != null && usedCount != null ? Math.max(total - usedCount, 0) : null)

  return { total, used, remaining }
}

function buildWindow(input: {
  key: string
  label: string
  total: number | null
  used: number | null
  remaining: number | null
  remainingPercent: number | null
  status: number | null
  remainsTime: number | null
  startTime: number | null
  endTime: number | null
  isUnlimited?: boolean
}): UsageWindow | null {
  if (
    !input.isUnlimited &&
    input.total == null &&
    input.used == null &&
    input.remaining == null &&
    input.remainingPercent == null &&
    input.status == null &&
    input.remainsTime == null &&
    input.startTime == null &&
    input.endTime == null
  ) {
    return null
  }

  const remaining =
    input.remaining ??
    (input.total != null && input.used != null
      ? Math.max(input.total - input.used, 0)
      : null)
  const hasQuota =
    Boolean(input.isUnlimited) ||
    (input.total != null && input.total > 0) ||
    (input.used != null && input.used > 0) ||
    (remaining != null && remaining > 0)

  if (!hasQuota && input.remainingPercent == null && input.status == null) {
    return null
  }

  const remainingPercent =
    input.remainingPercent ??
    (input.total != null && input.total > 0 && remaining != null
      ? Math.floor(clampPercent((remaining / input.total) * 100))
      : null)
  let percent = 0
  if (input.total != null && input.total > 0 && input.used != null) {
    percent = Math.floor(clampPercent((input.used / input.total) * 100))
  } else if (remainingPercent != null) {
    percent = Math.floor(clampPercent(100 - remainingPercent))
  }
  const total = input.isUnlimited || !hasQuota ? null : input.total
  const normalizedRemaining = input.isUnlimited || !hasQuota ? null : remaining

  return {
    ...input,
    total,
    used: hasQuota ? input.used : null,
    remaining: normalizedRemaining,
    isUnlimited: Boolean(input.isUnlimited),
    remainingPercent,
    percent,
  }
}

function resolveCurrentWindowLabel(
  startTime: number | null,
  endTime: number | null,
  t: (key: string) => string
): string {
  if (startTime == null || endTime == null || endTime <= startTime) {
    return t('Current Window')
  }

  const durationMs = endTime - startTime
  const fiveHoursMs = 5 * 60 * 60 * 1000
  const oneDayMs = 24 * 60 * 60 * 1000

  if (Math.abs(durationMs - fiveHoursMs) <= 30 * 60 * 1000) {
    return t('5-Hour Window')
  }
  if (Math.abs(durationMs - oneDayMs) <= 2 * 60 * 60 * 1000) {
    return t('Daily Quota')
  }
  return t('Current Window')
}

function resolveModelWindows(
  item: MiniMaxModelRemain,
  t: (key: string) => string
): UsageWindow[] {
  const currentStartTime = normalizeEpochMs(item.start_time)
  const currentEndTime = normalizeEpochMs(item.end_time)
  const weeklyStartTime = normalizeEpochMs(item.weekly_start_time)
  const weeklyEndTime = normalizeEpochMs(item.weekly_end_time)
  const currentIntervalQuota = resolveWindowQuota({
    total: item.current_interval_total_count,
    remaining: item.current_interval_remaining_count,
    upstreamUsageCount: item.current_interval_usage_count,
  })
  const currentWeeklyQuota = resolveWindowQuota({
    total: item.current_weekly_total_count,
    remaining: item.current_weekly_remaining_count,
    upstreamUsageCount: item.current_weekly_usage_count,
  })
  const hasUnlimitedWeeklyLimit = isMiniMaxUnlimitedWeeklyLimit(item)

  return [
    buildWindow({
      key: 'current_interval',
      label: resolveCurrentWindowLabel(currentStartTime, currentEndTime, t),
      total: currentIntervalQuota.total,
      used: currentIntervalQuota.used,
      remaining: currentIntervalQuota.remaining,
      remainingPercent: toOptionalNumber(
        item.current_interval_remaining_percent
      ),
      status: toOptionalNumber(item.current_interval_status),
      remainsTime: toNumber(item.remains_time),
      startTime: currentStartTime,
      endTime: currentEndTime,
    }),
    buildWindow({
      key: 'current_weekly',
      label: t('Weekly Quota'),
      total: currentWeeklyQuota.total,
      used: currentWeeklyQuota.used,
      remaining: currentWeeklyQuota.remaining,
      remainingPercent:
        toOptionalNumber(item.current_weekly_remaining_percent) ??
        (hasUnlimitedWeeklyLimit ? 100 : null),
      status: toOptionalNumber(item.current_weekly_status),
      remainsTime: toNumber(item.weekly_remains_time),
      startTime: weeklyStartTime,
      endTime: weeklyEndTime,
      isUnlimited: hasUnlimitedWeeklyLimit,
    }),
  ].filter(Boolean) as UsageWindow[]
}

function getZhipuCodingPlanSource(
  response: ChannelPlanUsageResponse | null
): Record<string, unknown> | null {
  const upstream = toRecord(response?.data)
  if (!upstream) return null
  return toRecord(upstream.data) ?? upstream
}

function resolveZhipuLimitTitle(
  item: Record<string, unknown>,
  t: (key: string) => string
): string {
  if (isZhipuTimeLimit(item)) return t('MCP Tool Quota')
  if (isZhipuWeeklyLimit(item)) return t('Weekly Quota')
  if (isZhipuTokenLimit(item)) return t('5-Hour Quota')
  return item.type ? String(item.type) : t('Quota Window')
}

function isZhipuTokenLimit(item: Record<string, unknown>): boolean {
  const type = String(item.type || '')
  return /token.*5\s*hour/i.test(type) || type === 'TOKENS_LIMIT'
}

function isZhipuTimeLimit(item: Record<string, unknown>): boolean {
  const type = String(item.type || '')
  return type === 'TIME_LIMIT' || /mcp.*1\s*month/i.test(type)
}

function isZhipuWeeklyLimit(item: Record<string, unknown>): boolean {
  return String(item.type || '') === 'TOKENS_LIMIT' && Number(item.unit) === 6
}

function formatPlanLevel(value: unknown, t: (key: string) => string): string {
  const text = String(value || '').trim()
  if (!text) return t('Unknown')
  if (/^[a-z0-9_-]+$/i.test(text)) {
    return text
      .split(/[_-]+/)
      .filter(Boolean)
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join(' ')
  }
  return text
}

function normalizeZhipuLimitCards(
  response: ChannelPlanUsageResponse | null,
  t: (key: string) => string
): ZhipuLimitCard[] {
  const source = getZhipuCodingPlanSource(response)
  const limits = Array.isArray(source?.limits) ? source.limits : []
  const orderWeight = (item: Record<string, unknown>) => {
    if (isZhipuTokenLimit(item) && !isZhipuWeeklyLimit(item)) return 1
    if (isZhipuWeeklyLimit(item)) return 2
    if (isZhipuTimeLimit(item)) return 3
    return 9
  }

  return limits
    .map((item) => toRecord(item))
    .filter(isRecordResult)
    .sort((left, right) => orderWeight(left) - orderWeight(right))
    .map((item, index) => {
      const currentValue = toNumber(item.currentValue ?? item.currentUsage)
      const totalValue = toNumber(item.usage ?? item.totol ?? item.total)
      const usageLabel =
        currentValue != null && totalValue != null
          ? `${currentValue.toLocaleString()}/${totalValue.toLocaleString()}`
          : null
      const details = Array.isArray(item.usageDetails)
        ? item.usageDetails
            .map((detail, detailIndex) => {
              const detailRecord = toRecord(detail)
              if (!detailRecord) return null
              const modelCode = String(detailRecord.modelCode || '')
              return {
                key: `${modelCode || 'detail'}-${detailIndex}`,
                name:
                  TOOL_NAME_MAP[modelCode] || modelCode || t('Unknown Tool'),
                usage: toNumber(detailRecord.usage),
              }
            })
            .filter(
              (
                detail
              ): detail is {
                key: string
                name: string
                usage: number | null
              } => detail !== null
            )
        : []

      return {
        key: `${String(item.type || 'limit')}-${String(item.unit || 0)}-${index}`,
        title: resolveZhipuLimitTitle(item, t),
        percentage:
          item.percentage == null && currentValue != null && totalValue
            ? clampPercent((currentValue / totalValue) * 100)
            : clampPercent(item.percentage),
        usageLabel,
        nextResetTime: formatResetTime(item.nextResetTime, t),
        details: details as ZhipuLimitCard['details'],
      }
    })
}

function getPlanRegionLabel(
  channel: Pick<Channel, 'base_url'> | null | undefined,
  t: (key: string) => string
): string {
  const baseURL = String(channel?.base_url || '').trim()
  if (
    baseURL === 'glm-coding-plan-international' ||
    baseURL.includes('api.z.ai')
  ) {
    return t('International')
  }
  if (baseURL === 'glm-coding-plan' || baseURL.includes('bigmodel.cn')) {
    return t('Domestic')
  }
  return t('Unknown')
}

function MetaBlock({
  label,
  value,
}: {
  label: string
  value: React.ReactNode
}) {
  return (
    <div className='bg-muted/30 min-w-0 rounded-lg border p-3'>
      <div className='text-muted-foreground mb-1 text-xs'>{label}</div>
      <div className='min-w-0 text-sm break-all'>{value || '-'}</div>
    </div>
  )
}

function UsageWindowCard({ windowInfo }: { windowInfo: UsageWindow }) {
  const { t } = useTranslation()
  const isUnlimited = windowInfo.isUnlimited === true
  const variant = getProgressVariant(windowInfo.percent)
  const remainingVariant = getRemainingVariant(windowInfo.remainingPercent)
  const statusVariant = getWindowStatusVariant(windowInfo.status)

  return (
    <div className='rounded-lg border p-3'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-sm font-medium'>{windowInfo.label}</div>
        <div className='flex flex-wrap items-center justify-end gap-2'>
          <StatusBadge
            label={`${t('Used')}: ${formatPercent(windowInfo.percent)}`}
            variant={variant}
            copyable={false}
          />
          {windowInfo.remainingPercent != null && (
            <StatusBadge
              label={`${t('Remaining')}: ${
                isUnlimited
                  ? INFINITE_QUOTA_LABEL
                  : formatPercent(windowInfo.remainingPercent)
              }`}
              variant={isUnlimited ? 'purple' : remainingVariant}
              copyable={false}
            />
          )}
          {windowInfo.status != null && (
            <StatusBadge
              label={`${t('Status')}: ${formatWindowStatus(
                windowInfo.status,
                t
              )}`}
              variant={statusVariant}
              copyable={false}
            />
          )}
        </div>
      </div>
      {isUnlimited ? (
        <div className='relative mt-3 h-4 overflow-hidden rounded-full bg-purple-100 dark:bg-purple-950/40'>
          <div className='absolute inset-0 rounded-full bg-purple-500' />
          <div className='absolute inset-0 flex items-center justify-center text-xs leading-none font-semibold text-white'>
            {INFINITE_QUOTA_LABEL}
          </div>
        </div>
      ) : (
        <div className='mt-3'>
          <Progress value={windowInfo.percent} />
        </div>
      )}
      <div className='text-muted-foreground mt-2 grid gap-1 text-xs sm:grid-cols-2'>
        <div>
          {t('Used')}: {formatCount(windowInfo.used)}
        </div>
        <div>
          {t('Remaining')}:{' '}
          {formatWindowCount(windowInfo.remaining, isUnlimited)}
          {!isUnlimited && windowInfo.remainingPercent != null
            ? ` (${formatPercent(windowInfo.remainingPercent)})`
            : ''}
        </div>
        <div>
          {t('Total')}: {formatWindowCount(windowInfo.total, isUnlimited)}
        </div>
        {windowInfo.status != null && (
          <div>
            {t('Status')}: {formatWindowStatus(windowInfo.status, t)}
          </div>
        )}
        <div>
          {t('Resets in:')} {formatDurationMs(windowInfo.remainsTime, t)}
        </div>
        <div>
          {t('Start')}: {formatDateTime(windowInfo.startTime)}
        </div>
        <div>
          {t('End')}: {formatDateTime(windowInfo.endTime)}
        </div>
      </div>
    </div>
  )
}

function MiniMaxModelCard({
  item,
  windows,
}: {
  item: MiniMaxModelRemain
  windows: UsageWindow[]
}) {
  const { t } = useTranslation()
  const modelName = String(item.model_name || item.model || t('Unnamed Model'))

  return (
    <div className='rounded-lg border p-4'>
      <div className='mb-3 flex flex-wrap items-center justify-between gap-2'>
        <div className='min-w-0 text-sm font-semibold break-all'>
          {modelName}
        </div>
        <StatusBadge label={t('Token Plan')} variant='cyan' copyable={false} />
      </div>
      <div className='grid gap-3 md:grid-cols-2'>
        {windows.map((windowInfo) => (
          <UsageWindowCard key={windowInfo.key} windowInfo={windowInfo} />
        ))}
      </div>
    </div>
  )
}

function MiniMaxUsageView({
  response,
  onRefresh,
  isRefreshing,
}: {
  response: ChannelPlanUsageResponse | null
  onRefresh: () => void
  isRefreshing?: boolean
}) {
  const { t } = useTranslation()
  const upstreamData = toRecord(response?.data)
  const rawModelRemains = upstreamData?.model_remains
  const parsedModels = useMemo(() => {
    const modelRemains = Array.isArray(rawModelRemains) ? rawModelRemains : []
    return modelRemains
      .map((item) => toRecord(item))
      .filter(isRecordResult)
      .map((item) => ({
        item,
        windows: resolveModelWindows(item, t),
      }))
  }, [rawModelRemains, t])
  const activeModels = parsedModels.filter((entry) => entry.windows.length > 0)
  const unavailableModels = parsedModels
    .filter((entry) => entry.windows.length === 0)
    .map((entry) => String(entry.item.model_name || entry.item.model || ''))
    .filter(Boolean)
  const hasUnlimitedWeeklyLimit = parsedModels.some((entry) =>
    isMiniMaxUnlimitedWeeklyLimit(entry.item)
  )
  const baseResp = toRecord(upstreamData?.base_resp)
  const shouldShowEmptyState = response != null && response.success !== false
  let businessStatus = t('Unknown')
  if (baseResp?.status_code === 0) {
    businessStatus = t('Normal')
  } else if (baseResp?.status_code != null) {
    businessStatus = String(baseResp.status_code)
  }
  let planLabel = '-'
  if (parsedModels.length > 0) {
    planLabel = hasUnlimitedWeeklyLimit ? t('Old Plan') : t('New Plan')
  }

  return (
    <div className='space-y-4'>
      {response?.success === false && (
        <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
          {response.message || t('Failed to fetch Token Plan usage')}
        </div>
      )}

      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
        <MetaBlock
          label={t('Upstream Status')}
          value={response?.upstream_status ?? '-'}
        />
        <MetaBlock label={t('Request URL')} value={response?.request_url} />
        <MetaBlock label={t('Business Status')} value={businessStatus} />
        <MetaBlock
          label={t('Business Message')}
          value={String(baseResp?.status_msg || response?.message || '-')}
        />
        <MetaBlock label={t('Plan')} value={planLabel} />
      </div>

      <div className='space-y-3'>
        {activeModels.length > 0 &&
          activeModels.map(({ item, windows }) => (
            <MiniMaxModelCard
              key={String(
                item.model_name || item.model || JSON.stringify(item)
              )}
              item={item}
              windows={windows}
            />
          ))}
        {activeModels.length === 0 && shouldShowEmptyState && (
          <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-sm'>
            {t(
              'No parsed quota windows were found. The key may not be a plan key, or the upstream response format changed.'
            )}
          </div>
        )}
        {unavailableModels.length > 0 && (
          <div className='rounded-lg border border-dashed p-4'>
            <div className='mb-2 text-sm font-medium'>
              {t('Models not included in the current plan')}
            </div>
            <div className='flex flex-wrap gap-2'>
              {unavailableModels.map((modelName) => (
                <StatusBadge
                  key={modelName}
                  label={modelName}
                  variant='neutral'
                  copyable={false}
                />
              ))}
            </div>
          </div>
        )}
      </div>

      <div className='flex justify-end'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={onRefresh}
          disabled={Boolean(isRefreshing)}
        >
          <RefreshCw className='mr-1.5 h-3.5 w-3.5' />
          {t('Refresh')}
        </Button>
      </div>
    </div>
  )
}

function ZhipuLimitCard({ card }: { card: ZhipuLimitCard }) {
  const { t } = useTranslation()
  const variant = getProgressVariant(card.percentage)

  return (
    <div className='rounded-lg border p-3'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-sm font-medium'>{card.title}</div>
        <StatusBadge
          label={`${Math.floor(card.percentage)}%`}
          variant={variant}
          copyable={false}
        />
      </div>
      <div className='text-muted-foreground mt-2 flex items-center justify-between gap-3 text-xs'>
        <span>{t('Current Usage')}</span>
        <span>{card.usageLabel || '-'}</span>
      </div>
      <Progress value={Math.floor(card.percentage)} className='mt-3' />
      <div className='text-muted-foreground mt-2 text-xs'>
        {t('Reset Time')}: {card.nextResetTime}
      </div>
      {card.details.length > 0 && (
        <div className='mt-3 space-y-1 border-t pt-2'>
          {card.details.map((detail) => (
            <div
              key={detail.key}
              className='flex items-center justify-between gap-2 text-xs'
            >
              <span className='text-muted-foreground min-w-0 truncate'>
                {detail.name}
              </span>
              <span>{formatCount(detail.usage)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function ZhipuUsageView({
  channel,
  response,
  onRefresh,
  isRefreshing,
}: {
  channel?: Pick<Channel, 'base_url'> | null
  response: ChannelPlanUsageResponse | null
  onRefresh: () => void
  isRefreshing?: boolean
}) {
  const { t } = useTranslation()
  const source = getZhipuCodingPlanSource(response)
  const cards = useMemo(
    () => normalizeZhipuLimitCards(response, t),
    [response, t]
  )
  const hasWeeklyLimit = cards.some((card) => card.title === t('Weekly Quota'))
  const shouldShowEmptyState = response != null && response.success !== false

  return (
    <div className='space-y-4'>
      {response?.success === false && (
        <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
          {response.message || t('Failed to fetch Coding Plan usage')}
        </div>
      )}

      <div className='rounded-lg border p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <StatusBadge
              label={getPlanRegionLabel(channel, t)}
              variant='blue'
              copyable={false}
            />
            <StatusBadge
              label={`${t('Plan Level')}: ${formatPlanLevel(source?.level, t)}`}
              variant='cyan'
              copyable={false}
            />
            <StatusBadge
              label={hasWeeklyLimit ? t('New Plan') : t('Old Plan')}
              variant={hasWeeklyLimit ? 'success' : 'warning'}
              copyable={false}
            />
            {typeof response?.upstream_status === 'number' && (
              <StatusBadge
                label={`${t('Upstream Status')}: ${response.upstream_status}`}
                variant='neutral'
                copyable={false}
              />
            )}
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={onRefresh}
            disabled={Boolean(isRefreshing)}
          >
            <RefreshCw className='mr-1.5 h-3.5 w-3.5' />
            {t('Refresh')}
          </Button>
        </div>
        <div className='text-muted-foreground mt-2 text-xs break-all'>
          {t('Request URL')}: {response?.request_url || '-'}
        </div>
      </div>

      {cards.length > 0 && (
        <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
          {cards.map((card) => (
            <ZhipuLimitCard key={card.key} card={card} />
          ))}
        </div>
      )}
      {cards.length === 0 && shouldShowEmptyState && (
        <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-sm'>
          {t(
            'No parsed quota windows were found. The key may not be a plan key, or the upstream response format changed.'
          )}
        </div>
      )}
    </div>
  )
}

function getKimiCodingPlanSource(
  response: ChannelPlanUsageResponse | null
): Record<string, unknown> | null {
  return toRecord(response?.data)
}

function formatKimiResetHint(
  detail: Record<string, unknown>,
  t: (key: string) => string
): string | null {
  const resetCandidate =
    detail.resetAt ?? detail.reset_at ?? detail.resetTime ?? detail.reset_time
  if (resetCandidate != null && resetCandidate !== '') {
    const epochMs = parseResetTime(resetCandidate)
    if (epochMs != null) {
      const deltaMs = epochMs - Date.now()
      if (deltaMs > 0) {
        return `${t('Resets in')} ${formatDurationMs(deltaMs, t)}`
      }
      return t('Reset')
    }
    return `${t('Reset Time')}: ${String(resetCandidate)}`
  }

  for (const key of ['reset_in', 'resetIn', 'ttl']) {
    const seconds = toNumber(detail[key])
    if (seconds != null && seconds > 0) {
      return `${t('Resets in')} ${formatDurationMs(seconds * 1000, t)}`
    }
  }
  return null
}

function formatKimiLimitLabel(
  item: Record<string, unknown>,
  detail: Record<string, unknown>,
  windowInfo: Record<string, unknown>,
  index: number,
  t: (key: string) => string
): string {
  for (const key of ['name', 'title', 'scope']) {
    const value = item[key] ?? detail[key]
    if (value != null && String(value).trim() !== '') {
      return String(value)
    }
  }

  const duration = toNumber(
    windowInfo.duration ?? item.duration ?? detail.duration
  )
  const timeUnit = String(
    windowInfo.timeUnit ?? item.timeUnit ?? detail.timeUnit ?? ''
  ).toUpperCase()

  if (duration && duration > 0) {
    if (timeUnit.includes('MINUTE')) {
      if (duration >= 60 && duration % 60 === 0) {
        return `${duration / 60}h ${t('Limit')}`
      }
      return `${duration}m ${t('Limit')}`
    }
    if (timeUnit.includes('HOUR')) return `${duration}h ${t('Limit')}`
    if (timeUnit.includes('DAY')) return `${duration}d ${t('Limit')}`
    return `${duration}s ${t('Limit')}`
  }

  return `${t('Limit')} #${index + 1}`
}

function buildKimiUsageRow(
  data: Record<string, unknown>,
  defaultLabel: string,
  rowKey: string,
  resetHint: string | null
): KimiUsageRow | null {
  const limit = toNumber(data.limit) ?? 0
  let used = toNumber(data.used)
  if (used == null) {
    const remaining = toNumber(data.remaining)
    if (remaining != null && limit > 0) {
      used = Math.max(limit - remaining, 0)
    }
  }
  if (used == null && limit <= 0) return null

  const usedSafe = used ?? 0
  const remaining = limit > 0 ? Math.max(limit - usedSafe, 0) : 0
  const percent =
    limit > 0 ? Math.floor(clampPercent((usedSafe / limit) * 100)) : 0
  const labelRaw = data.name ?? data.title
  const label =
    labelRaw != null && String(labelRaw).trim() !== ''
      ? String(labelRaw)
      : defaultLabel

  return {
    key: rowKey,
    label,
    used: usedSafe,
    limit,
    remaining,
    percent,
    resetHint,
  }
}

function normalizeKimiUsageRows(
  response: ChannelPlanUsageResponse | null,
  t: (key: string) => string
): { summary: KimiUsageRow | null; limits: KimiUsageRow[] } {
  const source = getKimiCodingPlanSource(response)
  if (!source) return { summary: null, limits: [] }

  const summarySource = toRecord(source.usage)
  const summary = summarySource
    ? buildKimiUsageRow(
        summarySource,
        t('Weekly Quota'),
        'usage-summary',
        formatKimiResetHint(summarySource, t)
      )
    : null

  const rawLimits = Array.isArray(source.limits) ? source.limits : []
  const limits: KimiUsageRow[] = []
  rawLimits.forEach((item, index) => {
    const itemRecord = toRecord(item)
    if (!itemRecord) return
    const detailRecord = toRecord(itemRecord.detail) ?? itemRecord
    const windowRecord = toRecord(itemRecord.window) ?? {}
    const label = formatKimiLimitLabel(
      itemRecord,
      detailRecord,
      windowRecord,
      index,
      t
    )
    const resetHint = formatKimiResetHint(detailRecord, t)
    const row = buildKimiUsageRow(
      detailRecord,
      label,
      `limit-${index}`,
      resetHint
    )
    if (row) {
      // Preserve the constructed window-based label even if detail also has a name.
      row.label =
        detailRecord.name != null && String(detailRecord.name).trim() !== ''
          ? String(detailRecord.name)
          : label
      limits.push(row)
    }
  })

  return { summary, limits }
}

function KimiUsageRowCard({ row }: { row: KimiUsageRow }) {
  const { t } = useTranslation()
  const variant = getProgressVariant(row.percent)

  return (
    <div className='rounded-lg border p-3'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-sm font-medium'>{row.label}</div>
        <StatusBadge
          label={`${row.percent}%`}
          variant={variant}
          copyable={false}
        />
      </div>
      <Progress value={row.percent} className='mt-3' />
      <div className='text-muted-foreground mt-2 grid gap-1 text-xs sm:grid-cols-2'>
        <div>
          {t('Used')}: {formatCount(row.used)}
        </div>
        <div>
          {t('Remaining')}: {formatCount(row.remaining)}
        </div>
        <div>
          {t('Total')}: {formatCount(row.limit)}
        </div>
        {row.resetHint && <div>{row.resetHint}</div>}
      </div>
    </div>
  )
}

function KimiUsageView({
  response,
  onRefresh,
  isRefreshing,
}: {
  response: ChannelPlanUsageResponse | null
  onRefresh: () => void
  isRefreshing?: boolean
}) {
  const { t } = useTranslation()
  const { summary, limits } = useMemo(
    () => normalizeKimiUsageRows(response, t),
    [response, t]
  )
  const shouldShowEmptyState = response != null && response.success !== false

  return (
    <div className='space-y-4'>
      {response?.success === false && (
        <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
          {response.message || t('Failed to fetch Coding Plan usage')}
        </div>
      )}

      <div className='rounded-lg border p-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <StatusBadge
              label={t('Kimi Coding Plan')}
              variant='cyan'
              copyable={false}
            />
            {typeof response?.upstream_status === 'number' && (
              <StatusBadge
                label={`${t('Upstream Status')}: ${response.upstream_status}`}
                variant='neutral'
                copyable={false}
              />
            )}
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={onRefresh}
            disabled={Boolean(isRefreshing)}
          >
            <RefreshCw className='mr-1.5 h-3.5 w-3.5' />
            {t('Refresh')}
          </Button>
        </div>
        <div className='text-muted-foreground mt-2 text-xs break-all'>
          {t('Request URL')}: {response?.request_url || '-'}
        </div>
      </div>

      {(summary || limits.length > 0) && (
        <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
          {summary && <KimiUsageRowCard key={summary.key} row={summary} />}
          {limits.map((row) => (
            <KimiUsageRowCard key={row.key} row={row} />
          ))}
        </div>
      )}
      {!summary && limits.length === 0 && shouldShowEmptyState && (
        <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-sm'>
          {t(
            'No parsed quota windows were found. The key may not be a plan key, or the upstream response format changed.'
          )}
        </div>
      )}
    </div>
  )
}

function KeyPager({
  response,
  currentKeyIndex,
  onKeyIndexChange,
  isRefreshing,
}: {
  response: ChannelPlanUsageResponse | null
  currentKeyIndex: number
  onKeyIndexChange: (keyIndex: number) => void
  isRefreshing?: boolean
}) {
  const { t } = useTranslation()
  const [jumpNumber, setJumpNumber] = useState(String(currentKeyIndex + 1))
  const keyCount = Math.max(Number(response?.key_count || 1), 1)

  useEffect(() => {
    setJumpNumber(String(currentKeyIndex + 1))
  }, [currentKeyIndex])

  if (keyCount <= 1) return null

  const keyStatusVariant: StatusBadgeProps['variant'] =
    Number(response?.key_status) === 2 ? 'danger' : 'success'
  const keyStatusLabel =
    Number(response?.key_status) === 2 ? t('Disabled') : t('Enabled')

  const jumpTo = () => {
    const requested = Number(jumpNumber)
    if (!Number.isFinite(requested) || requested < 1) return
    const target = Math.min(Math.max(Math.floor(requested), 1), keyCount) - 1
    onKeyIndexChange(target)
  }

  return (
    <div className='flex flex-col gap-3 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between'>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => onKeyIndexChange(Math.max(currentKeyIndex - 1, 0))}
          disabled={Boolean(isRefreshing) || currentKeyIndex <= 0}
        >
          <ChevronLeft className='mr-1 h-3.5 w-3.5' />
          {t('Previous key')}
        </Button>
        <StatusBadge
          label={response?.key_label || `Key #${currentKeyIndex + 1}`}
          variant='blue'
          copyable={false}
        />
        <StatusBadge
          label={keyStatusLabel}
          variant={keyStatusVariant}
          copyable={false}
        />
        <span className='text-muted-foreground text-xs'>
          {t('Key {{current}} / {{total}}', {
            current: currentKeyIndex + 1,
            total: keyCount,
          })}
        </span>
        {response?.disabled_reason && (
          <span className='text-destructive text-xs'>
            {t('Reason')}: {response.disabled_reason}
          </span>
        )}
      </div>
      <div className='flex flex-wrap items-center gap-2'>
        <Input
          type='number'
          min={1}
          max={keyCount}
          value={jumpNumber}
          disabled={Boolean(isRefreshing)}
          className='h-8 w-24'
          aria-label={t('Key Number')}
          onChange={(event) => setJumpNumber(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              jumpTo()
            }
          }}
        />
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={jumpTo}
          disabled={Boolean(isRefreshing)}
        >
          {t('Jump')}
        </Button>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() =>
            onKeyIndexChange(Math.min(currentKeyIndex + 1, keyCount - 1))
          }
          disabled={Boolean(isRefreshing) || currentKeyIndex >= keyCount - 1}
        >
          {t('Next key')}
          <ChevronRight className='ml-1 h-3.5 w-3.5' />
        </Button>
      </div>
    </div>
  )
}

export function ChannelPlanUsageDialog({
  open,
  onOpenChange,
  kind,
  channel,
  response,
  currentKeyIndex,
  onKeyIndexChange,
  onRefresh,
  isRefreshing,
}: ChannelPlanUsageDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [showRawJson, setShowRawJson] = useState(false)

  let title = t('Zhipu Coding Plan Usage')
  if (kind === 'minimax') {
    title = t('MiniMax Token Plan Usage')
  } else if (kind === 'kimi') {
    title = t('Kimi Coding Plan Usage')
  }

  const rawJsonText = useMemo(() => {
    if (!response) return ''
    try {
      return JSON.stringify(response, null, 2)
    } catch {
      return String(response?.data ?? '')
    }
  }, [response])
  const refreshCurrentKey = () => onRefresh(currentKeyIndex)
  let usageView = (
    <ZhipuUsageView
      channel={channel}
      response={response}
      onRefresh={refreshCurrentKey}
      isRefreshing={isRefreshing}
    />
  )
  if (kind === 'minimax') {
    usageView = (
      <MiniMaxUsageView
        response={response}
        onRefresh={refreshCurrentKey}
        isRefreshing={isRefreshing}
      />
    )
  } else if (kind === 'kimi') {
    usageView = (
      <KimiUsageView
        response={response}
        onRefresh={refreshCurrentKey}
        isRefreshing={isRefreshing}
      />
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-5xl'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {t('Channel:')} <strong>{channel?.name || '-'}</strong>{' '}
            {channel?.id ? `(#${channel.id})` : ''}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='min-h-0 flex-1 pr-4'>
          {isRefreshing && !response ? (
            <div className='text-muted-foreground flex min-h-64 items-center justify-center gap-2 text-sm'>
              <Loader2 className='h-4 w-4 animate-spin' />
              {t('Loading...')}
            </div>
          ) : (
            <div className='space-y-4 pb-1'>
              <KeyPager
                response={response}
                currentKeyIndex={currentKeyIndex}
                onKeyIndexChange={onKeyIndexChange}
                isRefreshing={isRefreshing}
              />

              {usageView}

              <div className='rounded-lg border'>
                <button
                  type='button'
                  className='hover:bg-muted/40 flex w-full items-center justify-between gap-2 p-3 transition-colors'
                  onClick={() => setShowRawJson((v) => !v)}
                >
                  <div className='text-sm font-medium'>{t('Raw JSON')}</div>
                  {showRawJson ? (
                    <ChevronUp className='text-muted-foreground h-4 w-4' />
                  ) : (
                    <ChevronDown className='text-muted-foreground h-4 w-4' />
                  )}
                </button>
                {showRawJson && (
                  <>
                    <div className='flex justify-end border-t px-3 py-2'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => copyToClipboard(rawJsonText)}
                        disabled={!rawJsonText}
                      >
                        {copiedText === rawJsonText ? (
                          <Check className='mr-1.5 h-3.5 w-3.5 text-green-600' />
                        ) : (
                          <Copy className='mr-1.5 h-3.5 w-3.5' />
                        )}
                        {t('Copy')}
                      </Button>
                    </div>
                    <ScrollArea className='max-h-[42vh]'>
                      <pre className='bg-muted/30 m-0 p-3 text-xs break-words whitespace-pre-wrap'>
                        {rawJsonText || '-'}
                      </pre>
                    </ScrollArea>
                  </>
                )}
              </div>
            </div>
          )}
        </ScrollArea>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
