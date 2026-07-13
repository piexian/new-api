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
import { UserMultipleIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { cn } from '@/lib/utils'

import { formatShare, formatTokens } from '../lib/format'
import type { RankingPeriod, UserRanking, UserRankingSelf } from '../types'
import { GrowthText } from './growth-text'

const PERIOD_DESCRIPTIONS: Record<RankingPeriod, string> = {
  today: 'Top users by token usage in the last 24 hours',
  week: 'Top users by token usage this week',
  month: 'Top users by token usage this month',
  year: 'Top users by token usage this year',
}

type UsersSectionProps = {
  rows: UserRanking[]
  period: RankingPeriod
  me?: UserRankingSelf | null
}

/**
 * User consumption leaderboard. Renders username only — never user_id.
 */
export function UsersSection(props: UsersSectionProps) {
  const { t } = useTranslation()
  const rows = props.rows
  const totalTokens = rows.reduce((sum, row) => sum + row.total_tokens, 0)

  return (
    <Card className='overflow-hidden'>
      <CardHeader className='border-b pb-4'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0 space-y-1'>
            <CardTitle className='flex items-center gap-2 text-base'>
              <HugeiconsIcon
                icon={UserMultipleIcon}
                className='size-4 shrink-0'
                strokeWidth={2}
              />
              {t('User Consumption Ranking')}
            </CardTitle>
            <CardDescription>
              {t(PERIOD_DESCRIPTIONS[props.period] ?? PERIOD_DESCRIPTIONS.week)}
            </CardDescription>
          </div>
          {totalTokens > 0 ? (
            <Badge variant='secondary' className='shrink-0 font-mono'>
              {formatTokens(totalTokens)}
            </Badge>
          ) : null}
        </div>
      </CardHeader>
      <CardContent className='p-0'>
        {props.me ? <MyRankBanner me={props.me} /> : null}
        {rows.length === 0 ? (
          <p className='text-muted-foreground px-4 py-10 text-center text-sm'>
            {t('No user ranking data for this period')}
          </p>
        ) : (
          <UserLeaderboard rows={rows} />
        )}
      </CardContent>
    </Card>
  )
}

function UserLeaderboard(props: { rows: UserRanking[] }) {
  const { t } = useTranslation()
  return (
    <div className='overflow-hidden'>
      <div className='text-muted-foreground bg-muted/30 hidden grid-cols-[3.5rem_minmax(0,1fr)_5rem_5rem_4.5rem_4.5rem] gap-4 border-b px-3 py-2 text-[11px] font-medium tracking-widest uppercase sm:grid'>
        <span>{t('Rank')}</span>
        <span>{t('Username')}</span>
        <span className='text-right'>{t('Share')}</span>
        <span className='text-right'>{t('Tokens')}</span>
        <span className='text-right'>{t('Requests')}</span>
        <span className='text-right'>{t('Growth')}</span>
      </div>
      {props.rows.map((row) => (
        <div
          key={row.username}
          className={cn(
            'hover:bg-muted/30 grid grid-cols-[2.25rem_minmax(0,1fr)_auto] items-center gap-3 border-b px-3 py-3 last:border-b-0 sm:grid-cols-[3.5rem_minmax(0,1fr)_5rem_5rem_4.5rem_4.5rem] sm:gap-4'
          )}
        >
          <span className='text-muted-foreground font-mono text-xs tabular-nums'>
            {row.rank}
          </span>
          <div className='min-w-0'>
            <div className='text-foreground truncate font-mono text-sm font-medium'>
              {row.username}
            </div>
            <p className='text-muted-foreground truncate text-xs sm:hidden'>
              {formatTokens(row.total_tokens)} · {formatShare(row.share)}
            </p>
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
          <div className='hidden text-right sm:block'>
            <div className='text-muted-foreground font-mono text-xs tabular-nums'>
              {Number(row.request_count || 0).toLocaleString()}
            </div>
          </div>
          <div className='text-right'>
            <GrowthText value={row.growth_pct} />
          </div>
        </div>
      ))}
    </div>
  )
}

function MyRankBanner(props: { me: UserRankingSelf }) {
  const { t } = useTranslation()
  const me = props.me
  const rankLabel = me.rank > 0 ? `#${me.rank}` : t('Unranked')
  return (
    <div className='bg-muted/20 border-b px-4 py-3'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='min-w-0'>
          <div className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
            {t('My ranking')}
          </div>
          <div className='text-foreground mt-0.5 truncate font-mono text-sm font-semibold'>
            {me.username}
            <span className='text-muted-foreground ml-2 font-sans text-xs font-normal'>
              {rankLabel}
              {me.total_users > 0
                ? ` / ${me.total_users.toLocaleString()} ${t('users')}`
                : ''}
              {me.in_top_list ? ` · ${t('In top board')}` : ''}
            </span>
          </div>
        </div>
        <div className='flex flex-wrap items-center gap-3 text-right'>
          <div>
            <div className='text-muted-foreground text-[11px] uppercase'>
              {t('Tokens')}
            </div>
            <div className='font-mono text-sm font-semibold tabular-nums'>
              {formatTokens(me.total_tokens || 0)}
            </div>
          </div>
          <div>
            <div className='text-muted-foreground text-[11px] uppercase'>
              {t('Share')}
            </div>
            <div className='font-mono text-sm tabular-nums'>
              {formatShare(me.share || 0)}
            </div>
          </div>
          <div>
            <div className='text-muted-foreground text-[11px] uppercase'>
              {t('Growth')}
            </div>
            <GrowthText value={me.growth_pct || 0} />
          </div>
        </div>
      </div>
    </div>
  )
}
