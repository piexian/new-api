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
import { Search, Send } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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

import { getAdminLoanFundings } from '../api'
import type { LoanAdminFunding } from '../types'

import { QueryErrorState } from './query-error'
import { TablePagination } from './table-pagination'
import { UserCell } from './user-cell'

const PAGE_SIZE = 10

const FUNDING_STATUS_OPTIONS = ['all', 'active', 'overdue', 'repaid', 'written_off'] as const

type FundingStatusFilter = (typeof FUNDING_STATUS_OPTIONS)[number]

function sourceLabel(
  t: (key: string) => string,
  source: LoanAdminFunding['source_type']
): string {
  if (source === 'pool') return t('Pool')
  if (source === 'ai') return t('AI')
  if (source === 'order') return t('Order (public listing)')
  return t('Platform')
}

function statusLabel(
  t: (key: string) => string,
  status: FundingStatusFilter
): string {
  if (status === 'all') return t('All Statuses')
  if (status === 'active') return t('Active')
  if (status === 'overdue') return t('Overdue')
  if (status === 'repaid') return t('Repaid')
  return t('Written Off')
}

function statusBadgeVariant(
  status: LoanAdminFunding['status']
): 'default' | 'secondary' | 'outline' | 'destructive' {
  if (status === 'overdue') return 'destructive'
  if (status === 'repaid') return 'secondary'
  if (status === 'written_off') return 'outline'
  return 'default'
}

function repayPlanLabel(
  t: (key: string) => string,
  plan: LoanAdminFunding['repay_plan']
): string {
  if (plan === 'full') return t('Full (compounding)')
  if (plan === 'no_penalty') return t('No Penalty')
  if (plan === 'interest_freeze') return t('Interest Freeze')
  return t('Principal Only')
}

function formatDailyRate(rate: number): string {
  return formatPercent(rate * 100)
}

// last_settled_day / due_day / penalty_started_day 为服务器本地日序号（unix/86400）
function formatLoanDay(day: number): string {
  if (!day || day < 0) return '-'
  return formatTimestamp(day * 86400)
}

export function LoanFundingsTable() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [lenderId, setLenderId] = useState('')
  const [lenderIdInput, setLenderIdInput] = useState('')
  const [loanUserId, setLoanUserId] = useState('')
  const [loanUserIdInput, setLoanUserIdInput] = useState('')
  const [status, setStatus] = useState<FundingStatusFilter>('all')

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: [
      'loan-admin-fundings',
      page,
      PAGE_SIZE,
      lenderId,
      loanUserId,
      status,
    ],
    queryFn: async () => {
      const res = await getAdminLoanFundings(
        page,
        PAGE_SIZE,
        lenderId,
        loanUserId,
        status === 'all' ? '' : status
      )
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch loan fundings')
    },
    staleTime: 10000,
  })

  const fundings = data?.items ?? []
  const total = data?.total ?? 0

  const applyFilters = () => {
    setPage(1)
    setLenderId(lenderIdInput.trim())
    setLoanUserId(loanUserIdInput.trim())
  }

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

    if (fundings.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No fundings found')}
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
                <TableHead>{t('Borrower')}</TableHead>
                <TableHead>{t('Source')}</TableHead>
                <TableHead>{t('Amount')}</TableHead>
                <TableHead>{t('Principal Remaining')}</TableHead>
                <TableHead>{t('Debt')}</TableHead>
                <TableHead>{t('Rate')}</TableHead>
                <TableHead>{t('Repay Plan')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Due Date')}</TableHead>
                <TableHead>{t('Created At')}</TableHead>
                <TableHead>{t('Updated At')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {fundings.map((funding) => (
                <TableRow key={funding.id}>
                  <TableCell className='text-muted-foreground text-xs'>
                    #{funding.id}
                  </TableCell>
                  <TableCell>
                    <UserCell
                      username={funding.lender_username}
                      userId={funding.lender_id}
                    />
                  </TableCell>
                  <TableCell>
                    <UserCell
                      username={funding.borrower_username}
                      userId={funding.loan_user_id}
                    />
                  </TableCell>
                  <TableCell>
                    <Badge variant='outline'>
                      {sourceLabel(t, funding.source_type)}
                    </Badge>
                  </TableCell>
                  <TableCell className='font-medium tabular-nums'>
                    {formatQuotaWithCurrency(funding.amount)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(funding.principal_remaining)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatQuotaWithCurrency(funding.debt_quota)}
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {formatDailyRate(funding.rate)}
                  </TableCell>
                  <TableCell>
                    <span className='text-xs'>
                      {repayPlanLabel(t, funding.repay_plan)}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusBadgeVariant(funding.status)}>
                      {statusLabel(t, funding.status)}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatLoanDay(funding.due_day)}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatTimestamp(funding.created_at)}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatTimestamp(funding.updated_at)}
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
              <Send className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
            </IconBadge>
            <div className='min-w-0'>
              <CardTitle className='text-lg'>{t('Fundings')}</CardTitle>
              <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
                {t('All lending fundings on the market')}
              </p>
            </div>
          </div>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              applyFilters()
            }}
            className='flex flex-wrap items-center gap-2'
          >
            <Input
              value={lenderIdInput}
              onChange={(e) => setLenderIdInput(e.target.value)}
              placeholder={t('Lender ID')}
              className='h-8 w-28 sm:w-32'
            />
            <Input
              value={loanUserIdInput}
              onChange={(e) => setLoanUserIdInput(e.target.value)}
              placeholder={t('Borrower ID')}
              className='h-8 w-28 sm:w-32'
            />
            <Select
              value={status}
              onValueChange={(value) => setStatus(value as FundingStatusFilter)}
            >
              <SelectTrigger className='h-8 w-32'>
                <SelectValue placeholder={t('All Statuses')} />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {FUNDING_STATUS_OPTIONS.map((option) => (
                    <SelectItem key={option} value={option}>
                      {statusLabel(t, option)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
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
