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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { formatQuotaWithCurrency } from '@/lib/currency'

import {
  getCheckinRiskContrast,
  getCheckinRiskWatches,
  releaseCheckinRiskWatch,
  type CheckinDailyContrast,
  type CheckinRiskWatch,
  type CheckinRiskWatchStatus,
  type PageData,
} from '../api'

const PAGE_SIZE = 10

export function CheckinRiskPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState('')

  const [contrastUser, setContrastUser] = useState<CheckinRiskWatch | null>(
    null
  )
  const [releaseUser, setReleaseUser] = useState<CheckinRiskWatch | null>(null)
  const [releaseNote, setReleaseNote] = useState('')
  const [releaseLoading, setReleaseLoading] = useState(false)

  const {
    data: watchesData,
    error: watchesError,
    isLoading,
  } = useQuery({
    queryKey: ['risk', 'checkin-risk', page, status],
    queryFn: async () => {
      const res = await getCheckinRiskWatches({
        p: page,
        page_size: PAGE_SIZE,
        status,
      })
      if (res.success) return res.data
      throw new Error(res.message || t('Failed to load check-in risk watches'))
    },
  })

  const { data: contrastData, isLoading: contrastLoading } = useQuery({
    queryKey: ['risk', 'checkin-risk', 'contrast', contrastUser?.user_id],
    queryFn: async () => {
      if (!contrastUser) return []
      const res = await getCheckinRiskContrast(contrastUser.user_id, 30)
      if (res.success) return res.data
      throw new Error(res.message || t('Failed to load daily contrast'))
    },
    enabled: contrastUser !== null,
  })

  const watches = watchesData as PageData<CheckinRiskWatch> | undefined
  const contrastRows = (contrastData ?? []) as CheckinDailyContrast[]
  const totalPages = watches
    ? Math.max(1, Math.ceil(watches.total / (watches.page_size || PAGE_SIZE)))
    : 1

  const formatTime = (ts: number) =>
    ts ? dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss') : '-'

  const statusBadgeVariant = (
    value: CheckinRiskWatchStatus
  ): 'default' | 'secondary' | 'destructive' | 'outline' => {
    switch (value) {
      case 'watching':
        return 'default'
      case 'locked':
        return 'destructive'
      case 'released':
        return 'secondary'
      default:
        return 'outline'
    }
  }

  const statusLabel = (value: CheckinRiskWatchStatus) => {
    switch (value) {
      case 'watching':
        return t('Watching')
      case 'locked':
        return t('Locked')
      case 'released':
        return t('Released')
      default:
        return value
    }
  }

  const handleRelease = async () => {
    if (!releaseUser) return
    setReleaseLoading(true)
    try {
      const res = await releaseCheckinRiskWatch(releaseUser.user_id, releaseNote)
      if (res.success) {
        toast.success(t('Released successfully'))
        setReleaseUser(null)
        setReleaseNote('')
        queryClient.invalidateQueries({ queryKey: ['risk', 'checkin-risk'] })
      } else {
        toast.error(res.message || t('Release failed'))
      }
    } catch {
      toast.error(t('Release failed'))
    } finally {
      setReleaseLoading(false)
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Check-in Risk')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-6'>
          {watchesError && (
            <div className='border-destructive/40 bg-destructive/10 text-destructive rounded-lg border px-4 py-3 text-sm'>
              {watchesError.message}
            </div>
          )}

          {/* Filters */}
          <div className='flex flex-wrap items-end gap-4'>
            <div className='flex flex-col gap-1.5'>
              <label className='text-sm font-medium'>{t('Status')}</label>
              <select
                className='border-border bg-background block h-9 rounded-lg border px-3 text-sm shadow-xs'
                value={status}
                onChange={(e) => {
                  setStatus(e.target.value)
                  setPage(1)
                }}
              >
                <option value=''>{t('All')}</option>
                <option value='watching'>{t('Watching')}</option>
                <option value='locked'>{t('Locked')}</option>
                <option value='released'>{t('Released')}</option>
              </select>
            </div>
          </div>

          {/* Table */}
          <div className='border-border overflow-x-auto rounded-lg border'>
            <table className='divide-border min-w-full divide-y text-sm'>
              <thead className='bg-muted/50'>
                <tr>
                  <th className='text-muted-foreground px-4 py-3 text-left font-medium'>
                    {t('User ID')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-left font-medium'>
                    {t('Username')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-left font-medium'>
                    {t('Status')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-right font-medium'>
                    {t('Check-in Streak')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-right font-medium'>
                    {t('Avg Calls')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-right font-medium'>
                    {t('Avg Consumption')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-right font-medium'>
                    {t('Avg Check-in Reward')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-left font-medium'>
                    {t('Reason')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-left font-medium'>
                    {t('Updated At')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-right font-medium'>
                    {t('Actions')}
                  </th>
                </tr>
              </thead>
              <tbody className='divide-border divide-y'>
                {/* eslint-disable-next-line no-nested-ternary */}
                {isLoading ? (
                  <tr>
                    <td
                      colSpan={10}
                      className='text-muted-foreground px-4 py-8 text-center'
                    >
                      {t('Loading...')}
                    </td>
                  </tr>
                ) : !watches || watches.items.length === 0 ? (
                  <tr>
                    <td
                      colSpan={10}
                      className='text-muted-foreground px-4 py-8 text-center'
                    >
                      {t('No data')}
                    </td>
                  </tr>
                ) : (
                  watches.items.map((item) => (
                    <tr key={item.id}>
                      <td className='px-4 py-3'>{item.user_id}</td>
                      <td className='px-4 py-3'>{item.username || '-'}</td>
                      <td className='px-4 py-3'>
                        <Badge variant={statusBadgeVariant(item.status)}>
                          {statusLabel(item.status)}
                        </Badge>
                      </td>
                      <td className='px-4 py-3 text-right tabular-nums'>
                        {item.streak_days}
                      </td>
                      <td className='px-4 py-3 text-right tabular-nums'>
                        {item.avg_calls}
                      </td>
                      <td className='px-4 py-3 text-right tabular-nums'>
                        {formatQuotaWithCurrency(item.avg_quota)}
                      </td>
                      <td className='px-4 py-3 text-right tabular-nums'>
                        {formatQuotaWithCurrency(item.avg_awarded)}
                      </td>
                      <td className='max-w-64 truncate px-4 py-3'>
                        {item.reason || '-'}
                      </td>
                      <td className='px-4 py-3'>
                        {formatTime(item.updated_at)}
                      </td>
                      <td className='px-4 py-3 text-right'>
                        <div className='flex justify-end gap-2'>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() => setContrastUser(item)}
                          >
                            {t('Daily Contrast')}
                          </Button>
                          {item.status !== 'released' && (
                            <Button
                              size='sm'
                              onClick={() => {
                                setReleaseUser(item)
                                setReleaseNote('')
                              }}
                            >
                              {t('Release')}
                            </Button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {watches && watches.total > 0 && (
            <div className='flex items-center justify-between'>
              <p className='text-muted-foreground text-sm'>
                {t('Total')}: {watches.total}
              </p>
              <div className='flex items-center gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  {t('Prev')}
                </Button>
                <span className='text-muted-foreground flex items-center px-2 text-sm'>
                  {page} / {totalPages}
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

      {/* Daily Contrast Dialog */}
      <Dialog
        open={contrastUser !== null}
        onOpenChange={(open) => {
          if (!open) setContrastUser(null)
        }}
      >
        <DialogContent className='max-w-3xl'>
          <DialogHeader>
            <DialogTitle>
              {t('Daily Contrast')}
              {contrastUser ? ` - ${contrastUser.username}` : ''}
            </DialogTitle>
            <DialogDescription className='sr-only'>
              {t('Daily Contrast')}
            </DialogDescription>
          </DialogHeader>
          <div className='border-border max-h-[60vh] overflow-auto rounded-lg border'>
            <table className='divide-border min-w-full divide-y text-sm'>
              <thead className='bg-muted/50'>
                <tr>
                  <th className='text-muted-foreground px-4 py-3 text-left font-medium'>
                    {t('Date')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-right font-medium'>
                    {t('Check-in Reward')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-center font-medium'>
                    {t('Make-up')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-right font-medium'>
                    {t('Calls')}
                  </th>
                  <th className='text-muted-foreground px-4 py-3 text-right font-medium'>
                    {t('Consumption')}
                  </th>
                </tr>
              </thead>
              <tbody className='divide-border divide-y'>
                {/* eslint-disable-next-line no-nested-ternary */}
                {contrastLoading ? (
                  <tr>
                    <td
                      colSpan={5}
                      className='text-muted-foreground px-4 py-8 text-center'
                    >
                      {t('Loading...')}
                    </td>
                  </tr>
                ) : contrastRows.length === 0 ? (
                  <tr>
                    <td
                      colSpan={5}
                      className='text-muted-foreground px-4 py-8 text-center'
                    >
                      {t('No data')}
                    </td>
                  </tr>
                ) : (
                  contrastRows.map((row) => (
                    <tr key={row.date}>
                      <td className='px-4 py-3 tabular-nums'>{row.date}</td>
                      <td className='px-4 py-3 text-right tabular-nums'>
                        {formatQuotaWithCurrency(row.quota_awarded)}
                      </td>
                      <td className='px-4 py-3 text-center'>
                        {row.is_makeup ? t('Yes') : t('No')}
                      </td>
                      <td className='px-4 py-3 text-right tabular-nums'>
                        {row.calls}
                      </td>
                      <td className='px-4 py-3 text-right tabular-nums'>
                        {formatQuotaWithCurrency(row.quota)}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </DialogContent>
      </Dialog>

      {/* Release Dialog */}
      <Dialog
        open={releaseUser !== null}
        onOpenChange={(open) => {
          if (!open) setReleaseUser(null)
        }}
      >
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle>
              {t('Release Risk Watch')}
              {releaseUser ? ` - ${releaseUser.username}` : ''}
            </DialogTitle>
            <DialogDescription className='sr-only'>
              {t('Release Risk Watch')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            <Input
              value={releaseNote}
              onChange={(e) => setReleaseNote(e.target.value)}
              placeholder={t('Release note (optional)')}
            />
            <div className='flex justify-end gap-2'>
              <Button variant='outline' onClick={() => setReleaseUser(null)}>
                {t('Cancel')}
              </Button>
              <Button onClick={handleRelease} disabled={releaseLoading}>
                {releaseLoading ? t('Loading...') : t('Confirm')}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </SectionPageLayout>
  )
}
