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
import { PieChart } from 'lucide-react'
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
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'

import { formatShare, formatTokens } from '../lib/format'
import type { RankingPeriod, VendorRanking, VendorShareSeries } from '../types'
import { VendorLink } from './entity-links'

const PERIOD_DESCRIPTIONS: Record<RankingPeriod, string> = {
  today: 'Token share by model author across the last 24 hours',
  week: 'Token share by model author across the past few weeks',
  month: 'Token share by model author across the past month',
  year: 'Token share by model author across the past year',
}

/** Stable colour palette for vendors, used in both the share chart and the
 * legend dots. Falls back to a neutral palette for unknown vendors so that
 * future additions still render. */
const VENDOR_COLOURS: Record<string, string> = {
  OpenAI: '#10a37f',
  Anthropic: '#d97757',
  Google: '#4285f4',
  DeepSeek: '#7c5cff',
  Alibaba: '#ff9900',
  xAI: '#1f2937',
  Meta: '#1877f2',
  Moonshot: '#ec4899',
  Zhipu: '#06b6d4',
  Mistral: '#ff7000',
  ByteDance: '#3b82f6',
  Tencent: '#22c55e',
  MiniMax: '#a855f7',
  Cohere: '#fb923c',
  Baidu: '#ef4444',
  Others: '#94a3b8',
}

const FALLBACK_PALETTE = [
  '#0ea5e9',
  '#22c55e',
  '#a855f7',
  '#f97316',
  '#14b8a6',
  '#eab308',
  '#ec4899',
  '#84cc16',
  '#6366f1',
  '#10b981',
  '#f43f5e',
  '#0891b2',
  '#94a3b8',
]

function buildVendorColourMap(names: string[]): Record<string, string> {
  const result: Record<string, string> = {}
  let fallbackIdx = 0
  for (const name of names) {
    if (VENDOR_COLOURS[name]) {
      result[name] = VENDOR_COLOURS[name]
    } else {
      result[name] = FALLBACK_PALETTE[fallbackIdx % FALLBACK_PALETTE.length]
      fallbackIdx += 1
    }
  }
  return result
}

const MAX_VENDORS_IN_LIST = 12

type MarketShareSectionProps = {
  history: VendorShareSeries
  rows: VendorRanking[]
  period: RankingPeriod
}

/**
 * Combined "Market Share" card: a 100%-stacked bar chart showing each
 * vendor's slice of total token volume, paired with a compact vendor list.
 */
export function MarketShareSection(props: MarketShareSectionProps) {
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

  const colourMap = useMemo(
    () => buildVendorColourMap(props.history.vendors.map((v) => v.name)),
    [props.history]
  )

  const orderedPoints = useMemo(() => {
    const order = new Map(
      props.history.vendors.map((v, idx) => [v.name, idx] as const)
    )
    return [...props.history.points].sort((a, b) => {
      const tsCmp = a.ts.localeCompare(b.ts)
      if (tsCmp !== 0) return tsCmp
      return (order.get(a.vendor) ?? 999) - (order.get(b.vendor) ?? 999)
    })
  }, [props.history])

  const spec = useMemo(() => {
    if (orderedPoints.length === 0) return null
    return {
      type: 'bar' as const,
      data: [{ id: 'vendor-share', values: orderedPoints }],
      xField: 'label',
      yField: 'share',
      seriesField: 'vendor',
      stack: true,
      paddingInner: 0.02,
      legends: { visible: false },
      bar: {
        style: { cornerRadius: 0, lineWidth: 0 },
      },
      color: { specified: colourMap },
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
          min: 0,
          max: 1,
          label: {
            formatMethod: (val: number | string) =>
              `${Math.round(Number(val) * 100)}%`,
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
                String(datum?.vendor ?? ''),
              value: (datum: Record<string, unknown>) =>
                `${(Number(datum?.share) * 100).toFixed(1)}% · ${formatTokens(Number(datum?.tokens) || 0)}`,
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
                String(datum?.vendor ?? ''),
              value: (datum: Record<string, unknown>) =>
                Number(datum?.share) || 0,
            },
          ],
          updateContent: (
            array: Array<{ key: string; value: string | number }>
          ) => {
            return array
              .filter((item) => Number(item.value) > 0.001)
              .sort((a, b) => Number(b.value) - Number(a.value))
              .map((item) => ({
                key: item.key,
                value: `${(Number(item.value) * 100).toFixed(1)}%`,
              }))
          },
        },
      },
      animationAppear: { duration: 500 },
    }
  }, [chartGridColor, chartTextColor, colourMap, orderedPoints])

  const visible = props.rows.slice(0, MAX_VENDORS_IN_LIST)
  const leadingVendor = visible[0]
  const totalTokens = useMemo(
    () => props.rows.reduce((sum, row) => sum + row.total_tokens, 0),
    [props.rows]
  )

  return (
    <Card className='rounded-lg'>
      <CardHeader className='gap-2 px-5'>
        <div className='flex min-w-0 items-start gap-3'>
          <span className='bg-muted text-muted-foreground flex size-9 shrink-0 items-center justify-center rounded-md'>
            <PieChart className='size-4' />
          </span>
          <div className='min-w-0'>
            <CardTitle className='text-base font-semibold'>
              {t('Market Share')}
            </CardTitle>
            <CardDescription>
              {t(PERIOD_DESCRIPTIONS[props.period])}
            </CardDescription>
          </div>
        </div>
        <CardAction className='text-right'>
          <div className='text-foreground font-mono text-2xl leading-none font-semibold tabular-nums'>
            {leadingVendor ? formatShare(leadingVendor.share) : '0%'}
          </div>
          <div className='text-muted-foreground mt-1 text-[10px] font-medium tracking-widest uppercase'>
            {leadingVendor?.vendor ?? t('Top vendor')}
          </div>
        </CardAction>
      </CardHeader>

      <CardContent className='px-5 pb-5'>
        <div className='space-y-5'>
          <div className='min-w-0'>
            <div className='mb-3 flex items-center justify-between gap-3'>
              <h3 className='text-foreground text-sm font-medium'>
                {t('Share over time')}
              </h3>
              <Badge variant='outline' className='font-mono'>
                {formatTokens(totalTokens)} {t('tokens')}
              </Badge>
            </div>
            <div className='h-72 lg:h-80'>
              {themeReady && spec ? (
                <VChart
                  key={`vendor-share-${resolvedTheme}-${props.period}`}
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

          <section className='border-t pt-4'>
            <div className='mb-3 flex items-end justify-between gap-3'>
              <div>
                <h3 className='text-foreground text-sm font-semibold'>
                  {t('By model author')}
                </h3>
                <p className='text-muted-foreground mt-0.5 text-xs'>
                  {t('Vendors ranked by aggregated token volume')}
                </p>
              </div>
              <span className='text-muted-foreground text-[11px] font-medium tracking-widest uppercase'>
                {t('Share')}
              </span>
            </div>
            {visible.length === 0 ? (
              <div className='text-muted-foreground flex h-28 items-center justify-center text-sm'>
                {t('No vendor data available')}
              </div>
            ) : (
              <VendorList rows={visible} colourMap={colourMap} />
            )}
          </section>
        </div>
      </CardContent>
    </Card>
  )
}

function VendorList(props: {
  rows: VendorRanking[]
  colourMap: Record<string, string>
}) {
  return (
    <div className='grid gap-x-6 md:grid-cols-2 xl:grid-cols-3'>
      {props.rows.map((vendor) => (
        <div
          key={vendor.vendor}
          className='border-border/60 flex min-w-0 items-start gap-3 border-b py-2.5 last:border-b-0'
        >
          <span className='text-muted-foreground w-6 shrink-0 text-right font-mono text-xs tabular-nums'>
            {vendor.rank}
          </span>
          <span
            aria-hidden
            className='mt-1.5 size-2.5 shrink-0 rounded-full'
            style={{
              backgroundColor: props.colourMap[vendor.vendor] ?? '#94a3b8',
            }}
          />
          <div className='min-w-0 flex-1'>
            <div className='flex items-center justify-between gap-3'>
              <VendorLink
                vendor={vendor.vendor}
                className='text-foreground min-w-0 truncate text-sm font-medium'
              >
                {vendor.vendor}
              </VendorLink>
              <span className='text-foreground shrink-0 font-mono text-sm font-semibold tabular-nums'>
                {formatShare(vendor.share)}
              </span>
            </div>
            <div className='bg-muted mt-2 h-1 overflow-hidden'>
              <div
                className='h-full'
                style={{
                  width: `${Math.max(2, Math.min(100, vendor.share * 100))}%`,
                  backgroundColor: props.colourMap[vendor.vendor] ?? '#94a3b8',
                }}
              />
            </div>
            <div className='text-muted-foreground mt-1.5 flex items-center justify-between gap-3 text-[11px]'>
              <span className='truncate'>
                {vendor.models_count} · {vendor.top_model}
              </span>
              <span className='shrink-0 font-mono tabular-nums'>
                {formatTokens(vendor.total_tokens)}
              </span>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
