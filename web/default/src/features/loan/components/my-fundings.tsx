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
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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

import {
  getLoanMarketFundings,
  resolveLoanMarketFunding,
  setLoanMarketFundingRepayPlan,
} from '../api'
import {
  LOAN_REPAY_PLAN_KEYS,
  type LoanFunding,
  type LoanFundingStatus,
  type LoanRepayPlan,
} from '../types'
import { QueryErrorState } from './query-error'

const PAGE_SIZE = 10

type ResolveAction = 'extend' | 'writeoff' | 'perpetual'

function useSourceLabel() {
  const { t } = useTranslation()
  return (source: LoanFunding['source_type']) => {
    if (source === 'pool') return t('Pool')
    if (source === 'ai') return t('AI')
    if (source === 'order') return t('Order (public listing)')
    return t('Platform')
  }
}

function useStatusLabel() {
  const { t } = useTranslation()
  return (status: LoanFundingStatus) => {
    if (status === 'active') return t('Active')
    if (status === 'overdue') return t('Overdue')
    if (status === 'repaid') return t('Repaid')
    return t('Written Off')
  }
}

function useRepayPlanLabel() {
  const { t } = useTranslation()
  return (plan: LoanRepayPlan) => {
    if (plan === 'full') return t('Full (compounding)')
    if (plan === 'no_penalty') return t('No Penalty')
    if (plan === 'interest_freeze') return t('Interest Freeze')
    return t('Principal Only')
  }
}

function statusBadgeVariant(
  status: LoanFundingStatus
): 'default' | 'secondary' | 'outline' | 'destructive' {
  if (status === 'overdue') return 'destructive'
  if (status === 'repaid') return 'secondary'
  if (status === 'written_off') return 'outline'
  return 'default'
}

function formatDay(day: number): string {
  if (!day) return '-'
  return dayjs.unix(day * 86400).format('YYYY-MM-DD')
}

// 秒结清惩罚条款展示：0 = 不收；窗口 0 = 仅当天
function penaltyText(
  funding: LoanFunding,
  t: (key: string, options?: Record<string, unknown>) => string
): string {
  if (!funding.fast_repay_penalty_quota) return t('None')
  const windowText =
    funding.fast_repay_window_days === 0
      ? t('Same day')
      : t('{{days}} days', { days: funding.fast_repay_window_days })
  return `${formatQuotaWithCurrency(funding.fast_repay_penalty_quota)} / ${windowText}`
}

export function MyFundings() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const sourceLabel = useSourceLabel()
  const statusLabel = useStatusLabel()
  const repayPlanLabel = useRepayPlanLabel()
  const [page, setPage] = useState(1)
  const [extendTarget, setExtendTarget] = useState<LoanFunding | null>(null)
  const [extendDays, setExtendDays] = useState('7')
  const [confirmTarget, setConfirmTarget] = useState<{
    funding: LoanFunding
    action: 'writeoff' | 'perpetual'
  } | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['loan-market-fundings', page, PAGE_SIZE],
    queryFn: async () => {
      const res = await getLoanMarketFundings(page, PAGE_SIZE)
      if (res.success && res.data) {
        return res.data
      }
      throw new Error(res.message || 'Failed to fetch fundings')
    },
    staleTime: 10000,
  })

  const fundings = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['loan-market-fundings'] })
    queryClient.invalidateQueries({ queryKey: ['loan-market-offers'] })
  }

  const runResolve = async (
    funding: LoanFunding,
    action: ResolveAction,
    days: number
  ) => {
    setSubmitting(true)
    try {
      const res = await resolveLoanMarketFunding(funding.id, action, days)
      if (res.success) {
        toast.success(t('Funding resolved'))
        invalidate()
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Operation failed'))
    } finally {
      setSubmitting(false)
    }
  }

  const handleExtend = async () => {
    const funding = extendTarget
    if (!funding) return
    const days = Number(extendDays)
    if (!Number.isInteger(days) || days <= 0) {
      toast.error(t('Please enter a valid number of days'))
      return
    }
    setExtendTarget(null)
    await runResolve(funding, 'extend', days)
  }

  const handleConfirmResolve = async () => {
    const target = confirmTarget
    if (!target) return
    setConfirmTarget(null)
    await runResolve(target.funding, target.action, 0)
  }

  const handleRepayPlanChange = async (
    funding: LoanFunding,
    plan: LoanRepayPlan
  ) => {
    if (funding.repay_plan === plan) return
    try {
      const res = await setLoanMarketFundingRepayPlan(funding.id, plan)
      if (res.success) {
        toast.success(t('Repay plan updated'))
        invalidate()
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Failed to update repay plan'))
    }
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
          {['a', 'b', 'c'].map((slot) => (
            <Skeleton key={slot} className='h-10 rounded-lg' />
          ))}
        </div>
      )
    }

    if (fundings.length === 0) {
      return (
        <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('No fundings yet')}
        </div>
      )
    }

    return (
      <>
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Time')}</TableHead>
                <TableHead>{t('Source')}</TableHead>
                <TableHead>{t('Amount Lent')}</TableHead>
                <TableHead>{t('Repaid Principal')}</TableHead>
                <TableHead>{t('Current Debt')}</TableHead>
                <TableHead>{t('Rate')}</TableHead>
                <TableHead>{t('Fast Penalty')}</TableHead>
                <TableHead>{t('Due Date')}</TableHead>
                <TableHead>{t('Borrower Credit')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Repay Plan')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {fundings.map((funding) => {
                const editable =
                  funding.status === 'active' || funding.status === 'overdue'
                return (
                  <TableRow
                    key={funding.id}
                    className={
                      funding.status === 'overdue'
                        ? 'bg-destructive/5'
                        : undefined
                    }
                  >
                    <TableCell className='text-muted-foreground text-xs'>
                      {formatTimestamp(funding.created_at)}
                    </TableCell>
                    <TableCell>{sourceLabel(funding.source_type)}</TableCell>
                    <TableCell className='font-medium tabular-nums'>
                      {formatQuotaWithCurrency(funding.amount)}
                    </TableCell>
                    <TableCell className='tabular-nums'>
                      {formatQuotaWithCurrency(funding.repaid_principal)}
                    </TableCell>
                    <TableCell className='font-medium tabular-nums'>
                      {formatQuotaWithCurrency(funding.debt)}
                    </TableCell>
                    <TableCell className='text-muted-foreground'>
                      {formatPercent(funding.rate * 100)}
                    </TableCell>
                    <TableCell className='tabular-nums'>
                      {penaltyText(funding, t)}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-xs'>
                      {formatDay(funding.due_day)}
                    </TableCell>
                    <TableCell className='tabular-nums'>
                      {funding.borrower_credit_score}
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusBadgeVariant(funding.status)}>
                        {statusLabel(funding.status)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {editable ? (
                        <Select
                          value={funding.repay_plan}
                          onValueChange={(value) => {
                            if (value !== null) {
                              handleRepayPlanChange(
                                funding,
                                value as LoanRepayPlan
                              )
                            }
                          }}
                        >
                          <SelectTrigger className='w-40'>
                            <SelectValue>
                              {repayPlanLabel(funding.repay_plan)}
                            </SelectValue>
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {LOAN_REPAY_PLAN_KEYS.map((key) => (
                                <SelectItem key={key} value={key}>
                                  {repayPlanLabel(key)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      ) : (
                        <span className='text-muted-foreground text-sm'>
                          {repayPlanLabel(funding.repay_plan)}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className='text-right'>
                      {funding.status === 'overdue' ? (
                        <div className='flex flex-wrap items-center justify-end gap-1.5'>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() => {
                              setExtendDays('7')
                              setExtendTarget(funding)
                            }}
                          >
                            {t('Extend')}
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() =>
                              setConfirmTarget({ funding, action: 'writeoff' })
                            }
                          >
                            {t('Write Off')}
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() =>
                              setConfirmTarget({ funding, action: 'perpetual' })
                            }
                          >
                            {t('Perpetual')}
                          </Button>
                        </div>
                      ) : null}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>

        <div className='mt-4 flex flex-col items-center gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground text-xs sm:text-sm'>
            {t('Showing')} {(page - 1) * PAGE_SIZE + 1}-
            {Math.min(page * PAGE_SIZE, total)} {t('of')} {total}
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => p - 1)}
              disabled={page <= 1}
              className='h-8 w-8 p-0'
              aria-label={t('Previous page')}
            >
              <ChevronLeft className='h-4 w-4' />
            </Button>
            <div className='text-muted-foreground flex items-center gap-1 text-sm'>
              <span className='font-medium'>{page}</span>
              <span>/</span>
              <span>{totalPages}</span>
            </div>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setPage((p) => p + 1)}
              disabled={page >= totalPages}
              className='h-8 w-8 p-0'
              aria-label={t('Next page')}
            >
              <ChevronRight className='h-4 w-4' />
            </Button>
          </div>
        </div>
      </>
    )
  })()

  return (
    <>
      <div className='min-w-0'>
        <p className='text-sm font-medium'>{t('My Fundings')}</p>
        <p className='text-muted-foreground mt-0.5 text-xs'>
          {t(
            'Every loan funded by your offers; overdue rows can be extended, written off or kept perpetual.'
          )}
        </p>
      </div>

      <div className='mt-4'>{content}</div>

      {/* 延长弹窗 */}
      <AlertDialog
        open={extendTarget !== null}
        onOpenChange={(open) => {
          if (!open) setExtendTarget(null)
        }}
      >
        <AlertDialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Extend Overdue Funding')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Set a new due date. The accumulated penalty interest is kept and the funding returns to active.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className='space-y-2'>
            <Label>{t('Extend Days')}</Label>
            <Input
              type='number'
              min={1}
              step={1}
              value={extendDays}
              onChange={(e) => setExtendDays(e.target.value)}
            />
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleExtend} disabled={submitting}>
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 核销 / 永续确认弹窗 */}
      <AlertDialog
        open={confirmTarget !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmTarget(null)
        }}
      >
        <AlertDialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-md'>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirmTarget?.action === 'writeoff'
                ? t('Write Off Funding')
                : t('Make Funding Perpetual')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirmTarget?.action === 'writeoff'
                ? t(
                    'The outstanding debt is destroyed and the borrower is penalized. This cannot be undone.'
                  )
                : t(
                    'The funding stays overdue and keeps accruing interest until it is repaid.'
                  )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant={
                confirmTarget?.action === 'writeoff' ? 'destructive' : 'default'
              }
              onClick={handleConfirmResolve}
              disabled={submitting}
            >
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
