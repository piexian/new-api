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
import { Ban, Eye, Fingerprint, Search } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

import {
  banSharedSessionUsers,
  getSessionRiskConfig,
  getSharedSessions,
  updateSessionRiskRecording,
  type SessionRiskItem,
  type SessionRiskKind,
} from '../api'

const PAGE_SIZE = 10

const WINDOW_OPTIONS = [
  { label: 'Last 24 Hours', seconds: 24 * 3600 },
  { label: 'Last 7 Days', seconds: 7 * 24 * 3600 },
  { label: 'Last 30 Days', seconds: 30 * 24 * 3600 },
] as const

function formatTime(timestamp: number) {
  return timestamp ? dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm:ss') : '-'
}

export function SessionRiskPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [kind, setKind] = useState<SessionRiskKind>('session')
  const [windowSeconds, setWindowSeconds] = useState<number>(
    WINDOW_OPTIONS[2].seconds
  )
  const [page, setPage] = useState(1)
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [selectedItem, setSelectedItem] = useState<SessionRiskItem | null>(null)
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([])
  const [banReason, setBanReason] = useState('')
  const [banDuration, setBanDuration] = useState(0)

  const configQuery = useQuery({
    queryKey: ['risk', 'session', 'config'],
    queryFn: async () => {
      const response = await getSessionRiskConfig()
      if (!response.success) {
        throw new Error(response.message || t('Failed to load data'))
      }
      return response.data
    },
  })

  const recordingMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      const response = await updateSessionRiskRecording(enabled)
      if (!response.success) {
        throw new Error(response.message || t('Failed to save settings'))
      }
      return response.data
    },
    onSuccess: async () => {
      toast.success(t('Settings saved'))
      await queryClient.invalidateQueries({
        queryKey: ['risk', 'session', 'config'],
      })
    },
    onError: (mutationError: Error) => toast.error(mutationError.message),
  })

  const now = Math.floor(Date.now() / 1000)
  const start = now - windowSeconds
  const { data, error, isLoading } = useQuery({
    queryKey: ['risk', 'session', 'shared', kind, windowSeconds, page, keyword],
    queryFn: async () => {
      const response = await getSharedSessions({
        kind,
        p: page,
        page_size: PAGE_SIZE,
        keyword,
        start_timestamp: start,
        end_timestamp: now,
      })
      if (!response.success) {
        throw new Error(response.message || t('Failed to load data'))
      }
      return response.data
    },
    refetchInterval: 60000,
  })

  const banMutation = useMutation({
    mutationFn: async () => {
      if (!selectedItem || !data) throw new Error(t('No data'))
      const response = await banSharedSessionUsers({
        kind,
        identifier: selectedItem.identifier,
        start: data.start,
        end: data.end,
        user_ids: selectedUserIds,
        reason: banReason.trim(),
        duration_minutes: banDuration,
      })
      if (!response.success) {
        throw new Error(response.message || t('Ban failed'))
      }
      return response.data
    },
    onSuccess: async (result) => {
      const failures = result?.failures ?? []
      if (failures.length > 0) {
        toast.warning(
          t('Ban completed with failures', {
            success: result?.success ?? 0,
            failed: failures.length,
          })
        )
      } else {
        toast.success(t('Account banned successfully'))
      }
      setSelectedItem(null)
      setSelectedUserIds([])
      await queryClient.invalidateQueries({
        queryKey: ['risk', 'session', 'shared'],
      })
      await queryClient.invalidateQueries({ queryKey: ['risk', 'ban-logs'] })
    },
    onError: (mutationError: Error) => toast.error(mutationError.message),
  })

  const applySearch = () => {
    setKeyword(keywordInput.trim())
    setPage(1)
  }

  const openReviewDialog = (item: SessionRiskItem) => {
    setSelectedItem(item)
    setSelectedUserIds(item.users.map((user) => user.user_id))
    setBanReason(
      kind === 'session'
        ? t('Administrator confirmed shared session ID abuse')
        : t('Administrator confirmed shared cache key abuse')
    )
    setBanDuration(0)
  }

  const toggleUser = (userId: number, checked: boolean) => {
    setSelectedUserIds((current) =>
      checked
        ? [...new Set([...current, userId])]
        : current.filter((id) => id !== userId)
    )
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1

  const kindLabel = (value: string) =>
    value === 'cache' ? t('Cache Key') : t('Session ID')

  const renderItemCards = () => (
    <div className='grid gap-3 sm:hidden'>
      {isLoading && (
        <div className='border-border text-muted-foreground rounded-lg border px-4 py-10 text-center text-sm'>
          {t('Loading...')}
        </div>
      )}
      {!isLoading && !data?.items.length && (
        <div className='border-border text-muted-foreground rounded-lg border px-4 py-10 text-center text-sm'>
          {t('No data')}
        </div>
      )}
      {!isLoading &&
        data?.items.map((item) => (
          <Card key={item.identifier} className='min-w-0'>
            <CardHeader className='gap-3 pb-3'>
              <div className='flex min-w-0 items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {kindLabel(item.kind)}
                  </div>
                  <CardTitle className='mt-1 font-mono text-sm break-all'>
                    {item.identifier}
                  </CardTitle>
                </div>
                <Badge variant='destructive'>
                  {t('Users')}: {item.user_count}
                </Badge>
              </div>
            </CardHeader>
            <CardContent className='min-w-0 space-y-3 text-sm'>
              <div className='flex flex-wrap gap-1.5'>
                {item.users.map((user) => (
                  <Badge
                    key={user.user_id}
                    variant='outline'
                    className='h-auto max-w-full whitespace-normal'
                  >
                    #{user.user_id} {user.username}
                  </Badge>
                ))}
              </div>
              <div className='text-muted-foreground flex items-center justify-between'>
                <span>{t('Requests')}</span>
                <span>
                  {item.request_count} ({t('Errors')}: {item.error_count})
                </span>
              </div>
              <div className='text-muted-foreground flex items-center justify-between'>
                <span>{t('Last Seen')}</span>
                <span>{formatTime(item.last_seen_at)}</span>
              </div>
              <Button
                variant='outline'
                className='w-full'
                onClick={() => openReviewDialog(item)}
              >
                <Eye data-icon='inline-start' className='size-4' />
                {t('Review')}
              </Button>
            </CardContent>
          </Card>
        ))}
    </div>
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Session Risk')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='min-w-0 space-y-6'>
          {error && (
            <div className='border-destructive/40 bg-destructive/10 text-destructive rounded-lg border px-4 py-3 text-sm'>
              {error.message}
            </div>
          )}

          <Card className='min-w-0'>
            <CardHeader className='pb-2'>
              <CardTitle className='text-sm font-medium'>
                {t('Record Session Identifiers')}
              </CardTitle>
            </CardHeader>
            <CardContent className='space-y-3'>
              <div className='flex items-center gap-3'>
                <Switch
                  checked={configQuery.data?.session_recording_enabled ?? false}
                  disabled={
                    configQuery.isLoading || recordingMutation.isPending
                  }
                  onCheckedChange={(checked) =>
                    recordingMutation.mutate(checked)
                  }
                />
                <span className='text-sm'>
                  {configQuery.data?.session_recording_enabled
                    ? t('Recording enabled')
                    : t('Recording disabled')}
                </span>
              </div>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'When enabled, session IDs and cache affinity identifiers from original client requests are recorded in logs for shared-session risk analysis. Existing logs are not backfilled.'
                )}
              </p>
            </CardContent>
          </Card>

          <div className='flex flex-col gap-3 lg:flex-row lg:items-center'>
            <div className='flex gap-2'>
              {(['session', 'cache'] as const).map((value) => (
                <Button
                  key={value}
                  variant={kind === value ? 'default' : 'outline'}
                  size='sm'
                  onClick={() => {
                    setKind(value)
                    setPage(1)
                  }}
                >
                  {kindLabel(value)}
                </Button>
              ))}
            </div>
            <select
              className='border-input bg-background h-9 rounded-lg border px-3 text-sm shadow-xs'
              value={windowSeconds}
              onChange={(event) => {
                setWindowSeconds(Number(event.target.value))
                setPage(1)
              }}
            >
              {WINDOW_OPTIONS.map((option) => (
                <option key={option.seconds} value={option.seconds}>
                  {t(option.label)}
                </option>
              ))}
            </select>
            <div className='relative w-full max-w-xl'>
              <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
              <Input
                className='pl-9'
                value={keywordInput}
                onChange={(event) => setKeywordInput(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && applySearch()}
                placeholder={t('Search identifier, account, token, or IP')}
              />
            </div>
            <Button className='w-full sm:w-auto' onClick={applySearch}>
              <Search data-icon='inline-start' className='size-4' />
              {t('Search')}
            </Button>
          </div>

          {renderItemCards()}

          <div className='border-border hidden max-w-full overflow-x-auto rounded-lg border sm:block'>
            <table className='divide-border min-w-[64rem] table-fixed divide-y text-sm'>
              <thead className='bg-muted/50'>
                <tr>
                  <th className='text-muted-foreground min-w-60 px-4 py-3 text-left font-medium'>
                    {kindLabel(kind)}
                  </th>
                  <th className='text-muted-foreground min-w-56 px-4 py-3 text-left font-medium'>
                    {t('Related Accounts')}
                  </th>
                  <th className='text-muted-foreground w-28 px-4 py-3 text-left font-medium'>
                    {t('Requests')}
                  </th>
                  <th className='text-muted-foreground w-28 px-4 py-3 text-left font-medium'>
                    {t('Errors')}
                  </th>
                  <th className='text-muted-foreground w-44 px-4 py-3 text-left font-medium'>
                    {t('Last Seen')}
                  </th>
                  <th className='text-muted-foreground w-28 px-4 py-3 text-right font-medium'>
                    {t('Actions')}
                  </th>
                </tr>
              </thead>
              <tbody className='divide-border divide-y'>
                {/* eslint-disable-next-line no-nested-ternary */}
                {isLoading ? (
                  <tr>
                    <td
                      colSpan={6}
                      className='text-muted-foreground px-4 py-10 text-center'
                    >
                      {t('Loading...')}
                    </td>
                  </tr>
                ) : !data?.items.length ? (
                  <tr>
                    <td
                      colSpan={6}
                      className='text-muted-foreground px-4 py-10 text-center'
                    >
                      {t('No data')}
                    </td>
                  </tr>
                ) : (
                  data.items.map((item) => (
                    <tr key={item.identifier} className='hover:bg-muted/30'>
                      <td className='px-4 py-3'>
                        <div className='font-mono text-xs break-all'>
                          {item.identifier}
                        </div>
                        <div className='text-muted-foreground mt-1 flex flex-wrap gap-1'>
                          {item.sources.map((source) => (
                            <Badge key={source} variant='outline'>
                              {source}
                            </Badge>
                          ))}
                        </div>
                      </td>
                      <td className='px-4 py-3'>
                        <div className='flex flex-wrap gap-1.5'>
                          {item.users.map((user) => (
                            <Badge
                              key={user.user_id}
                              variant='outline'
                              className='h-auto max-w-full whitespace-normal'
                            >
                              #{user.user_id} {user.username}
                            </Badge>
                          ))}
                        </div>
                      </td>
                      <td className='px-4 py-3'>{item.request_count}</td>
                      <td className='px-4 py-3'>{item.error_count}</td>
                      <td className='px-4 py-3'>
                        {formatTime(item.last_seen_at)}
                      </td>
                      <td className='px-4 py-3 text-right'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => openReviewDialog(item)}
                        >
                          <Eye data-icon='inline-start' className='size-4' />
                          {t('Review')}
                        </Button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>

          {data && data.total > 0 && (
            <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
              <span className='text-muted-foreground text-sm'>
                {t('Total')}: {data.total}
              </span>
              <div className='flex w-full items-center justify-between gap-2 sm:w-auto sm:justify-end'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page <= 1}
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                >
                  {t('Prev')}
                </Button>
                <span className='text-muted-foreground px-2 text-sm'>
                  {page} / {totalPages}
                </span>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page >= totalPages}
                  onClick={() => setPage((current) => current + 1)}
                >
                  {t('Next')}
                </Button>
              </div>
            </div>
          )}
        </div>

        <Dialog
          open={selectedItem !== null}
          onOpenChange={(open) => {
            if (!open && !banMutation.isPending) setSelectedItem(null)
          }}
        >
          <DialogContent className='max-h-[90vh] overflow-x-hidden overflow-y-auto sm:max-w-4xl'>
            <DialogHeader>
              <DialogTitle className='flex min-w-0 items-start gap-2 pr-8'>
                <Fingerprint className='size-5 shrink-0' />
                <span className='min-w-0 break-words'>
                  {t('Shared Identifier Review')}
                </span>
              </DialogTitle>
              <DialogDescription className='break-words'>
                {selectedItem && (
                  <span className='flex flex-wrap items-center gap-2'>
                    <Badge variant='destructive'>
                      {t('Users')}: {selectedItem.user_count}
                    </Badge>
                    <span className='font-mono text-xs break-all'>
                      {selectedItem.identifier}
                    </span>
                  </span>
                )}
              </DialogDescription>
            </DialogHeader>

            {selectedItem && (
              <div className='min-w-0 space-y-6'>
                <section className='space-y-3'>
                  <h3 className='font-medium'>{t('Related Accounts')}</h3>
                  <div className='space-y-2'>
                    {selectedItem.users.map((user) => (
                      <label
                        key={user.user_id}
                        className='border-border flex min-w-0 items-center gap-3 rounded-lg border p-3'
                      >
                        <Checkbox
                          checked={selectedUserIds.includes(user.user_id)}
                          onCheckedChange={(checked) =>
                            toggleUser(user.user_id, checked === true)
                          }
                        />
                        <span className='min-w-0 flex-1 font-medium break-all'>
                          #{user.user_id} {user.username}
                        </span>
                        <span className='text-muted-foreground shrink-0 text-sm'>
                          {t('Requests')}: {user.request_count}
                        </span>
                      </label>
                    ))}
                  </div>
                </section>

                <section className='space-y-3'>
                  <h3 className='font-medium'>{t('Related Tokens')}</h3>
                  <div className='flex flex-wrap gap-1.5'>
                    {selectedItem.tokens.map((token) => (
                      <Badge
                        key={token.token_id}
                        variant='outline'
                        className='h-auto max-w-full whitespace-normal'
                      >
                        #{token.token_id} {token.token_name} ({token.username})
                      </Badge>
                    ))}
                  </div>
                </section>

                <section className='grid gap-x-6 gap-y-3 md:grid-cols-2'>
                  <div>
                    <div className='text-muted-foreground text-xs font-medium'>
                      {t('IP Addresses')}
                    </div>
                    <div className='mt-1 font-mono text-xs break-all'>
                      {selectedItem.ips.length
                        ? selectedItem.ips.join(', ')
                        : '-'}
                    </div>
                  </div>
                  <div>
                    <div className='text-muted-foreground text-xs font-medium'>
                      {t('Observed At')}
                    </div>
                    <div className='mt-1'>
                      {formatTime(selectedItem.first_seen_at)} -{' '}
                      {formatTime(selectedItem.last_seen_at)}
                    </div>
                  </div>
                </section>

                <section className='grid gap-4'>
                  <div className='grid gap-2'>
                    <Label htmlFor='session-risk-ban-reason'>
                      {t('Ban Reason')}
                    </Label>
                    <Input
                      id='session-risk-ban-reason'
                      value={banReason}
                      maxLength={5000}
                      onChange={(event) => setBanReason(event.target.value)}
                    />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='session-risk-ban-duration'>
                      {t('Ban Duration')}
                    </Label>
                    <select
                      id='session-risk-ban-duration'
                      className='border-input bg-background h-9 w-full rounded-lg border px-3 text-sm shadow-xs'
                      value={banDuration}
                      onChange={(event) =>
                        setBanDuration(Number(event.target.value))
                      }
                    >
                      <option value={0}>{t('Permanent Ban')}</option>
                      <option value={1440}>{t('1 Day')}</option>
                      <option value={10080}>{t('7 Days')}</option>
                      <option value={43200}>{t('30 Days')}</option>
                    </select>
                  </div>
                </section>

                <DialogFooter>
                  <Button
                    variant='outline'
                    className='w-full sm:w-auto'
                    disabled={banMutation.isPending}
                    onClick={() => setSelectedItem(null)}
                  >
                    {t('Cancel')}
                  </Button>
                  <Button
                    variant='destructive'
                    className='w-full sm:w-auto'
                    disabled={
                      selectedUserIds.length === 0 ||
                      !banReason.trim() ||
                      banMutation.isPending
                    }
                    onClick={() => banMutation.mutate()}
                  >
                    <Ban data-icon='inline-start' className='size-4' />
                    {banMutation.isPending
                      ? t('Processing...')
                      : t('Ban Selected Users', {
                          count: selectedUserIds.length,
                        })}
                  </Button>
                </DialogFooter>
              </div>
            )}
          </DialogContent>
        </Dialog>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
