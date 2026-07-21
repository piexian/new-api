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
import { Input } from '@/components/ui/input'

import {
  getProbeGuardUserOffenses,
  unbanProbeGuardUser,
  type ProbeUserOffense,
} from '../api'

const PAGE_SIZE = 10

export function ProbeGuardUserOffensesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [searchKeyword, setSearchKeyword] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['probeGuardUserOffenses', page, searchKeyword],
    queryFn: async () => {
      const res = await getProbeGuardUserOffenses({
        p: page,
        page_size: PAGE_SIZE,
        keyword: searchKeyword,
      })
      if (res.success) return res.data
      throw new Error(res.message || t('Failed to load user offenses'))
    },
    placeholderData: (prev) => prev,
  })

  const unbanMutation = useMutation({
    mutationFn: (id: number) => unbanProbeGuardUser(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('User unbanned successfully'))
        queryClient.invalidateQueries({
          queryKey: ['probeGuardUserOffenses'],
        })
      } else {
        toast.error(res.message || t('Failed to unban user'))
      }
    },
    onError: () => {
      toast.error(t('Failed to unban user'))
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

  const handleUnban = (item: ProbeUserOffense) => {
    if (window.confirm(t('Are you sure?'))) {
      unbanMutation.mutate(item.id)
    }
  }

  const items = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('User Offenses')}</span>
          <Badge variant='outline' className='shrink-0'>
            Root
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          {/* Search */}
          <div className='flex items-center gap-2'>
            <Input
              placeholder={t('Search by user ID or IP...')}
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              onKeyDown={handleKeyDown}
              className='max-w-sm'
            />
            <Button variant='secondary' onClick={handleSearch}>
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
            <div className='overflow-x-auto rounded-md border'>
              <table className='w-full text-sm'>
                <thead>
                  <tr className='border-b bg-muted/50'>
                    <th className='px-4 py-3 text-left font-medium'>
                      {t('User ID')}
                    </th>
                    <th className='px-4 py-3 text-left font-medium'>
                      {t('Offense Count')}
                    </th>
                    <th className='px-4 py-3 text-left font-medium'>
                      {t('Last Offense At')}
                    </th>
                    <th className='px-4 py-3 text-left font-medium'>
                      {t('Last IP')}
                    </th>
                    <th className='px-4 py-3 text-left font-medium'>
                      {t('Last Models')}
                    </th>
                    <th className='px-4 py-3 text-left font-medium'>
                      {t('Actions')}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((item) => (
                    <tr key={item.id} className='border-b last:border-0 hover:bg-muted/30'>
                      <td className='px-4 py-3 font-mono text-xs'>{item.user_id}</td>
                      <td className='px-4 py-3'>{item.offense_count}</td>
                      <td className='px-4 py-3 text-muted-foreground'>
                        {dayjs.unix(item.last_offense_at).format('YYYY-MM-DD HH:mm:ss')}
                      </td>
                      <td className='px-4 py-3 font-mono text-xs'>{item.last_ip}</td>
                      <td className='px-4 py-3 max-w-[200px] truncate font-mono text-xs'>
                        {item.last_models}
                      </td>
                      <td className='px-4 py-3'>
                        <Button
                          variant='destructive'
                          size='sm'
                          onClick={() => handleUnban(item)}
                          disabled={unbanMutation.isPending}
                        >
                          {t('Unban')}
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Pagination */}
          {totalPages > 1 && (
            <div className='flex items-center justify-between'>
              <div className='text-sm text-muted-foreground'>
                {t('Total: {{total}}', { total })}
              </div>
              <div className='flex items-center gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  {t('Prev')}
                </Button>
                <span className='text-sm text-muted-foreground'>
                  {t('Page {{page}} / {{total}}', { page, total: totalPages })}
                </span>
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
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
