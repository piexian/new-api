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

import { Check, Circle, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  getPasswordChecks,
  isValidLoginPassword,
  PASSWORD_POLICY_MESSAGE,
} from '@/lib/password'
import { useAuthStore } from '@/stores/auth-store'

import { logout as logoutApi } from '../../../auth/api'
import { updateSelfPassword } from '../../api'

// ============================================================================
// Change Password Dialog Component
// ============================================================================

interface ChangePasswordDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  username: string
  hasPassword: boolean
  onUpdated?: () => void
}

export function ChangePasswordDialog({
  open,
  onOpenChange,
  username,
  hasPassword,
  onUpdated,
}: ChangePasswordDialogProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [formData, setFormData] = useState({
    originalPassword: '',
    newPassword: '',
    confirmPassword: '',
  })

  // 未设置过密码（第三方注册等）时进入强制模式：不可关闭，必须完成设置
  const required = !hasPassword

  const handleChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }))
  }

  const checks = getPasswordChecks(formData.newPassword)
  const checkLabels: Record<string, string> = {
    length: t('At least 8 characters'),
    bytes: t('At most 72 UTF-8 bytes'),
    digit: t('Contains a digit (0-9)'),
    upper: t('Contains an uppercase letter (A-Z)'),
    lower: t('Contains a lowercase letter (a-z)'),
    symbol: t('Contains ASCII punctuation (e.g. !@#$%)'),
  }

  const handleLogout = async () => {
    try {
      await logoutApi()
    } finally {
      useAuthStore.getState().auth.reset()
      window.location.assign('/sign-in')
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (hasPassword && !formData.originalPassword) {
      toast.error(t('Please enter your current password'))
      return
    }

    if (!isValidLoginPassword(formData.newPassword)) {
      toast.error(t(PASSWORD_POLICY_MESSAGE))
      return
    }

    if (formData.newPassword !== formData.confirmPassword) {
      toast.error(t('Passwords do not match'))
      return
    }

    try {
      setLoading(true)
      const response = await updateSelfPassword({
        ...(hasPassword
          ? { original_password: formData.originalPassword }
          : {}),
        password: formData.newPassword,
      })

      if (response.success) {
        toast.success(
          hasPassword
            ? t('Password changed successfully')
            : t('Password set successfully')
        )
        // 立即更新全局用户状态，解除路由守卫
        const { auth } = useAuthStore.getState()
        if (auth.user) {
          auth.setUser({ ...auth.user, has_password: true })
        }
        onOpenChange(false)
        setFormData({
          originalPassword: '',
          newPassword: '',
          confirmPassword: '',
        })
        onUpdated?.()
      } else {
        toast.error(
          response.message ||
            (hasPassword
              ? t('Failed to change password')
              : t('Failed to set password'))
        )
      }
    } catch {
      toast.error(
        hasPassword
          ? t('Failed to change password')
          : t('Failed to set password')
      )
    } finally {
      setLoading(false)
    }
  }

  const formId = 'change-password-form'
  let submitLabel = hasPassword ? t('Change Password') : t('Set Password')
  if (loading) {
    submitLabel = hasPassword ? t('Changing...') : t('Saving...')
  }

  return (
    <Dialog
      open={open || required}
      onOpenChange={(next) => {
        if (required || loading) return
        onOpenChange(next)
      }}
      title={hasPassword ? t('Change Password') : t('Set Login Password')}
      description={
        required ? (
          t(
            'This account was created through third-party sign-in or has no login password yet. For your account security, set a login password before continuing.'
          )
        ) : (
          <>
            {hasPassword
              ? t('Update your password for account:')
              : t('Set a password for account:')}{' '}
            <strong>{username}</strong>
          </>
        )
      }
      contentClassName='sm:max-w-md'
      contentHeight='auto'
      bodyClassName='space-y-4'
      showCloseButton={!required}
      footer={
        <>
          {required ? (
            <Button
              type='button'
              variant='outline'
              onClick={handleLogout}
              disabled={loading}
            >
              {t('Sign Out')}
            </Button>
          ) : (
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              {t('Cancel')}
            </Button>
          )}
          <Button type='submit' form={formId} disabled={loading}>
            {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {submitLabel}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className='space-y-4'>
        {hasPassword && (
          <div className='space-y-2'>
            <Label htmlFor='currentPassword'>{t('Current Password')}</Label>
            <PasswordInput
              id='currentPassword'
              value={formData.originalPassword}
              onChange={(e) => handleChange('originalPassword', e.target.value)}
              disabled={loading}
              required
              autoComplete='current-password'
            />
          </div>
        )}

        <div className='space-y-2'>
          <Label htmlFor='newPassword'>{t('New Password')}</Label>
          <PasswordInput
            id='newPassword'
            value={formData.newPassword}
            onChange={(e) => handleChange('newPassword', e.target.value)}
            disabled={loading}
            required
            autoComplete='new-password'
          />
          <ul className='space-y-1 text-xs' aria-live='polite'>
            {Object.entries(checks).map(([key, passed]) => (
              <li
                key={key}
                className={
                  passed
                    ? 'flex items-center gap-1.5 text-green-600'
                    : 'text-muted-foreground flex items-center gap-1.5'
                }
              >
                {passed ? (
                  <Check className='h-3 w-3' />
                ) : (
                  <Circle className='h-3 w-3' />
                )}
                {checkLabels[key]}
              </li>
            ))}
          </ul>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='confirmPassword'>{t('Confirm New Password')}</Label>
          <PasswordInput
            id='confirmPassword'
            value={formData.confirmPassword}
            onChange={(e) => handleChange('confirmPassword', e.target.value)}
            disabled={loading}
            required
            autoComplete='new-password'
          />
          {formData.confirmPassword &&
            formData.newPassword !== formData.confirmPassword && (
              <p className='text-destructive text-xs'>
                {t('Passwords do not match')}
              </p>
            )}
        </div>
      </form>
    </Dialog>
  )
}
