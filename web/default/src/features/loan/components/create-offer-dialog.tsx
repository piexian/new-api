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
import { useMemo, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
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
import { useSystemConfigStore } from '@/stores/system-config-store'

import { createLoanMarketOffer } from '../api'
import { LOAN_OFFER_MODE_KEYS, type LoanOfferMode } from '../types'

interface CreateOfferDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}

// 利率输入采用"日利率百分比"（如 0.1 = 0.1%/天），提交时换算为小数
function percentToRate(percent: string): number {
  const n = Number(percent)
  return Number.isFinite(n) ? n / 100 : 0
}

function isValidPositive(v: string): boolean {
  const n = Number(v)
  return Number.isFinite(n) && n > 0
}

function isValidRate(v: string): boolean {
  return isValidPositive(v) && Number(v) <= 100
}

function buildSchema(t: (key: string) => string) {
  return z
    .object({
      mode: z.enum(LOAN_OFFER_MODE_KEYS),
      amount: z
        .string()
        .trim()
        .min(1, t('Please enter an amount'))
        .refine(isValidPositive, t('Please enter a valid positive amount')),
      rateFixed: z
        .string()
        .trim()
        .min(1, t('Please enter a daily rate'))
        .refine(isValidRate, t('Please enter a rate between 0 and 100')),
      rateMin: z
        .string()
        .trim()
        .min(1, t('Please enter a minimum daily rate'))
        .refine(isValidRate, t('Please enter a rate between 0 and 100')),
      rateMax: z
        .string()
        .trim()
        .min(1, t('Please enter a maximum daily rate'))
        .refine(isValidRate, t('Please enter a rate between 0 and 100')),
      perLoanCap: z
        .string()
        .trim()
        .min(1, t('Please enter a per-loan cap'))
        .refine(isValidPositive, t('Please enter a valid positive amount')),
      minCreditScore: z
        .string()
        .trim()
        .min(1, t('Please enter a minimum credit score'))
        .refine((v) => {
          const n = Number(v)
          return Number.isInteger(n) && n >= -50 && n <= 100
        }, t('Credit score must be between -50 and 100')),
    })
    .refine((v) => v.mode !== 'ai' || Number(v.rateMin) <= Number(v.rateMax), {
      path: ['rateMax'],
      message: t('Minimum rate cannot exceed maximum rate'),
    })
}

type Values = z.infer<ReturnType<typeof buildSchema>>

const MODE_DEFAULT_VALUES: Record<LoanOfferMode, Partial<Values>> = {
  pool: { rateFixed: '0.1' },
  ai: { rateMin: '0.1', rateMax: '0.3', perLoanCap: '' },
  order: { rateFixed: '0.1' },
}

export function CreateOfferDialog(props: CreateOfferDialogProps) {
  const { t } = useTranslation()
  const quotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const [submitting, setSubmitting] = useState(false)

  const schema = useMemo(() => buildSchema(t), [t])

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      mode: 'pool',
      amount: '',
      rateFixed: '0.1',
      rateMin: '0.1',
      rateMax: '0.3',
      perLoanCap: '',
      minCreditScore: '-50',
    },
  })

  const mode = form.watch('mode')

  function switchMode(next: LoanOfferMode) {
    form.setValue('mode', next)
    form.clearErrors()
    // 切换模式时回填该模式的默认利率，避免残留值误导
    const defaults = MODE_DEFAULT_VALUES[next]
    for (const [key, value] of Object.entries(defaults)) {
      form.setValue(key as keyof Values, value as never)
    }
  }

  async function onSubmit(values: Values) {
    setSubmitting(true)
    try {
      const res = await createLoanMarketOffer({
        mode: values.mode,
        amount_usd: values.amount.trim(),
        rate_fixed:
          values.mode === 'ai' ? '0' : String(percentToRate(values.rateFixed)),
        rate_min: values.mode === 'ai' ? percentToRate(values.rateMin) : 0,
        rate_max: values.mode === 'ai' ? percentToRate(values.rateMax) : 0,
        per_loan_cap:
          values.mode === 'ai'
            ? Math.round(Number(values.perLoanCap) * quotaPerUnit)
            : 0,
        min_credit_score: Number(values.minCreditScore),
      })
      if (res.success) {
        toast.success(t('Offer created'))
        form.reset()
        props.onOpenChange(false)
        props.onCreated()
        return
      }
      // Backend message is already toasted by the api interceptor
    } catch {
      toast.error(t('Failed to create offer'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Create Loan Offer')}
      description={t(
        'Lend idle quota to the token loan market and earn daily interest.'
      )}
      contentClassName='sm:max-w-lg'
      bodyClassName='space-y-4'
      footer={
        <Button
          onClick={form.handleSubmit(onSubmit)}
          disabled={submitting}
          className='w-full sm:w-auto'
        >
          {submitting ? t('Submitting...') : t('Create Offer')}
        </Button>
      }
    >
      <Form {...form}>
        <form autoComplete='off' className='space-y-4'>
          <div className='space-y-2'>
            <Label>{t('Lending Mode')}</Label>
            <Select
              value={mode}
              onValueChange={(value) => {
                if (value !== null) switchMode(value as LoanOfferMode)
              }}
            >
              <SelectTrigger className='w-full'>
                <SelectValue>{modeLabel(mode, t)}</SelectValue>
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {LOAN_OFFER_MODE_KEYS.map((key) => (
                    <SelectItem key={key} value={key}>
                      {modeLabel(key, t)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <p className='text-muted-foreground text-xs'>
              {modeDescription(mode, t)}
            </p>
          </div>

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
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'The quota is deducted from your balance and locked into the offer.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {mode === 'ai' ? (
            <>
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='rateMin'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Minimum Daily Rate (%)')}</FormLabel>
                      <FormControl>
                        <Input type='number' min={0} step='any' {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='rateMax'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Maximum Daily Rate (%)')}</FormLabel>
                      <FormControl>
                        <Input type='number' min={0} step='any' {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <FormField
                control={form.control}
                name='perLoanCap'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Per-Loan Cap (USD)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step='any'
                        placeholder='0.00'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Maximum quota a single AI-approved loan can take from this offer.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          ) : (
            <FormField
              control={form.control}
              name='rateFixed'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Fixed Daily Rate (%)')}</FormLabel>
                  <FormControl>
                    <Input type='number' min={0} step='any' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('e.g. 0.1 means 0.1% interest per day')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}

          <FormField
            control={form.control}
            name='minCreditScore'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Minimum Credit Score')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={-50}
                    max={100}
                    step={1}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('-50 means no limit, 0 to 100 requires a minimum score')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}

// 模式文案（Label 与描述都走 t()，避免在常量中散落英文）
function modeLabel(mode: LoanOfferMode, t: (key: string) => string): string {
  if (mode === 'pool') return t('Pool (fixed rate)')
  if (mode === 'ai') return t('AI (rate range)')
  return t('Order (public listing)')
}

function modeDescription(
  mode: LoanOfferMode,
  t: (key: string) => string
): string {
  if (mode === 'pool') {
    return t(
      'Your quota joins a shared pool and is auto-matched to borrowers at your fixed rate.'
    )
  }
  if (mode === 'ai') {
    return t(
      'The AI officer negotiates each loan within your rate range and per-loan cap.'
    )
  }
  return t(
    'Your offer is publicly listed and borrowers can borrow from it directly.'
  )
}
