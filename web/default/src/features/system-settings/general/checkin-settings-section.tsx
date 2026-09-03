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
import type { ReactNode } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  enabled: z.boolean(),
  minQuota: z.coerce.number().int().min(0),
  maxQuota: z.coerce.number().int().min(0),
  specialEnabled: z.boolean(),
  specialWeekday: z.enum(['1', '2', '3', '4', '5', '6', '7']),
  specialQuota: z.coerce.number().int().min(0),
  clientCheckEnabled: z.boolean(),
  decayEnabled: z.boolean(),
  decayRate: z.coerce.number().min(0).max(1),
  decayFloor: z.coerce.number().int().min(0),
  usageBoostEnabled: z.boolean(),
  usageBoostDays: z.coerce.number().int().min(1),
  highRewardThreshold: z.coerce.number().min(0).max(1),
  baseHighProbability: z.coerce.number().min(0).max(1),
  boostMaxProbability: z.coerce.number().min(0).max(1),
  makeupEnabled: z.boolean(),
  makeupMaxDays: z.coerce.number().int().min(0),
  makeupCountsTowardProgress: z.boolean(),
  makeupRewardEnabled: z.boolean(),
  riskWatchEnabled: z.boolean(),
  riskWatchDays: z.coerce.number().int().min(1),
  riskMinDailyCalls: z.coerce.number().int().min(0),
  riskMinDailyQuota: z.coerce.number().int().min(0),
  expireEnabled: z.boolean(),
  expireMode: z.enum(['unused', 'all']),
})

type Values = z.infer<typeof schema>

const OPTION_KEYS: Array<[keyof Values, string]> = [
  ['enabled', 'checkin_setting.enabled'],
  ['minQuota', 'checkin_setting.min_quota'],
  ['maxQuota', 'checkin_setting.max_quota'],
  ['specialEnabled', 'checkin_setting.special_enabled'],
  ['specialWeekday', 'checkin_setting.special_weekday'],
  ['specialQuota', 'checkin_setting.special_quota'],
  ['clientCheckEnabled', 'checkin_setting.client_check_enabled'],
  ['decayEnabled', 'checkin_setting.decay_enabled'],
  ['decayRate', 'checkin_setting.decay_rate'],
  ['decayFloor', 'checkin_setting.decay_floor'],
  ['usageBoostEnabled', 'checkin_setting.usage_boost_enabled'],
  ['usageBoostDays', 'checkin_setting.usage_boost_days'],
  ['highRewardThreshold', 'checkin_setting.high_reward_threshold'],
  ['baseHighProbability', 'checkin_setting.base_high_probability'],
  ['boostMaxProbability', 'checkin_setting.boost_max_probability'],
  ['makeupEnabled', 'checkin_setting.makeup_enabled'],
  ['makeupMaxDays', 'checkin_setting.makeup_max_days'],
  [
    'makeupCountsTowardProgress',
    'checkin_setting.makeup_counts_toward_progress',
  ],
  ['makeupRewardEnabled', 'checkin_setting.makeup_reward_enabled'],
  ['riskWatchEnabled', 'checkin_setting.risk_watch_enabled'],
  ['riskWatchDays', 'checkin_setting.risk_watch_days'],
  ['riskMinDailyCalls', 'checkin_setting.risk_min_daily_calls'],
  ['riskMinDailyQuota', 'checkin_setting.risk_min_daily_quota'],
  ['expireEnabled', 'checkin_setting.expire_enabled'],
  ['expireMode', 'checkin_setting.expire_mode'],
]

function GroupTitle({ children }: { children: ReactNode }) {
  return (
    <h4 className='text-muted-foreground text-sm font-medium'>{children}</h4>
  )
}

export function CheckinSettingsSection({
  defaultValues,
}: {
  defaultValues: Values
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues,
  })

  const { isDirty, isSubmitting } = form.formState
  const enabled = form.watch('enabled')
  const specialEnabled = form.watch('specialEnabled')
  const decayEnabled = form.watch('decayEnabled')
  const usageBoostEnabled = form.watch('usageBoostEnabled')
  const makeupEnabled = form.watch('makeupEnabled')
  const riskWatchEnabled = form.watch('riskWatchEnabled')
  const expireEnabled = form.watch('expireEnabled')
  const disabled = updateOption.isPending || isSubmitting

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []

    for (const [field, key] of OPTION_KEYS) {
      if (values[field] !== defaultValues[field]) {
        updates.push({ key, value: String(values[field]) })
      }
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

  function renderSwitch(
    name:
      | 'enabled'
      | 'specialEnabled'
      | 'clientCheckEnabled'
      | 'decayEnabled'
      | 'usageBoostEnabled'
      | 'makeupEnabled'
      | 'makeupCountsTowardProgress'
      | 'makeupRewardEnabled'
      | 'riskWatchEnabled'
      | 'expireEnabled',
    label: string,
    description: string
  ) {
    return (
      <FormField
        control={form.control}
        name={name}
        render={({ field }) => (
          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormLabel>{t(label)}</FormLabel>
              <FormDescription>{t(description)}</FormDescription>
            </SettingsSwitchContent>
            <FormControl>
              <Switch
                checked={field.value}
                onCheckedChange={field.onChange}
                disabled={disabled}
              />
            </FormControl>
          </SettingsSwitchItem>
        )}
      />
    )
  }

  function renderNumberField(
    name:
      | 'minQuota'
      | 'maxQuota'
      | 'specialQuota'
      | 'decayRate'
      | 'decayFloor'
      | 'usageBoostDays'
      | 'highRewardThreshold'
      | 'baseHighProbability'
      | 'boostMaxProbability'
      | 'makeupMaxDays'
      | 'riskWatchDays'
      | 'riskMinDailyCalls'
      | 'riskMinDailyQuota',
    label: string,
    description: string,
    options?: { min?: number; max?: number; step?: string }
  ) {
    return (
      <FormField
        control={form.control}
        name={name}
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t(label)}</FormLabel>
            <FormControl>
              <Input
                type='number'
                min={options?.min ?? 0}
                max={options?.max}
                step={options?.step}
                disabled={disabled}
                {...field}
              />
            </FormControl>
            <FormDescription>{t(description)}</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    )
  }

  return (
    <SettingsSection title={t('Check-in Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={disabled}
            isSaveDisabled={!isDirty}
            saveLabel='Save check-in settings'
          />
          {renderSwitch(
            'enabled',
            'Enable check-in feature',
            'Allow users to check in daily for random quota rewards'
          )}

          {enabled && (
            <>
              <div className='grid gap-6 sm:grid-cols-2'>
                {renderNumberField(
                  'minQuota',
                  'Minimum check-in quota',
                  'Minimum quota amount awarded for check-in'
                )}
                {renderNumberField(
                  'maxQuota',
                  'Maximum check-in quota',
                  'Maximum quota amount awarded for check-in'
                )}
              </div>

              <GroupTitle>{t('Special weekday')}</GroupTitle>
              {renderSwitch(
                'specialEnabled',
                'Enable special weekday reward',
                'Award a fixed quota on the selected weekday'
              )}
              {specialEnabled && (
                <div className='grid gap-6 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='specialWeekday'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Special weekday')}</FormLabel>
                        <Select
                          items={[
                            { value: '1', label: t('Monday') },
                            { value: '2', label: t('Tuesday') },
                            { value: '3', label: t('Wednesday') },
                            { value: '4', label: t('Thursday') },
                            { value: '5', label: t('Friday') },
                            { value: '6', label: t('Saturday') },
                            { value: '7', label: t('Sunday') },
                          ]}
                          value={field.value}
                          onValueChange={(value) =>
                            value !== null && field.onChange(value)
                          }
                          disabled={disabled}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='1'>{t('Monday')}</SelectItem>
                              <SelectItem value='2'>{t('Tuesday')}</SelectItem>
                              <SelectItem value='3'>
                                {t('Wednesday')}
                              </SelectItem>
                              <SelectItem value='4'>{t('Thursday')}</SelectItem>
                              <SelectItem value='5'>{t('Friday')}</SelectItem>
                              <SelectItem value='6'>{t('Saturday')}</SelectItem>
                              <SelectItem value='7'>{t('Sunday')}</SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {t('Weekday that grants the special quota')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  {renderNumberField(
                    'specialQuota',
                    'Special weekday quota',
                    'Fixed quota awarded on the special weekday'
                  )}
                </div>
              )}

              <GroupTitle>{t('Anti-script')}</GroupTitle>
              {renderSwitch(
                'clientCheckEnabled',
                'Enable client check',
                'Require client-side verification before check-in'
              )}

              <GroupTitle>{t('Guaranteed quota decay')}</GroupTitle>
              {renderSwitch(
                'decayEnabled',
                'Enable guaranteed quota decay',
                'Decay the guaranteed minimum quota for consecutive check-ins'
              )}
              {decayEnabled && (
                <div className='grid gap-6 sm:grid-cols-2'>
                  {renderNumberField(
                    'decayRate',
                    'Decay rate',
                    'Multiplier applied to the guaranteed quota per day (0-1)',
                    { min: 0, max: 1, step: 'any' }
                  )}
                  {renderNumberField(
                    'decayFloor',
                    'Decay floor',
                    'Lower bound of the guaranteed quota; 0 falls back to the minimum quota'
                  )}
                </div>
              )}

              <GroupTitle>{t('Usage boost')}</GroupTitle>
              {renderSwitch(
                'usageBoostEnabled',
                'Enable usage boost',
                'Increase high-reward probability for active users'
              )}
              {usageBoostEnabled && (
                <div className='grid gap-6 sm:grid-cols-2'>
                  {renderNumberField(
                    'usageBoostDays',
                    'Usage window days',
                    'Number of recent days used to evaluate user activity',
                    { min: 1 }
                  )}
                  {renderNumberField(
                    'highRewardThreshold',
                    'High reward threshold',
                    'Quota ratio above which a reward counts as high (0-1)',
                    { min: 0, max: 1, step: 'any' }
                  )}
                  {renderNumberField(
                    'baseHighProbability',
                    'Base high-reward probability',
                    'Base probability of a high reward (0-1)',
                    { min: 0, max: 1, step: 'any' }
                  )}
                  {renderNumberField(
                    'boostMaxProbability',
                    'Max boosted probability',
                    'Maximum high-reward probability after boosting (0-1)',
                    { min: 0, max: 1, step: 'any' }
                  )}
                </div>
              )}

              <GroupTitle>{t('Makeup check-in')}</GroupTitle>
              {renderSwitch(
                'makeupEnabled',
                'Enable makeup check-in',
                'Allow users to make up missed check-in days'
              )}
              {makeupEnabled && (
                <>
                  <div className='grid gap-6 sm:grid-cols-2'>
                    {renderNumberField(
                      'makeupMaxDays',
                      'Max makeup days',
                      'Maximum number of past days eligible for makeup check-in'
                    )}
                  </div>
                  {renderSwitch(
                    'makeupCountsTowardProgress',
                    'Makeup counts toward streak',
                    'Count makeup check-ins toward consecutive check-in progress'
                  )}
                  {renderSwitch(
                    'makeupRewardEnabled',
                    'Makeup grants reward',
                    'Grant check-in quota on makeup; when off, makeup only fills the record'
                  )}
                </>
              )}

              <GroupTitle>{t('Risk watch')}</GroupTitle>
              {renderSwitch(
                'riskWatchEnabled',
                'Enable risk watch',
                'Watch users whose check-ins lack real usage'
              )}
              {riskWatchEnabled && (
                <div className='grid gap-6 sm:grid-cols-2'>
                  {renderNumberField(
                    'riskWatchDays',
                    'Watch window days',
                    'Number of recent days used to evaluate risk',
                    { min: 1 }
                  )}
                  {renderNumberField(
                    'riskMinDailyCalls',
                    'Min daily calls',
                    'Minimum average daily API calls to be considered active'
                  )}
                  {renderNumberField(
                    'riskMinDailyQuota',
                    'Min daily quota usage',
                    'Minimum average daily quota usage to be considered active'
                  )}
                </div>
              )}

              <GroupTitle>{t('Quota expiration')}</GroupTitle>
              {renderSwitch(
                'expireEnabled',
                'Enable check-in quota expiration',
                'Expire quota earned from check-ins'
              )}
              {expireEnabled && (
                <div className='grid gap-6 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='expireMode'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Expiration mode')}</FormLabel>
                        <Select
                          items={[
                            { value: 'unused', label: t('Unused only') },
                            { value: 'all', label: t('All earned quota') },
                          ]}
                          value={field.value}
                          onValueChange={(value) =>
                            value !== null && field.onChange(value)
                          }
                          disabled={disabled}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='unused'>
                                {t('Unused only')}
                              </SelectItem>
                              <SelectItem value='all'>
                                {t('All earned quota')}
                              </SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {t(
                            'Whether only unused quota expires or all earned quota expires'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}
            </>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
