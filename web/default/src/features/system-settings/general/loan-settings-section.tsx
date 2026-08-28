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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useFieldArray, useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useSystemConfigStore } from '@/stores/system-config-store'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

// schema 需要 t() 文案，在组件内构建
function buildSchema(t: (key: string) => string) {
  return z
    .object({
      enabled: z.boolean(),
      maxTotalUsd: z.coerce.number().min(0),
      dailyRate: z.coerce.number().min(0),
      repayFeeRate: z.coerce.number().min(0),
      minRegisterDays: z.coerce.number().int().min(0),
      maxPerBorrowUsd: z.coerce.number().min(0),
      checkinRepayEnabled: z.boolean(),
      lenderOverflowNotifyEnabled: z.boolean(),
      aiEnabled: z.boolean(),
      aiModels: z.array(
        z.object({
          model: z.string().trim().min(1, t('Model name is required')),
          contextWindow: z.coerce
            .number()
            .int(t('Context window must be an integer'))
            .positive(t('Context window must be greater than 0')),
        })
      ),
      creditTiers: z.array(
        z.object({
          // 允许负信用分（信用分下限为 -50）
          minScore: z.coerce.number().int(),
          maxTotalUsd: z.coerce.number().min(0),
        })
      ),
      aiMaxLimitUsd: z.coerce.number().min(0),
      aiMinRate: z.coerce.number().min(0),
      aiMaxGraceDays: z.coerce.number().int().min(0),
      aiMaxActiveApplications: z.coerce.number().int().min(0),
      aiDailyLimit: z.coerce.number().int().min(0),
      aiMaxRounds: z.coerce.number().int().min(0),
      aiMaxOutput: z.coerce.number().int().min(0),
      aiPrompt: z.string(),
      termsEnabled: z.boolean(),
      termsText: z.string(),
    })
    .superRefine((values, ctx) => {
      // AI 可批准的最低日利率不得高于全局日利率，否则审批边界自相矛盾
      if (
        values.enabled &&
        values.aiEnabled &&
        values.aiMinRate > values.dailyRate
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['aiMinRate'],
          message: t(
            'AI minimum daily rate cannot exceed the daily interest rate'
          ),
        })
      }
    })
}

type Values = z.infer<ReturnType<typeof buildSchema>>

type AiModelRow = Values['aiModels'][number]

type CreditTierRow = Values['creditTiers'][number]

export type LoanSettingsDefaults = {
  enabled: boolean
  maxTotal: number
  dailyRate: number
  repayFeeRate: number
  minRegisterDays: number
  maxPerBorrow: number
  checkinRepayEnabled: boolean
  lenderOverflowNotifyEnabled: boolean
  aiEnabled: boolean
  aiModels: string
  aiMaxLimit: number
  aiMinRate: number
  aiMaxGraceDays: number
  aiMaxActiveApplications: number
  aiDailyLimit: number
  aiMaxRounds: number
  aiMaxOutput: number
  aiPrompt: string
  creditTierLimits: string
  termsEnabled: boolean
  termsText: string
}

// ai_models 后端以 JSON 字符串存储，解析失败时回退为空列表
function parseAiModels(raw: string): AiModelRow[] {
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter(
        (item): item is { model: string; context_window: number } =>
          !!item && typeof item === 'object' && 'model' in item
      )
      .map((item) => ({
        model: String(item.model),
        contextWindow: Number(item.context_window) || 0,
      }))
  } catch {
    return []
  }
}

// 序列化时过滤掉模型名为空的行
function serializeAiModels(rows: AiModelRow[]): string {
  const normalized = rows
    .filter((row) => row.model.trim() !== '')
    .map((row) => ({
      model: row.model.trim(),
      context_window: Math.round(row.contextWindow) || 0,
    }))
  return JSON.stringify(normalized)
}

// credit_tier_limits 后端以 JSON 字符串存储（min_score 整数、max_total 为 quota），
// 解析失败时回退为空列表；max_total 换算为 USD 供编辑
function parseCreditTiers(raw: string, quotaPerUnit: number): CreditTierRow[] {
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter(
        (item): item is { min_score: number; max_total: number } =>
          !!item && typeof item === 'object' && 'min_score' in item
      )
      .map((item) => ({
        minScore: Number(item.min_score) || 0,
        maxTotalUsd: (Number(item.max_total) || 0) / quotaPerUnit,
      }))
  } catch {
    return []
  }
}

// 序列化时过滤掉 max_total 无效的行，min_score 取整、max_total 由 USD 换算回 quota
function serializeCreditTiers(
  rows: CreditTierRow[],
  quotaPerUnit: number
): string {
  const normalized = rows
    .filter(
      (row) =>
        Number.isFinite(Number(row.maxTotalUsd)) && Number(row.maxTotalUsd) >= 0
    )
    .map((row) => ({
      min_score: Math.round(Number(row.minScore) || 0),
      max_total: Math.round((Number(row.maxTotalUsd) || 0) * quotaPerUnit),
    }))
  return JSON.stringify(normalized)
}

export function LoanSettingsSection(props: {
  defaultValues: LoanSettingsDefaults
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const quotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )

  const defaults = props.defaultValues
  const initialValues: Values = useMemo(
    () => ({
      enabled: defaults.enabled,
      maxTotalUsd: defaults.maxTotal / quotaPerUnit,
      dailyRate: defaults.dailyRate,
      repayFeeRate: defaults.repayFeeRate,
      minRegisterDays: defaults.minRegisterDays,
      maxPerBorrowUsd: defaults.maxPerBorrow / quotaPerUnit,
      checkinRepayEnabled: defaults.checkinRepayEnabled,
      lenderOverflowNotifyEnabled: defaults.lenderOverflowNotifyEnabled,
      aiEnabled: defaults.aiEnabled,
      aiModels: parseAiModels(defaults.aiModels),
      creditTiers: parseCreditTiers(defaults.creditTierLimits, quotaPerUnit),
      aiMaxLimitUsd: defaults.aiMaxLimit / quotaPerUnit,
      aiMinRate: defaults.aiMinRate,
      aiMaxGraceDays: defaults.aiMaxGraceDays,
      aiMaxActiveApplications: defaults.aiMaxActiveApplications,
      aiDailyLimit: defaults.aiDailyLimit,
      aiMaxRounds: defaults.aiMaxRounds,
      aiMaxOutput: defaults.aiMaxOutput,
      aiPrompt: defaults.aiPrompt,
      termsEnabled: defaults.termsEnabled,
      termsText: defaults.termsText,
    }),
    [defaults, quotaPerUnit]
  )

  const schema = useMemo(() => buildSchema(t), [t])

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: initialValues,
  })

  // 选项与额度汇率均为异步加载，就绪后同步进表单，否则首次打开会一直显示内置默认值
  useEffect(() => {
    form.reset(initialValues)
  }, [initialValues, form])

  const aiModelFields = useFieldArray({
    control: form.control,
    name: 'aiModels',
  })

  const creditTierFields = useFieldArray({
    control: form.control,
    name: 'creditTiers',
  })

  const { isDirty, isSubmitting } = form.formState
  const enabled = form.watch('enabled')
  const aiEnabled = form.watch('aiEnabled')
  const termsEnabled = form.watch('termsEnabled')
  const maxTotalUsd = form.watch('maxTotalUsd')
  const maxPerBorrowUsd = form.watch('maxPerBorrowUsd')
  const aiMaxLimitUsd = form.watch('aiMaxLimitUsd')

  const toQuota = (usd: number) => Math.round((Number(usd) || 0) * quotaPerUnit)
  const quotaHint = (usd: number) =>
    t('Saved as {{quota}} quota', { quota: toQuota(usd).toLocaleString() })

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []

    if (values.enabled !== defaults.enabled) {
      updates.push({
        key: 'loan_setting.enabled',
        value: String(values.enabled),
      })
    }

    const maxTotal = toQuota(values.maxTotalUsd)
    if (maxTotal !== defaults.maxTotal) {
      updates.push({ key: 'loan_setting.max_total', value: String(maxTotal) })
    }

    if (values.dailyRate !== defaults.dailyRate) {
      updates.push({
        key: 'loan_setting.daily_rate',
        value: String(values.dailyRate),
      })
    }

    if (values.repayFeeRate !== defaults.repayFeeRate) {
      updates.push({
        key: 'loan_setting.repay_fee_rate',
        value: String(values.repayFeeRate),
      })
    }

    if (values.minRegisterDays !== defaults.minRegisterDays) {
      updates.push({
        key: 'loan_setting.min_register_days',
        value: String(values.minRegisterDays),
      })
    }

    const maxPerBorrow = toQuota(values.maxPerBorrowUsd)
    if (maxPerBorrow !== defaults.maxPerBorrow) {
      updates.push({
        key: 'loan_setting.max_per_borrow',
        value: String(maxPerBorrow),
      })
    }

    if (values.checkinRepayEnabled !== defaults.checkinRepayEnabled) {
      updates.push({
        key: 'loan_setting.checkin_repay_enabled',
        value: String(values.checkinRepayEnabled),
      })
    }

    if (
      values.lenderOverflowNotifyEnabled !==
      defaults.lenderOverflowNotifyEnabled
    ) {
      updates.push({
        key: 'notify_setting.loan_lender_overflow',
        value: String(values.lenderOverflowNotifyEnabled),
      })
    }

    if (values.aiEnabled !== defaults.aiEnabled) {
      updates.push({
        key: 'loan_setting.ai_enabled',
        value: String(values.aiEnabled),
      })
    }

    const aiModels = serializeAiModels(values.aiModels)
    if (aiModels !== serializeAiModels(parseAiModels(defaults.aiModels))) {
      updates.push({ key: 'loan_setting.ai_models', value: aiModels })
    }

    const creditTiers = serializeCreditTiers(values.creditTiers, quotaPerUnit)
    if (
      creditTiers !==
      serializeCreditTiers(
        parseCreditTiers(defaults.creditTierLimits, quotaPerUnit),
        quotaPerUnit
      )
    ) {
      updates.push({
        key: 'loan_setting.credit_tier_limits',
        value: creditTiers,
      })
    }

    const aiMaxLimit = toQuota(values.aiMaxLimitUsd)
    if (aiMaxLimit !== defaults.aiMaxLimit) {
      updates.push({
        key: 'loan_setting.ai_max_limit',
        value: String(aiMaxLimit),
      })
    }

    if (values.aiMinRate !== defaults.aiMinRate) {
      updates.push({
        key: 'loan_setting.ai_min_rate',
        value: String(values.aiMinRate),
      })
    }

    if (values.aiMaxGraceDays !== defaults.aiMaxGraceDays) {
      updates.push({
        key: 'loan_setting.ai_max_grace_days',
        value: String(values.aiMaxGraceDays),
      })
    }

    if (values.aiMaxActiveApplications !== defaults.aiMaxActiveApplications) {
      updates.push({
        key: 'loan_setting.ai_max_active_applications',
        value: String(values.aiMaxActiveApplications),
      })
    }

    if (values.aiDailyLimit !== defaults.aiDailyLimit) {
      updates.push({
        key: 'loan_setting.ai_daily_limit',
        value: String(values.aiDailyLimit),
      })
    }

    if (values.aiMaxRounds !== defaults.aiMaxRounds) {
      updates.push({
        key: 'loan_setting.ai_max_rounds',
        value: String(values.aiMaxRounds),
      })
    }

    if (values.aiMaxOutput !== defaults.aiMaxOutput) {
      updates.push({
        key: 'loan_setting.ai_max_output',
        value: String(values.aiMaxOutput),
      })
    }

    if (values.aiPrompt !== defaults.aiPrompt) {
      updates.push({ key: 'loan_setting.ai_prompt', value: values.aiPrompt })
    }

    if (values.termsEnabled !== defaults.termsEnabled) {
      updates.push({
        key: 'loan_setting.terms_enabled',
        value: String(values.termsEnabled),
      })
    }

    if (values.termsText !== defaults.termsText) {
      updates.push({ key: 'loan_setting.terms_text', value: values.termsText })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    form.reset(values)
  }

  return (
    <SettingsSection title={t('Token Loan Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save loan settings'
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable token loan')}</FormLabel>
                  <FormDescription>
                    {t('Allow users to borrow quota and repay it later')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <>
              <div className='grid gap-6 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='maxTotalUsd'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Max total credit (USD)')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          step='any'
                          placeholder='5'
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Total outstanding quota a user can borrow')}.{' '}
                        {quotaHint(maxTotalUsd)}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='maxPerBorrowUsd'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Max per borrow (USD)')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          step='any'
                          placeholder='0'
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Single borrow limit; 0 follows max total credit')}.{' '}
                        {quotaHint(maxPerBorrowUsd)}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='dailyRate'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Daily interest rate')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          step='any'
                          placeholder='0.001'
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Daily compound rate, e.g. 0.001 means 0.1% per day'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='repayFeeRate'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Early repayment fee rate')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          step='any'
                          placeholder='0.0001'
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Fee charged on the principal part of manual early repayments, e.g. 0.0001 means 0.01%; check-in auto repayment is never charged'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='minRegisterDays'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Minimum registration days')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={0}
                          placeholder='0'
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Accounts must be registered for at least this many days to borrow'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name='checkinRepayEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Check-in auto repayment')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Check-in rewards are automatically used to repay outstanding loans'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              <FormField
                control={form.control}
                name='lenderOverflowNotifyEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Lender overflow notifications')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Notify the admin when a repayment rolls back because a lender balance is full'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              <FormField
                control={form.control}
                name='aiEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable AI loan officer')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Let users negotiate credit limit, rate and grace period with an AI officer'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              {aiEnabled && (
                <div className='grid gap-6'>
                  <FormItem>
                    <FormLabel>{t('AI officer models')}</FormLabel>
                    <div className='grid gap-2'>
                      {aiModelFields.fields.map((item, index) => (
                        <div key={item.id} className='flex items-start gap-2'>
                          <FormField
                            control={form.control}
                            name={`aiModels.${index}.model`}
                            render={({ field }) => (
                              <FormItem className='flex-1'>
                                <FormControl>
                                  <Input
                                    placeholder={t('Model name')}
                                    {...field}
                                  />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <FormField
                            control={form.control}
                            name={`aiModels.${index}.contextWindow`}
                            render={({ field }) => (
                              <FormItem className='w-36'>
                                <FormControl>
                                  <Input
                                    type='number'
                                    min={0}
                                    placeholder={t('Context window')}
                                    {...field}
                                  />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <Button
                            type='button'
                            variant='outline'
                            size='icon'
                            onClick={() => aiModelFields.remove(index)}
                            aria-label={t('Remove model')}
                          >
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                      ))}
                      <div>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() =>
                            aiModelFields.append({
                              model: '',
                              contextWindow: 0,
                            })
                          }
                        >
                          <Plus className='h-4 w-4' />
                          {t('Add model')}
                        </Button>
                      </div>
                    </div>
                    <FormDescription>
                      {t(
                        'Models available to the AI officer, with each model context window in tokens'
                      )}
                    </FormDescription>
                  </FormItem>

                  <div className='grid gap-6 sm:grid-cols-2'>
                    <FormField
                      control={form.control}
                      name='aiMaxLimitUsd'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('AI max credit limit (USD)')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              step='any'
                              placeholder='20'
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Highest credit limit the AI officer may approve'
                            )}
                            . {quotaHint(aiMaxLimitUsd)}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='aiMinRate'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('AI minimum daily rate')}</FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              step='any'
                              placeholder='0.0005'
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('Lowest daily rate the AI officer may approve')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='aiMaxGraceDays'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('AI max interest-free days')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              placeholder='30'
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Longest interest-free period the AI officer may grant'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='aiMaxActiveApplications'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('Max active applications per user')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              placeholder='1'
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'How many AI officer applications a user can run at once'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='aiDailyLimit'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('Daily application limit per user')}
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              placeholder='3'
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'How many AI officer applications a user can start per day'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='aiMaxRounds'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Max conversation rounds')}</FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              placeholder='10'
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('Maximum conversation rounds per application')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='aiMaxOutput'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Max output tokens')}</FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              placeholder='2048'
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('Maximum output tokens per AI officer reply')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <FormItem>
                    <FormLabel>{t('Credit score tier limits')}</FormLabel>
                    <div className='grid gap-2'>
                      {creditTierFields.fields.map((item, index) => (
                        <div key={item.id} className='flex items-start gap-2'>
                          <FormField
                            control={form.control}
                            name={`creditTiers.${index}.minScore`}
                            render={({ field }) => (
                              <FormItem className='w-36'>
                                <FormControl>
                                  <Input
                                    type='number'
                                    placeholder={t('Min score')}
                                    {...field}
                                  />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <FormField
                            control={form.control}
                            name={`creditTiers.${index}.maxTotalUsd`}
                            render={({ field }) => (
                              <FormItem className='flex-1'>
                                <FormControl>
                                  <Input
                                    type='number'
                                    min={0}
                                    step='any'
                                    placeholder={t('Max total (USD)')}
                                    {...field}
                                  />
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                          <Button
                            type='button'
                            variant='outline'
                            size='icon'
                            onClick={() => creditTierFields.remove(index)}
                            aria-label={t('Remove tier')}
                          >
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                      ))}
                      <div>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={() =>
                            creditTierFields.append({
                              minScore: 0,
                              maxTotalUsd: 0,
                            })
                          }
                        >
                          <Plus className='h-4 w-4' />
                          {t('Add tier')}
                        </Button>
                      </div>
                    </div>
                    <FormDescription>
                      {t(
                        'Cap the total AI-granted credit limit by user credit score tier; the tier cap applies to the total limit granted, not per borrow'
                      )}
                    </FormDescription>
                  </FormItem>

                  <FormField
                    control={form.control}
                    name='aiPrompt'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('AI officer system prompt')}</FormLabel>
                        <FormControl>
                          <Textarea rows={8} {...field} />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'System prompt template for the AI officer; keep the hard-boundary placeholders intact'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}

              <FormField
                control={form.control}
                name='termsEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Require terms confirmation')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Users must agree to the loan terms before borrowing'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              {termsEnabled && (
                <FormField
                  control={form.control}
                  name='termsText'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Loan terms text')}</FormLabel>
                      <FormControl>
                        <Textarea rows={5} {...field} />
                      </FormControl>
                      <FormDescription>
                        {t('Terms shown to users before their first borrow')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
