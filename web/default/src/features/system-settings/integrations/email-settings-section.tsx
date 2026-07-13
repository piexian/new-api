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
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
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
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const createEmailSchema = (t: (key: string) => string) =>
  z.object({
    EmailProvider: z.string(),
    SMTPServer: z.string(),
    SMTPPort: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^\d+$/.test(trimmed)
    }, t('Port must be a positive integer')),
    SMTPAccount: z.string(),
    SMTPFrom: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)
    }, t('Enter a valid email or leave blank')),
    SMTPToken: z.string(),
    SMTPSSLEnabled: z.boolean(),
    SMTPStartTLSEnabled: z.boolean(),
    SMTPInsecureSkipVerify: z.boolean(),
    SMTPForceAuthLogin: z.boolean(),
    CFEmailAccountID: z.string(),
    CFEmailAPIToken: z.string(),
    CFEmailFrom: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)
    }, t('Enter a valid email or leave blank')),
    EmailDailyLimit: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^\d+$/.test(trimmed)
    }, t('Must be a non-negative integer')),
    EmailVerificationDailyLimitPerUser: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^\d+$/.test(trimmed)
    }, t('Must be a non-negative integer')),
  })

type EmailFormValues = z.infer<ReturnType<typeof createEmailSchema>>

type EmailSettingsSectionProps = {
  defaultValues: EmailFormValues
}

type SmtpSecurityMode = 'none' | 'ssl_tls' | 'starttls'

function getSmtpSecurityMode(values: {
  SMTPSSLEnabled: boolean
  SMTPStartTLSEnabled: boolean
}): SmtpSecurityMode {
  if (values.SMTPSSLEnabled) return 'ssl_tls'
  if (values.SMTPStartTLSEnabled) return 'starttls'
  return 'none'
}

export function EmailSettingsSection({
  defaultValues,
}: EmailSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const emailSchema = createEmailSchema(t)

  const form = useForm<EmailFormValues>({
    resolver: zodResolver(emailSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (values: EmailFormValues) => {
    const securityMode = getSmtpSecurityMode(values)
    const sanitized = {
      EmailProvider: values.EmailProvider.trim(),
      SMTPServer: values.SMTPServer.trim(),
      SMTPPort: values.SMTPPort.trim(),
      SMTPAccount: values.SMTPAccount.trim(),
      SMTPFrom: values.SMTPFrom.trim(),
      SMTPToken: values.SMTPToken.trim(),
      SMTPSSLEnabled: securityMode === 'ssl_tls',
      SMTPStartTLSEnabled: securityMode === 'starttls',
      SMTPInsecureSkipVerify: values.SMTPInsecureSkipVerify,
      SMTPForceAuthLogin: values.SMTPForceAuthLogin,
      CFEmailAccountID: values.CFEmailAccountID.trim(),
      CFEmailAPIToken: values.CFEmailAPIToken.trim(),
      CFEmailFrom: values.CFEmailFrom.trim(),
      EmailDailyLimit: values.EmailDailyLimit.trim(),
      EmailVerificationDailyLimitPerUser:
        values.EmailVerificationDailyLimitPerUser.trim(),
    }

    const stringKeys = [
      'EmailProvider',
      'SMTPServer',
      'SMTPPort',
      'SMTPAccount',
      'SMTPFrom',
      'SMTPToken',
      'CFEmailAccountID',
      'CFEmailFrom',
      'EmailDailyLimit',
      'EmailVerificationDailyLimitPerUser',
    ] as const

    const boolKeys = ['SMTPSSLEnabled', 'SMTPForceAuthLogin'] as const

    const updates: Array<{ key: string; value: string | boolean }> = []

    for (const key of stringKeys) {
      const newVal = sanitized[key]
      const oldVal = String(defaultValues[key] ?? '')
      if (newVal !== oldVal) {
        updates.push({ key, value: newVal })
      }
    }

    if (sanitized.CFEmailAPIToken) {
      updates.push({ key: 'CFEmailAPIToken', value: sanitized.CFEmailAPIToken })
    }

    for (const key of boolKeys) {
      if (sanitized[key] !== defaultValues[key]) {
        updates.push({ key, value: sanitized[key] })
      }
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  return (
    <SettingsSection title={t('Email Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save SMTP settings'
          />
          <FormField
            control={form.control}
            name='EmailProvider'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Email Provider')}</FormLabel>
                <Select onValueChange={field.onChange} value={field.value}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder={t('Select provider')} />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='smtp'>{t('SMTP')}</SelectItem>
                    <SelectItem value='cloudflare'>
                      {t('Cloudflare Email Service')}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t('Choose which email service to use for sending')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='SMTPServer'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('SMTP Host')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder={t('smtp.example.com')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Hostname or IP of your SMTP provider')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='SMTPPort'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Port')}</FormLabel>
                  <FormControl>
                    <Input
                      autoComplete='off'
                      type='number'
                      placeholder='587'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Common ports include 25, 465, and 587')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormItem>
              <FormLabel>{t('SMTP encryption')}</FormLabel>
              <FormControl>
                <RadioGroup
                  value={getSmtpSecurityMode({
                    SMTPSSLEnabled: form.watch('SMTPSSLEnabled'),
                    SMTPStartTLSEnabled: form.watch('SMTPStartTLSEnabled'),
                  })}
                  onValueChange={(value) => {
                    const mode = value as SmtpSecurityMode
                    form.setValue('SMTPSSLEnabled', mode === 'ssl_tls', {
                      shouldDirty: true,
                    })
                    form.setValue('SMTPStartTLSEnabled', mode === 'starttls', {
                      shouldDirty: true,
                    })
                  }}
                  className='gap-3'
                >
                  <div className='flex items-center gap-2'>
                    <RadioGroupItem value='none' id='smtp-security-none' />
                    <Label
                      htmlFor='smtp-security-none'
                      className='cursor-pointer font-normal'
                    >
                      {t('No encryption')}
                    </Label>
                  </div>
                  <div className='flex items-center gap-2'>
                    <RadioGroupItem
                      value='ssl_tls'
                      id='smtp-security-ssl-tls'
                    />
                    <Label
                      htmlFor='smtp-security-ssl-tls'
                      className='cursor-pointer font-normal'
                    >
                      {t('SSL/TLS')}
                    </Label>
                  </div>
                  <div className='flex items-center gap-2'>
                    <RadioGroupItem
                      value='starttls'
                      id='smtp-security-starttls'
                    />
                    <Label
                      htmlFor='smtp-security-starttls'
                      className='cursor-pointer font-normal'
                    >
                      {t('STARTTLS')}
                    </Label>
                  </div>
                </RadioGroup>
              </FormControl>
              <FormDescription>
                {t('Choose one SMTP transport security mode')}
              </FormDescription>
            </FormItem>

            <FormField
              control={form.control}
              name='SMTPInsecureSkipVerify'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('Skip SMTP TLS certificate verification')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Allow self-signed or hostname-mismatched SMTP certificates'
                      )}
                    </FormDescription>
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

            <FormField
              control={form.control}
              name='SMTPForceAuthLogin'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Force AUTH LOGIN')}</FormLabel>
                    <FormDescription>
                      {t('Force SMTP authentication using AUTH LOGIN method')}
                    </FormDescription>
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
          </div>

          <FormField
            control={form.control}
            name='SMTPAccount'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Username')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder={t('noreply@example.com')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Account used when authenticating with the SMTP server')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='SMTPFrom'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('From Address')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder={t('New API <noreply@example.com>')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Display name and email used in outgoing messages')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='SMTPToken'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Password / Access Token')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    type='password'
                    placeholder={t('Enter new token to update')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Leave blank to keep the existing credential')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <hr className='my-8' />

          <h3 className='text-lg font-medium'>
            {t('Cloudflare Email Service')}
          </h3>

          <FormField
            control={form.control}
            name='CFEmailAccountID'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Account ID')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder={t('Cloudflare Account ID')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='CFEmailAPIToken'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('API Token')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    type='password'
                    placeholder={t('Enter new API Token to update')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Account-level token with Email Sending:Edit permission. Leave blank to keep existing.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='CFEmailFrom'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('From Address')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder={t('noreply@mail.example.com')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Verified sending domain email address')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <hr className='my-8' />

          <h3 className='text-lg font-medium'>{t('Sending Limits')}</h3>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='EmailDailyLimit'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Daily Email Limit')}</FormLabel>
                  <FormControl>
                    <Input
                      autoComplete='off'
                      type='number'
                      placeholder='0'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Max emails sent per day site-wide. 0 means unlimited.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='EmailVerificationDailyLimitPerUser'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Daily Verification Limit Per Email')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      autoComplete='off'
                      type='number'
                      placeholder='5'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Max verification code emails per address per day. 0 means unlimited.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
