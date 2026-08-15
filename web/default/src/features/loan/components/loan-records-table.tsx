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
import { ChevronLeft, ChevronRight, ReceiptText } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
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
import { formatTimestamp } from '@/lib/format'

import { getLoanRecords } from '../api'
import type { LoanRecord } from '../types'

import { QueryErrorState } from './query-error'

const PAGE_SIZE = 10

function useTypeLabel() {
  const { t } = useTranslation()
  return (type: LoanRecord['type']) => {
    if (type === 'borrow') return t('Borrow')
    if (type === 'credit') return t('Credit Change')
    return t('Repay')
  }
}

function useSourceLabel() {
  const { t } = useTranslation()
  return (source: LoanRecord['source']) => {
    if (source === 'checkin') return t('Check-in')
    if (source === 'ai') return t('AI Officer')
    if (source === 'repay_bonus') return t('Repay Bonus')
    if (source === 'fast_repay') return t('Fast Repay Penalty')
    if (source === 'writeoff') return t('Write-off')
    return t('Manual')
  }
}

// credit 行的 Amount 是带符号的信用分变动 delta（+5/-2/-20），DebtAfter 是变动后信用分
function formatSignedDelta(value: number) {
  return value > 0 ? `+${value}` : `${value}`
}

export function LoanRecordsTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const typeLabel = useTypeLabel()
  const sourceLabel = useSourceLabel()

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['loan-records', page, PAGE_SIZE],
    queryFn: async () => {
      const res = await getLoanRecords(page, PAGE_SIZE)
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch loan records')
    },
    staleTime: 10000,
  })

  const records = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const content = (() => {
    if (isError) {
      return (
        <QueryErrorState
          message={error.message}
          onRetry={() => refetch()}
        />
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

    if (records.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No loan records yet')}
        </div>
      )
    }

    return (
      <>
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Time')}</TableHead>
                <TableHead>{t('Type')}</TableHead>
                <TableHead>{t('Amount')}</TableHead>
                <TableHead>{t('Interest Part')}</TableHead>
                <TableHead>{t('Principal Part')}</TableHead>
                <TableHead>{t('Fee')}</TableHead>
                <TableHead>{t('Debt After')}</TableHead>
                <TableHead>{t('Source')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.map((record) => (
                <TableRow key={record.id}>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatTimestamp(record.created_at)}
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        record.type === 'borrow' ? 'default' : 'secondary'
                      }
                    >
                      {typeLabel(record.type)}
                    </Badge>
                  </TableCell>
                  <TableCell className='font-medium tabular-nums'>
                    {record.type === 'credit'
                      ? formatSignedDelta(record.amount)
                      : formatQuotaWithCurrency(record.amount)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(record.interest_part)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(record.principal_part)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(record.fee_part)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {record.type === 'credit'
                      ? String(record.debt_after)
                      : formatQuotaWithCurrency(record.debt_after)}
                  </TableCell>
                  <TableCell className='text-muted-foreground'>
                    {sourceLabel(record.source)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>

        <div className='flex flex-col items-center gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground text-xs sm:text-sm'>
            {t('Showing')} {(page - 1) * PAGE_SIZE + 1}-
            {Math.min(page * PAGE_SIZE, total)} {t('of')} {total}
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => p - 1)}
              disabled={page <= 1}
              className='h-8 w-8 p-0'
              aria-label={t('Previous page')}
            >
              <ChevronLeft className='h-4 w-4' />
            </Button>
            <div className='text-muted-foreground flex items-center gap-1 text-sm'>
              <span className='font-medium'>{page}</span>
              <span>/</span>
              <span>{totalPages}</span>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => p + 1)}
              disabled={page >= totalPages}
              className='h-8 w-8 p-0'
              aria-label={t('Next page')}
            >
              <ChevronRight className='h-4 w-4' />
            </Button>
          </div>
        </div>
      </>
    )
  })()

  return (
    <Card className='gap-0 py-0'>
      <CardHeader className='border-b p-4 sm:p-5'>
        <div className='flex items-center gap-3'>
          <IconBadge tone='neutral' size='lg'>
            <ReceiptText className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
          </IconBadge>
          <div className='min-w-0'>
            <CardTitle className='text-lg'>{t('Loan Records')}</CardTitle>
            <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
              {t('Borrow and repay history')}
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent className='p-4 sm:p-5'>{content}</CardContent>
    </Card>
  )
}
