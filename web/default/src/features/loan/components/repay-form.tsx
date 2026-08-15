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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Wallet } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { getSelf } from '@/lib/api'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatPercent } from '@/lib/format'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { repayLoan } from '../api'
import type { LoanRepayResult, LoanStatus } from '../types'

// 提前还款手续费说明的"不再显示"标记（localStorage，按浏览器记住）
const FEE_NOTICE_HIDDEN_KEY = 'loan-repay-fee-notice-hidden'

interface RepayFormProps {
  status?: LoanStatus
}

// 还款结果明细行
function BreakdownRow({
  label,
  value,
  emphasis,
}: {
  label: string
  value: string
  emphasis?: boolean
}) {
  return (
    <div className='flex items-center justify-between gap-3'>
      <span className='text-muted-foreground'>{label}</span>
      <span
        className={`tabular-nums ${emphasis ? 'text-destructive font-medium' : ''}`}
      >
        {value}
      </span>
    </div>
  )
}

// schema 需要 t() 文案，在组件内构建
function buildSchema(t: (key: string) => string) {
  return z.object({
    amount: z
      .string()
      .trim()
      .min(1, t('Please enter an amount'))
      .refine((v) => {
        const n = Number(v)
        return Number.isFinite(n) && n > 0
      }, t('Please enter a valid positive amount')),
  })
}

type Values = z.infer<ReturnType<typeof buildSchema>>

export function RepayForm(props: RepayFormProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const quotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const [submitting, setSubmitting] = useState(false)
  const [feeNoticeHidden, setFeeNoticeHidden] = useState(
    () => localStorage.getItem(FEE_NOTICE_HIDDEN_KEY) === '1'
  )
  // 最近一次还款的结果拆分（后端 repay 字段），有惩罚时高亮展示
  const [lastRepay, setLastRepay] = useState<LoanRepayResult | null>(null)

  const schema = useMemo(() => buildSchema(t), [t])

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: { amount: '' },
  })

  const status = props.status
  const debt = status?.debt ?? 0

  // 钱包余额独立查询，还款成功后失效刷新
  const { data: balance } = useQuery({
    queryKey: ['user-self-quota'],
    queryFn: async () => {
      const res = await getSelf()
      if (res.success && res.data) {
        return (res.data as { quota?: number }).quota ?? 0
      }
      throw new Error(res.message || 'Failed to fetch user balance')
    },
    staleTime: 10000,
  })
  const balanceUsd = balance !== undefined ? balance / quotaPerUnit : null

  function applyResponse(data: LoanStatus) {
    toast.success(t('Repay successful'))
    queryClient.setQueryData(['loan-status'], data)
    queryClient.invalidateQueries({ queryKey: ['loan-records'] })
    queryClient.invalidateQueries({ queryKey: ['user-self-quota'] })
    setLastRepay(data.repay ?? null)
    form.reset()
  }

  async function submit(amountUsd: string) {
    setSubmitting(true)
    try {
      const res = await repayLoan(amountUsd)
      if (res.success && res.data) {
        applyResponse(res.data)
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Repay failed'))
    } finally {
      setSubmitting(false)
    }
  }

  async function onSubmit(values: Values) {
    const amount = Number(values.amount)
    if (balanceUsd !== null && amount > balanceUsd) {
      form.setError('amount', {
        type: 'manual',
        message: t('Amount exceeds your wallet balance'),
      })
      return
    }
    await submit(values.amount)
  }

  return (
    <Card className='gap-0 py-0'>
      <CardHeader className='border-b p-4 sm:p-5'>
        <div className='flex items-center gap-3'>
          <IconBadge tone='neutral' size='lg'>
            <Wallet className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
          </IconBadge>
          <div className='min-w-0'>
            <CardTitle className='text-lg'>{t('Early Repayment')}</CardTitle>
            <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
              {t('Repay with your wallet balance, interest first')}
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent className='p-4 sm:p-5'>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            autoComplete='off'
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='amount'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Amount (USD)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step='any'
                      placeholder='0.00'
                      disabled={!status || debt <= 0}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Wallet Balance')}:{' '}
                    {balance !== undefined
                      ? formatQuotaWithCurrency(balance)
                      : '-'}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            {status && status.repay_fee_rate > 0 && !feeNoticeHidden ? (
              <div className='bg-muted/30 space-y-2 rounded-lg border p-3 text-xs'>
                <p className='text-muted-foreground'>
                  {t(
                    'Early repayment incurs a {{rate}} fee on the principal part, charged from your wallet balance together with any interest.',
                    { rate: formatPercent(status.repay_fee_rate * 100) }
                  )}
                </p>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='h-7 px-2 text-xs'
                  onClick={() => {
                    localStorage.setItem(FEE_NOTICE_HIDDEN_KEY, '1')
                    setFeeNoticeHidden(true)
                  }}
                >
                  {t("Don't show again")}
                </Button>
              </div>
            ) : null}
            {status && (status.fast_repay_penalty_estimate ?? 0) > 0 ? (
              <div className='border-destructive/30 bg-destructive/5 rounded-lg border p-3 text-xs'>
                <p className='text-destructive'>
                  {t(
                    'Your debt includes funds with a fast-settle penalty: a manual full settlement right now will charge an extra {{amount}}. Check-in auto-repayment does not trigger this penalty and repays normally.',
                    {
                      amount: formatQuotaWithCurrency(
                        status.fast_repay_penalty_estimate ?? 0
                      ),
                    }
                  )}
                </p>
              </div>
            ) : null}
            {lastRepay ? (
              <div className='bg-muted/30 space-y-1.5 rounded-lg border p-3 text-xs'>
                <p className='text-muted-foreground font-medium'>
                  {t('Repay Result')}
                </p>
                <BreakdownRow
                  label={t('Repaid')}
                  value={formatQuotaWithCurrency(lastRepay.amount)}
                />
                <BreakdownRow
                  label={t('Interest')}
                  value={formatQuotaWithCurrency(lastRepay.interest_part)}
                />
                <BreakdownRow
                  label={t('Principal')}
                  value={formatQuotaWithCurrency(lastRepay.principal_part)}
                />
                {lastRepay.fee_part > 0 ? (
                  <BreakdownRow
                    label={t('Fee')}
                    value={formatQuotaWithCurrency(lastRepay.fee_part)}
                  />
                ) : null}
                {lastRepay.penalty_part > 0 ? (
                  <BreakdownRow
                    label={t('Fast-Settle Penalty')}
                    value={formatQuotaWithCurrency(lastRepay.penalty_part)}
                    emphasis
                  />
                ) : null}
                <BreakdownRow
                  label={t('Debt After')}
                  value={formatQuotaWithCurrency(lastRepay.debt_after)}
                />
              </div>
            ) : null}
            <div className='flex flex-col gap-2 sm:flex-row'>
              <Button
                type='submit'
                disabled={!status || debt <= 0 || submitting}
                className='w-full sm:w-auto'
              >
                {submitting ? t('Submitting...') : t('Repay')}
              </Button>
              <Button
                type='button'
                variant='outline'
                disabled={!status || debt <= 0 || submitting}
                onClick={() => submit('all')}
                className='w-full sm:w-auto'
              >
                {t('Repay All')}
              </Button>
            </div>
          </form>
        </Form>
      </CardContent>
    </Card>
  )
}
