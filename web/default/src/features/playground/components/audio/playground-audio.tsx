import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ModelGroupSelector } from '@/components/model-group-selector'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { API_ENDPOINTS, AUDIO_VOICE_OPTIONS } from '../../constants'
import type { AudioInterface, GroupOption, ModelOption } from '../../types'

const AUDIO_FORMATS = ['mp3', 'opus', 'aac', 'flac', 'wav', 'pcm'] as const

interface PlaygroundAudioProps {
  models: ModelOption[]
  groups: GroupOption[]
  selectedModel: string
  selectedGroup: string
  onModelChange: (value: string) => void
  onGroupChange: (value: string) => void
}

export function PlaygroundAudio({
  models,
  groups,
  selectedModel,
  selectedGroup,
  onModelChange,
  onGroupChange,
}: PlaygroundAudioProps) {
  const { t } = useTranslation()
  const [audioInterface, setAudioInterface] = useState<AudioInterface>('speech')
  const [text, setText] = useState('')
  const [voice, setVoice] = useState('alloy')
  const [format, setFormat] = useState('mp3')
  const [speed, setSpeed] = useState(1)
  const [isLoading, setIsLoading] = useState(false)
  const [audioUrl, setAudioUrl] = useState<string | null>(null)
  const [transcription, setTranscription] = useState<string | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  const handleSpeech = useCallback(async () => {
    if (!text.trim() || !selectedModel) return
    setIsLoading(true)
    setAudioUrl(null)
    try {
      const res = await fetch(API_ENDPOINTS.AUDIO_SPEECH, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          model: selectedModel,
          group: selectedGroup,
          input: text,
          voice,
          response_format: format,
          speed,
        }),
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err?.error?.message || `HTTP ${res.status}`)
      }

      const blob = await res.blob()
      setAudioUrl(URL.createObjectURL(blob))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setIsLoading(false)
    }
  }, [text, selectedModel, selectedGroup, voice, format, speed])

  const handleTranscription = useCallback(async () => {
    const file = fileRef.current?.files?.[0]
    if (!file || !selectedModel) return
    setIsLoading(true)
    setTranscription(null)
    try {
      const formData = new FormData()
      formData.append('model', selectedModel)
      formData.append('group', selectedGroup)
      formData.append('file', file)

      const res = await fetch(API_ENDPOINTS.AUDIO_TRANSCRIPTIONS, {
        method: 'POST',
        credentials: 'include',
        body: formData,
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err?.error?.message || `HTTP ${res.status}`)
      }

      const data = await res.json()
      setTranscription(data.text || JSON.stringify(data))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setIsLoading(false)
    }
  }, [selectedModel, selectedGroup])

  return (
    <div className='mx-auto flex w-full max-w-4xl flex-col gap-4 p-4'>
      {/* 模式切换 */}
      <Select
        value={audioInterface}
        onValueChange={(v) => setAudioInterface(v as AudioInterface)}
      >
        <SelectTrigger className='w-48'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='speech'>{t('Text to Speech')}</SelectItem>
          <SelectItem value='transcriptions'>
            {t('Audio Transcription')}
          </SelectItem>
        </SelectContent>
      </Select>

      {/* 模型 + 分组 */}
      <ModelGroupSelector
        selectedModel={selectedModel}
        models={models}
        onModelChange={onModelChange}
        selectedGroup={selectedGroup}
        groups={groups}
        onGroupChange={onGroupChange}
      />

      {audioInterface === 'speech' ? (
        <>
          <Textarea
            placeholder={t('Enter text to convert to speech...')}
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={6}
          />
          <div className='flex flex-wrap gap-2'>
            <div className='flex flex-col gap-1'>
              <Label className='text-xs'>{t('Voice')}</Label>
              <Select value={voice} onValueChange={(v) => v && setVoice(v)}>
                <SelectTrigger className='w-32'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {AUDIO_VOICE_OPTIONS.map((v) => (
                    <SelectItem key={v} value={v}>
                      {v}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='flex flex-col gap-1'>
              <Label className='text-xs'>{t('Format')}</Label>
              <Select value={format} onValueChange={(v) => v && setFormat(v)}>
                <SelectTrigger className='w-28'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {AUDIO_FORMATS.map((f) => (
                    <SelectItem key={f} value={f}>
                      {f}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='flex flex-col gap-1'>
              <Label className='text-xs'>{t('Speed')}</Label>
              <Input
                type='number'
                step={0.25}
                min={0.25}
                max={4}
                value={speed}
                onChange={(e) => setSpeed(Number(e.target.value))}
                className='w-20'
              />
            </div>
          </div>
          <Button onClick={handleSpeech} disabled={isLoading || !text.trim()}>
            {isLoading ? t('Generating...') : t('Generate Speech')}
          </Button>
          {audioUrl && <audio src={audioUrl} controls className='w-full' />}
        </>
      ) : (
        <>
          <div className='flex flex-col gap-2'>
            <Label>{t('Upload audio file')}</Label>
            <Input ref={fileRef} type='file' accept='audio/*' />
          </div>
          <Button onClick={handleTranscription} disabled={isLoading}>
            {isLoading ? t('Transcribing...') : t('Transcribe')}
          </Button>
          {transcription && (
            <div className='rounded-lg border p-4'>
              <Label className='mb-2 block text-xs'>{t('Transcription')}</Label>
              <p className='text-sm whitespace-pre-wrap'>{transcription}</p>
            </div>
          )}
        </>
      )}
    </div>
  )
}
