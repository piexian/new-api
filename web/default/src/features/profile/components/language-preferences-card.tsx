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
import { useEffect, useMemo, useState } from 'react'
import {
  INTERFACE_LANGUAGE_OPTIONS,
  normalizeInterfaceLanguage,
} from '@/i18n/languages'
import { Languages, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { TitledCard } from '@/components/ui/titled-card'
import { updateUserLanguage, updateUserLogLanguage } from '../api'
import { parseUserSettings } from '../lib'
import type { UserProfile } from '../types'

type LanguagePreferencesCardProps = {
  profile: UserProfile | null
  onProfileUpdate: () => void
}

const LOG_LANGUAGE_OPTIONS = [
  { value: 'follow', labelKey: 'Follow admin default' },
  { value: 'zh', label: '简体中文' },
  { value: 'en', label: 'English' },
] as const

function normalizeLogLanguage(value?: string | null) {
  if (!value) return 'follow'
  const normalized = value.trim().replace(/_/g, '-').toLowerCase()
  if (normalized.startsWith('zh')) return 'zh'
  if (normalized.startsWith('en')) return 'en'
  return 'follow'
}

export function LanguagePreferencesCard(props: LanguagePreferencesCardProps) {
  const { t, i18n } = useTranslation()
  const { auth } = useAuthStore()
  const [saving, setSaving] = useState(false)
  const [savingLogLanguage, setSavingLogLanguage] = useState(false)

  const savedSettings = useMemo(
    () => parseUserSettings(props.profile?.setting),
    [props.profile?.setting]
  )

  const savedLanguage = useMemo(() => {
    return normalizeInterfaceLanguage(savedSettings.language || i18n.language)
  }, [savedSettings.language, i18n.language])
  const savedLogLanguage = useMemo(
    () => normalizeLogLanguage(savedSettings.log_language),
    [savedSettings.log_language]
  )

  const [currentLanguage, setCurrentLanguage] = useState(savedLanguage)
  const [currentLogLanguage, setCurrentLogLanguage] =
    useState(savedLogLanguage)

  useEffect(() => {
    setCurrentLanguage(savedLanguage)
  }, [savedLanguage])

  useEffect(() => {
    setCurrentLogLanguage(savedLogLanguage)
  }, [savedLogLanguage])

  const updateAuthSetting = (nextSetting: Record<string, unknown>) => {
    if (!auth.user) return
    const existingSetting =
      typeof auth.user.setting === 'string'
        ? parseUserSettings(auth.user.setting)
        : (auth.user.setting ?? {})
    auth.setUser({
      ...auth.user,
      setting: JSON.stringify({
        ...existingSetting,
        ...nextSetting,
      }),
    })
  }

  const handleLanguageChange = async (language: string | null) => {
    if (!language) return
    const nextLanguage = normalizeInterfaceLanguage(language)
    if (nextLanguage === currentLanguage) return

    const previousLanguage = currentLanguage
    setCurrentLanguage(nextLanguage)
    setSaving(true)
    await i18n.changeLanguage(nextLanguage)

    try {
      const response = await updateUserLanguage(nextLanguage)
      if (!response.success) {
        throw new Error(response.message || t('Failed to update settings'))
      }

      updateAuthSetting({ language: nextLanguage })

      props.onProfileUpdate()
      toast.success(t('Language preference saved'))
    } catch (_error) {
      setCurrentLanguage(previousLanguage)
      await i18n.changeLanguage(previousLanguage)
      toast.error(t('Failed to update settings'))
    } finally {
      setSaving(false)
    }
  }

  const handleLogLanguageChange = async (language: string | null) => {
    if (!language) return
    const nextLogLanguage = normalizeLogLanguage(language)
    if (nextLogLanguage === currentLogLanguage) return

    const previousLogLanguage = currentLogLanguage
    setCurrentLogLanguage(nextLogLanguage)
    setSavingLogLanguage(true)

    try {
      const value = nextLogLanguage === 'follow' ? '' : nextLogLanguage
      const response = await updateUserLogLanguage(value)
      if (!response.success) {
        throw new Error(response.message || t('Failed to update settings'))
      }

      updateAuthSetting({ log_language: value })
      props.onProfileUpdate()
      toast.success(t('Log language preference saved'))
    } catch (_error) {
      setCurrentLogLanguage(previousLogLanguage)
      toast.error(t('Failed to update settings'))
    } finally {
      setSavingLogLanguage(false)
    }
  }

  return (
    <TitledCard
      title={t('Language Preferences')}
      description={t('Set the language used across the interface')}
      icon={<Languages className='h-4 w-4' />}
    >
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4'>
        <div className='space-y-1'>
          <div className='text-sm font-medium'>{t('Interface Language')}</div>
          <p className='text-muted-foreground line-clamp-2 text-xs sm:text-sm'>
            {t(
              'Language preferences sync across your signed-in devices and affect API error messages.'
            )}
          </p>
        </div>
        <div className='flex items-center gap-2 sm:min-w-48'>
          <Select
            items={[
              ...INTERFACE_LANGUAGE_OPTIONS.map((language) => ({
                value: language.code,
                label: language.label,
              })),
            ]}
            value={currentLanguage}
            onValueChange={handleLanguageChange}
            disabled={saving}
          >
            <SelectTrigger className='w-full sm:w-48'>
              <SelectValue placeholder={t('Select language')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {INTERFACE_LANGUAGE_OPTIONS.map((language) => (
                  <SelectItem key={language.code} value={language.code}>
                    {language.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          {saving && (
            <Loader2 className='text-muted-foreground size-4 animate-spin' />
          )}
        </div>
      </div>
      <div className='mt-4 flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between sm:gap-4'>
        <div className='space-y-1'>
          <div className='text-sm font-medium'>{t('Log Display Language')}</div>
          <p className='text-muted-foreground line-clamp-2 text-xs sm:text-sm'>
            {t(
              'Controls how New API log content is displayed. Leave it on default to follow the administrator preference.'
            )}
          </p>
        </div>
        <div className='flex items-center gap-2 sm:min-w-48'>
          <Select
            items={LOG_LANGUAGE_OPTIONS.map((option) => ({
              value: option.value,
              label: 'label' in option ? option.label : t(option.labelKey),
            }))}
            value={currentLogLanguage}
            onValueChange={handleLogLanguageChange}
            disabled={savingLogLanguage}
          >
            <SelectTrigger className='w-full sm:w-48'>
              <SelectValue placeholder={t('Select language')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {LOG_LANGUAGE_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {'label' in option ? option.label : t(option.labelKey)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          {savingLogLanguage && (
            <Loader2 className='text-muted-foreground size-4 animate-spin' />
          )}
        </div>
      </div>
    </TitledCard>
  )
}
