import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { PasswordInput } from '@/components/password-input'
import { Turnstile } from '@/components/turnstile'
import { updateUserProfile } from '../../api'

// ============================================================================
// Change Password Dialog Component
// ============================================================================

interface ChangePasswordDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  username: string
  hasPassword: boolean
  turnstileEnabled: boolean
  turnstileSiteKey: string
  onUpdated?: () => void
}

export function ChangePasswordDialog({
  open,
  onOpenChange,
  username,
  hasPassword,
  turnstileEnabled,
  turnstileSiteKey,
  onUpdated,
}: ChangePasswordDialogProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [turnstileToken, setTurnstileToken] = useState('')
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0)
  const [formData, setFormData] = useState({
    originalPassword: '',
    newPassword: '',
    confirmPassword: '',
  })

  const handleChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    // Validation
    if (hasPassword && !formData.originalPassword) {
      toast.error(t('Please enter your current password'))
      return
    }

    if (!formData.newPassword) {
      toast.error(t('Please enter a new password'))
      return
    }

    if (formData.newPassword.length < 8) {
      toast.error(t('Password must be at least 8 characters'))
      return
    }

    if (hasPassword && formData.originalPassword === formData.newPassword) {
      toast.error(t('New password must be different from current password'))
      return
    }

    if (formData.newPassword !== formData.confirmPassword) {
      toast.error(t('Passwords do not match'))
      return
    }

    if (turnstileEnabled && !turnstileToken) {
      toast.error(t('Please complete Turnstile verification'))
      return
    }

    try {
      setLoading(true)
      const response = await updateUserProfile(
        {
          ...(hasPassword
            ? { original_password: formData.originalPassword }
            : {}),
          password: formData.newPassword,
        },
        turnstileToken
      )

      if (response.success) {
        toast.success(
          hasPassword
            ? t('Password changed successfully')
            : t('Password set successfully')
        )
        onOpenChange(false)
        setFormData({
          originalPassword: '',
          newPassword: '',
          confirmPassword: '',
        })
        setTurnstileToken('')
        setTurnstileWidgetKey((value) => value + 1)
        onUpdated?.()
      } else {
        toast.error(
          response.message ||
            (hasPassword
              ? t('Failed to change password')
              : t('Failed to set password'))
        )
        if (turnstileEnabled) {
          setTurnstileToken('')
          setTurnstileWidgetKey((value) => value + 1)
        }
      }
    } catch (_error) {
      toast.error(
        hasPassword
          ? t('Failed to change password')
          : t('Failed to set password')
      )
      if (turnstileEnabled) {
        setTurnstileToken('')
        setTurnstileWidgetKey((value) => value + 1)
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>
              {hasPassword ? t('Change Password') : t('Set Password')}
            </DialogTitle>
            <DialogDescription>
              {hasPassword
                ? t('Update your password for account:')
                : t('Set a password for account:')}{' '}
              <strong>{username}</strong>
            </DialogDescription>
          </DialogHeader>

          <div className='my-6 space-y-4'>
            {hasPassword && (
              <div className='space-y-2'>
                <Label htmlFor='currentPassword'>{t('Current Password')}</Label>
                <PasswordInput
                  id='currentPassword'
                  value={formData.originalPassword}
                  onChange={(e) =>
                    handleChange('originalPassword', e.target.value)
                  }
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
                minLength={8}
                autoComplete='new-password'
              />
              <p className='text-muted-foreground text-xs'>
                {t('Must be at least 8 characters')}
              </p>
            </div>

            <div className='space-y-2'>
              <Label htmlFor='confirmPassword'>
                {t('Confirm New Password')}
              </Label>
              <PasswordInput
                id='confirmPassword'
                value={formData.confirmPassword}
                onChange={(e) =>
                  handleChange('confirmPassword', e.target.value)
                }
                disabled={loading}
                required
                autoComplete='new-password'
              />
            </div>

            {turnstileEnabled && (
              <div className='flex justify-center pt-2'>
                <Turnstile
                  key={turnstileWidgetKey}
                  siteKey={turnstileSiteKey}
                  onVerify={setTurnstileToken}
                  onExpire={() => setTurnstileToken('')}
                />
              </div>
            )}
          </div>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              {t('Cancel')}
            </Button>
            <Button type='submit' disabled={loading}>
              {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {loading
                ? hasPassword
                  ? t('Changing...')
                  : t('Saving...')
                : hasPassword
                  ? t('Change Password')
                  : t('Set Password')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
