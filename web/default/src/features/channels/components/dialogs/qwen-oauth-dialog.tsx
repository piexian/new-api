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
import { CheckCircle2, Copy, ExternalLink, Loader2 } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import { completeQwenOAuth, startQwenOAuth } from '../../api'

export type QwenOAuthIdentity = {
  email?: string
  aliyunId?: string
  expiresAt?: string
}

type QwenOAuthDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  apiKey: string
  channelId?: number
  onSuccess: (key: string | undefined, identity: QwenOAuthIdentity) => void
}

export function QwenOAuthDialog({
  open,
  onOpenChange,
  apiKey,
  channelId,
  onSuccess,
}: QwenOAuthDialogProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const [verificationUrl, setVerificationUrl] = useState('')
  const [status, setStatus] = useState<'idle' | 'pending' | 'complete'>('idle')
  const [isStarting, setIsStarting] = useState(false)
  const [identity, setIdentity] = useState<QwenOAuthIdentity | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const cancelledRef = useRef(false)

  const stopPolling = useCallback(() => {
    cancelledRef.current = true
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [])

  useEffect(() => stopPolling, [stopPolling])

  useEffect(() => {
    if (!open) {
      stopPolling()
      setVerificationUrl('')
      setStatus('idle')
      setIdentity(null)
    }
  }, [open, stopPolling])

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) stopPolling()
    onOpenChange(nextOpen)
  }

  const startAuthorization = async () => {
    const normalizedAPIKey = apiKey.trim()
    if (!channelId && !normalizedAPIKey.startsWith('sk-sp-')) {
      toast.error(t('Enter a valid sk-sp- Token Plan API key first'))
      return
    }

    stopPolling()
    cancelledRef.current = false
    setIsStarting(true)
    setStatus('idle')
    setIdentity(null)

    try {
      const response = await startQwenOAuth(channelId)
      const url = response.data?.verification_url?.trim() || ''
      if (!response.success || !url) {
        throw new Error(response.message || t('Failed to start authorization'))
      }

      const initialInterval = Math.max(response.data?.interval || 5, 1)
      const expiresAt =
        Date.now() + Math.max(response.data?.expires_in || 600, 1) * 1000
      setVerificationUrl(url)
      setStatus('pending')
      window.open(url, '_blank', 'noopener,noreferrer')

      const schedulePoll = (intervalSeconds: number, failureCount: number) => {
        const intervalMs = intervalSeconds * 1000
        const jitterWindowMs =
          failureCount <= 0
            ? 1000
            : Math.min(intervalMs * 2 ** Math.min(failureCount, 3), 30_000)
        timerRef.current = setTimeout(
          () => void poll(intervalSeconds, failureCount),
          intervalMs + Math.floor(Math.random() * jitterWindowMs)
        )
      }

      const poll = async (
        intervalSeconds: number,
        failureCount: number
      ): Promise<void> => {
        if (cancelledRef.current) return
        if (Date.now() >= expiresAt) {
          stopPolling()
          setStatus('idle')
          toast.error(t('Authorization session expired'))
          return
        }
        try {
          const result = await completeQwenOAuth(normalizedAPIKey, channelId)
          if (!result.success) {
            stopPolling()
            setStatus('idle')
            toast.error(result.message || t('Authorization failed'))
            return
          }

          const nextStatus = result.data?.status || 'authorization_pending'
          if (nextStatus === 'complete') {
            const nextIdentity = {
              email: result.data?.email,
              aliyunId: result.data?.aliyun_id,
              expiresAt: result.data?.expires_at,
            }
            stopPolling()
            setStatus('complete')
            setIdentity(nextIdentity)
            onSuccess(result.data?.key, nextIdentity)
            toast.success(t('Qwen Token Plan authorization bound'))
            return
          }
          if (nextStatus === 'access_denied') {
            throw new Error(t('Authorization was denied'))
          }
          if (nextStatus === 'expired_token') {
            throw new Error(t('Authorization session expired'))
          }

          const nextInterval =
            nextStatus === 'slow_down' ? intervalSeconds + 5 : intervalSeconds
          schedulePoll(nextInterval, 0)
        } catch {
          if (cancelledRef.current) return
          schedulePoll(intervalSeconds, failureCount + 1)
        }
      }

      schedulePoll(initialInterval, 0)
    } catch (error) {
      stopPolling()
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to start authorization')
      )
    } finally {
      setIsStarting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Qwen Token Plan authorization')}</DialogTitle>
          <DialogDescription>
            {t(
              'Sign in with the QianWen account used for Token Plan usage lookup. The API key remains the inference credential.'
            )}
          </DialogDescription>
        </DialogHeader>

        <Alert>
          <AlertDescription>
            {t(
              'QianWen does not expose an API that proves an sk-sp- key belongs to the OAuth account. New API stores both in one bound credential and validates them separately.'
            )}
          </AlertDescription>
        </Alert>

        <div className='flex flex-col gap-3'>
          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              onClick={startAuthorization}
              disabled={isStarting || status === 'pending'}
            >
              {isStarting || status === 'pending' ? (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              ) : (
                <ExternalLink className='mr-2 h-4 w-4' />
              )}
              {status === 'pending'
                ? t('Waiting for authorization...')
                : t('Open authorization page')}
            </Button>
            <Button
              type='button'
              variant='outline'
              disabled={!verificationUrl}
              onClick={() => void copyToClipboard(verificationUrl)}
            >
              <Copy className='mr-2 h-4 w-4' />
              {t('Copy authorization link')}
            </Button>
          </div>

          {verificationUrl && (
            <p className='text-muted-foreground text-xs break-all'>
              {verificationUrl}
            </p>
          )}

          {status === 'complete' && identity && (
            <div className='border-border flex items-start gap-3 rounded-md border p-3'>
              <CheckCircle2 className='mt-0.5 h-5 w-5 text-emerald-600' />
              <div className='min-w-0 text-sm'>
                <p className='font-medium'>{t('Authorization completed')}</p>
                <p className='text-muted-foreground break-all'>
                  {identity.email || identity.aliyunId || t('Unknown account')}
                </p>
                {identity.expiresAt && (
                  <p className='text-muted-foreground text-xs'>
                    {t('Expires at')}: {identity.expiresAt}
                  </p>
                )}
              </div>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
          >
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
