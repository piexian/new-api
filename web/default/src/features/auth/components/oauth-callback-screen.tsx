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
import {
  Loader2,
  Send,
  Shield,
  ShieldAlert,
  UserRound,
  type LucideIcon,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { SiGithub, SiLinux, SiSteam, SiWechat } from 'react-icons/si'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSeparator,
  InputOTPSlot,
} from '@/components/ui/input-otp'
import { Spinner } from '@/components/ui/spinner'

import type { OAuthOwnershipTransferView } from '../api'
import { AuthLayout } from '../auth-layout'

type OAuthCallbackScreenProps = {
  provider: string
  mode: 'login' | 'bind'
  ownershipTransfer?: OAuthOwnershipTransferView | null
  ownershipError?: string
  isSendingCode?: boolean
  isConfirming?: boolean
  onSendCode?: () => Promise<void>
  onConfirm?: (code: string) => Promise<void>
  onReturn?: () => void
}

type ProviderMeta = {
  label: string
  Icon: LucideIcon | ((props: { className?: string }) => React.JSX.Element)
}

const providerDictionary: Record<string, ProviderMeta> = {
  github: {
    label: 'GitHub',
    Icon: (props: { className?: string }) => (
      <SiGithub className={props.className} focusable='false' />
    ),
  },
  oidc: { label: 'OIDC', Icon: Shield },
  linuxdo: {
    label: 'LinuxDO',
    Icon: (props: { className?: string }) => (
      <SiLinux className={props.className} focusable='false' />
    ),
  },
  telegram: { label: 'Telegram', Icon: Send },
  wechat: {
    label: 'WeChat',
    Icon: (props: { className?: string }) => (
      <SiWechat className={props.className} focusable='false' />
    ),
  },
  steam: {
    label: 'Steam',
    Icon: (props: { className?: string }) => (
      <SiSteam className={props.className} focusable='false' />
    ),
  },
}

export function OAuthCallbackScreen({
  provider,
  mode,
  ownershipTransfer,
  ownershipError,
  isSendingCode = false,
  isConfirming = false,
  onSendCode,
  onConfirm,
  onReturn,
}: OAuthCallbackScreenProps) {
  const { t } = useTranslation()
  const [verificationCode, setVerificationCode] = useState('')
  const [remainingSeconds, setRemainingSeconds] = useState(0)
  const { label, Icon } = useMemo(() => {
    const normalized = provider?.toLowerCase() ?? ''
    return (
      providerDictionary[normalized] || {
        label: 'account',
        Icon: UserRound,
      }
    )
  }, [provider])

  const providerLabel = t(label)
  const isBindMode = mode === 'bind'

  useEffect(() => {
    if (!ownershipTransfer?.code_sent || !ownershipTransfer.expires_at) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setRemainingSeconds(0)
      return
    }
    const updateRemaining = () => {
      setRemainingSeconds(
        Math.max(
          0,
          ownershipTransfer.expires_at - Math.floor(Date.now() / 1000)
        )
      )
    }
    updateRemaining()
    const timer = window.setInterval(updateRemaining, 1000)
    return () => window.clearInterval(timer)
  }, [ownershipTransfer?.code_sent, ownershipTransfer?.expires_at])

  if (ownershipTransfer) {
    const transferProvider = ownershipTransfer.provider || providerLabel
    const minutes = Math.floor(remainingSeconds / 60)
    const seconds = remainingSeconds % 60
    const countdown = `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
    const isExpired =
      ownershipTransfer.code_sent &&
      ownershipTransfer.expires_at > 0 &&
      Math.floor(Date.now() / 1000) >= ownershipTransfer.expires_at
    const isClosed = ownershipTransfer.closed || isExpired
    const canConfirm =
      ownershipTransfer.code_sent &&
      !isClosed &&
      verificationCode.length === 6 &&
      !isConfirming

    return (
      <AuthLayout>
        <div className='w-full space-y-6'>
          <div className='flex flex-col items-center space-y-3 text-center'>
            <div className='bg-destructive/10 text-destructive flex h-14 w-14 items-center justify-center rounded-lg'>
              <ShieldAlert className='h-7 w-7' />
            </div>
            <div className='space-y-1.5'>
              <h2 className='text-xl font-semibold'>
                {t('OAuth email ownership verification')}
              </h2>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'The email {{email}} is already attached to another account.',
                  {
                    email: ownershipTransfer.email || '***',
                  }
                )}
              </p>
            </div>
          </div>

          <Alert>
            <ShieldAlert />
            <AlertTitle>{t('Permanent account action')}</AlertTitle>
            <AlertDescription>
              {t(
                'This is the only verification opportunity for this {{provider}} account and email. Success permanently disables the previous account and moves the email here. The code is sent once, expires with the countdown, and five incorrect entries close this opportunity forever.',
                { provider: transferProvider }
              )}
            </AlertDescription>
          </Alert>

          {ownershipError ? (
            <Alert variant='destructive'>
              <ShieldAlert />
              <AlertTitle>{t('Verification failed')}</AlertTitle>
              <AlertDescription>{ownershipError}</AlertDescription>
            </Alert>
          ) : null}

          {isClosed && (
            <div className='space-y-4 text-center'>
              <p className='text-destructive text-sm font-medium'>
                {t(
                  'This ownership verification is permanently closed and will not reopen.'
                )}
              </p>
              <Button
                type='button'
                variant='outline'
                className='w-full'
                onClick={onReturn}
              >
                {t('Return to sign in')}
              </Button>
            </div>
          )}
          {!isClosed && ownershipTransfer.code_sent && (
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor='oauth-ownership-code'>
                  {t('Verification Code')}
                </FieldLabel>
                <InputOTP
                  id='oauth-ownership-code'
                  maxLength={6}
                  value={verificationCode}
                  onChange={setVerificationCode}
                  disabled={isConfirming}
                  containerClassName='justify-between sm:[&>[data-slot="input-otp-group"]>div]:w-12'
                >
                  <InputOTPGroup>
                    <InputOTPSlot index={0} />
                    <InputOTPSlot index={1} />
                  </InputOTPGroup>
                  <InputOTPSeparator />
                  <InputOTPGroup>
                    <InputOTPSlot index={2} />
                    <InputOTPSlot index={3} />
                  </InputOTPGroup>
                  <InputOTPSeparator />
                  <InputOTPGroup>
                    <InputOTPSlot index={4} />
                    <InputOTPSlot index={5} />
                  </InputOTPGroup>
                </InputOTP>
                <FieldDescription>
                  {t(
                    'Time remaining: {{countdown}}. Incorrect attempts remaining: {{remaining}}.',
                    {
                      countdown,
                      remaining: ownershipTransfer.attempts_remaining,
                    }
                  )}
                </FieldDescription>
              </Field>
              <Button
                type='button'
                variant='destructive'
                className='w-full'
                disabled={!canConfirm}
                onClick={() => onConfirm?.(verificationCode)}
              >
                {isConfirming ? <Spinner data-icon='inline-start' /> : null}
                {t('Confirm transfer and permanently disable previous account')}
              </Button>
            </FieldGroup>
          )}
          {!isClosed && !ownershipTransfer.code_sent && (
            <Button
              type='button'
              variant='destructive'
              className='w-full'
              disabled={isSendingCode}
              onClick={onSendCode}
            >
              {isSendingCode ? (
                <Spinner data-icon='inline-start' />
              ) : (
                <Send data-icon='inline-start' />
              )}
              {t('I understand, send the only verification code')}
            </Button>
          )}
        </div>
      </AuthLayout>
    )
  }

  const headline = isBindMode
    ? t('Binding your {{provider}} account', { provider: providerLabel })
    : t('Signing you in with {{provider}}', { provider: providerLabel })

  const description = isBindMode
    ? t('Hang tight while we securely link this account to your profile.')
    : t('Hang tight while we finish connecting your account.')

  const secondaryNote = isBindMode
    ? t(
        'You can close this tab once the binding completes or a success message appears in the original window.'
      )
    : t(
        "You'll be redirected automatically. You can return to the previous page if nothing happens after a few seconds."
      )

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='flex flex-col items-center space-y-4 text-center'>
          <div className='bg-muted flex h-16 w-16 items-center justify-center rounded-2xl'>
            <Icon className='h-8 w-8' />
          </div>
          <div className='space-y-2'>
            <h2 className='text-center text-2xl font-semibold tracking-tight'>
              {headline}
            </h2>
            <p className='text-muted-foreground text-sm sm:text-base'>
              {description}
            </p>
          </div>
        </div>

        <div className='space-y-4 text-center'>
          <div className='flex items-center justify-center gap-2 text-sm font-medium'>
            <Loader2 className='h-4 w-4 animate-spin' />
            <span>{t('Processing OAuth response...')}</span>
          </div>
          <p className='text-muted-foreground text-sm'>{secondaryNote}</p>
          <p className='text-muted-foreground text-xs'>
            {t(
              'This may take a few moments while we validate the request and update your session.'
            )}
          </p>
        </div>
      </div>
    </AuthLayout>
  )
}
