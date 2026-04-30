import { useEffect, useMemo, useState } from 'react'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Turnstile } from '@/components/turnstile'
import { updateUserProfile } from '../../api'
import type { UserProfile } from '../../types'

const DEFAULT_USERNAME_CHANGE_LIMIT = 3
const USERNAME_MAX_LENGTH = 20

function formatResetTime(timestamp?: number) {
  if (!timestamp) return ''
  return new Date(timestamp * 1000).toLocaleString()
}

interface ChangeUsernameDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  profile: UserProfile
  turnstileEnabled: boolean
  turnstileSiteKey: string
  onUpdated?: () => void
}

export function ChangeUsernameDialog({
  open,
  onOpenChange,
  profile,
  turnstileEnabled,
  turnstileSiteKey,
  onUpdated,
}: ChangeUsernameDialogProps) {
  const { t } = useTranslation()
  const [username, setUsername] = useState(profile.username)
  const [loading, setLoading] = useState(false)
  const [turnstileToken, setTurnstileToken] = useState('')
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0)

  const usernameQuota = useMemo(() => {
    const limit = profile.username_change_limit ?? DEFAULT_USERNAME_CHANGE_LIMIT
    const count = profile.username_change_count ?? 0
    const remaining = profile.username_change_remaining ?? limit
    const resetAt = profile.username_change_reset_at ?? 0
    const windowStarted = count > 0 && resetAt > 0
    const exhausted = windowStarted && remaining <= 0

    return { limit, count, remaining, resetAt, windowStarted, exhausted }
  }, [
    profile.username_change_count,
    profile.username_change_limit,
    profile.username_change_remaining,
    profile.username_change_reset_at,
  ])

  useEffect(() => {
    if (open) {
      setUsername(profile.username)
      setTurnstileToken('')
      setTurnstileWidgetKey((value) => value + 1)
    }
  }, [open, profile.username])

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()

    const trimmed = username.trim()
    if (!trimmed) {
      toast.error(t('Username cannot be empty'))
      return
    }
    if (trimmed.length > USERNAME_MAX_LENGTH) {
      toast.error(
        t('Username cannot exceed {{max}} characters', {
          max: USERNAME_MAX_LENGTH,
        })
      )
      return
    }
    if (trimmed === profile.username) {
      onOpenChange(false)
      return
    }
    if (usernameQuota.exhausted) {
      toast.error(t('Username change quota has been used up'))
      return
    }
    if (turnstileEnabled && !turnstileToken) {
      toast.error(t('Please complete Turnstile verification'))
      return
    }

    try {
      setLoading(true)
      const response = await updateUserProfile(
        { username: trimmed },
        turnstileToken
      )

      if (response.success) {
        toast.success(t('Username updated successfully'))
        onOpenChange(false)
        setTurnstileToken('')
        setTurnstileWidgetKey((value) => value + 1)
        onUpdated?.()
      } else {
        toast.error(response.message || t('Failed to update username'))
        if (turnstileEnabled) {
          setTurnstileToken('')
          setTurnstileWidgetKey((value) => value + 1)
        }
      }
    } catch (_error) {
      toast.error(t('Failed to update username'))
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
            <DialogTitle>{t('Update Username')}</DialogTitle>
            <DialogDescription>
              {t('Change the username used for account display and login.')}
            </DialogDescription>
          </DialogHeader>

          <div className='my-6 space-y-4'>
            <div className='bg-muted/40 rounded-lg border p-3 text-sm'>
              <div className='text-muted-foreground'>
                {t('Current username')}: @{profile.username}
              </div>
              <div className='text-muted-foreground mt-2'>
                {usernameQuota.windowStarted
                  ? t(
                      'Username changes: {{count}} used, {{remaining}} remaining out of {{limit}} per year.',
                      {
                        count: usernameQuota.count,
                        remaining: usernameQuota.remaining,
                        limit: usernameQuota.limit,
                      }
                    )
                  : t('You can change your username up to {{limit}} times per year.', {
                      limit: usernameQuota.limit,
                    })}
              </div>
              {usernameQuota.windowStarted && (
                <div className='text-muted-foreground mt-1'>
                  {t('Quota resets at {{time}}', {
                    time: formatResetTime(usernameQuota.resetAt),
                  })}
                </div>
              )}
            </div>

            <div className='space-y-2'>
              <Label htmlFor='username'>{t('New Username')}</Label>
              <Input
                id='username'
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                disabled={loading || usernameQuota.exhausted}
                maxLength={USERNAME_MAX_LENGTH}
                autoComplete='username'
                autoFocus
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
            <Button
              type='submit'
              disabled={loading || usernameQuota.exhausted}
            >
              {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {loading ? t('Saving...') : t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
