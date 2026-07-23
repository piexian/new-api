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
import { ChevronLeft, ChevronRight, Eye, RefreshCw, Search } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress, ProgressValue } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  getRiskLiveRules,
  getRiskLiveTargets,
  toggleRiskLiveRule,
  type PageData,
  type RiskLiveDimension,
  type RiskLiveRuleSummary,
  type RiskLiveTarget,
} from '../api'

const PAGE_SIZE = 10
const REFRESH_STORAGE_KEY = 'new-api:risk-live-progress:refresh:v1'
const REFRESH_OPTIONS = [0, 5, 10, 15, 30, 60] as const

function loadRefreshSeconds(): number {
  try {
    const value = Number(window.localStorage.getItem(REFRESH_STORAGE_KEY))
    return REFRESH_OPTIONS.includes(value as (typeof REFRESH_OPTIONS)[number])
      ? value
      : 0
  } catch {
    return 0
  }
}

function saveRefreshSeconds(value: number) {
  try {
    window.localStorage.setItem(REFRESH_STORAGE_KEY, String(value))
  } catch {
    // Storage can be unavailable in private browsing.
  }
}

function formatTimestamp(value: number): string {
  return value ? dayjs.unix(value).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function getStatusVariant(status: RiskLiveTarget['status']) {
  if (status === 'threshold_reached') return 'destructive' as const
  if (status === 'near_threshold') return 'secondary' as const
  return 'outline' as const
}

export function LiveProgressPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [refreshSeconds, setRefreshSeconds] = useState(loadRefreshSeconds)
  const [selectedKey, setSelectedKey] = useState('')
  const [dimension, setDimension] = useState<
    '' | Exclude<RiskLiveDimension, 'both'>
  >('')
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [searchKeyword, setSearchKeyword] = useState('')
  const refreshInterval = refreshSeconds > 0 ? refreshSeconds * 1000 : false

  const rulesQuery = useQuery({
    queryKey: ['risk', 'live-progress', 'rules'],
    queryFn: async () => {
      const response = await getRiskLiveRules()
      if (!response.success) throw new Error(response.message)
      return response.data
    },
    refetchInterval: refreshInterval,
    refetchIntervalInBackground: false,
  })

  const rules = rulesQuery.data ?? []
  const selectedRule =
    rules.find((rule) => `${rule.source}:${rule.rule_id}` === selectedKey) ??
    rules[0]
  const selectedRuleKey = selectedRule
    ? `${selectedRule.source}:${selectedRule.rule_id}`
    : ''
  let targetDimension: '' | Exclude<RiskLiveDimension, 'both'> = ''
  if (selectedRule) {
    targetDimension =
      selectedRule.dimension === 'both' ? dimension : selectedRule.dimension
  }

  const getDimensionLabel = (value: RiskLiveDimension) => {
    if (value === 'both') return t('IP + User')
    if (value === 'ip') return t('IP')
    return t('User')
  }

  const getStatusLabel = (status: RiskLiveTarget['status']) => {
    if (status === 'threshold_reached') return t('Threshold reached')
    if (status === 'near_threshold') return t('Near threshold')
    return t('Observing')
  }

  const targetsQuery = useQuery<PageData<RiskLiveTarget>>({
    queryKey: [
      'risk',
      'live-progress',
      'targets',
      selectedRule?.source,
      selectedRule?.rule_id,
      targetDimension,
      page,
      searchKeyword,
    ],
    queryFn: async () => {
      if (!selectedRule) {
        return { page: 1, page_size: PAGE_SIZE, total: 0, items: [] }
      }
      const response = await getRiskLiveTargets({
        source: selectedRule.source,
        rule_id: selectedRule.rule_id,
        dimension: targetDimension,
        p: page,
        page_size: PAGE_SIZE,
        keyword: searchKeyword,
      })
      if (!response.success) throw new Error(response.message)
      return response.data
    },
    enabled: Boolean(selectedRule),
    placeholderData: (previous) => previous,
    refetchInterval: refreshInterval,
    refetchIntervalInBackground: false,
  })

  const toggleMutation = useMutation({
    mutationFn: toggleRiskLiveRule,
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message)
        return
      }
      toast.success(t('Rule status updated'))
      queryClient.invalidateQueries({ queryKey: ['risk', 'live-progress'] })
      queryClient.invalidateQueries({ queryKey: ['risk', 'probe-guard'] })
      queryClient.invalidateQueries({ queryKey: ['risk', 'error-ban'] })
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const selectRule = (rule: RiskLiveRuleSummary) => {
    setSelectedKey(`${rule.source}:${rule.rule_id}`)
    setDimension('')
    setPage(1)
    setKeyword('')
    setSearchKeyword('')
  }

  const updateRefreshSeconds = (value: string | null) => {
    const next = Number(value ?? 0)
    setRefreshSeconds(next)
    saveRefreshSeconds(next)
  }

  const handleRefresh = () => {
    void Promise.all([rulesQuery.refetch(), targetsQuery.refetch()])
  }

  const handleSearch = () => {
    setPage(1)
    setSearchKeyword(keyword.trim())
  }

  const targets = targetsQuery.data?.items ?? []
  const total = targetsQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const isRefreshing = rulesQuery.isFetching || targetsQuery.isFetching

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Live Progress')}</span>
          <Badge variant='outline' className='shrink-0'>
            Root
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-6'>
          <div className='flex flex-wrap items-end justify-end gap-2'>
            <div className='flex flex-col gap-2'>
              <Label>{t('Auto refresh')}</Label>
              <Select
                value={String(refreshSeconds)}
                onValueChange={updateRefreshSeconds}
              >
                <SelectTrigger className='w-36'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {REFRESH_OPTIONS.map((seconds) => (
                      <SelectItem key={seconds} value={String(seconds)}>
                        {seconds === 0
                          ? t('Off')
                          : t('{{seconds}} seconds', { seconds })}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant='outline'
                    size='icon'
                    disabled={isRefreshing}
                    onClick={handleRefresh}
                  />
                }
              >
                {isRefreshing ? (
                  <Spinner />
                ) : (
                  <RefreshCw data-icon='inline-start' />
                )}
                <span className='sr-only'>{t('Refresh')}</span>
              </TooltipTrigger>
              <TooltipContent>{t('Refresh')}</TooltipContent>
            </Tooltip>
          </div>

          <div className='overflow-x-auto rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Rule')}</TableHead>
                  <TableHead>{t('Dimension')}</TableHead>
                  <TableHead>{t('Threshold')}</TableHead>
                  <TableHead>{t('Active Targets')}</TableHead>
                  <TableHead>{t('Near Threshold')}</TableHead>
                  <TableHead>{t('Max Progress')}</TableHead>
                  <TableHead>{t('Last Activity')}</TableHead>
                  <TableHead>{t('Enabled')}</TableHead>
                  <TableHead className='w-14'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rules.map((rule) => {
                  const key = `${rule.source}:${rule.rule_id}`
                  return (
                    <TableRow
                      key={key}
                      data-state={
                        key === selectedRuleKey ? 'selected' : undefined
                      }
                    >
                      <TableCell>
                        <div className='flex min-w-48 items-center gap-2'>
                          <span className='truncate font-medium'>
                            {rule.system
                              ? t('Probe Guard')
                              : rule.rule_name || rule.rule_id}
                          </span>
                          {rule.system && (
                            <Badge variant='secondary'>
                              {t('System Rule')}
                            </Badge>
                          )}
                          {rule.dry_run && (
                            <Badge variant='outline'>{t('Dry Run')}</Badge>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>{getDimensionLabel(rule.dimension)}</TableCell>
                      <TableCell className='tabular-nums'>
                        {rule.threshold}
                      </TableCell>
                      <TableCell className='tabular-nums'>
                        {rule.active_targets}
                      </TableCell>
                      <TableCell className='tabular-nums'>
                        {rule.near_threshold_targets}
                      </TableCell>
                      <TableCell className='min-w-40'>
                        <Progress value={rule.max_progress_percent}>
                          <ProgressValue>
                            {() => `${rule.max_progress_percent}%`}
                          </ProgressValue>
                        </Progress>
                      </TableCell>
                      <TableCell className='whitespace-nowrap'>
                        {formatTimestamp(rule.last_seen_at)}
                      </TableCell>
                      <TableCell>
                        <Switch
                          checked={rule.enabled}
                          disabled={toggleMutation.isPending}
                          aria-label={t('Toggle rule {{name}}', {
                            name: rule.system
                              ? t('Probe Guard')
                              : rule.rule_name || rule.rule_id,
                          })}
                          onCheckedChange={(enabled) =>
                            toggleMutation.mutate({
                              source: rule.source,
                              rule_id: rule.rule_id,
                              enabled,
                            })
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                variant='ghost'
                                size='icon-sm'
                                onClick={() => selectRule(rule)}
                              />
                            }
                          >
                            <Eye />
                            <span className='sr-only'>{t('Details')}</span>
                          </TooltipTrigger>
                          <TooltipContent>{t('Details')}</TooltipContent>
                        </Tooltip>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
            {!rulesQuery.isLoading && rules.length === 0 && (
              <Empty className='h-40 border-0'>
                <EmptyHeader>
                  <EmptyTitle>{t('No data')}</EmptyTitle>
                </EmptyHeader>
              </Empty>
            )}
          </div>

          {selectedRule && (
            <section className='flex flex-col gap-4'>
              <div className='flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between'>
                <div>
                  <h2 className='text-lg font-semibold'>
                    {selectedRule.system
                      ? t('Probe Guard')
                      : selectedRule.rule_name || selectedRule.rule_id}
                  </h2>
                  <p className='text-muted-foreground text-sm'>
                    {t('Window {{window}}s / Threshold {{threshold}}', {
                      window: selectedRule.window_seconds,
                      threshold: selectedRule.threshold,
                    })}
                  </p>
                </div>
                <div className='flex flex-wrap items-end gap-2'>
                  {selectedRule.dimension === 'both' && (
                    <div className='flex flex-col gap-2'>
                      <Label>{t('Dimension')}</Label>
                      <Select
                        value={dimension || 'all'}
                        onValueChange={(value) => {
                          setDimension(
                            value === 'all' ? '' : (value as 'ip' | 'user')
                          )
                          setPage(1)
                        }}
                      >
                        <SelectTrigger className='w-32'>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            <SelectItem value='all'>{t('All')}</SelectItem>
                            <SelectItem value='ip'>{t('IP')}</SelectItem>
                            <SelectItem value='user'>{t('User')}</SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                  <Input
                    value={keyword}
                    onChange={(event) => setKeyword(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') handleSearch()
                    }}
                    placeholder={t('Search...')}
                    className='w-64'
                  />
                  <Button variant='secondary' onClick={handleSearch}>
                    <Search data-icon='inline-start' />
                    {t('Search')}
                  </Button>
                </div>
              </div>

              <div className='overflow-x-auto rounded-md border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Target')}</TableHead>
                      <TableHead>{t('Dimension')}</TableHead>
                      <TableHead>{t('Context')}</TableHead>
                      <TableHead>{t('Current Progress')}</TableHead>
                      {selectedRule.system && (
                        <TableHead>{t('Current Models')}</TableHead>
                      )}
                      <TableHead>{t('Window Remaining')}</TableHead>
                      <TableHead>{t('Last Activity')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {targets.map((target) => (
                      <TableRow key={target.id}>
                        <TableCell>
                          <div className='flex min-w-36 flex-col'>
                            <span className='font-mono text-xs'>
                              {target.target}
                            </span>
                            {target.username && (
                              <span className='text-muted-foreground text-xs'>
                                {target.username}
                              </span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          {target.dimension === 'ip' ? t('IP') : t('User')}
                        </TableCell>
                        <TableCell
                          className='max-w-48 truncate'
                          title={target.context}
                        >
                          {target.context || '-'}
                        </TableCell>
                        <TableCell className='min-w-44'>
                          <Progress value={target.progress_percent}>
                            <ProgressValue>
                              {() =>
                                `${target.current_count} / ${target.threshold}`
                              }
                            </ProgressValue>
                          </Progress>
                        </TableCell>
                        {selectedRule.system && (
                          <TableCell
                            className='max-w-64 truncate'
                            title={target.members.join(', ')}
                          >
                            {target.members.join(', ') || '-'}
                          </TableCell>
                        )}
                        <TableCell className='tabular-nums'>
                          {t('{{seconds}} seconds', {
                            seconds: target.remaining_seconds,
                          })}
                        </TableCell>
                        <TableCell className='whitespace-nowrap'>
                          {formatTimestamp(target.last_seen_at)}
                        </TableCell>
                        <TableCell>
                          <Badge variant={getStatusVariant(target.status)}>
                            {getStatusLabel(target.status)}
                          </Badge>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
                {!targetsQuery.isLoading && targets.length === 0 && (
                  <Empty className='h-40 border-0'>
                    <EmptyHeader>
                      <EmptyTitle>{t('No active progress')}</EmptyTitle>
                    </EmptyHeader>
                  </Empty>
                )}
              </div>

              {total > 0 && (
                <div className='flex items-center justify-between gap-3'>
                  <span className='text-muted-foreground text-sm'>
                    {t('Page {{current}} of {{total}}', {
                      current: page,
                      total: totalPages,
                    })}
                  </span>
                  <div className='flex gap-2'>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant='outline'
                            size='icon-sm'
                            disabled={page <= 1}
                            onClick={() =>
                              setPage((current) => Math.max(1, current - 1))
                            }
                          />
                        }
                      >
                        <ChevronLeft />
                        <span className='sr-only'>{t('Prev')}</span>
                      </TooltipTrigger>
                      <TooltipContent>{t('Prev')}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger
                        render={
                          <Button
                            variant='outline'
                            size='icon-sm'
                            disabled={page >= totalPages}
                            onClick={() => setPage((current) => current + 1)}
                          />
                        }
                      >
                        <ChevronRight />
                        <span className='sr-only'>{t('Next')}</span>
                      </TooltipTrigger>
                      <TooltipContent>{t('Next')}</TooltipContent>
                    </Tooltip>
                  </div>
                </div>
              )}
            </section>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
