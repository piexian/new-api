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
import { Ban, Eye, Search, UsersRound } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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

import {
  banMultiAccountUser,
  getMultiAccountClusters,
  type MultiAccountCluster,
  type MultiAccountEvidence,
  type MultiAccountRiskLevel,
  type MultiAccountUser,
} from '../api'

const PAGE_SIZE = 10

function formatTime(timestamp: number) {
  return timestamp ? dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function riskBadgeVariant(
  level: MultiAccountRiskLevel
): 'destructive' | 'secondary' | 'outline' {
  if (level === 'high') return 'destructive'
  if (level === 'medium') return 'secondary'
  return 'outline'
}

export function MultiAccountPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [selectedCluster, setSelectedCluster] =
    useState<MultiAccountCluster | null>(null)
  const [banTarget, setBanTarget] = useState<MultiAccountUser | null>(null)
  const [banReason, setBanReason] = useState('')
  const [banDuration, setBanDuration] = useState(0)

  const { data, error, isLoading } = useQuery({
    queryKey: ['risk', 'multi-account', page, keyword],
    queryFn: async () => {
      const response = await getMultiAccountClusters({
        p: page,
        page_size: PAGE_SIZE,
        keyword,
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
      if (!banTarget) throw new Error(t('User not found'))
      const response = await banMultiAccountUser(banTarget.id, {
        reason: banReason.trim(),
        duration_minutes: banDuration,
      })
      if (!response.success) {
        throw new Error(response.message || t('Ban failed'))
      }
      return response.data
    },
    onSuccess: async (updatedAccount) => {
      toast.success(t('Account banned successfully'))
      if (updatedAccount) {
        setSelectedCluster((current) =>
          current
            ? {
                ...current,
                accounts: current.accounts.map((account) =>
                  account.id === updatedAccount.id ? updatedAccount : account
                ),
              }
            : null
        )
      }
      setBanTarget(null)
      await queryClient.invalidateQueries({
        queryKey: ['risk', 'multi-account'],
      })
      await queryClient.invalidateQueries({ queryKey: ['risk', 'ban-logs'] })
    },
    onError: (mutationError: Error) => toast.error(mutationError.message),
  })

  const applySearch = () => {
    setKeyword(keywordInput.trim())
    setPage(1)
  }

  const openBanDialog = (account: MultiAccountUser) => {
    setBanTarget(account)
    setBanReason(t('Administrator confirmed multi-account abuse'))
    setBanDuration(0)
  }

  const riskLevelLabel = (level: MultiAccountRiskLevel) => {
    if (level === 'high') return t('High Risk')
    if (level === 'medium') return t('Medium Risk')
    return t('Low Risk')
  }

  const evidenceLabel = (evidence: MultiAccountEvidence) =>
    evidence.type === 'github_email_conflict'
      ? t('GitHub Email Conflict')
      : t('Shared IP and Browser')

  const accountStatus = (account: MultiAccountUser) => {
    if (account.deleted) return t('Deleted')
    return account.status === 1 ? t('Enabled') : t('Disabled')
  }

  const roleLabel = (role: number) => {
    if (role === 100) return t('Root User')
    if (role === 10) return t('Administrator')
    return t('Common User')
  }

  const accountDetailFields = (
    account: MultiAccountUser
  ): Array<[string, string | number, string]> => [
    [t('Email'), account.email || '-', 'email'],
    [t('GitHub ID'), account.github_id || '-', 'github'],
    ...account.oauth_identities
      .filter((identity) => identity.provider_key !== 'github')
      .map((identity): [string, string, string] => [
        `${identity.provider_name} ID`,
        identity.provider_user_id,
        identity.provider_key,
      ]),
    [t('Role'), roleLabel(account.role), 'role'],
    [t('Status'), accountStatus(account), 'status'],
    [t('Created At'), formatTime(account.created_at), 'created_at'],
    [t('Last Login'), formatTime(account.last_login_at), 'last_login_at'],
    [t('Disabled Until'), formatTime(account.disabled_until), 'disabled_until'],
    [t('Ban Reason'), account.disable_reason || '-', 'disable_reason'],
  ]

  const totalPages = data
    ? Math.max(1, Math.ceil(data.total / data.page_size))
    : 1

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Multi-account Detection')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='min-w-0 space-y-6'>
          {error && (
            <div className='border-destructive/40 bg-destructive/10 text-destructive rounded-lg border px-4 py-3 text-sm'>
              {error.message}
            </div>
          )}

          <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-5'>
            {[
              [t('Multi-account Clusters'), data?.stats.total_clusters],
              [t('High Risk'), data?.stats.high_risk_clusters],
              [t('Related Accounts'), data?.stats.related_accounts],
              [t('GitHub Email Conflicts'), data?.stats.email_conflicts],
              [t('Shared Environments'), data?.stats.shared_environments],
            ].map(([label, value]) => (
              <Card key={String(label)} className='min-w-0'>
                <CardHeader className='pb-2'>
                  <CardTitle className='text-sm font-medium'>{label}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className='text-2xl font-bold'>{value ?? '-'}</div>
                </CardContent>
              </Card>
            ))}
          </div>

          <div className='flex flex-col gap-3 sm:flex-row sm:items-center'>
            <div className='relative w-full max-w-xl'>
              <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
              <Input
                className='pl-9'
                value={keywordInput}
                onChange={(event) => setKeywordInput(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && applySearch()}
                placeholder={t('Search account, email, IP, or browser')}
              />
            </div>
            <Button className='w-full sm:w-auto' onClick={applySearch}>
              <Search data-icon='inline-start' className='size-4' />
              {t('Search')}
            </Button>
          </div>

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
              data?.items.map((cluster) => (
                <Card key={cluster.id} className='min-w-0'>
                  <CardHeader className='gap-3 pb-3'>
                    <div className='flex min-w-0 items-start justify-between gap-3'>
                      <div className='min-w-0'>
                        <div className='text-muted-foreground text-xs font-medium'>
                          {t('Rank')}
                        </div>
                        <CardTitle className='mt-1 text-base'>
                          #{cluster.rank}
                        </CardTitle>
                      </div>
                      <div className='flex shrink-0 flex-col items-end gap-1'>
                        <span className='text-lg font-semibold'>
                          {cluster.risk_score}
                        </span>
                        <Badge variant={riskBadgeVariant(cluster.risk_level)}>
                          {riskLevelLabel(cluster.risk_level)}
                        </Badge>
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className='min-w-0 space-y-4'>
                    <div className='min-w-0'>
                      <div className='text-muted-foreground text-xs font-medium'>
                        {t('Related Accounts')}
                      </div>
                      <div className='mt-2 space-y-2'>
                        {cluster.accounts.map((account) => (
                          <div key={account.id} className='min-w-0'>
                            <div className='font-medium break-all'>
                              #{account.id} {account.username}
                            </div>
                            <div className='text-muted-foreground text-sm break-all'>
                              {account.email || '-'}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>

                    <div className='min-w-0'>
                      <div className='text-muted-foreground text-xs font-medium'>
                        {t('Evidence')}
                      </div>
                      <div className='mt-2 flex min-w-0 flex-wrap gap-1.5'>
                        {cluster.evidence.map((item) => (
                          <Badge
                            key={`${item.type}-${item.user_ids.join('-')}-${item.email}-${item.ip}-${item.user_agent}-${item.last_seen_at}`}
                            variant='outline'
                            className='h-auto max-w-full text-left whitespace-normal'
                          >
                            {evidenceLabel(item)} x{item.hit_count}
                          </Badge>
                        ))}
                      </div>
                    </div>

                    <div className='flex min-w-0 items-start justify-between gap-3 text-sm'>
                      <span className='text-muted-foreground shrink-0'>
                        {t('Last Seen')}
                      </span>
                      <span className='min-w-0 text-right break-words'>
                        {formatTime(cluster.last_seen_at)}
                      </span>
                    </div>

                    <Button
                      variant='outline'
                      className='w-full'
                      onClick={() => setSelectedCluster(cluster)}
                    >
                      <Eye data-icon='inline-start' className='size-4' />
                      {t('Review')}
                    </Button>
                  </CardContent>
                </Card>
              ))}
          </div>

          <div className='border-border hidden max-w-full overflow-x-auto rounded-lg border sm:block'>
            <table className='divide-border min-w-[64rem] table-fixed divide-y text-sm'>
              <thead className='bg-muted/50'>
                <tr>
                  <th className='text-muted-foreground w-16 px-4 py-3 text-left font-medium'>
                    {t('Rank')}
                  </th>
                  <th className='text-muted-foreground w-28 px-4 py-3 text-left font-medium'>
                    {t('Risk Score')}
                  </th>
                  <th className='text-muted-foreground min-w-80 px-4 py-3 text-left font-medium'>
                    {t('Related Accounts')}
                  </th>
                  <th className='text-muted-foreground w-52 px-4 py-3 text-left font-medium'>
                    {t('Evidence')}
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
                  data.items.map((cluster) => (
                    <tr key={cluster.id} className='hover:bg-muted/30'>
                      <td className='px-4 py-3 font-semibold'>
                        #{cluster.rank}
                      </td>
                      <td className='px-4 py-3'>
                        <div className='flex flex-col items-start gap-1'>
                          <span className='text-lg font-semibold'>
                            {cluster.risk_score}
                          </span>
                          <Badge variant={riskBadgeVariant(cluster.risk_level)}>
                            {riskLevelLabel(cluster.risk_level)}
                          </Badge>
                        </div>
                      </td>
                      <td className='px-4 py-3'>
                        <div className='space-y-2'>
                          {cluster.accounts.map((account) => (
                            <div key={account.id} className='min-w-0'>
                              <div className='font-medium'>
                                #{account.id} {account.username}
                              </div>
                              <div className='text-muted-foreground break-all'>
                                {account.email || '-'}
                              </div>
                            </div>
                          ))}
                        </div>
                      </td>
                      <td className='px-4 py-3'>
                        <div className='flex flex-wrap gap-1.5'>
                          {cluster.evidence.map((item) => (
                            <Badge
                              key={`${item.type}-${item.user_ids.join('-')}-${item.email}-${item.ip}-${item.user_agent}-${item.last_seen_at}`}
                              variant='outline'
                            >
                              {evidenceLabel(item)} x{item.hit_count}
                            </Badge>
                          ))}
                        </div>
                      </td>
                      <td className='px-4 py-3'>
                        {formatTime(cluster.last_seen_at)}
                      </td>
                      <td className='px-4 py-3 text-right'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => setSelectedCluster(cluster)}
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
          open={selectedCluster !== null}
          onOpenChange={(open) => !open && setSelectedCluster(null)}
        >
          <DialogContent className='max-h-[90vh] overflow-x-hidden overflow-y-auto sm:max-w-5xl'>
            <DialogHeader>
              <DialogTitle className='flex min-w-0 items-start gap-2 pr-8'>
                <UsersRound className='size-5 shrink-0' />
                <span className='min-w-0 break-words'>
                  {t('Multi-account Review')}
                </span>
              </DialogTitle>
              <DialogDescription>
                {selectedCluster && (
                  <span className='flex flex-wrap items-center gap-2'>
                    <Badge
                      variant={riskBadgeVariant(selectedCluster.risk_level)}
                    >
                      {riskLevelLabel(selectedCluster.risk_level)}
                    </Badge>
                    <span>
                      {t('Risk Score')}: {selectedCluster.risk_score}
                    </span>
                    <span>ID: {selectedCluster.id}</span>
                  </span>
                )}
              </DialogDescription>
            </DialogHeader>

            {selectedCluster && (
              <div className='min-w-0 space-y-6'>
                <section className='space-y-3'>
                  <h3 className='font-medium'>{t('Account Details')}</h3>
                  {selectedCluster.accounts.map((account) => (
                    <div
                      key={account.id}
                      className='border-border min-w-0 space-y-3 rounded-lg border p-3 sm:p-4'
                    >
                      <div className='flex min-w-0 flex-col items-stretch gap-3 sm:flex-row sm:items-center sm:justify-between'>
                        <div className='min-w-0 font-semibold break-all'>
                          #{account.id} {account.username}
                        </div>
                        <div className='flex min-w-0 flex-col items-stretch gap-2 sm:flex-row sm:items-center'>
                          <Badge
                            className='self-start'
                            variant={account.can_ban ? 'outline' : 'secondary'}
                          >
                            {accountStatus(account)}
                          </Badge>
                          <Button
                            variant='destructive'
                            size='sm'
                            className='w-full sm:w-auto'
                            disabled={!account.can_ban}
                            onClick={() => openBanDialog(account)}
                          >
                            <Ban data-icon='inline-start' className='size-4' />
                            {t('Ban Account')}
                          </Button>
                        </div>
                      </div>
                      <div className='grid gap-x-6 gap-y-3 md:grid-cols-2'>
                        {accountDetailFields(account).map(
                          ([label, value, fieldKey]) => (
                            <div key={fieldKey} className='min-w-0'>
                              <div className='text-muted-foreground text-xs font-medium'>
                                {label}
                              </div>
                              <div className='mt-1 break-all'>{value}</div>
                            </div>
                          )
                        )}
                      </div>
                    </div>
                  ))}
                </section>

                <section className='space-y-3'>
                  <h3 className='font-medium'>{t('Evidence Details')}</h3>
                  {selectedCluster.evidence.map((item) => (
                    <div
                      key={`${item.type}-${item.user_ids.join('-')}-${item.email}-${item.ip}-${item.user_agent}-${item.last_seen_at}`}
                      className='border-border min-w-0 space-y-3 rounded-lg border p-3 sm:p-4'
                    >
                      <div className='flex min-w-0 flex-col items-start gap-2 sm:flex-row sm:items-center sm:justify-between'>
                        <Badge
                          variant='outline'
                          className='h-auto max-w-full text-left whitespace-normal'
                        >
                          {evidenceLabel(item)}
                        </Badge>
                        <span className='text-muted-foreground text-xs'>
                          {t('Hit Count')}: {item.hit_count}
                        </span>
                      </div>
                      <div className='grid gap-x-6 gap-y-3 md:grid-cols-2'>
                        {item.email && (
                          <div>
                            <div className='text-muted-foreground text-xs font-medium'>
                              {t('Full Email')}
                            </div>
                            <div className='mt-1 break-all'>{item.email}</div>
                          </div>
                        )}
                        {item.ip && (
                          <div>
                            <div className='text-muted-foreground text-xs font-medium'>
                              {t('IP Address')}
                            </div>
                            <div className='mt-1 font-mono break-all'>
                              {item.ip}
                            </div>
                          </div>
                        )}
                        {item.user_agent && (
                          <div className='md:col-span-2'>
                            <div className='text-muted-foreground text-xs font-medium'>
                              {t('Browser / User Agent')}
                            </div>
                            <div className='mt-1 font-mono text-xs break-all'>
                              {item.user_agent}
                            </div>
                          </div>
                        )}
                        <div>
                          <div className='text-muted-foreground text-xs font-medium'>
                            {t('Related User IDs')}
                          </div>
                          <div className='mt-1'>{item.user_ids.join(', ')}</div>
                        </div>
                        <div>
                          <div className='text-muted-foreground text-xs font-medium'>
                            {t('Observed At')}
                          </div>
                          <div className='mt-1'>
                            {formatTime(item.first_seen_at)} -{' '}
                            {formatTime(item.last_seen_at)}
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </section>
              </div>
            )}
          </DialogContent>
        </Dialog>

        <Dialog
          open={banTarget !== null}
          onOpenChange={(open) => !open && setBanTarget(null)}
        >
          <DialogContent className='sm:max-w-lg'>
            <DialogHeader>
              <DialogTitle>{t('Confirm Ban')}</DialogTitle>
              <DialogDescription className='break-words'>
                {t('Ban account confirmation', {
                  id: banTarget?.id ?? '',
                  username: banTarget?.username ?? '',
                })}
              </DialogDescription>
            </DialogHeader>
            <div className='space-y-4'>
              <div className='space-y-2'>
                <Label htmlFor='multi-account-ban-reason'>
                  {t('Ban Reason')}
                </Label>
                <Input
                  id='multi-account-ban-reason'
                  value={banReason}
                  maxLength={5000}
                  onChange={(event) => setBanReason(event.target.value)}
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='multi-account-ban-duration'>
                  {t('Ban Duration')}
                </Label>
                <select
                  id='multi-account-ban-duration'
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
            </div>
            <DialogFooter>
              <Button
                variant='outline'
                className='w-full sm:w-auto'
                onClick={() => setBanTarget(null)}
              >
                {t('Cancel')}
              </Button>
              <Button
                variant='destructive'
                className='w-full sm:w-auto'
                disabled={!banReason.trim() || banMutation.isPending}
                onClick={() => banMutation.mutate()}
              >
                <Ban data-icon='inline-start' className='size-4' />
                {banMutation.isPending ? t('Processing...') : t('Confirm Ban')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
