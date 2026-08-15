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
import { useQueryClient } from '@tanstack/react-query'
import { HandCoins, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
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
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatPercent } from '@/lib/format'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { borrowLoan } from '../api'
import type { LoanStatus, MarketOffer } from '../types'

interface BorrowFormProps {
  status?: LoanStatus
  /** 市场浏览中选中的 order 挂单；选中后本次借款优先定向匹配该挂单 */
  presetOrder?: MarketOffer | null
  onClearOrder?: () => void
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

export function BorrowForm(props: BorrowFormProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const quotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const [submitting, setSubmitting] = useState(false)
  // 只接官方资金：跳过市场撮合，整笔由平台放款（官方资金无秒结清惩罚条款）
  const [platformOnly, setPlatformOnly] = useState(false)

  const schema = useMemo(() => buildSchema(t), [t])

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: { amount: '' },
  })

  const status = props.status
  const termsBlocked = !!status && status.terms_enabled && !status.terms_agreed
  const availableUsd = status ? status.available / quotaPerUnit : 0

  async function onSubmit(values: Values) {
    if (!status) return
    const amount = Number(values.amount)
    if (amount > availableUsd) {
      form.setError('amount', {
        type: 'manual',
        message: t('Amount exceeds your available credit'),
      })
      return
    }
    setSubmitting(true)
    try {
      const res = await borrowLoan(
        values.amount,
        props.presetOrder?.id,
        platformOnly
      )
      if (res.success && res.data) {
        toast.success(t('Borrow successful'))
        // 借款后即时提示：本次放款含秒结清惩罚条款的资金，说明手动提前结清的
        // 惩罚与签到自动还款的豁免（签到视为正常还款，不触发惩罚）
        const penaltyTotal = (res.data.fundings ?? []).reduce(
          (sum, f) => sum + (f.fast_repay_penalty_quota > 0 ? f.fast_repay_penalty_quota : 0),
          0
        )
        if (penaltyTotal > 0) {
          toast.warning(
            t(
              'This borrow includes funds with a fast-settle penalty of {{amount}}: a manual full early settlement within the lender window will be charged. Check-in auto-repayment never triggers this penalty and repays normally.',
              { amount: formatQuotaWithCurrency(penaltyTotal) }
            ),
            { duration: 10000 }
          )
        }
        queryClient.setQueryData(['loan-status'], res.data)
        queryClient.invalidateQueries({ queryKey: ['loan-records'] })
        queryClient.invalidateQueries({ queryKey: ['loan-market-fundings'] })
        queryClient.invalidateQueries({ queryKey: ['loan-market-offers'] })
        form.reset()
        props.onClearOrder?.()
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Borrow failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card className='gap-0 py-0'>
      <CardHeader className='border-b p-4 sm:p-5'>
        <div className='flex items-center gap-3'>
          <IconBadge tone='neutral' size='lg'>
            <HandCoins className='h-4 w-4 sm:h-5 sm:w-5' strokeWidth={2} />
          </IconBadge>
          <div className='min-w-0'>
            <CardTitle className='text-lg'>{t('Borrow')}</CardTitle>
            <p className='text-muted-foreground mt-0.5 text-xs sm:text-sm'>
              {t('Borrowed quota is credited to your balance immediately')}
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
                      disabled={!status || termsBlocked}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Available to Borrow')}:{' '}
                    {status ? formatQuotaWithCurrency(status.available) : '-'}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            {termsBlocked ? (
              <p className='text-muted-foreground text-xs'>
                {t('Please agree to the loan terms before borrowing')}
              </p>
            ) : null}
            {props.presetOrder ? (
              <div className='bg-muted/30 flex items-start justify-between gap-2 rounded-lg border p-3 text-xs'>
                <div className='space-y-0.5'>
                  <p className='text-muted-foreground'>
                    {t('Borrowing from order offer #{{id}}', {
                      id: props.presetOrder.id,
                    })}
                  </p>
                  <p>
                    {t('Fixed rate')}:{' '}
                    {formatPercent(props.presetOrder.rate_fixed * 100)} ·{' '}
                    {t('Available')}:{' '}
                    {formatQuotaWithCurrency(
                      props.presetOrder.amount_available
                    )}
                  </p>
                  <p className='text-muted-foreground'>
                    {t(
                      'If the order cannot cover the full amount, the rest is covered by the platform.'
                    )}
                  </p>
                </div>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='h-6 w-6 shrink-0'
                  aria-label={t('Clear order selection')}
                  onClick={() => props.onClearOrder?.()}
                >
                  <X className='h-3.5 w-3.5' />
                </Button>
              </div>
            ) : null}
            {status?.market_enabled && !props.presetOrder ? (
              <div className='flex items-start gap-2'>
                <Checkbox
                  id='borrow-platform-only'
                  checked={platformOnly}
                  onCheckedChange={(checked) =>
                    setPlatformOnly(checked === true)
                  }
                />
                <div className='grid gap-0.5 leading-none'>
                  <label
                    htmlFor='borrow-platform-only'
                    className='text-sm font-medium peer-disabled:cursor-not-allowed peer-disabled:opacity-70'
                  >
                    {t('Official funds only')}
                  </label>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Skip marketplace offers and borrow entirely from the official pool, which never carries fast-settle penalties.'
                    )}
                  </p>
                </div>
              </div>
            ) : null}
            <Button
              type='submit'
              disabled={!status || termsBlocked || submitting}
              className='w-full sm:w-auto'
            >
              {submitting ? t('Submitting...') : t('Borrow')}
            </Button>
          </form>
        </Form>
      </CardContent>
    </Card>
  )
}
