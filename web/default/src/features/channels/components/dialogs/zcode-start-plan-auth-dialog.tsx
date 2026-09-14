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
import { useQueryClient } from '@tanstack/react-query'
import {
  CircleCheck,
  CircleX,
  ExternalLink,
  KeyRound,
  Loader2,
  RefreshCw,
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { IconBadge } from '@/components/ui/icon-badge'
import { formatTimestampToDate } from '@/lib/format'

import {
  initZcodeStartPlanAuth,
  pollZcodeStartPlanAuth,
  type ZcodeStartPlanAuthPollResponse,
} from '../../api'
import { channelsQueryKeys } from '../../lib'
import { useChannels } from '../channels-provider'

type ZcodeStartPlanAuthDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type FlowPhase = 'idle' | 'starting' | 'authorizing' | 'polling' | 'ready'

const DEFAULT_POLL_INTERVAL_MS = 3000

export function ZcodeStartPlanAuthDialog({
  open,
  onOpenChange,
}: ZcodeStartPlanAuthDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const queryClient = useQueryClient()

  const [phase, setPhase] = useState<FlowPhase>('idle')
  const [authorizeUrl, setAuthorizeUrl] = useState('')
  const [failure, setFailure] = useState('')
  const [result, setResult] = useState<
    ZcodeStartPlanAuthPollResponse['data'] | null
  >(null)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const closedRef = useRef(false)

  const clearPollTimer = () => {
    if (pollTimerRef.current) {
      clearTimeout(pollTimerRef.current)
      pollTimerRef.current = null
    }
  }

  const resetState = useCallback(() => {
    clearPollTimer()
    setPhase('idle')
    setAuthorizeUrl('')
    setFailure('')
    setResult(null)
  }, [])

  const schedulePoll = useCallback(
    (channelId: number, delayMs: number) => {
      clearPollTimer()
      pollTimerRef.current = setTimeout(async () => {
        if (closedRef.current) return
        try {
          const response = await pollZcodeStartPlanAuth(channelId)
          if (closedRef.current) return
          if (!response.success) {
            // 后端会话丢失/网络抖动：提示但不终止，等待下一次轮询重试。
            setPhase('polling')
            schedulePoll(channelId, delayMs)
            return
          }
          const status = response.data?.status
          if (status === 'ready') {
            setPhase('ready')
            setResult(response.data ?? null)
            toast.success(t('Authorization succeeded, channel key updated'))
            await queryClient.invalidateQueries({
              queryKey: channelsQueryKeys.lists(),
            })
            return
          }
          if (status === 'failed') {
            setPhase('idle')
            setFailure(t('Authorization failed or was cancelled'))
            return
          }
          if (status === 'expired') {
            setPhase('idle')
            setFailure(t('Authorization flow expired, please retry'))
            return
          }
          schedulePoll(channelId, delayMs)
        } catch {
          if (!closedRef.current) {
            schedulePoll(channelId, delayMs)
          }
        }
      }, delayMs)
    },
    [queryClient, t]
  )

  const startFlow = useCallback(async () => {
    const channelId = currentRow?.id
    if (!channelId) return
    setFailure('')
    setResult(null)
    setPhase('starting')
    try {
      const response = await initZcodeStartPlanAuth(channelId)
      if (!response.success || !response.data?.authorize_url) {
        setPhase('idle')
        setFailure(response.message || t('Failed to start authorization'))
        return
      }
      setAuthorizeUrl(response.data.authorize_url)
      const intervalSec = Number(response.data.poll_interval_sec)
      const delayMs =
        Number.isFinite(intervalSec) && intervalSec > 0
          ? Math.max(intervalSec * 1000, 1000)
          : DEFAULT_POLL_INTERVAL_MS
      setPhase('authorizing')
      schedulePoll(channelId, delayMs)
    } catch (error: unknown) {
      setPhase('idle')
      setFailure(
        error instanceof Error
          ? error.message
          : t('Failed to start authorization')
      )
    }
  }, [currentRow, schedulePoll, t])

  useEffect(() => {
    closedRef.current = false
    if (open) {
      startFlow()
    } else {
      resetState()
    }
    return () => {
      closedRef.current = true
      clearPollTimer()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  if (!currentRow) return null

  const handleClose = () => {
    closedRef.current = true
    clearPollTimer()
    onOpenChange(false)
  }

  const jwtExpiresAt = result?.jwt_expires_at

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose()
      }}
      title={t('StartPlan Re-authorization')}
      description={
        <>
          {t('Re-authorize ZCode StartPlan for:')}
          <strong>{currentRow.name}</strong>
        </>
      }
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <Button variant='outline' onClick={handleClose}>
          {t('Close')}
        </Button>
      }
    >
      <div className='space-y-4 py-4'>
        <div className='bg-muted/50 rounded-lg border p-4'>
          <div className='text-muted-foreground mb-2 flex items-center gap-2 text-sm'>
            <IconBadge tone='info' size='xs'>
              <KeyRound />
            </IconBadge>
            <span>{t('ZCode StartPlan uses a JWT channel key')}</span>
          </div>
          <div className='text-muted-foreground text-xs'>
            {t(
              'The official client has no refresh token flow. When the JWT expires, re-run this authorization: open the page, log in to z.ai once, and the new key is saved automatically.'
            )}
          </div>
        </div>

        {(phase === 'starting' || phase === 'polling') && (
          <div className='text-muted-foreground flex items-center gap-2 text-sm'>
            <Loader2 className='h-4 w-4 animate-spin' />
            {phase === 'starting'
              ? t('Initializing authorization flow…')
              : t('Waiting for authorization to complete…')}
          </div>
        )}

        {phase === 'authorizing' && authorizeUrl && (
          <div className='space-y-2'>
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <Loader2 className='h-4 w-4 animate-spin' />
              {t('Waiting for authorization to complete…')}
            </div>
            <Button
              className='w-full'
              onClick={() => window.open(authorizeUrl, '_blank', 'noopener')}
            >
              <ExternalLink className='mr-2 h-4 w-4' />
              {t('Open Authorization Page')}
            </Button>
          </div>
        )}

        {phase === 'ready' && (
          <div className='space-y-2'>
            <div className='flex items-center gap-2 text-sm'>
              <IconBadge tone='success' size='xs'>
                <CircleCheck />
              </IconBadge>
              <span>{t('Authorization succeeded, channel key updated')}</span>
            </div>
            {result?.user_name && (
              <div className='text-muted-foreground text-xs'>
                {t('Account:')} {result.user_name}
                {result.email ? ` (${result.email})` : ''}
              </div>
            )}
            {jwtExpiresAt ? (
              <div className='text-muted-foreground text-xs'>
                {t('JWT Expires At')}{' '}
                {formatTimestampToDate(jwtExpiresAt * 1000)}
              </div>
            ) : null}
          </div>
        )}

        {failure && (
          <div className='flex items-center gap-2 text-sm'>
            <IconBadge tone='destructive' size='xs'>
              <CircleX />
            </IconBadge>
            <span>{failure}</span>
          </div>
        )}

        {(phase === 'idle' || failure) && (
          <Button className='w-full' onClick={startFlow} disabled={!currentRow}>
            <RefreshCw className='mr-2 h-4 w-4' />
            {failure ? t('Retry') : t('Start Authorization')}
          </Button>
        )}
      </div>
    </Dialog>
  )
}
