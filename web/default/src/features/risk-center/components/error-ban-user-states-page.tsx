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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from 'dayjs'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

import {
  getErrorBanUserStates,
  getErrorBanStats,
  resetErrorBanUserState,
  type ErrorBanUserState,
} from '../api'

export function ErrorBanUserStatesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [searchKeyword, setSearchKeyword] = useState('')
  const [page, setPage] = useState(1)
  const pageSize = 10

  const { data: userStatesData, isLoading } = useQuery({
    queryKey: ['risk', 'error-ban', 'user-states', page, searchKeyword],
    queryFn: async () => {
      const res = await getErrorBanUserStates({
        p: page,
        page_size: pageSize,
        keyword: searchKeyword || undefined,
      })
      if (res.success) return res.data
      throw new Error(res.message || t('Failed to load user states'))
    },
  })

  const { data: statsData } = useQuery({
    queryKey: ['risk', 'error-ban', 'stats'],
    queryFn: async () => {
      const res = await getErrorBanStats()
      if (res.success) return res.data
      throw new Error(res.message || 'Failed to load stats')
    },
    refetchInterval: 30000,
  })

  const resetMutation = useMutation({
    mutationFn: (id: number) => resetErrorBanUserState(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('User state reset successfully'))
        queryClient.invalidateQueries({
          queryKey: ['risk', 'error-ban', 'user-states'],
        })
        queryClient.invalidateQueries({
          queryKey: ['risk', 'error-ban', 'stats'],
        })
      } else {
        toast.error(res.message || t('Failed to reset user state'))
      }
    },
    onError: () => {
      toast.error(t('Failed to reset user state'))
    },
  })

  const handleSearch = () => {
    setPage(1)
    setSearchKeyword(keyword)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch()
    }
  }

  const items = userStatesData?.items ?? []
  const total = userStatesData?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const stats = statsData

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('User States')}</span>
          <Badge variant='outline' className='shrink-0'>
            Root
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {/* Stats cards */}
        <div className='mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-sm font-medium text-muted-foreground'>
                {t('IP States')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {stats?.total_ip_states ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-sm font-medium text-muted-foreground'>
                {t('User States')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {stats?.total_user_states ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-sm font-medium text-muted-foreground'>
                {t('Total Offenses')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {stats?.total_offenses ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-sm font-medium text-muted-foreground'>
                {t('Active Rules')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {stats?.active_rules ?? '-'}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Search */}
        <div className='mb-4 flex items-center gap-2'>
          <Input
            placeholder={t('Search by User ID...')}
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={handleKeyDown}
            className='max-w-xs'
          />
          <Button onClick={handleSearch} variant='secondary'>
            {t('Search')}
          </Button>
        </div>

        {/* Table */}
        {/* eslint-disable-next-line no-nested-ternary */}
        {isLoading ? (
          <div className='flex items-center justify-center py-12 text-muted-foreground'>
            {t('Loading...')}
          </div>
        ) : items.length === 0 ? (
          <div className='flex items-center justify-center py-12 text-muted-foreground'>
            {t('No data')}
          </div>
        ) : (
          <>
            <div className='overflow-x-auto rounded-md border'>
              <table className='w-full text-left text-sm'>
                <thead>
                  <tr className='border-b bg-muted/50'>
                    <th className='px-4 py-3 font-medium'>{t('User ID')}</th>
                    <th className='px-4 py-3 font-medium'>{t('Rule ID')}</th>
                    <th className='px-4 py-3 font-medium'>
                      {t('Offense Count')}
                    </th>
                    <th className='px-4 py-3 font-medium'>
                      {t('Window Count')}
                    </th>
                    <th className='px-4 py-3 font-medium'>
                      {t('Window Start')}
                    </th>
                    <th className='px-4 py-3 font-medium'>
                      {t('Last Error')}
                    </th>
                    <th className='px-4 py-3 font-medium'>
                      {t('Last Offense At')}
                    </th>
                    <th className='px-4 py-3 font-medium'>{t('Actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((item: ErrorBanUserState) => (
                    <tr
                      key={item.id}
                      className='border-b last:border-0 hover:bg-muted/30'
                    >
                      <td className='px-4 py-3 font-mono text-xs'>
                        {item.user_id}
                      </td>
                      <td className='px-4 py-3 font-mono text-xs'>
                        {item.rule_id}
                      </td>
                      <td className='px-4 py-3'>{item.offense_count}</td>
                      <td className='px-4 py-3'>{item.window_count}</td>
                      <td className='px-4 py-3 text-xs'>
                        {dayjs.unix(item.window_start).format('YYYY-MM-DD HH:mm:ss')}
                      </td>
                      <td className='max-w-[200px] truncate px-4 py-3 font-mono text-xs'>
                        {item.last_error}
                      </td>
                      <td className='px-4 py-3 text-xs'>
                        {dayjs.unix(item.last_offense_at).format('YYYY-MM-DD HH:mm:ss')}
                      </td>
                      <td className='px-4 py-3'>
                        <Button
                          size='sm'
                          variant='outline'
                          onClick={() => resetMutation.mutate(item.user_id)}
                          disabled={resetMutation.isPending}
                        >
                          {t('Reset')}
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            <div className='mt-4 flex items-center justify-between'>
              <div className='text-sm text-muted-foreground'>
                {t('Page {{page}} of {{total}}', {
                  page,
                  total: totalPages,
                })}
              </div>
              <div className='flex items-center gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  {t('Previous')}
                </Button>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                >
                  {t('Next')}
                </Button>
              </div>
            </div>
          </>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
