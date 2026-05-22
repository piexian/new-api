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
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { formatShare, formatTokens } from '../lib/format'
import type { ModelRanking } from '../types'
import { ModelLink, VendorLink } from './entity-links'
import { GrowthText } from './growth-text'

type ModelLeaderboardProps = {
  rows: ModelRanking[]
  /** Density variant. `compact` is used inside per-category sections; the
   * default fits the larger overall "Top Models" section. */
  variant?: 'default' | 'compact'
  /** Optional cap (rows beyond this are dropped). */
  limit?: number
}

/**
 * Two-column model leaderboard list: "rank · model
 * (with vendor below) · tokens (with growth below)" rendering. Splits
 * `rows` evenly between the two columns so the visual rhythm matches a
 * single ranked list rather than two independent lists.
 *
 * Both the model name and vendor name are clickable: model jumps to
 * `/pricing/{modelName}` and vendor jumps to `/pricing?vendor={vendor}`.
 */
export function ModelLeaderboard(props: ModelLeaderboardProps) {
  const limited = props.limit ? props.rows.slice(0, props.limit) : props.rows
  const variant = props.variant ?? 'default'

  if (limited.length === 0) {
    return null
  }

  return <ModelList rows={limited} variant={variant} />
}

function ModelList(props: {
  rows: ModelRanking[]
  variant: 'default' | 'compact'
}) {
  const { t } = useTranslation()
  const compact = props.variant === 'compact'
  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='text-muted-foreground bg-muted/30 hidden grid-cols-[3.5rem_minmax(0,1fr)_5rem_5rem_4.5rem] gap-4 border-b px-3 py-2 text-[11px] font-medium tracking-widest uppercase sm:grid'>
        <span>{t('Rank')}</span>
        <span>{t('Model')}</span>
        <span className='text-right'>{t('Share')}</span>
        <span className='text-right'>{t('Tokens')}</span>
        <span className='text-right'>{t('Growth')}</span>
      </div>
      {props.rows.map((row) => (
        <div
          key={row.model_name}
          className={cn(
            'hover:bg-muted/30 grid grid-cols-[2.25rem_minmax(0,1fr)_auto] items-center gap-3 border-b px-3 last:border-b-0 sm:grid-cols-[3.5rem_minmax(0,1fr)_5rem_5rem_4.5rem] sm:gap-4',
            compact ? 'py-2' : 'py-3'
          )}
        >
          <span className='text-muted-foreground font-mono text-xs tabular-nums'>
            {row.rank}
          </span>
          <div className='flex min-w-0 items-center gap-3'>
            <span className='shrink-0'>
              {getLobeIcon(row.vendor_icon, compact ? 20 : 22)}
            </span>
            <div className='min-w-0'>
              <ModelLink
                modelName={row.model_name}
                className={
                  compact
                    ? 'text-foreground block truncate font-mono text-xs font-medium'
                    : 'text-foreground block truncate font-mono text-sm font-medium'
                }
              >
                {row.model_name}
              </ModelLink>
              <p
                className={
                  compact
                    ? 'text-muted-foreground truncate text-[11px]'
                    : 'text-muted-foreground truncate text-xs'
                }
              >
                <VendorLink vendor={row.vendor}>{row.vendor}</VendorLink>
              </p>
            </div>
          </div>
          <div className='hidden text-right sm:block'>
            <Badge variant='outline' className='font-mono'>
              {formatShare(row.share)}
            </Badge>
          </div>
          <div className='hidden text-right sm:block'>
            <div className='text-foreground font-mono text-sm font-semibold tabular-nums'>
              {formatTokens(row.total_tokens)}
            </div>
          </div>
          <div className='text-right'>
            <GrowthText value={row.growth_pct} />
          </div>
          <div className='col-span-3 flex items-center gap-2 sm:hidden'>
            <div className='bg-muted h-1.5 min-w-0 flex-1 overflow-hidden rounded-full'>
              <div
                className='bg-primary h-full rounded-full'
                style={{
                  width: `${Math.max(2, Math.min(100, row.share * 100))}%`,
                }}
              />
            </div>
            <span className='text-muted-foreground font-mono text-[11px] tabular-nums'>
              {formatTokens(row.total_tokens)}
            </span>
          </div>
        </div>
      ))}
    </div>
  )
}
