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
import { MailSend01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
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
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import { sendTestEmail } from '../api'
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
  z
    .object({
      EmailProvider: z.string(),
      SMTPServer: z.string(),
      SMTPPort: z.string(),
      SMTPAccount: z.string(),
      SMTPFrom: z.string(),
      SMTPToken: z.string(),
      SMTPSSLEnabled: z.boolean(),
      SMTPStartTLSEnabled: z.boolean(),
      SMTPInsecureSkipVerify: z.boolean(),
      SMTPForceAuthLogin: z.boolean(),
      CFEmailAccountID: z.string(),
      CFEmailAPIToken: z.string(),
      CFEmailFrom: z.string(),
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
      BalanceLowNotifyEnabled: z.boolean(),
      QuotaRemindThreshold: z.string().refine((value) => {
        const trimmed = value.trim()
        return Boolean(trimmed) && /^\d+$/.test(trimmed)
      }, t('Must be a non-negative integer')),
    })
    .superRefine((values, context) => {
      if (values.EmailProvider === 'smtp') {
        const port = values.SMTPPort.trim()
        if (port && !/^\d+$/.test(port)) {
          context.addIssue({
            code: 'custom',
            path: ['SMTPPort'],
            message: t('Port must be a positive integer'),
          })
        }
        const from = values.SMTPFrom.trim()
        if (from && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(from)) {
          context.addIssue({
            code: 'custom',
            path: ['SMTPFrom'],
            message: t('Enter a valid email or leave blank'),
          })
        }
      }

      if (values.EmailProvider === 'cloudflare') {
        const from = values.CFEmailFrom.trim()
        if (from && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(from)) {
          context.addIssue({
            code: 'custom',
            path: ['CFEmailFrom'],
            message: t('Enter a valid email or leave blank'),
          })
        }
      }
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
  const [testReceiver, setTestReceiver] = useState('')

  const form = useForm<EmailFormValues>({
    resolver: zodResolver(emailSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const emailProvider = useWatch({
    control: form.control,
    name: 'EmailProvider',
  })
  const smtpSSLEnabled = useWatch({
    control: form.control,
    name: 'SMTPSSLEnabled',
  })
  const smtpStartTLSEnabled = useWatch({
    control: form.control,
    name: 'SMTPStartTLSEnabled',
  })
  const balanceLowNotifyEnabled = useWatch({
    control: form.control,
    name: 'BalanceLowNotifyEnabled',
  })

  const testEmailMutation = useMutation({
    mutationFn: sendTestEmail,
    onSuccess: (data) => {
      if (data.success) {
        toast.success(t('Test email sent successfully'))
        return
      }
      toast.error(data.message || t('Failed to send test email'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to send test email'))
    },
  })

  const handleSendTestEmail = () => {
    const receiver = testReceiver.trim()
    if (!z.string().email().safeParse(receiver).success) {
      toast.error(t('Please enter a valid email address'))
      return
    }
    testEmailMutation.mutate({ receiver })
  }

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
      BalanceLowNotifyEnabled: values.BalanceLowNotifyEnabled,
      QuotaRemindThreshold: values.QuotaRemindThreshold.trim(),
    }

    const sharedStringKeys = [
      'EmailProvider',
      'EmailDailyLimit',
      'EmailVerificationDailyLimitPerUser',
      'QuotaRemindThreshold',
    ] as const

    const providerStringKeys =
      sanitized.EmailProvider === 'cloudflare'
        ? (['CFEmailAccountID', 'CFEmailFrom'] as const)
        : ([
            'SMTPServer',
            'SMTPPort',
            'SMTPAccount',
            'SMTPFrom',
            'SMTPToken',
          ] as const)
    const stringKeys = [...sharedStringKeys, ...providerStringKeys]

    const providerBoolKeys =
      sanitized.EmailProvider === 'smtp'
        ? ([
            'SMTPSSLEnabled',
            'SMTPStartTLSEnabled',
            'SMTPInsecureSkipVerify',
            'SMTPForceAuthLogin',
          ] as const)
        : []
    const boolKeys = ['BalanceLowNotifyEnabled', ...providerBoolKeys] as const

    const updates: Array<{ key: string; value: string | boolean }> = []

    for (const key of stringKeys) {
      const newVal = sanitized[key]
      const oldVal = String(defaultValues[key] ?? '')
      if (newVal !== oldVal) {
        updates.push({ key, value: newVal })
      }
    }

    if (sanitized.EmailProvider === 'cloudflare' && sanitized.CFEmailAPIToken) {
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
            saveLabel='Save email settings'
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

          {emailProvider === 'smtp' ? (
            <>
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
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
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
                        SMTPSSLEnabled: smtpSSLEnabled,
                        SMTPStartTLSEnabled: smtpStartTLSEnabled,
                      })}
                      onValueChange={(value) => {
                        const mode = value as SmtpSecurityMode
                        form.setValue('SMTPSSLEnabled', mode === 'ssl_tls', {
                          shouldDirty: true,
                        })
                        form.setValue(
                          'SMTPStartTLSEnabled',
                          mode === 'starttls',
                          {
                            shouldDirty: true,
                          }
                        )
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
                          {t(
                            'Force SMTP authentication using AUTH LOGIN method'
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
                      {t(
                        'Account used when authenticating with the SMTP server'
                      )}
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
            </>
          ) : null}

          {emailProvider === 'cloudflare' ? (
            <>
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
            </>
          ) : null}

          <Separator />

          <FormItem>
            <FormLabel>{t('Test email delivery')}</FormLabel>
            <FormControl>
              <InputGroup>
                <InputGroupInput
                  autoComplete='email'
                  type='email'
                  value={testReceiver}
                  onChange={(event) => setTestReceiver(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      event.preventDefault()
                      handleSendTestEmail()
                    }
                  }}
                  placeholder={t('Enter the test recipient email')}
                  disabled={testEmailMutation.isPending}
                />
                <InputGroupAddon align='inline-end'>
                  <InputGroupButton
                    size='sm'
                    onClick={handleSendTestEmail}
                    disabled={
                      testEmailMutation.isPending || updateOption.isPending
                    }
                  >
                    {testEmailMutation.isPending ? (
                      <Spinner data-icon='inline-start' />
                    ) : (
                      <HugeiconsIcon
                        icon={MailSend01Icon}
                        strokeWidth={2}
                        data-icon='inline-start'
                      />
                    )}
                    {testEmailMutation.isPending
                      ? t('Sending...')
                      : t('Send test email')}
                  </InputGroupButton>
                </InputGroupAddon>
              </InputGroup>
            </FormControl>
            <FormDescription>
              {t(
                'Uses the currently saved email provider settings. Save changes before testing.'
              )}
            </FormDescription>
          </FormItem>

          <Separator />

          <div>
            <h3 className='text-base font-medium'>
              {t('Low balance reminder')}
            </h3>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Send an email when a user balance falls below the threshold')}
            </p>
          </div>

          <FormField
            control={form.control}
            name='BalanceLowNotifyEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable low balance reminder')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Apply the default threshold to users without an override'
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

          <div className='grid gap-4'>
            <FormField
              control={form.control}
              name='QuotaRemindThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default reminder threshold')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      disabled={!balanceLowNotifyEnabled}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Used when a user has not set a custom threshold')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <Alert>
              <AlertDescription>
                {t(
                  'Recharge buttons use the server address from General Settings and are omitted when it is empty.'
                )}
              </AlertDescription>
            </Alert>
          </div>

          <Separator />

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
