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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

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

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const botProtectionSchema = z.object({
  TurnstileCheckEnabled: z.boolean(),
  TurnstileLoginEnabled: z.boolean(),
  TurnstileRegisterEnabled: z.boolean(),
  TurnstileRegisterEmailVerificationEnabled: z.boolean(),
  TurnstileEmailBindingVerificationEnabled: z.boolean(),
  TurnstilePasswordResetEnabled: z.boolean(),
  TurnstileCheckinEnabled: z.boolean(),
  TurnstileSensitiveUpdateEnabled: z.boolean(),
  TurnstileSiteKey: z.string().optional(),
  TurnstileSecretKey: z.string().optional(),
})

type BotProtectionFormValues = z.infer<typeof botProtectionSchema>

const turnstileToggleKeys = [
  'TurnstileLoginEnabled',
  'TurnstileRegisterEnabled',
  'TurnstileRegisterEmailVerificationEnabled',
  'TurnstileEmailBindingVerificationEnabled',
  'TurnstilePasswordResetEnabled',
  'TurnstileCheckinEnabled',
  'TurnstileSensitiveUpdateEnabled',
] as const

type TurnstileToggleKey = (typeof turnstileToggleKeys)[number]

// 新增 Turnstile 校验入口时，必须在这里和后端配置中增加专属开关，不要复用已有开关。
const turnstileScopes: Array<{
  key: TurnstileToggleKey
  label: string
  description: string
}> = [
  {
    key: 'TurnstileLoginEnabled',
    label: 'Login verification',
    description: 'Require Turnstile before password login',
  },
  {
    key: 'TurnstileRegisterEnabled',
    label: 'Registration verification',
    description: 'Require Turnstile before password registration',
  },
  {
    key: 'TurnstileRegisterEmailVerificationEnabled',
    label: 'Registration email code verification',
    description: 'Require Turnstile before sending registration email codes',
  },
  {
    key: 'TurnstileEmailBindingVerificationEnabled',
    label: 'Email binding code verification',
    description: 'Require Turnstile before sending email binding codes',
  },
  {
    key: 'TurnstilePasswordResetEnabled',
    label: 'Password reset verification',
    description: 'Require Turnstile before sending password reset emails',
  },
  {
    key: 'TurnstileCheckinEnabled',
    label: 'Daily check-in verification',
    description: 'Require Turnstile before daily check-in',
  },
  {
    key: 'TurnstileSensitiveUpdateEnabled',
    label: 'Sensitive profile update verification',
    description: 'Require Turnstile before username or password changes',
  },
]

type BotProtectionSectionProps = {
  defaultValues: BotProtectionFormValues
}

export function BotProtectionSection({
  defaultValues,
}: BotProtectionSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<BotProtectionFormValues>({
    resolver: zodResolver(botProtectionSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (data: BotProtectionFormValues) => {
    const updates = Object.entries(data).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof BotProtectionFormValues]
    )

    const orderedUpdates = updates.sort(([left], [right]) => {
      const keyOrder = ['TurnstileSiteKey', 'TurnstileSecretKey']
      const leftIndex = keyOrder.indexOf(left)
      const rightIndex = keyOrder.indexOf(right)
      if (leftIndex === -1 && rightIndex === -1) return 0
      if (leftIndex === -1) return 1
      if (rightIndex === -1) return -1
      return leftIndex - rightIndex
    })

    for (const [key, value] of orderedUpdates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  return (
    <SettingsSection title={t('Bot Protection')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <div className='space-y-3'>
            {turnstileScopes.map((scope) => (
              <FormField
                key={scope.key}
                control={form.control}
                name={scope.key}
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t(scope.label)}</FormLabel>
                      <FormDescription>{t(scope.description)}</FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            ))}
          </div>

          <FormField
            control={form.control}
            name='TurnstileSiteKey'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Site Key')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('Your Turnstile site key')}
                    autoComplete='off'
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='TurnstileSecretKey'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Secret Key')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    placeholder={t('Your Turnstile secret key')}
                    autoComplete='new-password'
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
