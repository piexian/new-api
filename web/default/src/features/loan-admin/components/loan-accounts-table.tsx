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
import { Search, Wallet } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

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

import { getAdminLoanAccounts } from '../api'

import { QueryErrorState } from './query-error'
import { TablePagination } from './table-pagination'
import { UserCell } from './user-cell'

const PAGE_SIZE = 10

/**
 * Format a day-number (server local loanDay) as a date; 0 = none
 */
function formatDayNumber(day: number): string {
  if (!day || day < 0) return '-'
  return formatTimestamp(day * 86400)
}

export function LoanAccountsTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [keywordInput, setKeywordInput] = useState('')

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['loan-admin-accounts', page, PAGE_SIZE, keyword],
    queryFn: async () => {
      const res = await getAdminLoanAccounts(page, PAGE_SIZE, keyword)
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch loan accounts')
    },
    staleTime: 10000,
  })

  const accounts = data?.items ?? []
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

    if (accounts.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No loan accounts found')}
        </div>
      )
    }

    return (
      <>
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('Principal')}</TableHead>
                <TableHead>{t('Debt')}</TableHead>
                <TableHead>{t('Interest')}</TableHead>
                <TableHead>{t('Max Total')}</TableHead>
                <TableHead>{t('Daily Rate')}</TableHead>
                <TableHead>{t('Interest Free Until')}</TableHead>
                <TableHead>{t('Terms')}</TableHead>
                <TableHead>{t('Total Borrowed')}</TableHead>
                <TableHead>{t('Total Repaid')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {accounts.map((account) => (
                <TableRow key={account.user_id}>
                  <TableCell>
                    <UserCell username={account.username} userId={account.user_id} />
                  </TableCell>
                  <TableCell className='font-medium tabular-nums'>
                    {formatQuotaWithCurrency(account.principal_quota)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(account.debt_now)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(account.interest_now)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {account.custom_max_total > 0
                      ? formatQuotaWithCurrency(account.custom_max_total)
                      : t('Default')}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {account.custom_daily_rate > 0
                      ? formatPercent(account.custom_daily_rate * 100)
                      : t('Default')}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatDayNumber(account.interest_free_until)}
                  </TableCell>
                  <TableCell>
                    {account.terms_agreed_at > 0 ? (
                      <span className='tabular-nums text-xs'>
                        {formatTimestamp(account.terms_agreed_at)}
                      </span>
                    ) : (
                      <span className='text-muted-foreground text-xs'>
                        {t('Not agreed')}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(account.total_borrowed)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(account.total_repaid)}
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
              <Wallet className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
            </IconBadge>
            <div className='min-w-0'>
              <CardTitle className='text-lg'>{t('Loan Accounts')}</CardTitle>
              <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
                {t('Accounts with outstanding or historical token loans')}
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
              placeholder={t('Search by username or user ID')}
              className='h-8 w-48 sm:w-56'
            />
            <Button type='submit' variant='outline' size='sm' className='h-8 px-2.5'>
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