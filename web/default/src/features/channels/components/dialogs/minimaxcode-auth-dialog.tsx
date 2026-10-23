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
  CircleCheck,
  CircleX,
  ExternalLink,
  Loader2,
  KeyRound,
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { IconBadge } from '@/components/ui/icon-badge'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { formatTimestampToDate } from '@/lib/format'

import {
  pollMiniMaxCodeAuth,
  startMiniMaxCodeAuth,
  type MiniMaxCodeRegion,
} from '../../api'
import { useChannels } from '../channels-provider'

type MiniMaxCodeAuthDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /**
   * 传入 channelId 时为渠道重授权模式（就绪后直接保存到渠道）；
   * 不传时为新建渠道模式（就绪后通过 onKeyGenerated 返回凭证 JSON）。
   */
  channelId?: number
  onKeyGenerated?: (key: string) => void
}

type FlowPhase = 'idle' | 'starting' | 'authorizing' | 'failed' | 'ready'

const DEFAULT_POLL_INTERVAL_MS = 5000

export function MiniMaxCodeAuthDialog({
  open,
  onOpenChange,
  channelId,
  onKeyGenerated,
}: MiniMaxCodeAuthDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()

  const resolvedChannelId = channelId ?? currentRow?.id
  const isChannelMode = Boolean(resolvedChannelId)

  const [region, setRegion] = useState<MiniMaxCodeRegion>('cn')
  const [phase, setPhase] = useState<FlowPhase>('idle')
  const [userCode, setUserCode] = useState('')
  const [verificationUrl, setVerificationUrl] = useState('')
  const [verificationUrlComplete, setVerificationUrlComplete] = useState('')
  const [failure, setFailure] = useState('')
  const [result, setResult] = useState<{
    expires_at?: string
    group_id?: string
    key?: string
  } | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const closedRef = useRef(true)
  const generationRef = useRef(0)
  const busyRef = useRef(false)
  const intervalMsRef = useRef(DEFAULT_POLL_INTERVAL_MS)

  const clearPollTimer = () => {
    if (pollTimerRef.current) {
      clearTimeout(pollTimerRef.current)
      pollTimerRef.current = null
    }
  }

  const resetState = useCallback(() => {
    generationRef.current += 1
    clearPollTimer()
    busyRef.current = false
    setPhase('idle')
    setUserCode('')
    setVerificationUrl('')
    setFailure('')
    setResult(null)
  }, [])

  const schedulePoll = useCallback(
    (channelIdForPoll: number | undefined, generation: number) => {
      clearPollTimer()
      pollTimerRef.current = setTimeout(async () => {
        if (closedRef.current || generation !== generationRef.current) return
        try {
          const response = await pollMiniMaxCodeAuth(channelIdForPoll)
          if (closedRef.current || generation !== generationRef.current) return
          if (!response.success) {
            // 后端会话丢失/网络抖动：提示但不终止，等待下一次轮询重试。
            schedulePoll(channelIdForPoll, generation)
            return
          }
          const status = response.data?.status
          if (status === 'ready') {
            busyRef.current = false
            setPhase('ready')
            setResult(response.data ?? null)
            if (channelIdForPoll) {
              toast.success(t('Authorization succeeded, channel key updated'))
            }
            if (response.data?.key && !channelIdForPoll) {
              onKeyGenerated?.(response.data.key)
            }
            return
          }
          if (status === 'denied') {
            busyRef.current = false
            setUserCode('')
            setVerificationUrl('')
            setPhase('failed')
            setFailure(t('Authorization denied'))
            return
          }
          if (status === 'expired') {
            busyRef.current = false
            setUserCode('')
            setVerificationUrl('')
            setPhase('failed')
            setFailure(t('Authorization flow expired, please retry'))
            return
          }
          schedulePoll(channelIdForPoll, generation)
        } catch {
          if (!closedRef.current && generation === generationRef.current) {
            schedulePoll(channelIdForPoll, generation)
          }
        }
      }, intervalMsRef.current)
    },
    [onKeyGenerated, t]
  )

  const startFlow = useCallback(async () => {
    if (closedRef.current || busyRef.current) return
    busyRef.current = true
    const generation = ++generationRef.current
    clearPollTimer()
    setFailure('')
    setResult(null)
    setUserCode('')
    setVerificationUrl('')
    setPhase('starting')
    try {
      const response = await startMiniMaxCodeAuth(
        region,
        resolvedChannelId || undefined
      )
      if (closedRef.current || generation !== generationRef.current) return
      if (!response.success || !response.data?.user_code) {
        busyRef.current = false
        setPhase('failed')
        setFailure(response.message || t('Failed to start authorization'))
        return
      }
      setUserCode(response.data.user_code)
      setVerificationUrl(response.data.verification_url || '')
      setVerificationUrlComplete(response.data.verification_url_complete || '')
      const interval = Number(response.data.interval_ms)
      intervalMsRef.current =
        Number.isFinite(interval) && interval > 0
          ? Math.max(interval, 1000)
          : DEFAULT_POLL_INTERVAL_MS
      setPhase('authorizing')
      schedulePoll(resolvedChannelId || undefined, generation)
    } catch (error: unknown) {
      if (closedRef.current || generation !== generationRef.current) return
      busyRef.current = false
      setPhase('failed')
      setFailure(
        error instanceof Error
          ? error.message
          : t('Failed to start authorization')
      )
    }
  }, [region, resolvedChannelId, schedulePoll, t])

  useEffect(() => {
    closedRef.current = !open
    resetState()
    return () => {
      closedRef.current = true
      generationRef.current += 1
      clearPollTimer()
    }
  }, [open, resolvedChannelId, resetState])

  if (isChannelMode && !currentRow) return null

  const handleClose = () => {
    closedRef.current = true
    generationRef.current += 1
    clearPollTimer()
    onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose()
      }}
      title={t('MiniMax Code OAuth Login')}
      description={
        isChannelMode ? (
          <>
            {t('Re-authorize MiniMax Code for:')}
            <strong>{currentRow?.name}</strong>
          </>
        ) : (
          t('Log in with a MiniMax account to generate the channel credential')
        )
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
            <span>
              {t('MiniMax Code uses an OAuth JSON credential as the key')}
            </span>
          </div>
          <div className='text-muted-foreground text-xs'>
            {t(
              'Open the verification page, enter the code, and approve. The access token is refreshed automatically before expiry.'
            )}
          </div>
        </div>

        <FieldSet disabled={phase !== 'idle'}>
          <FieldLegend variant='label'>{t('Account region')}</FieldLegend>
          <RadioGroup
            value={region}
            disabled={phase !== 'idle'}
            onValueChange={(value) => {
              if (phase === 'idle' && (value === 'cn' || value === 'global')) {
                setRegion(value)
              }
            }}
          >
            <FieldGroup className='gap-3'>
              <Field orientation='horizontal' data-disabled={phase !== 'idle'}>
                <RadioGroupItem value='cn' id='minimaxcode-region-cn' />
                <FieldLabel htmlFor='minimaxcode-region-cn'>
                  {t('MiniMax China (minimax.cn)')}
                </FieldLabel>
              </Field>
              <Field orientation='horizontal' data-disabled={phase !== 'idle'}>
                <RadioGroupItem value='global' id='minimaxcode-region-global' />
                <FieldLabel htmlFor='minimaxcode-region-global'>
                  {t('MiniMax Global (minimax.io)')}
                </FieldLabel>
              </Field>
            </FieldGroup>
          </RadioGroup>
        </FieldSet>

        {phase === 'starting' && (
          <div className='text-muted-foreground flex items-center gap-2 text-sm'>
            <Loader2 className='h-4 w-4 animate-spin' />
            {t('Initializing authorization flow…')}
          </div>
        )}

        {phase === 'authorizing' && (
          <div className='space-y-3'>
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <Loader2 className='h-4 w-4 animate-spin' />
              {t('Waiting for authorization to complete…')}
            </div>
            <div className='flex items-center gap-2'>
              <span className='text-muted-foreground text-sm'>
                {t('Verification code:')}
              </span>
              <span className='font-mono text-base font-semibold tracking-widest'>
                {userCode}
              </span>
            </div>
            {(verificationUrlComplete || verificationUrl) && (
              <Button
                className='w-full'
                onClick={() =>
                  window.open(
                    verificationUrlComplete || verificationUrl,
                    '_blank',
                    'noopener'
                  )
                }
              >
                <ExternalLink className='mr-2 h-4 w-4' />
                {t('Open Authorization Page')}
              </Button>
            )}
          </div>
        )}

        {phase === 'ready' && (
          <div className='space-y-2'>
            <div className='flex items-center gap-2 text-sm'>
              <IconBadge tone='success' size='xs'>
                <CircleCheck />
              </IconBadge>
              <span>
                {isChannelMode
                  ? t('Authorization succeeded, channel key updated')
                  : t('Credential generated')}
              </span>
            </div>
            {result?.group_id && (
              <div className='text-muted-foreground text-xs'>
                {t('Plan group')}: {result.group_id}
              </div>
            )}
            {result?.expires_at && (
              <div className='text-muted-foreground text-xs'>
                {t('Token expires at')}{' '}
                {formatTimestampToDate(
                  new Date(result.expires_at).getTime(),
                  'milliseconds'
                )}
              </div>
            )}
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

        {(phase === 'idle' || phase === 'failed') && (
          <Button
            className='w-full'
            onClick={startFlow}
            disabled={isChannelMode && !currentRow}
          >
            {phase === 'failed' ? t('Retry') : t('Start Authorization')}
          </Button>
        )}
        {(phase === 'failed' || phase === 'ready') && (
          <Button className='w-full' variant='outline' onClick={resetState}>
            {phase === 'ready' ? t('Start new authorization') : t('Back')}
          </Button>
        )}
      </div>
    </Dialog>
  )
}
