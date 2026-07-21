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
import dayjs from 'dayjs'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

import {
  getProbeGuardIPOffenses,
  getProbeGuardStats,
  resetProbeGuardIPOffense,
  type ProbeIPOffense,
} from '../api'

export function ProbeGuardIPOffensesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const pageSize = 10

  const { data: statsData } = useQuery({
    queryKey: ['risk', 'probe-guard', 'stats'],
    queryFn: async () => {
      const res = await getProbeGuardStats()
      if (res.success) return res.data
      throw new Error(res.message || 'Failed to load stats')
    },
    refetchInterval: 30000,
  })

  const {
    data: offensesData,
    isLoading: offensesLoading,
  } = useQuery({
    queryKey: ['risk', 'probe-guard', 'ip-offenses', { p: page, page_size: pageSize, keyword }],
    queryFn: async () => {
      const res = await getProbeGuardIPOffenses({ p: page, page_size: pageSize, keyword })
      if (res.success) return res.data
      throw new Error(res.message || 'Failed to load IP offenses')
    },
  })

  const resetMutation = useMutation({
    mutationFn: (ip: string) => resetProbeGuardIPOffense(ip),
    onSuccess: () => {
      toast.success(t('IP offense reset successfully'))
      queryClient.invalidateQueries({ queryKey: ['risk', 'probe-guard', 'ip-offenses'] })
      queryClient.invalidateQueries({ queryKey: ['risk', 'probe-guard', 'stats'] })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to reset IP offense'))
    },
  })

  const totalPages = offensesData ? Math.ceil(offensesData.total / pageSize) : 1

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('IP Offenses')}</span>
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
                {statsData?.total_ip_states ?? '-'}
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
                {statsData?.total_user_states ?? '-'}
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
                {statsData?.total_offenses ?? '-'}
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-sm font-medium text-muted-foreground'>
                {t('Recent Offenses')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold'>
                {statsData?.recent_offenses ?? '-'}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Search input */}
        <div className='mb-4'>
          <Input
            placeholder={t('Search...')}
            value={keyword}
            onChange={(e) => {
              setKeyword(e.target.value)
              setPage(1)
            }}
          />
        </div>

        {/* Table */}
        <div className='rounded-md border'>
          <table className='w-full text-sm'>
            <thead>
              <tr className='border-b bg-muted/50'>
                <th className='px-4 py-3 text-left font-medium'>{t('Target IP')}</th>
                <th className='px-4 py-3 text-left font-medium'>{t('Last User ID')}</th>
                <th className='px-4 py-3 text-left font-medium'>{t('Offense Count')}</th>
                <th className='px-4 py-3 text-left font-medium'>{t('Last Offense At')}</th>
                <th className='px-4 py-3 text-left font-medium'>{t('Last Models')}</th>
                <th className='px-4 py-3 text-left font-medium'>{t('Actions')}</th>
              </tr>
            </thead>
            <tbody>
              {/* eslint-disable-next-line no-nested-ternary */}
              {offensesLoading || !offensesData ? (
                <tr>
                  <td colSpan={6} className='px-4 py-8 text-center text-muted-foreground'>
                    {offensesLoading ? t('Loading...') : t('No data')}
                  </td>
                </tr>
              ) : offensesData.items.length === 0 ? (
                <tr>
                  <td colSpan={6} className='px-4 py-8 text-center text-muted-foreground'>
                    {t('No data')}
                  </td>
                </tr>
              ) : (
                offensesData.items.map((offense: ProbeIPOffense) => (
                  <tr key={offense.id} className='border-b last:border-b-0 hover:bg-muted/30'>
                    <td className='px-4 py-3 font-mono text-xs'>{offense.target_ip}</td>
                    <td className='px-4 py-3'>{offense.last_user_id}</td>
                    <td className='px-4 py-3'>{offense.offense_count}</td>
                    <td className='px-4 py-3'>
                      {dayjs.unix(offense.last_offense_at).format('YYYY-MM-DD HH:mm:ss')}
                    </td>
                    <td className='max-w-[200px] truncate px-4 py-3'>
                      <span className='text-xs' title={offense.last_models}>
                        {offense.last_models}
                      </span>
                    </td>
                    <td className='px-4 py-3'>
                      <Button
                        variant='destructive'
                        size='sm'
                        onClick={() => resetMutation.mutate(offense.target_ip)}
                        disabled={resetMutation.isPending}
                      >
                        {t('Reset')}
                      </Button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {offensesData && offensesData.total > 0 && (
          <div className='mt-4 flex items-center justify-between'>
            <div className='text-sm text-muted-foreground'>
              {t('Page {{current}} of {{total}}', { current: page, total: totalPages })}
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
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
