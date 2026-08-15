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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatPercent } from '@/lib/format'

import { getLoanMarketList } from '../api'
import type { MarketOffer } from '../types'
import { QueryErrorState } from './query-error'

interface MarketBrowseProps {
  onBorrow: (offer: MarketOffer) => void
}

export function MarketBrowse(props: MarketBrowseProps) {
  const { t } = useTranslation()

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['loan-market-list'],
    queryFn: async () => {
      const res = await getLoanMarketList()
      if (res.success && res.data) {
        return res.data.offers
      }
      throw new Error(res.message || 'Failed to fetch market offers')
    },
    staleTime: 10000,
  })

  const offers = data ?? []

  const content = (() => {
    if (isError) {
      return (
        <QueryErrorState message={error.message} onRetry={() => refetch()} />
      )
    }

    if (isLoading) {
      return (
        <div className='space-y-2'>
          {['a', 'b', 'c'].map((slot) => (
            <Skeleton key={slot} className='h-14 rounded-lg' />
          ))}
        </div>
      )
    }

    if (offers.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No public offers right now')}
        </div>
      )
    }

    return (
      <div className='space-y-2'>
        {offers.map((offer) => (
          <div
            key={offer.id}
            className='hover:bg-muted/50 grid gap-3 rounded-lg border p-3 transition-colors sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'
          >
            <div className='grid gap-2 text-sm sm:grid-cols-4'>
              <div>
                <p className='text-muted-foreground text-xs'>
                  {t('Available')}
                </p>
                <p className='font-medium tabular-nums'>
                  {formatQuotaWithCurrency(offer.amount_available)}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground text-xs'>
                  {t('Fixed Rate')}
                </p>
                <p className='tabular-nums'>
                  {formatPercent(offer.rate_fixed * 100)}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground text-xs'>
                  {t('Min Credit')}
                </p>
                <p className='tabular-nums'>
                  {offer.min_credit_score === -50
                    ? t('No Limit')
                    : offer.min_credit_score}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground text-xs'>
                  {t('Lender Credit')}
                </p>
                <p className='tabular-nums'>{offer.lender_credit_score}</p>
              </div>
            </div>
            <div className='flex items-center justify-end gap-2'>
              <Badge variant='outline'>
                {t('Order (public listing)')}
              </Badge>
              <Button size='sm' onClick={() => props.onBorrow(offer)}>
                {t('Borrow This')}
              </Button>
            </div>
          </div>
        ))}
      </div>
    )
  })()

  return (
    <>
      <div className='min-w-0'>
        <p className='text-sm font-medium'>{t('Market Browse')}</p>
        <p className='text-muted-foreground mt-0.5 text-xs'>
          {t(
            'Public order offers from other lenders; borrow from one directly.'
          )}
        </p>
      </div>
      <div className='mt-4'>{content}</div>
    </>
  )
}
