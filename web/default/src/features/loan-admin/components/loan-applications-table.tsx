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
import { Bot, Search } from 'lucide-react'
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
import { formatTimestamp } from '@/lib/format'

import { getAdminLoanApplications } from '../api'
import { QueryErrorState } from './query-error'
import { TablePagination } from './table-pagination'
import { UserCell } from './user-cell'

const PAGE_SIZE = 10

const STATUS_OPTIONS = ['all', 'open', 'closed'] as const

function useTopicLabel() {
  const { t } = useTranslation()
  return (topic: string) => {
    if (topic === 'credit') return t('Credit Limit Increase')
    if (topic === 'rate') return t('Interest Rate Reduction')
    if (topic === 'grace') return t('Grace Period Extension')
    return t('Other')
  }
}

function useStatusLabel() {
  const { t } = useTranslation()
  return (status: (typeof STATUS_OPTIONS)[number]) => {
    if (status === 'all') return t('All Statuses')
    if (status === 'open') return t('Open')
    return t('Closed')
  }
}

export function LoanApplicationsTable() {
  const { t } = useTranslation()
  const topicLabel = useTopicLabel()
  const statusLabel = useStatusLabel()
  const [page, setPage] = useState(1)
  const [userId, setUserId] = useState('')
  const [userIdInput, setUserIdInput] = useState('')
  const [status, setStatus] = useState<(typeof STATUS_OPTIONS)[number]>('all')

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['loan-admin-applications', page, PAGE_SIZE, userId, status],
    queryFn: async () => {
      const res = await getAdminLoanApplications(
        page,
        PAGE_SIZE,
        userId,
        status === 'all' ? '' : status
      )
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch loan applications')
    },
    staleTime: 10000,
  })

  const applications = data?.items ?? []
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

    if (applications.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No loan applications found')}
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
                <TableHead>{t('User')}</TableHead>
                <TableHead>{t('Topic')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Rating')}</TableHead>
                <TableHead>{t('Rating Comment')}</TableHead>
                <TableHead>{t('Created At')}</TableHead>
                <TableHead>{t('Updated At')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {applications.map((application) => (
                <TableRow key={application.id}>
                  <TableCell className='tabular-nums'>
                    #{application.id}
                  </TableCell>
                  <TableCell>
                    <UserCell
                      username={application.username}
                      userId={application.user_id}
                    />
                  </TableCell>
                  <TableCell>{topicLabel(application.topic)}</TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        application.status === 'open' ? 'default' : 'secondary'
                      }
                    >
                      {application.status === 'open' ? t('Open') : t('Closed')}
                    </Badge>
                  </TableCell>
                  <TableCell className='tabular-nums'>
                    {application.rating > 0
                      ? `${application.rating}/5`
                      : t('Unrated')}
                  </TableCell>
                  <TableCell className='text-muted-foreground max-w-64'>
                    {application.rating_comment || '-'}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatTimestamp(application.created_at)}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-xs'>
                    {formatTimestamp(application.updated_at)}
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
              <Bot className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
            </IconBadge>
            <div className='min-w-0'>
              <CardTitle className='text-lg'>
                {t('Loan Applications')}
              </CardTitle>
              <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
                {t('AI loan officer applications')}
              </p>
            </div>
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <Select
              value={status}
              onValueChange={(value) => {
                if (value !== null) {
                  setStatus(value as (typeof STATUS_OPTIONS)[number])
                  setPage(1)
                }
              }}
            >
              <SelectTrigger className='h-8 w-28'>
                <SelectValue>{statusLabel(status)}</SelectValue>
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {STATUS_OPTIONS.map((option) => (
                    <SelectItem key={option} value={option}>
                      {statusLabel(option)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                setUserId(userIdInput.trim())
                setPage(1)
              }}
              className='flex items-center gap-2'
            >
              <Input
                value={userIdInput}
                onChange={(e) => setUserIdInput(e.target.value)}
                placeholder={t('Filter by user ID')}
                className='h-8 w-40 sm:w-44'
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
        </div>
      </CardHeader>
      <CardContent className='p-4 sm:p-5'>{content}</CardContent>
    </Card>
  )
}
