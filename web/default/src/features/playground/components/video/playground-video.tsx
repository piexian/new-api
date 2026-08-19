import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

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
import { ModelGroupSelector } from '@/components/model-group-selector'
import { Textarea } from '@/components/ui/textarea'
import { toast } from 'sonner'

import { API_ENDPOINTS } from '../../constants'
import type { GroupOption, ModelOption, VideoInterface, VideoTaskResponse } from '../../types'

const VIDEO_SIZE_OPTIONS = ['640x480', '1280x720', '1920x1080'] as const

interface PlaygroundVideoProps {
  models: ModelOption[]
  groups: GroupOption[]
  selectedModel: string
  selectedGroup: string
  onModelChange: (value: string) => void
  onGroupChange: (value: string) => void
}

export function PlaygroundVideo({
  models,
  groups,
  selectedModel,
  selectedGroup,
  onModelChange,
  onGroupChange,
}: PlaygroundVideoProps) {
  const { t } = useTranslation()
  const [videoInterface, setVideoInterface] = useState<VideoInterface>('generations')
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState<string>('1280x720')
  const [duration, setDuration] = useState(5)
  const [fps, setFps] = useState(24)
  const [isLoading, setIsLoading] = useState(false)
  const [result, setResult] = useState<VideoTaskResponse | null>(null)
  const [imageInput, setImageInput] = useState<string>('')
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPolling = () => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const pollTask = useCallback(
    async (taskId: string) => {
      stopPolling()
      pollRef.current = setInterval(async () => {
        try {
          const res = await fetch(`/api/video/task/${taskId}`, {
            credentials: 'include',
          })
          if (!res.ok) return
          const data: VideoTaskResponse = await res.json()
          setResult(data)
          if (data.status === 'succeeded' || data.status === 'failed') {
            stopPolling()
            setIsLoading(false)
          }
        } catch {
          // 忽略轮询错误
        }
      }, 3000)
    },
    [],
  )

  const handleSubmit = useCallback(async () => {
    if (!prompt.trim() || !selectedModel) return
    setIsLoading(true)
    setResult(null)
    try {
      const endpoint =
        videoInterface === 'generations'
          ? API_ENDPOINTS.VIDEO_GENERATIONS
          : videoInterface === 'edits'
            ? API_ENDPOINTS.VIDEO_EDITS
            : API_ENDPOINTS.VIDEO_EXTENSIONS

      const body: Record<string, unknown> = {
        model: selectedModel,
        group: selectedGroup,
        prompt,
        size,
        duration,
        fps,
      }

      if (videoInterface !== 'generations') {
        body.image = imageInput
      }

      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(body),
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err?.error?.message || `HTTP ${res.status}`)
      }

      const data: VideoTaskResponse = await res.json()
      setResult(data)

      // 异步任务开始轮询
      if (data.status === 'queued' || data.status === 'processing') {
        pollTask(data.id)
      } else {
        setIsLoading(false)
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
      setIsLoading(false)
    }
  }, [prompt, selectedModel, selectedGroup, videoInterface, size, duration, fps, imageInput, pollTask])

  return (
    <div className='mx-auto flex w-full max-w-4xl flex-col gap-4 p-4'>
      {/* 模式切换 */}
      <Select value={videoInterface} onValueChange={(v) => v && setVideoInterface(v as VideoInterface)}>
        <SelectTrigger className='w-48'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='generations'>{t('Video Generation')}</SelectItem>
          <SelectItem value='edits'>{t('Video Edit')}</SelectItem>
          <SelectItem value='extensions'>{t('Video Extension')}</SelectItem>
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

      {/* Prompt */}
      <Textarea
        placeholder={t('Describe the video you want to generate...')}
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        rows={4}
      />

      {/* 编辑/扩展模式：输入参考图 */}
      {videoInterface !== 'generations' && (
        <div className='flex flex-col gap-2'>
          <Label>{t('Reference image URL')}</Label>
          <Input
            placeholder={t('Enter image URL...')}
            value={imageInput}
            onChange={(e) => setImageInput(e.target.value)}
          />
        </div>
      )}

      {/* 参数 */}
      <div className='flex flex-wrap gap-2'>
        <div className='flex flex-col gap-1'>
          <Label className='text-xs'>{t('Size')}</Label>
          <Select value={size} onValueChange={(v) => v && setSize(v)}>
            <SelectTrigger className='w-32'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {VIDEO_SIZE_OPTIONS.map((s) => (
                <SelectItem key={s} value={s}>
                  {s}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='flex flex-col gap-1'>
          <Label className='text-xs'>{t('Duration (s)')}</Label>
          <Input
            type='number'
            min={1}
            max={60}
            value={duration}
            onChange={(e) => setDuration(Number(e.target.value))}
            className='w-24'
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label className='text-xs'>FPS</Label>
          <Input
            type='number'
            min={1}
            max={60}
            value={fps}
            onChange={(e) => setFps(Number(e.target.value))}
            className='w-20'
          />
        </div>
      </div>

      {/* 生成按钮 */}
      <Button onClick={handleSubmit} disabled={isLoading || !prompt.trim()}>
        {isLoading ? t('Processing...') : t('Generate')}
      </Button>

      {/* 结果展示 */}
      {result && (
        <div className='flex flex-col gap-2'>
          <div className='flex items-center gap-2'>
            <span className='text-sm font-medium'>{t('Status')}:</span>
            <span className='text-sm capitalize'>{result.status}</span>
          </div>
          {result.status === 'succeeded' && result.video?.url && (
            <video
              src={result.video.url}
              controls
              className='w-full rounded-lg border'
            />
          )}
          {result.status === 'failed' && result.error && (
            <p className='text-sm text-destructive'>{result.error.message}</p>
          )}
        </div>
      )}
    </div>
  )
}
