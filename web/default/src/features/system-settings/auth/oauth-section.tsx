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
import axios from 'axios'
import { ExternalLink } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  buildOAuthCallbackUrl,
  resolveOAuthSiteUrl,
} from './oauth-callback-url'

/**
 * react-hook-form 7 treats dotted `name` strings as nested paths. To keep
 * form state, schema validation, and dirty tracking aligned, the
 * `discord.*` and `oidc.*` fields are modeled as nested objects here and
 * flattened back to dotted server keys only when persisting.
 */
const oauthSchema = z.object({
  OAuthRegisterEnabled: z.boolean(),
  GitHubOAuthEnabled: z.boolean(),
  GitHubClientId: z.string().optional(),
  GitHubClientSecret: z.string().optional(),
  GitHubMinimumAccountAge: z.string().optional(),
  GitHubMinimumAccountAgeUnit: z.enum(['day', 'month', 'year']).optional(),
  SteamOAuthEnabled: z.boolean(),
  SteamWebAPIKey: z.string().optional(),
  discord: z.object({
    enabled: z.boolean(),
    client_id: z.string(),
    client_secret: z.string(),
  }),
  oidc: z.object({
    enabled: z.boolean(),
    client_id: z.string(),
    client_secret: z.string(),
    well_known: z.string(),
    authorization_endpoint: z.string(),
    token_endpoint: z.string(),
    user_info_endpoint: z.string(),
  }),
  TelegramOAuthEnabled: z.boolean(),
  TelegramBotToken: z.string(),
  TelegramBotName: z.string(),
  LinuxDOOAuthEnabled: z.boolean(),
  LinuxDOClientId: z.string().optional(),
  LinuxDOClientSecret: z.string().optional(),
  LinuxDOMinimumTrustLevel: z.string().optional(),
  QQOAuthEnabled: z.boolean(),
  QQClientId: z.string().optional(),
  QQClientSecret: z.string().optional(),
  WeChatAuthEnabled: z.boolean(),
  WeChatServerAddress: z.string(),
  WeChatServerToken: z.string(),
  WeChatAccountQRCodeImageURL: z.string(),
})

type OAuthFormValues = z.infer<typeof oauthSchema>

type FlatOAuthDefaults = {
  OAuthRegisterEnabled: boolean
  GitHubOAuthEnabled: boolean
  GitHubClientId: string
  GitHubClientSecret: string
  GitHubMinimumAccountAge: string
  GitHubMinimumAccountAgeUnit: 'day' | 'month' | 'year'
  SteamOAuthEnabled: boolean
  SteamWebAPIKey: string
  'discord.enabled': boolean
  'discord.client_id': string
  'discord.client_secret': string
  'oidc.enabled': boolean
  'oidc.client_id': string
  'oidc.client_secret': string
  'oidc.well_known': string
  'oidc.authorization_endpoint': string
  'oidc.token_endpoint': string
  'oidc.user_info_endpoint': string
  TelegramOAuthEnabled: boolean
  TelegramBotToken: string
  TelegramBotName: string
  LinuxDOOAuthEnabled: boolean
  LinuxDOClientId: string
  LinuxDOClientSecret: string
  LinuxDOMinimumTrustLevel: string
  QQOAuthEnabled: boolean
  QQClientId: string
  QQClientSecret: string
  WeChatAuthEnabled: boolean
  WeChatServerAddress: string
  WeChatServerToken: string
  WeChatAccountQRCodeImageURL: string
}

const oauthTabContentClassName =
  'grid min-w-0 gap-x-5 gap-y-6 lg:grid-cols-2 [&>[data-slot=form-item]]:min-w-0 lg:[&>[data-slot=form-item]:has([data-slot=switch])]:col-span-2'

type OAuthSetupGuideRow = {
  label: ReactNode
  value: string
  copyLabel: string
}

type OAuthSetupGuideProps = {
  title: string
  description: ReactNode
  rows: OAuthSetupGuideRow[]
  children?: ReactNode
}

function OAuthSetupGuide(props: OAuthSetupGuideProps) {
  return (
    <Alert className='lg:col-span-2'>
      <AlertTitle>{props.title}</AlertTitle>
      <AlertDescription className='space-y-3 text-sm'>
        <div>{props.description}</div>
        <div className='space-y-2'>
          {props.rows.map((row) => (
            <div
              key={`${String(row.label)}-${row.value}`}
              className='flex min-w-0 flex-col gap-1.5 sm:flex-row sm:items-center sm:justify-between'
            >
              <span className='text-muted-foreground shrink-0'>
                {row.label}
              </span>
              <span className='flex min-w-0 items-center gap-2'>
                <code className='bg-muted text-foreground min-w-0 rounded px-1.5 py-0.5 text-xs break-all'>
                  {row.value}
                </code>
                <CopyButton
                  value={row.value}
                  size='icon'
                  className='size-7'
                  tooltip={row.copyLabel}
                  aria-label={row.copyLabel}
                />
              </span>
            </div>
          ))}
        </div>
        {props.children}
      </AlertDescription>
    </Alert>
  )
}

const buildFormDefaults = (defaults: FlatOAuthDefaults): OAuthFormValues => ({
  OAuthRegisterEnabled: defaults.OAuthRegisterEnabled ?? true,
  GitHubOAuthEnabled: defaults.GitHubOAuthEnabled,
  GitHubClientId: defaults.GitHubClientId ?? '',
  GitHubClientSecret: defaults.GitHubClientSecret ?? '',
  GitHubMinimumAccountAge: defaults.GitHubMinimumAccountAge ?? '0',
  GitHubMinimumAccountAgeUnit: defaults.GitHubMinimumAccountAgeUnit ?? 'day',
  SteamOAuthEnabled: defaults.SteamOAuthEnabled,
  SteamWebAPIKey: defaults.SteamWebAPIKey ?? '',
  discord: {
    enabled: defaults['discord.enabled'],
    client_id: defaults['discord.client_id'] ?? '',
    client_secret: defaults['discord.client_secret'] ?? '',
  },
  oidc: {
    enabled: defaults['oidc.enabled'],
    client_id: defaults['oidc.client_id'] ?? '',
    client_secret: defaults['oidc.client_secret'] ?? '',
    well_known: defaults['oidc.well_known'] ?? '',
    authorization_endpoint: defaults['oidc.authorization_endpoint'] ?? '',
    token_endpoint: defaults['oidc.token_endpoint'] ?? '',
    user_info_endpoint: defaults['oidc.user_info_endpoint'] ?? '',
  },
  TelegramOAuthEnabled: defaults.TelegramOAuthEnabled,
  TelegramBotToken: defaults.TelegramBotToken ?? '',
  TelegramBotName: defaults.TelegramBotName ?? '',
  LinuxDOOAuthEnabled: defaults.LinuxDOOAuthEnabled,
  LinuxDOClientId: defaults.LinuxDOClientId ?? '',
  LinuxDOClientSecret: defaults.LinuxDOClientSecret ?? '',
  LinuxDOMinimumTrustLevel: defaults.LinuxDOMinimumTrustLevel ?? '',
  QQOAuthEnabled: defaults.QQOAuthEnabled,
  QQClientId: defaults.QQClientId ?? '',
  QQClientSecret: defaults.QQClientSecret ?? '',
  WeChatAuthEnabled: defaults.WeChatAuthEnabled,
  WeChatServerAddress: defaults.WeChatServerAddress ?? '',
  WeChatServerToken: defaults.WeChatServerToken ?? '',
  WeChatAccountQRCodeImageURL: defaults.WeChatAccountQRCodeImageURL ?? '',
})

const normalizeFormValues = (values: OAuthFormValues): FlatOAuthDefaults => ({
  OAuthRegisterEnabled: values.OAuthRegisterEnabled,
  GitHubOAuthEnabled: values.GitHubOAuthEnabled,
  GitHubClientId: values.GitHubClientId ?? '',
  GitHubClientSecret: values.GitHubClientSecret ?? '',
  GitHubMinimumAccountAge: values.GitHubMinimumAccountAge ?? '0',
  GitHubMinimumAccountAgeUnit: values.GitHubMinimumAccountAgeUnit ?? 'day',
  SteamOAuthEnabled: values.SteamOAuthEnabled,
  SteamWebAPIKey: values.SteamWebAPIKey ?? '',
  'discord.enabled': values.discord.enabled,
  'discord.client_id': values.discord.client_id,
  'discord.client_secret': values.discord.client_secret,
  'oidc.enabled': values.oidc.enabled,
  'oidc.client_id': values.oidc.client_id,
  'oidc.client_secret': values.oidc.client_secret,
  'oidc.well_known': values.oidc.well_known,
  'oidc.authorization_endpoint': values.oidc.authorization_endpoint,
  'oidc.token_endpoint': values.oidc.token_endpoint,
  'oidc.user_info_endpoint': values.oidc.user_info_endpoint,
  TelegramOAuthEnabled: values.TelegramOAuthEnabled,
  TelegramBotToken: values.TelegramBotToken ?? '',
  TelegramBotName: values.TelegramBotName ?? '',
  LinuxDOOAuthEnabled: values.LinuxDOOAuthEnabled,
  LinuxDOClientId: values.LinuxDOClientId ?? '',
  LinuxDOClientSecret: values.LinuxDOClientSecret ?? '',
  LinuxDOMinimumTrustLevel: values.LinuxDOMinimumTrustLevel ?? '',
  QQOAuthEnabled: values.QQOAuthEnabled,
  QQClientId: values.QQClientId ?? '',
  QQClientSecret: values.QQClientSecret ?? '',
  WeChatAuthEnabled: values.WeChatAuthEnabled,
  WeChatServerAddress: values.WeChatServerAddress,
  WeChatServerToken: values.WeChatServerToken,
  WeChatAccountQRCodeImageURL: values.WeChatAccountQRCodeImageURL,
})

type OAuthSectionProps = {
  defaultValues: FlatOAuthDefaults
  serverAddress: string
}

export function OAuthSection(props: OAuthSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [activeTab, setActiveTab] = useState('github')
  const siteUrl = resolveOAuthSiteUrl(props.serverAddress, t('Site URL'))
  const githubCallbackUrl = buildOAuthCallbackUrl(
    props.serverAddress,
    'github',
    t('Site URL')
  )
  const discordCallbackUrl = buildOAuthCallbackUrl(
    props.serverAddress,
    'discord',
    t('Site URL')
  )
  const oidcCallbackUrl = buildOAuthCallbackUrl(
    props.serverAddress,
    'oidc',
    t('Site URL')
  )
  const linuxDOCallbackUrl = buildOAuthCallbackUrl(
    props.serverAddress,
    'linuxdo',
    t('Site URL')
  )

  const formDefaults = buildFormDefaults(props.defaultValues)

  const form = useForm<OAuthFormValues>({
    resolver: zodResolver(oauthSchema),
    defaultValues: formDefaults,
  })

  const baselineRef = useRef<FlatOAuthDefaults>(props.defaultValues)
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(props.defaultValues)
  )

  useEffect(() => {
    const serialized = JSON.stringify(props.defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = props.defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildFormDefaults(props.defaultValues))
  }, [props.defaultValues, form])

  const onSubmit = async (values: OAuthFormValues) => {
    let finalValues = values

    if (values.oidc.well_known && values.oidc.well_known.trim() !== '') {
      const wellKnown = values.oidc.well_known.trim()
      if (
        !wellKnown.startsWith('http://') &&
        !wellKnown.startsWith('https://')
      ) {
        toast.error(t('Well-Known URL must start with http:// or https://'))
        return
      }

      try {
        const res = await axios.create().get(wellKnown)
        const authEndpoint = res.data['authorization_endpoint'] || ''
        const tokenEndpoint = res.data['token_endpoint'] || ''
        const userInfoEndpoint = res.data['userinfo_endpoint'] || ''

        finalValues = {
          ...values,
          oidc: {
            ...values.oidc,
            authorization_endpoint: authEndpoint,
            token_endpoint: tokenEndpoint,
            user_info_endpoint: userInfoEndpoint,
          },
        }

        form.setValue('oidc.authorization_endpoint', authEndpoint)
        form.setValue('oidc.token_endpoint', tokenEndpoint)
        form.setValue('oidc.user_info_endpoint', userInfoEndpoint)

        toast.success(t('OIDC configuration fetched successfully'))
      } catch (err) {
        // eslint-disable-next-line no-console
        console.error(err)
        toast.error(
          t(
            'Failed to fetch OIDC configuration. Please check the URL and network status'
          )
        )
        return
      }
    }

    const normalized = normalizeFormValues(finalValues)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatOAuthDefaults>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of changedKeys) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }

    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
    form.reset(buildFormDefaults(normalized))
  }

  const handleReset = () => {
    form.reset(buildFormDefaults(baselineRef.current))
    toast.success(t('Form reset to saved values'))
  }

  return (
    <>
      <FormNavigationGuard when={form.formState.isDirty} />

      <SettingsSection title={t('OAuth Integrations')}>
        <Form {...form}>
          <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
            <SettingsPageFormActions
              onSave={form.handleSubmit(onSubmit)}
              onReset={handleReset}
              isSaving={updateOption.isPending}
              isResetDisabled={!form.formState.isDirty}
            />
            <FormDirtyIndicator isDirty={form.formState.isDirty} />

            <FormField
              control={form.control}
              name='OAuthRegisterEnabled'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>
                      {t('OAuth Registration')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Allow new users to register with third-party OAuth accounts'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            <Tabs value={activeTab} onValueChange={setActiveTab}>
              <TabsList className='grid w-full grid-cols-8'>
                <TabsTrigger value='github'>{t('GitHub')}</TabsTrigger>
                <TabsTrigger value='discord'>{t('Discord')}</TabsTrigger>
                <TabsTrigger value='oidc'>{t('OIDC')}</TabsTrigger>
                <TabsTrigger value='telegram'>{t('Telegram')}</TabsTrigger>
                <TabsTrigger value='linuxdo'>{t('LinuxDO')}</TabsTrigger>
                <TabsTrigger value='qq'>{t('QQ')}</TabsTrigger>
                <TabsTrigger value='wechat'>{t('WeChat')}</TabsTrigger>
                <TabsTrigger value='steam'>{t('Steam')}</TabsTrigger>
              </TabsList>

              <TabsContent value='github' className={oauthTabContentClassName}>
                <OAuthSetupGuide
                  title={t('Setup guide')}
                  description={t(
                    'Set these values in the provider application before enabling login.'
                  )}
                  rows={[
                    {
                      label: t('Homepage URL'),
                      value: siteUrl,
                      copyLabel: t('Copy homepage URL'),
                    },
                    {
                      label: t('Authorization callback URL'),
                      value: githubCallbackUrl,
                      copyLabel: t('Copy callback URL'),
                    },
                  ]}
                />

                <FormField
                  control={form.control}
                  name='GitHubOAuthEnabled'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Enable GitHub OAuth')}</FormLabel>
                        <FormDescription>
                          {t('Allow users to sign in with GitHub')}
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
                  name='GitHubClientId'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Client ID')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('Your GitHub OAuth Client ID')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='GitHubClientSecret'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Client Secret')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Your GitHub OAuth Client Secret')}
                          autoComplete='new-password'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <div className='grid gap-4 md:grid-cols-[minmax(0,1fr)_180px]'>
                  <FormField
                    control={form.control}
                    name='GitHubMinimumAccountAge'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Minimum GitHub account age')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            step={1}
                            placeholder='0'
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Require new GitHub OAuth registrations to use accounts at least this old. Set 0 to disable.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='GitHubMinimumAccountAgeUnit'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Age unit')}</FormLabel>
                        <FormControl>
                          <Select
                            value={field.value ?? 'day'}
                            onValueChange={field.onChange}
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='day'>{t('Day')}</SelectItem>
                              <SelectItem value='month'>
                                {t('Month')}
                              </SelectItem>
                              <SelectItem value='year'>{t('Year')}</SelectItem>
                            </SelectContent>
                          </Select>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </TabsContent>

              <TabsContent value='steam' className='space-y-4'>
                <FormField
                  control={form.control}
                  name='SteamOAuthEnabled'
                  render={({ field }) => (
                    <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-base'>
                          {t('Enable Steam Login')}
                        </FormLabel>
                        <FormDescription>
                          {t('Allow users to sign in with Steam')}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='SteamWebAPIKey'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Steam Web API Key')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Your Steam Web API Key')}
                          autoComplete='new-password'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>

              <TabsContent value='discord' className={oauthTabContentClassName}>
                <OAuthSetupGuide
                  title={t('Setup guide')}
                  description={t(
                    'Set these values in the provider application before enabling login.'
                  )}
                  rows={[
                    {
                      label: t('Homepage URL'),
                      value: siteUrl,
                      copyLabel: t('Copy homepage URL'),
                    },
                    {
                      label: t('Authorization callback URL'),
                      value: discordCallbackUrl,
                      copyLabel: t('Copy callback URL'),
                    },
                  ]}
                />

                <FormField
                  control={form.control}
                  name='discord.enabled'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Enable Discord OAuth')}</FormLabel>
                        <FormDescription>
                          {t('Allow users to sign in with Discord')}
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
                  name='discord.client_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Client ID')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('Your Discord OAuth Client ID')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='discord.client_secret'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Client Secret')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Your Discord OAuth Client Secret')}
                          autoComplete='new-password'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>

              <TabsContent value='oidc' className={oauthTabContentClassName}>
                <OAuthSetupGuide
                  title={t('Setup guide')}
                  description={
                    <div className='space-y-1'>
                      <p>
                        {t(
                          'Set these values in the provider application before enabling login.'
                        )}
                      </p>
                      <p>
                        {t(
                          'OIDC discovery can fill the endpoint fields automatically when the provider supports it.'
                        )}
                      </p>
                    </div>
                  }
                  rows={[
                    {
                      label: t('Homepage URL'),
                      value: siteUrl,
                      copyLabel: t('Copy homepage URL'),
                    },
                    {
                      label: t('Redirect URL'),
                      value: oidcCallbackUrl,
                      copyLabel: t('Copy redirect URL'),
                    },
                  ]}
                />

                <FormField
                  control={form.control}
                  name='oidc.enabled'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Enable OIDC')}</FormLabel>
                        <FormDescription>
                          {t('Allow users to sign in with OpenID Connect')}
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
                  name='oidc.client_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Client ID')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('OIDC Client ID')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oidc.client_secret'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Client Secret')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('OIDC Client Secret')}
                          autoComplete='new-password'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oidc.well_known'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Well-Known URL')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t(
                            'https://provider.com/.well-known/openid-configuration'
                          )}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Auto-discovers endpoints from the provider')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oidc.authorization_endpoint'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('Authorization Endpoint (Optional)')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('Override auto-discovered endpoint')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oidc.token_endpoint'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Token Endpoint (Optional)')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('Override auto-discovered endpoint')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='oidc.user_info_endpoint'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('User Info Endpoint (Optional)')}
                      </FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('Override auto-discovered endpoint')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>

              <TabsContent
                value='telegram'
                className={oauthTabContentClassName}
              >
                <FormField
                  control={form.control}
                  name='TelegramOAuthEnabled'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Enable Telegram OAuth')}</FormLabel>
                        <FormDescription>
                          {t('Allow users to sign in with Telegram')}
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
                  name='TelegramBotToken'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Bot Token')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Your Telegram Bot Token')}
                          autoComplete='new-password'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='TelegramBotName'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Bot Name')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('Your Bot Name')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>

              <TabsContent value='linuxdo' className={oauthTabContentClassName}>
                <OAuthSetupGuide
                  title={t('Setup guide')}
                  description={t(
                    'Set these values in the provider application before enabling login.'
                  )}
                  rows={[
                    {
                      label: t('Authorization callback URL'),
                      value: linuxDOCallbackUrl,
                      copyLabel: t('Copy callback URL'),
                    },
                  ]}
                >
                  <a
                    href='https://connect.linux.do/'
                    target='_blank'
                    rel='noreferrer'
                    className='text-primary inline-flex w-fit items-center gap-1 underline underline-offset-3 hover:no-underline'
                  >
                    {t('Manage your LinuxDO OAuth app')}
                    <ExternalLink className='size-3' aria-hidden='true' />
                  </a>
                </OAuthSetupGuide>

                <FormField
                  control={form.control}
                  name='LinuxDOOAuthEnabled'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Enable LinuxDO OAuth')}</FormLabel>
                        <FormDescription>
                          {t('Allow users to sign in with LinuxDO')}
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
                  name='LinuxDOClientId'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Client ID')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('LinuxDO Client ID')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='LinuxDOClientSecret'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Client Secret')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('LinuxDO Client Secret')}
                          autoComplete='new-password'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='LinuxDOMinimumTrustLevel'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Minimum Trust Level')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder='0'
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Minimum LinuxDO trust level required')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>

              <TabsContent value='qq' className='space-y-4'>
                <FormField
                  control={form.control}
                  name='QQOAuthEnabled'
                  render={({ field }) => (
                    <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                      <div className='space-y-0.5'>
                        <FormLabel className='text-base'>
                          {t('Enable QQ OAuth')}
                        </FormLabel>
                        <FormDescription>
                          {t('Allow users to sign in with QQ')}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='QQClientId'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('App ID')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('QQ Connect App ID')}
                          autoComplete='off'
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Callback URL: {{url}}', {
                          url: `${window.location.origin}/oauth/qq`,
                        })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='QQClientSecret'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('App Key')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('QQ Connect App Key')}
                          autoComplete='new-password'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>

              <TabsContent value='wechat' className='space-y-4'>
                <FormField
                  control={form.control}
                  name='WeChatAuthEnabled'
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Enable WeChat Auth')}</FormLabel>
                        <FormDescription>
                          {t('Allow users to sign in with WeChat')}
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
                  name='WeChatServerAddress'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Server Address')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('https://wechat-server.example.com')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='WeChatServerToken'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Server Token')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Server Token')}
                          autoComplete='new-password'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='WeChatAccountQRCodeImageURL'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('QR Code Image URL')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('https://example.com/qr-code.png')}
                          autoComplete='off'
                          value={field.value ?? ''}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>
            </Tabs>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
