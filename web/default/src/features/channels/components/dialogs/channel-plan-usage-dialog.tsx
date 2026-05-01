import { useEffect, useMemo, useState } from 'react'
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
import { useTranslation } from 'react-i18next'
import dayjs from '@/lib/dayjs'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
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
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import type { ChannelPlanUsageResponse } from '../../api'
import type { Channel } from '../../types'

export type ChannelPlanUsageKind = 'minimax' | 'zhipu'

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
  percent: number
  remainsTime: number | null
  startTime: number | null
  endTime: number | null
}

type MiniMaxModelRemain = Record<string, unknown>

type ZhipuLimitCard = {
  key: string
  title: string
  percentage: number
  usageLabel: string | null
  nextResetTime: string
  details: { key: string; name: string; usage: number | null }[]
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

function clampPercent(value: unknown): number {
  const numericValue = Number(value)
  if (!Number.isFinite(numericValue)) return 0
  return Math.max(0, Math.min(100, numericValue))
}

function formatCount(value: unknown): string {
  const numericValue = toNumber(value)
  if (numericValue == null) return '-'
  return numericValue.toLocaleString()
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
  remainsTime: number | null
  startTime: number | null
  endTime: number | null
}): UsageWindow | null {
  if (
    input.total == null &&
    input.used == null &&
    input.remaining == null &&
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
    (input.total != null && input.total > 0) ||
    (input.used != null && input.used > 0) ||
    (remaining != null && remaining > 0)

  if (!hasQuota) return null

  const percent =
    input.total != null && input.total > 0 && input.used != null
      ? Math.floor(clampPercent((input.used / input.total) * 100))
      : 0

  return { ...input, remaining, percent }
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

  return [
    buildWindow({
      key: 'current_interval',
      label: resolveCurrentWindowLabel(currentStartTime, currentEndTime, t),
      total: currentIntervalQuota.total,
      used: currentIntervalQuota.used,
      remaining: currentIntervalQuota.remaining,
      remainsTime: toNumber(item.remains_time),
      startTime: currentStartTime,
      endTime: currentEndTime,
    }),
    buildWindow({
      key: 'current_weekly',
      label: t('Weekly Window'),
      total: currentWeeklyQuota.total,
      used: currentWeeklyQuota.used,
      remaining: currentWeeklyQuota.remaining,
      remainsTime: toNumber(item.weekly_remains_time),
      startTime: weeklyStartTime,
      endTime: weeklyEndTime,
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
  if (Number(item.unit) === 6) return t('Weekly Quota')
  switch (item.type) {
    case 'TOKENS_LIMIT':
      return t('5-Hour Quota')
    case 'TIME_LIMIT':
      return t('MCP Tool Quota')
    default:
      return item.type ? String(item.type) : t('Quota Window')
  }
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
    if (item.type === 'TOKENS_LIMIT') return 1
    if (Number(item.unit) === 6) return 2
    if (item.type === 'TIME_LIMIT') return 3
    return 9
  }

  return limits
    .map((item) => toRecord(item))
    .filter(isRecordResult)
    .sort((left, right) => orderWeight(left) - orderWeight(right))
    .map((item, index) => {
      const currentValue = toNumber(item.currentValue)
      const totalValue = toNumber(item.usage)
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
        percentage: clampPercent(item.percentage),
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

function MetaBlock({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='bg-muted/30 min-w-0 rounded-lg border p-3'>
      <div className='text-muted-foreground mb-1 text-xs'>{label}</div>
      <div className='min-w-0 text-sm break-all'>{value || '-'}</div>
    </div>
  )
}

function UsageWindowCard({ windowInfo }: { windowInfo: UsageWindow }) {
  const { t } = useTranslation()
  const variant = getProgressVariant(windowInfo.percent)

  return (
    <div className='rounded-lg border p-3'>
      <div className='flex items-center justify-between gap-2'>
        <div className='text-sm font-medium'>{windowInfo.label}</div>
        <StatusBadge
          label={`${windowInfo.percent}%`}
          variant={variant}
          copyable={false}
        />
      </div>
      <div className='mt-3'>
        <Progress value={windowInfo.percent} />
      </div>
      <div className='text-muted-foreground mt-2 grid gap-1 text-xs sm:grid-cols-2'>
        <div>
          {t('Used')}: {formatCount(windowInfo.used)}
        </div>
        <div>
          {t('Remaining')}: {formatCount(windowInfo.remaining)}
        </div>
        <div>
          {t('Total')}: {formatCount(windowInfo.total)}
        </div>
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
        <StatusBadge
          label={t('Token Plan')}
          variant='cyan'
          copyable={false}
        />
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
  const modelRemains = Array.isArray(upstreamData?.model_remains)
    ? upstreamData.model_remains
    : []
  const parsedModels = useMemo(
    () =>
      modelRemains
        .map((item) => toRecord(item))
        .filter(isRecordResult)
        .map((item) => ({
          item,
          windows: resolveModelWindows(item, t),
        })),
    [modelRemains, t]
  )
  const activeModels = parsedModels.filter((entry) => entry.windows.length > 0)
  const unavailableModels = parsedModels
    .filter((entry) => entry.windows.length === 0)
    .map((entry) => String(entry.item.model_name || entry.item.model || ''))
    .filter(Boolean)
  const baseResp = toRecord(upstreamData?.base_resp)
  const shouldShowEmptyState = response != null && response.success !== false

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
        <MetaBlock
          label={t('Business Status')}
          value={
            baseResp?.status_code === 0
              ? t('Normal')
              : baseResp?.status_code != null
                ? String(baseResp.status_code)
                : t('Unknown')
          }
        />
        <MetaBlock
          label={t('Business Message')}
          value={String(baseResp?.status_msg || response?.message || '-')}
        />
      </div>

      <div className='space-y-3'>
        {activeModels.length > 0 ? (
          activeModels.map(({ item, windows }, index) => (
            <MiniMaxModelCard
              key={`${String(item.model_name || item.model || 'model')}-${index}`}
              item={item}
              windows={windows}
            />
          ))
        ) : shouldShowEmptyState ? (
          <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-sm'>
            {t(
              'No parsed quota windows were found. The key may not be a plan key, or the upstream response format changed.'
            )}
          </div>
        ) : null}
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

      {cards.length > 0 ? (
        <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
          {cards.map((card) => (
            <ZhipuLimitCard key={card.key} card={card} />
          ))}
        </div>
      ) : shouldShowEmptyState ? (
        <div className='text-muted-foreground rounded-lg border border-dashed p-4 text-sm'>
          {t(
            'No parsed quota windows were found. The key may not be a plan key, or the upstream response format changed.'
          )}
        </div>
      ) : null}
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

  const title =
    kind === 'minimax'
      ? t('MiniMax Token Plan Usage')
      : t('Zhipu Coding Plan Usage')

  const rawJsonText = useMemo(() => {
    if (!response) return ''
    try {
      return JSON.stringify(response, null, 2)
    } catch {
      return String(response?.data ?? '')
    }
  }, [response])

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
            <div className='flex min-h-64 items-center justify-center gap-2 text-sm text-muted-foreground'>
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

              {kind === 'minimax' ? (
                <MiniMaxUsageView
                  response={response}
                  onRefresh={() => onRefresh(currentKeyIndex)}
                  isRefreshing={isRefreshing}
                />
              ) : (
                <ZhipuUsageView
                  channel={channel}
                  response={response}
                  onRefresh={() => onRefresh(currentKeyIndex)}
                  isRefreshing={isRefreshing}
                />
              )}

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
