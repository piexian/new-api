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
import { memo } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

export type ModelPerfBadgeData = {
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_rates?: number[]
}

export interface ModelPerfBadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  perf: ModelPerfBadgeData | undefined
}

function formatCompactNumber(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '—'
  return value > 1 ? String(Math.round(value)) : value.toFixed(1)
}

function formatCompactLatency(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  if (ms >= 1_000) return `${formatCompactNumber(ms / 1_000)}s`
  return `${formatCompactNumber(ms)}ms`
}

function formatCompactThroughput(tps: number): string {
  if (!Number.isFinite(tps) || tps <= 0) return '—'
  if (tps >= 1_000) return `${formatCompactNumber(tps / 1_000)}Kt`
  return `${formatCompactNumber(tps)}t`
}

function getStatusConfig(successRate: number) {
  if (!Number.isFinite(successRate)) {
    return {
      labelKey: 'No monitoring data',
      className: 'bg-muted text-muted-foreground ring-border',
    }
  }

  if (successRate < 80) {
    return {
      labelKey: 'Monitoring incident',
      className:
        'bg-destructive/10 text-destructive ring-destructive/20 dark:bg-destructive/15',
    }
  }

  if (successRate < 99) {
    return {
      labelKey: 'Monitoring low',
      className: 'bg-sky-500/10 text-sky-700 ring-sky-500/25 dark:text-sky-300',
    }
  }

  if (successRate < 99.9) {
    return {
      labelKey: 'Monitoring degraded',
      className:
        'bg-amber-500/10 text-amber-700 ring-amber-500/25 dark:text-amber-300',
    }
  }

  return {
    labelKey: 'Monitoring healthy',
    className:
      'bg-emerald-500/10 text-emerald-700 ring-emerald-500/25 dark:text-emerald-300',
  }
}

export const ModelPerfBadge = memo(function ModelPerfBadge(
  props: ModelPerfBadgeProps
) {
  const { t } = useTranslation()

  if (!props.perf) {
    return null
  }

  const { avg_latency_ms, avg_tps, success_rate } = props.perf
  const successRateText = Number.isFinite(success_rate)
    ? `${success_rate.toFixed(1)}%`
    : '—'
  const statusConfig = getStatusConfig(success_rate)

  return (
    <div
      className={cn(
        'hidden w-[154px] grid-cols-[38px_48px_44px] gap-x-2 text-right tabular-nums min-[460px]:grid',
        props.className
      )}
    >
      <div title={t('Average latency')} className='min-w-0'>
        <div className='text-muted-foreground/55 text-[10px] leading-4'>
          {t('Latency short')}
        </div>
        <div className='text-muted-foreground/80 font-mono text-xs leading-4 whitespace-nowrap'>
          {formatCompactLatency(avg_latency_ms)}
        </div>
      </div>
      <div title={t('Throughput')} className='min-w-0'>
        <div className='text-muted-foreground/55 truncate text-[10px] leading-4'>
          {t('Throughput short')}
        </div>
        <div className='text-muted-foreground/80 font-mono text-xs leading-4 whitespace-nowrap'>
          {formatCompactThroughput(avg_tps)}
        </div>
      </div>
      <div
        title={`${t('Success rate')}: ${successRateText}`}
        className='min-w-0'
      >
        <div className='text-muted-foreground/55 truncate text-[10px] leading-4'>
          {t('Status short')}
        </div>
        <div className='flex h-4 items-center justify-end'>
          <span
            className={cn(
              'inline-flex max-w-full items-center rounded-full px-1.5 text-[10px] leading-4 font-medium ring-1 ring-inset',
              statusConfig.className
            )}
          >
            {t(statusConfig.labelKey)}
          </span>
        </div>
      </div>
    </div>
  )
})
