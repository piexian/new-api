import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCountdown } from '@/hooks/use-countdown'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { Turnstile } from '@/components/turnstile'
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
import { sendEmailVerification, bindEmail } from '../../api'

// ============================================================================
// Email Bind Dialog Component
// ============================================================================

interface EmailBindDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentEmail?: string
  onSuccess: () => void
}

export function EmailBindDialog({
  open,
  onOpenChange,
  currentEmail,
  onSuccess,
}: EmailBindDialogProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [sendingCode, setSendingCode] = useState(false)
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const { status } = useStatus()
  const [turnstileToken, setTurnstileToken] = useState('')
  const [turnstileWidgetKey, setTurnstileWidgetKey] = useState(0)
  const turnstileEnabled = !!(
    status?.turnstile_check && status?.turnstile_site_key
  )
  const turnstileSiteKey = String(status?.turnstile_site_key || '')
  const {
    secondsLeft,
    isActive,
    start: startCountdown,
    reset: resetCountdown,
  } = useCountdown({
    initialSeconds: 300,
  })

  const resetTurnstile = () => {
    setTurnstileToken('')
    setTurnstileWidgetKey((value) => value + 1)
  }

  const handleSendCode = async () => {
    if (!email || !email.includes('@')) {
      toast.error(t('Please enter a valid email address'))
      return
    }
    if (turnstileEnabled && !turnstileToken) {
      toast.info(t('Please wait a moment, human check is initializing...'))
      return
    }

    try {
      setSendingCode(true)
      const response = await sendEmailVerification(email, turnstileToken)

      if (response.success) {
        toast.success(t('Verification code sent! Please check your email.'))
        startCountdown()
      } else {
        toast.error(response.message || t('Failed to send verification code'))
      }
    } catch (_error) {
      toast.error(t('Failed to send verification code'))
    } finally {
      resetTurnstile()
      setSendingCode(false)
    }
  }

  const handleBind = async () => {
    if (!email || !code) {
      toast.error(t('Please enter email and verification code'))
      return
    }

    try {
      setLoading(true)
      const response = await bindEmail(email, code)

      if (response.success) {
        toast.success(t('Email bound successfully!'))
        onOpenChange(false)
        onSuccess()
        // Reset form
        setEmail('')
        setCode('')
        resetTurnstile()
        resetCountdown()
      } else {
        toast.error(response.message || t('Failed to bind email'))
      }
    } catch (_error) {
      toast.error(t('Failed to bind email'))
    } finally {
      setLoading(false)
    }
  }

  const handleOpenChange = (open: boolean) => {
    if (!loading) {
      onOpenChange(open)
      if (!open) {
        // Reset form when closing
        setEmail('')
        setCode('')
        resetTurnstile()
        resetCountdown()
      }
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Bind Email')}</DialogTitle>
          <DialogDescription>
            {currentEmail
              ? t('Current email: {{email}}. Enter a new email to change.', {
                  email: currentEmail,
                })
              : t('Bind an email address to your account.')}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label htmlFor='email'>{t('Email Address')}</Label>
            <Input
              id='email'
              type='email'
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t('Enter your email')}
              disabled={loading}
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='code'>{t('Verification Code')}</Label>
            <div className='flex gap-2'>
              <Input
                id='code'
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder={t('Enter code')}
                disabled={loading}
                maxLength={6}
              />
              <Button
                type='button'
                variant='outline'
                onClick={handleSendCode}
                disabled={sendingCode || isActive || !email}
              >
                {isActive
                  ? `${secondsLeft}s`
                  : sendingCode
                    ? t('Sending...')
                    : t('Send')}
              </Button>
            </div>
          </div>

          {turnstileEnabled && (
            <div className='flex justify-center'>
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
            onClick={() => handleOpenChange(false)}
            disabled={loading}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={handleBind}
            disabled={loading || !email || !code}
          >
            {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {loading ? t('Binding...') : t('Bind Email')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
