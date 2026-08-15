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
import { useQuery } from '@tanstack/react-query'
import {
  Clock3,
  HandCoins,
  Landmark,
  Snowflake,
  Store,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuotaWithCurrency } from '@/lib/currency'

import { getAdminLoanMarketOverview } from '../api'

import { QueryErrorState } from './query-error'

function offerStatusLabel(
  t: (key: string) => string,
  status: string
): string {
  if (status === 'active') return t('Active')
  if (status === 'paused') return t('Paused')
  return t('Closed')
}

/**
 * Lending market aggregate overview cards (admin read-only)
 */
export function MarketOverviewCards() {
  const { t } = useTranslation()

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['loan-admin-market-overview'],
    queryFn: async () => {
      const res = await getAdminLoanMarketOverview()
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch market overview')
    },
    staleTime: 15000,
  })

  if (isError) {
    return (
      <QueryErrorState message={error.message} onRetry={() => refetch()} />
    )
  }

  if (isLoading || !data) {
    return (
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-5'>
        {['a', 'b', 'c', 'd', 'e'].map((slot) => (
          <Skeleton key={slot} className='h-28 rounded-xl' />
        ))}
      </div>
    )
  }

  const stats: {
    label: string
    value: string
    icon: LucideIcon
    tone: IconBadgeTone
  }[] = [
    {
      label: t('Frozen Idle'),
      value: formatQuotaWithCurrency(data.frozen_idle),
      icon: Snowflake,
      tone: 'info',
    },
    {
      label: t('In-Loan Principal'),
      value: formatQuotaWithCurrency(data.in_loan_principal),
      icon: Landmark,
      tone: 'chart-2',
    },
    {
      label: t('Interest Earned'),
      value: formatQuotaWithCurrency(data.total_interest_earned),
      icon: HandCoins,
      tone: 'success',
    },
    {
      label: t('Overdue Fundings'),
      value: data.overdue_fundings.toLocaleString(),
      icon: Clock3,
      tone: 'destructive',
    },
    {
      label: t('Active Offers'),
      value: data.active_offers.toLocaleString(),
      icon: Store,
      tone: 'chart-4',
    },
  ]

  return (
    <Card className='gap-0 py-0'>
      <CardHeader className='border-b p-4 sm:p-5'>
        <CardTitle className='text-lg'>{t('Market Overview')}</CardTitle>
      </CardHeader>
      <CardContent className='p-4 sm:p-5'>
        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-5'>
          {stats.map((item) => (
            <div
              key={item.label}
              className='border-muted flex min-w-0 flex-col gap-1.5 rounded-lg border p-3'
            >
              <div className='flex items-center gap-1.5 sm:gap-2'>
                <IconBadge tone={item.tone} size='sm'>
                  <item.icon />
                </IconBadge>
                <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wide sm:text-xs'>
                  {item.label}
                </div>
              </div>
              <div className='text-foreground font-mono text-base font-bold break-all tabular-nums sm:text-lg'>
                {item.value}
              </div>
            </div>
          ))}
        </div>
        {Object.keys(data.offers_by_status).length > 0 ? (
          <p className='text-muted-foreground mt-3 text-xs sm:text-sm'>
            {t('Offers by status')}:{' '}
            {Object.entries(data.offers_by_status)
              .map(
                ([status, count]) =>
                  `${offerStatusLabel(t, status)}: ${count.toLocaleString()}`
              )
              .join(' · ')}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
