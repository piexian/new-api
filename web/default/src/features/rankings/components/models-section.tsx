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
import { VChart } from '@visactor/react-vchart'
import { BarChart3, Trophy } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { getLobeIcon } from '@/lib/lobe-icon'
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'

import { formatShare, formatTokens } from '../lib/format'
import type { ModelHistorySeries, ModelRanking, RankingPeriod } from '../types'
import { ModelLink, VendorLink } from './entity-links'
import { GrowthText } from './growth-text'
import { ModelLeaderboard } from './model-leaderboard'

const PERIOD_DESCRIPTIONS: Record<RankingPeriod, string> = {
  today: 'Hourly token usage by model across the last 24 hours',
  week: 'Weekly token usage by model across the past few weeks',
  month: 'Daily token usage by model across the past month',
  year: 'Weekly token usage by model across the past year',
}

const TOOLTIP_MAX_ROWS = 10

type ModelsSectionProps = {
  history: ModelHistorySeries
  rows: ModelRanking[]
  period: RankingPeriod
}

/**
 * Combined "Top Models" card: a stacked bar chart showing token usage by
 * model over time, paired below with a two-column LLM Leaderboard. The
 * chart anchors the eye while the leaderboard provides the detailed key.
 */
export function ModelsSection(props: ModelsSectionProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const chartTextColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.68)'
      : 'rgba(15, 23, 42, 0.58)'
  const chartGridColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.12)'
      : 'rgba(15, 23, 42, 0.12)'

  // Order points so the largest model appears at the bottom of every stack.
  const orderedPoints = useMemo(() => {
    const order = new Map(
      props.history.models.map((m, idx) => [m.name, idx] as const)
    )
    return [...props.history.points].sort((a, b) => {
      const tsCmp = a.ts.localeCompare(b.ts)
      if (tsCmp !== 0) return tsCmp
      return (order.get(a.model) ?? 999) - (order.get(b.model) ?? 999)
    })
  }, [props.history])

  const totalTokens = useMemo(
    () => props.rows.reduce((s, r) => s + r.total_tokens, 0),
    [props.rows]
  )
  const featured = props.rows.slice(0, 5)

  const spec = useMemo(() => {
    if (orderedPoints.length === 0) return null
    return {
      type: 'bar' as const,
      data: [{ id: 'models-history', values: orderedPoints }],
      xField: 'label',
      yField: 'tokens',
      seriesField: 'model',
      stack: true,
      bar: {
        style: { cornerRadius: 0, lineWidth: 0 },
      },
      legends: { visible: false },
      axes: [
        {
          orient: 'bottom',
          label: {
            style: { fill: chartTextColor, fontSize: 10 },
            autoHide: true,
            autoLimit: true,
          },
          tick: { visible: false },
        },
        {
          orient: 'left',
          label: {
            formatMethod: (val: number | string) => formatTokens(Number(val)),
            style: { fill: chartTextColor, fontSize: 10 },
          },
          grid: {
            visible: true,
            style: { lineDash: [3, 3], stroke: chartGridColor },
          },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: (datum: Record<string, unknown>) =>
                String(datum?.model ?? ''),
              value: (datum: Record<string, unknown>) =>
                formatTokens(Number(datum?.tokens) || 0),
            },
          ],
        },
        dimension: {
          title: {
            value: (datum: Record<string, unknown>) =>
              String(datum?.label ?? ''),
          },
          content: [
            {
              key: (datum: Record<string, unknown>) =>
                String(datum?.model ?? ''),
              value: (datum: Record<string, unknown>) =>
                Number(datum?.tokens) || 0,
            },
          ],
          updateContent: (
            array: Array<{ key: string; value: string | number }>
          ) => {
            array.sort((a, b) => Number(b.value) - Number(a.value))
            const sum = array.reduce((s, x) => s + (Number(x.value) || 0), 0)
            const visible = array.slice(0, TOOLTIP_MAX_ROWS)
            const overflow = array.slice(TOOLTIP_MAX_ROWS)
            const result = visible.map((item) => ({
              key: item.key,
              value: formatTokens(Number(item.value) || 0),
            }))
            if (overflow.length > 0) {
              const otherSum = overflow.reduce(
                (s, item) => s + (Number(item.value) || 0),
                0
              )
              result.push({
                key: t('+{{count}} more', { count: overflow.length }),
                value: formatTokens(otherSum),
              })
            }
            result.unshift({ key: t('Total:'), value: formatTokens(sum) })
            return result
          },
        },
      },
      animationAppear: { duration: 500 },
    }
  }, [chartGridColor, chartTextColor, orderedPoints, t])

  return (
    <Card className='rounded-lg'>
      <CardHeader className='gap-2 px-5'>
        <div className='flex min-w-0 items-start gap-3'>
          <span className='bg-muted text-muted-foreground flex size-9 shrink-0 items-center justify-center rounded-md'>
            <BarChart3 className='size-4' />
          </span>
          <div className='min-w-0'>
            <CardTitle className='text-base font-semibold'>
              {t('Top Models')}
            </CardTitle>
            <CardDescription>
              {t(PERIOD_DESCRIPTIONS[props.period])}
            </CardDescription>
          </div>
        </div>
        <CardAction className='text-right'>
          <div className='text-foreground font-mono text-2xl leading-none font-semibold tabular-nums'>
            {formatTokens(totalTokens)}
          </div>
          <div className='text-muted-foreground mt-1 text-[10px] font-medium tracking-widest uppercase'>
            {t('tokens')}
          </div>
        </CardAction>
      </CardHeader>

      <CardContent className='px-5'>
        <div className='grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]'>
          <div className='min-w-0'>
            <div className='mb-3 flex items-center justify-between gap-3'>
              <h3 className='text-foreground text-sm font-medium'>
                {t('Usage over time')}
              </h3>
              <Badge variant='outline' className='font-mono'>
                {t('{{count}} models', { count: props.rows.length })}
              </Badge>
            </div>
            <div className='h-72'>
              {themeReady && spec ? (
                <VChart
                  key={`models-history-${resolvedTheme}-${props.period}`}
                  spec={{
                    ...spec,
                    theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                    background: 'transparent',
                  }}
                  option={VCHART_OPTION}
                />
              ) : (
                <div className='text-muted-foreground flex h-full items-center justify-center text-xs'>
                  {t('No history data available')}
                </div>
              )}
            </div>
          </div>

          <aside className='bg-muted/30 rounded-lg border p-3'>
            <div className='mb-2 flex items-center justify-between gap-3'>
              <h3 className='text-foreground inline-flex items-center gap-2 text-sm font-semibold'>
                <Trophy className='size-3.5 text-amber-500' />
                {t('Popular now')}
              </h3>
              <span className='text-muted-foreground text-[11px] font-medium tracking-widest uppercase'>
                {t('Share')}
              </span>
            </div>
            {featured.length === 0 ? (
              <div className='text-muted-foreground flex h-44 items-center justify-center text-xs'>
                {t('No models match the selected filters')}
              </div>
            ) : (
              <div className='flex flex-col'>
                {featured.map((row) => (
                  <FeaturedModelRow key={row.model_name} row={row} />
                ))}
              </div>
            )}
          </aside>
        </div>
      </CardContent>

      <div className='border-t px-5 pt-4 pb-5'>
        <header className='mb-2 flex flex-wrap items-end justify-between gap-3'>
          <div>
            <h3 className='text-foreground inline-flex items-center gap-2 text-sm font-semibold'>
              <Trophy className='size-3.5 text-amber-500' />
              {t('LLM Leaderboard')}
            </h3>
            <p className='text-muted-foreground mt-0.5 text-xs'>
              {t('Compare the most popular models on the platform')}
            </p>
          </div>
          <div className='text-muted-foreground hidden grid-cols-[5rem_4.5rem] gap-4 pr-1 text-right text-[11px] font-medium tracking-widest uppercase sm:grid'>
            <span>{t('Tokens')}</span>
            <span>{t('Growth')}</span>
          </div>
        </header>
        {props.rows.length === 0 ? (
          <div className='text-muted-foreground px-5 py-8 text-center text-sm'>
            {t('No models match the selected filters')}
          </div>
        ) : (
          <ModelLeaderboard rows={props.rows} />
        )}
      </div>
    </Card>
  )
}

function FeaturedModelRow(props: { row: ModelRanking }) {
  return (
    <div className='border-border/60 flex items-center gap-3 border-b py-3 last:border-b-0'>
      <span className='text-muted-foreground w-6 shrink-0 text-right font-mono text-xs tabular-nums'>
        {props.row.rank}
      </span>
      <span className='shrink-0'>{getLobeIcon(props.row.vendor_icon, 24)}</span>
      <div className='min-w-0 flex-1'>
        <ModelLink
          modelName={props.row.model_name}
          className='text-foreground block truncate font-mono text-sm font-medium'
        >
          {props.row.model_name}
        </ModelLink>
        <p className='text-muted-foreground truncate text-xs'>
          <VendorLink vendor={props.row.vendor}>{props.row.vendor}</VendorLink>
        </p>
      </div>
      <div className='shrink-0 text-right'>
        <div className='text-foreground font-mono text-sm font-semibold tabular-nums'>
          {formatShare(props.row.share)}
        </div>
        <GrowthText value={props.row.growth_pct} className='text-[10px]' />
      </div>
    </div>
  )
}
