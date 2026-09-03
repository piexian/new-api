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
import { Search, Store } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatPercent, formatTimestamp } from '@/lib/format'

import { getAdminLoanOffers } from '../api'
import type { LoanAdminOffer } from '../types'
import { QueryErrorState } from './query-error'
import { TablePagination } from './table-pagination'
import { UserCell } from './user-cell'

const PAGE_SIZE = 10

function modeLabel(
  t: (key: string) => string,
  mode: LoanAdminOffer['mode']
): string {
  if (mode === 'pool') return t('Pool')
  if (mode === 'ai') return t('AI')
  return t('Order (public listing)')
}

function statusLabel(
  t: (key: string) => string,
  status: LoanAdminOffer['status']
): string {
  if (status === 'active') return t('Active')
  if (status === 'paused') return t('Paused')
  return t('Closed')
}

function statusBadgeVariant(
  status: LoanAdminOffer['status']
): 'default' | 'secondary' | 'outline' {
  if (status === 'active') return 'default'
  if (status === 'paused') return 'secondary'
  return 'outline'
}

function formatDailyRate(rate: number): string {
  return formatPercent(rate * 100)
}

export function LoanOffersTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [keywordInput, setKeywordInput] = useState('')

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['loan-admin-offers', page, PAGE_SIZE, keyword],
    queryFn: async () => {
      const res = await getAdminLoanOffers(page, PAGE_SIZE, keyword)
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch loan offers')
    },
    staleTime: 10000,
  })

  const offers = data?.items ?? []
  const total = data?.total ?? 0

  const content = (() => {
    if (isError) {
      return (
        <QueryErrorState message={error.message} onRetry={() => refetch()} />
      )
    }

    if (isLoading) {
      return (
        <div className='space-y-2'>
          {['a', 'b', 'c', 'd'].map((slot) => (
            <Skeleton key={slot} className='h-10 rounded-lg' />
          ))}
        </div>
      )
    }

    if (offers.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No offers found')}
        </div>
      )
    }

    return (
      <>
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('ID')}</TableHead>
                <TableHead>{t('Lender')}</TableHead>
                <TableHead>{t('Mode')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Total Amount')}</TableHead>
                <TableHead>{t('Amount Available')}</TableHead>
                <TableHead>{t('Rate')}</TableHead>
                <TableHead>{t('Per-Loan Cap')}</TableHead>
                <TableHead>{t('Min Credit')}</TableHead>
                <TableHead>{t('Total Lent')}</TableHead>
                <TableHead>{t('Interest Earned')}</TableHead>
                <TableHead>{t('Created At')}</TableHead>
                <TableHead>{t('Updated At')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {offers.map((offer) => (
                <TableRow key={offer.id}>
                  <TableCell className='text-muted-foreground text-xs'>
                    #{offer.id}
                  </TableCell>
                  <TableCell>
                    <UserCell
                      username={offer.username}
                      userId={offer.lender_id}
                    />
                  </TableCell>
                  <TableCell>
                    <Badge variant='outline'>{modeLabel(t, offer.mode)}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusBadgeVariant(offer.status)}>
                      {statusLabel(t, offer.status)}
                    </Badge>
                  </TableCell>
                  <TableCell className='font-medium tabular-nums'>
                    {formatQuotaWithCurrency(offer.amount_total)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(offer.amount_available)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {offer.rate_fixed > 0
                      ? formatDailyRate(offer.rate_fixed)
                      : `${formatDailyRate(offer.rate_min)}-${formatDailyRate(offer.rate_max)}`}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(offer.per_loan_cap)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {offer.min_credit_score === -50
                      ? t('No Limit')
                      : offer.min_credit_score}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(offer.total_lent)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(offer.total_interest_earned)}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatTimestamp(offer.created_at)}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatTimestamp(offer.updated_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>

        <TablePagination
          page={page}
          pageSize={PAGE_SIZE}
          total={total}
          onPageChange={setPage}
        />
      </>
    )
  })()

  return (
    <Card className='gap-0 py-0'>
      <CardHeader className='border-b p-4 sm:p-5'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex items-center gap-3'>
            <IconBadge tone='neutral' size='lg'>
              <Store className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
            </IconBadge>
            <div className='min-w-0'>
              <CardTitle className='text-lg'>{t('Offers')}</CardTitle>
              <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
                {t('All lending offers on the market')}
              </p>
            </div>
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              setKeyword(keywordInput.trim())
              setPage(1)
            }}
            className='flex items-center gap-2'
          >
            <Input
              value={keywordInput}
              onChange={(e) => setKeywordInput(e.target.value)}
              placeholder={t('Search by lender username or ID')}
              className='h-8 w-48 sm:w-56'
            />
            <Button
              type='submit'
              variant='outline'
              size='sm'
              className='h-8 px-2.5'
            >
              <Search className='h-3.5 w-3.5' />
              {t('Search')}
            </Button>
          </form>
        </div>
      </CardHeader>
      <CardContent className='p-4 sm:p-5'>{content}</CardContent>
    </Card>
  )
}
