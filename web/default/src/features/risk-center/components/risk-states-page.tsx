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
import {
  ChevronLeft,
  ChevronRight,
  Eye,
  RotateCcw,
  Search,
  ShieldCheck,
} from 'lucide-react'
import { useState, type KeyboardEvent, type ReactNode } from 'react'
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
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

import {
  getErrorBanIPStates,
  getErrorBanStats,
  getErrorBanUserStates,
  getProbeGuardIPOffenses,
  getProbeGuardStats,
  getProbeGuardUserOffenses,
  resetErrorBanIPState,
  resetErrorBanUserState,
  resetProbeGuardIPOffense,
  unbanProbeGuardUser,
  type ApiResponse,
  type ErrorBanIPState,
  type ErrorBanUserState,
  type PageData,
  type ProbeIPOffense,
  type ProbeUserOffense,
} from '../api'

type RiskSource = 'probe_guard' | 'error_ban'
type RiskDimension = 'ip' | 'user'
type RiskState =
  | ProbeIPOffense
  | ProbeUserOffense
  | ErrorBanIPState
  | ErrorBanUserState

type StateRow = {
  key: string
  cells: ReactNode[]
  item: RiskState
}

type RiskAction = {
  item: RiskState
  source: RiskSource
  dimension: RiskDimension
}

type DetailField = {
  label: string
  value: ReactNode
  multiline?: boolean
  mono?: boolean
}

const PAGE_SIZE = 10

function formatTimestamp(timestamp: number) {
  return timestamp ? dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function getActionMessage(
  source: RiskSource,
  dimension: RiskDimension,
  success: boolean
) {
  if (source === 'probe_guard' && dimension === 'ip') {
    return success
      ? 'IP offense reset successfully'
      : 'Failed to reset IP offense'
  }
  if (source === 'probe_guard') {
    return success ? 'User unbanned successfully' : 'Failed to unban user'
  }
  if (dimension === 'ip') {
    return success ? 'IP state reset successfully' : 'Failed to reset IP state'
  }
  return success
    ? 'User state reset successfully'
    : 'Failed to reset user state'
}

function getDetailFields({
  item,
  source,
  dimension,
}: RiskAction): DetailField[] {
  if (source === 'probe_guard' && dimension === 'ip') {
    const state = item as ProbeIPOffense
    return [
      { label: 'ID', value: state.id },
      { label: 'Target IP', value: state.target_ip, mono: true },
      { label: 'Last User ID', value: state.last_user_id },
      { label: 'Offense Count', value: state.offense_count },
      { label: 'Last Models', value: state.last_models, multiline: true },
      {
        label: 'Last Offense At',
        value: formatTimestamp(state.last_offense_at),
      },
      { label: 'Created At', value: formatTimestamp(state.created_at) },
      { label: 'Updated', value: formatTimestamp(state.updated_at) },
    ]
  }

  if (source === 'probe_guard') {
    const state = item as ProbeUserOffense
    return [
      { label: 'ID', value: state.id },
      { label: 'User ID', value: state.user_id },
      { label: 'Last IP', value: state.last_ip, mono: true },
      { label: 'Offense Count', value: state.offense_count },
      { label: 'Last Models', value: state.last_models, multiline: true },
      {
        label: 'Last Offense At',
        value: formatTimestamp(state.last_offense_at),
      },
      { label: 'Created At', value: formatTimestamp(state.created_at) },
      { label: 'Updated', value: formatTimestamp(state.updated_at) },
    ]
  }

  const state = item as ErrorBanIPState | ErrorBanUserState
  return [
    { label: 'ID', value: state.id },
    dimension === 'ip'
      ? {
          label: 'Target IP',
          value: (state as ErrorBanIPState).target_ip,
          mono: true,
        }
      : { label: 'User ID', value: (state as ErrorBanUserState).user_id },
    { label: 'Rule ID', value: state.rule_id, mono: true },
    { label: 'Offense Count', value: state.offense_count },
    { label: 'Window Count', value: state.window_count },
    { label: 'Window Start', value: formatTimestamp(state.window_start) },
    { label: 'Last Error', value: state.last_error, multiline: true },
    {
      label: 'Last Offense At',
      value: formatTimestamp(state.last_offense_at),
    },
    { label: 'Created At', value: formatTimestamp(state.created_at) },
    { label: 'Updated', value: formatTimestamp(state.updated_at) },
  ]
}

export function RiskStatesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [source, setSource] = useState<RiskSource>('probe_guard')
  const [dimension, setDimension] = useState<RiskDimension>('ip')
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [searchKeyword, setSearchKeyword] = useState('')
  const [detail, setDetail] = useState<RiskAction | null>(null)

  const { data, isLoading } = useQuery<PageData<RiskState>>({
    queryKey: ['risk', 'states', source, dimension, page, searchKeyword],
    queryFn: async () => {
      let response: ApiResponse<PageData<RiskState>>
      const params = {
        p: page,
        page_size: PAGE_SIZE,
        keyword: searchKeyword || undefined,
      }
      if (source === 'probe_guard' && dimension === 'ip') {
        response = await getProbeGuardIPOffenses(params)
      } else if (source === 'probe_guard') {
        response = await getProbeGuardUserOffenses(params)
      } else if (dimension === 'ip') {
        response = await getErrorBanIPStates(params)
      } else {
        response = await getErrorBanUserStates(params)
      }
      if (!response.success) {
        throw new Error(response.message || t('No data'))
      }
      return response.data
    },
    placeholderData: (previous) => previous,
  })

  const { data: stats } = useQuery({
    queryKey: ['risk', 'states', source, 'stats'],
    queryFn: async () => {
      if (source === 'probe_guard') {
        const response = await getProbeGuardStats()
        if (!response.success) throw new Error(response.message)
        return {
          totalIP: response.data.total_ip_states,
          totalUser: response.data.total_user_states,
          totalOffenses: response.data.total_offenses,
          fourthLabel: 'Recent Offenses',
          fourthValue: response.data.recent_offenses,
        }
      }
      const response = await getErrorBanStats()
      if (!response.success) throw new Error(response.message)
      return {
        totalIP: response.data.total_ip_states,
        totalUser: response.data.total_user_states,
        totalOffenses: response.data.total_offenses,
        fourthLabel: 'Active Rules',
        fourthValue: response.data.active_rules,
      }
    },
    refetchInterval: 30000,
  })

  const actionMutation = useMutation({
    mutationFn: async ({
      item,
      source: actionSource,
      dimension: actionDimension,
    }: RiskAction): Promise<ApiResponse<null>> => {
      if (actionSource === 'probe_guard' && actionDimension === 'ip') {
        return resetProbeGuardIPOffense((item as ProbeIPOffense).target_ip)
      }
      if (actionSource === 'probe_guard') {
        return unbanProbeGuardUser((item as ProbeUserOffense).user_id)
      }
      if (actionDimension === 'ip') {
        return resetErrorBanIPState((item as ErrorBanIPState).target_ip)
      }
      return resetErrorBanUserState((item as ErrorBanUserState).user_id)
    },
    onSuccess: (response, action) => {
      if (!response.success) {
        toast.error(response.message)
        return
      }
      toast.success(t(getActionMessage(action.source, action.dimension, true)))
      queryClient.invalidateQueries({ queryKey: ['risk', 'states'] })
    },
    onError: (_error, action) => {
      toast.error(t(getActionMessage(action.source, action.dimension, false)))
    },
  })

  const setMode = (
    nextSource: RiskSource = source,
    nextDimension: RiskDimension = dimension
  ) => {
    setSource(nextSource)
    setDimension(nextDimension)
    setPage(1)
    setKeyword('')
    setSearchKeyword('')
  }

  const handleSearch = () => {
    setPage(1)
    setSearchKeyword(keyword.trim())
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') handleSearch()
  }

  const handleAction = (item: RiskState) => {
    if (
      source === 'probe_guard' &&
      dimension === 'user' &&
      !window.confirm(t('Are you sure?'))
    ) {
      return
    }
    actionMutation.mutate({ item, source, dimension })
  }

  const items = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const columns =
    source === 'probe_guard'
      ? [
          dimension === 'ip' ? 'Target IP' : 'User ID',
          dimension === 'ip' ? 'Last User ID' : 'Last IP',
          'Offense Count',
          'Last Models',
          'Last Offense At',
          'Actions',
        ]
      : [
          dimension === 'ip' ? 'Target IP' : 'User ID',
          'Rule ID',
          'Offense Count',
          'Window Count',
          'Window Start',
          'Last Error',
          'Last Offense At',
          'Actions',
        ]

  const rows: StateRow[] = items.map((item) => {
    if (source === 'probe_guard' && dimension === 'ip') {
      const state = item as ProbeIPOffense
      return {
        key: `probe-ip-${state.id}`,
        item,
        cells: [
          state.target_ip,
          state.last_user_id,
          state.offense_count,
          state.last_models,
          formatTimestamp(state.last_offense_at),
        ],
      }
    }
    if (source === 'probe_guard') {
      const state = item as ProbeUserOffense
      return {
        key: `probe-user-${state.id}`,
        item,
        cells: [
          state.user_id,
          state.last_ip,
          state.offense_count,
          state.last_models,
          formatTimestamp(state.last_offense_at),
        ],
      }
    }
    if (dimension === 'ip') {
      const state = item as ErrorBanIPState
      return {
        key: `error-ip-${state.id}`,
        item,
        cells: [
          state.target_ip,
          state.rule_id,
          state.offense_count,
          state.window_count,
          formatTimestamp(state.window_start),
          state.last_error,
          formatTimestamp(state.last_offense_at),
        ],
      }
    }
    const state = item as ErrorBanUserState
    return {
      key: `error-user-${state.id}`,
      item,
      cells: [
        state.user_id,
        state.rule_id,
        state.offense_count,
        state.window_count,
        formatTimestamp(state.window_start),
        state.last_error,
        formatTimestamp(state.last_offense_at),
      ],
    }
  })

  const statItems = [
    { label: 'IP States', value: stats?.totalIP },
    { label: 'User States', value: stats?.totalUser },
    { label: 'Total Offenses', value: stats?.totalOffenses },
    {
      label: stats?.fourthLabel ?? 'Recent Offenses',
      value: stats?.fourthValue,
    },
  ]
  const detailFields = detail ? getDetailFields(detail) : []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Risk States')}</span>
          <Badge variant='outline' className='shrink-0'>
            Root
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-6'>
          <div className='flex flex-col gap-4 lg:flex-row lg:items-end'>
            <div className='flex flex-col gap-2'>
              <Label>{t('Source')}</Label>
              <ToggleGroup
                value={[source]}
                onValueChange={(values) => {
                  const next = values.find((value) => value !== source)
                  if (next) setMode(next as RiskSource)
                }}
                variant='outline'
                aria-label={t('Source')}
              >
                <ToggleGroupItem value='probe_guard'>
                  {t('Probe Guard')}
                </ToggleGroupItem>
                <ToggleGroupItem value='error_ban'>
                  {t('Error Ban')}
                </ToggleGroupItem>
              </ToggleGroup>
            </div>
            <div className='flex flex-col gap-2'>
              <Label>{t('Dimension')}</Label>
              <ToggleGroup
                value={[dimension]}
                onValueChange={(values) => {
                  const next = values.find((value) => value !== dimension)
                  if (next) setMode(source, next as RiskDimension)
                }}
                variant='outline'
                aria-label={t('Dimension')}
              >
                <ToggleGroupItem value='ip'>{t('IP')}</ToggleGroupItem>
                <ToggleGroupItem value='user'>{t('User')}</ToggleGroupItem>
              </ToggleGroup>
            </div>
            <div className='flex min-w-0 flex-1 gap-2 lg:justify-end'>
              <Input
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={t('Search...')}
                className='max-w-sm'
              />
              <Button variant='secondary' onClick={handleSearch}>
                <Search data-icon='inline-start' className='size-4' />
                {t('Search')}
              </Button>
            </div>
          </div>

          <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
            {statItems.map((stat) => (
              <Card key={stat.label}>
                <CardHeader className='pb-2'>
                  <CardTitle className='text-muted-foreground text-sm font-medium'>
                    {t(stat.label)}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className='text-2xl font-bold'>{stat.value ?? '-'}</div>
                </CardContent>
              </Card>
            ))}
          </div>

          <div className='overflow-hidden rounded-md border'>
            {isLoading && (
              <div className='flex h-56 items-center justify-center'>
                <Spinner className='size-5' />
              </div>
            )}
            {!isLoading && rows.length === 0 && (
              <Empty className='h-56 border-0'>
                <EmptyHeader>
                  <EmptyTitle>{t('No data')}</EmptyTitle>
                </EmptyHeader>
              </Empty>
            )}
            {!isLoading && rows.length > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    {columns.map((column) => (
                      <TableHead key={column}>{t(column)}</TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((row) => (
                    <TableRow key={row.key}>
                      {row.cells.map((cell, index) => (
                        <TableCell
                          key={`${row.key}-${columns[index]}`}
                          className={
                            index === 0 ? 'font-mono text-xs' : undefined
                          }
                        >
                          <span
                            className={
                              index === row.cells.length - 2
                                ? 'block max-w-72 truncate'
                                : undefined
                            }
                            title={typeof cell === 'string' ? cell : undefined}
                          >
                            {cell || '-'}
                          </span>
                        </TableCell>
                      ))}
                      <TableCell>
                        <div className='flex items-center gap-2'>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() =>
                              setDetail({ item: row.item, source, dimension })
                            }
                          >
                            <Eye data-icon='inline-start' className='size-4' />
                            {t('Details')}
                          </Button>
                          <Button
                            variant={
                              source === 'probe_guard' && dimension === 'user'
                                ? 'default'
                                : 'destructive'
                            }
                            size='sm'
                            disabled={actionMutation.isPending}
                            onClick={() => handleAction(row.item)}
                          >
                            {source === 'probe_guard' &&
                            dimension === 'user' ? (
                              <ShieldCheck
                                data-icon='inline-start'
                                className='size-4'
                              />
                            ) : (
                              <RotateCcw
                                data-icon='inline-start'
                                className='size-4'
                              />
                            )}
                            {t(
                              source === 'probe_guard' && dimension === 'user'
                                ? 'Unban'
                                : 'Reset'
                            )}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>

          <Dialog
            open={detail !== null}
            onOpenChange={(open) => {
              if (!open) setDetail(null)
            }}
          >
            <DialogContent className='max-h-[80vh] overflow-y-auto sm:max-w-2xl'>
              <DialogHeader>
                <DialogTitle>{t('Details')}</DialogTitle>
                <DialogDescription className='sr-only'>
                  {t('Risk States')}
                </DialogDescription>
              </DialogHeader>
              <div className='space-y-3 text-sm'>
                {detailFields.map((field) => (
                  <div key={field.label} className='grid grid-cols-3 gap-3'>
                    <span className='text-muted-foreground font-medium'>
                      {t(field.label)}
                    </span>
                    <span
                      className={`col-span-2 break-words ${
                        field.multiline ? 'whitespace-pre-wrap' : ''
                      } ${field.mono ? 'font-mono text-xs' : ''}`}
                    >
                      {field.value === '' || field.value == null
                        ? '-'
                        : field.value}
                    </span>
                  </div>
                ))}
              </div>
            </DialogContent>
          </Dialog>

          {total > 0 && (
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground text-sm'>
                {t('Page {{current}} of {{total}}', {
                  current: page,
                  total: totalPages,
                })}
              </span>
              <div className='flex gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page <= 1}
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                >
                  <ChevronLeft data-icon='inline-start' className='size-4' />
                  {t('Prev')}
                </Button>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page >= totalPages}
                  onClick={() => setPage((current) => current + 1)}
                >
                  {t('Next')}
                  <ChevronRight data-icon='inline-end' className='size-4' />
                </Button>
              </div>
            </div>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
