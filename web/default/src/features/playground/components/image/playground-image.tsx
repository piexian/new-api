import { useCallback, useState } from 'react'
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

import {
  API_ENDPOINTS,
  IMAGE_QUALITY_OPTIONS,
  IMAGE_SIZE_OPTIONS,
  IMAGE_STYLE_OPTIONS,
} from '../../constants'
import type { ImageInterface, ImageResponse, ModelOption, GroupOption } from '../../types'

interface PlaygroundImageProps {
  models: ModelOption[]
  groups: GroupOption[]
  selectedModel: string
  selectedGroup: string
  onModelChange: (value: string) => void
  onGroupChange: (value: string) => void
}

export function PlaygroundImage({
  models,
  groups,
  selectedModel,
  selectedGroup,
  onModelChange,
  onGroupChange,
}: PlaygroundImageProps) {
  const { t } = useTranslation()
  const [imageInterface, setImageInterface] = useState<ImageInterface>('generations')
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState<string>('1024x1024')
  const [quality, setQuality] = useState<string>('standard')
  const [style, setStyle] = useState<string>('vivid')
  const [count, setCount] = useState(1)
  const [isLoading, setIsLoading] = useState(false)
  const [results, setResults] = useState<ImageResponse | null>(null)
  const [editImage, setEditImage] = useState<string>('')

  const handleSubmit = useCallback(async () => {
    if (!prompt.trim() || !selectedModel) return
    setIsLoading(true)
    setResults(null)
    try {
      const endpoint =
        imageInterface === 'generations'
          ? API_ENDPOINTS.IMAGE_GENERATIONS
          : API_ENDPOINTS.IMAGE_EDITS

      const body: Record<string, unknown> = {
        model: selectedModel,
        group: selectedGroup,
        prompt,
        n: count,
        size,
        response_format: 'url',
      }

      if (imageInterface === 'generations') {
        body.quality = quality
        body.style = style
      } else {
        body.image = editImage
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

      const data: ImageResponse = await res.json()
      setResults(data)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setIsLoading(false)
    }
  }, [prompt, selectedModel, selectedGroup, imageInterface, count, size, quality, style, editImage])

  return (
    <div className='mx-auto flex w-full max-w-4xl flex-col gap-4 p-4'>
      {/* 模式切换 */}
      <div className='flex items-center gap-2'>
        <Select value={imageInterface} onValueChange={(v) => setImageInterface(v as ImageInterface)}>
          <SelectTrigger className='w-40'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='generations'>{t('Image Generation')}</SelectItem>
            <SelectItem value='edits'>{t('Image Edit')}</SelectItem>
          </SelectContent>
        </Select>
      </div>

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
        placeholder={t('Describe the image you want to generate...')}
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        rows={4}
      />

      {/* 图片编辑模式：输入图片 */}
      {imageInterface === 'edits' && (
        <div className='flex flex-col gap-2'>
          <Label>{t('Image URL or base64')}</Label>
          <Input
            placeholder={t('Enter image URL or paste base64...')}
            value={editImage}
            onChange={(e) => setEditImage(e.target.value)}
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
              {IMAGE_SIZE_OPTIONS.map((s) => (
                <SelectItem key={s} value={s}>
                  {s}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        {imageInterface === 'generations' && (
          <>
            <div className='flex flex-col gap-1'>
              <Label className='text-xs'>{t('Quality')}</Label>
              <Select value={quality} onValueChange={(v) => v && setQuality(v)}>
                <SelectTrigger className='w-28'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {IMAGE_QUALITY_OPTIONS.map((q) => (
                    <SelectItem key={q} value={q}>
                      {q}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='flex flex-col gap-1'>
              <Label className='text-xs'>{t('Style')}</Label>
              <Select value={style} onValueChange={(v) => v && setStyle(v)}>
                <SelectTrigger className='w-28'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {IMAGE_STYLE_OPTIONS.map((s) => (
                    <SelectItem key={s} value={s}>
                      {s}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </>
        )}
        <div className='flex flex-col gap-1'>
          <Label className='text-xs'>{t('Count')}</Label>
          <Input
            type='number'
            min={1}
            max={4}
            value={count}
            onChange={(e) => setCount(Number(e.target.value))}
            className='w-20'
          />
        </div>
      </div>

      {/* 生成按钮 */}
      <Button onClick={handleSubmit} disabled={isLoading || !prompt.trim()}>
        {isLoading ? t('Generating...') : t('Generate')}
      </Button>

      {/* 结果展示 */}
      {results && (
        <div className='grid grid-cols-2 gap-4 sm:grid-cols-3'>
          {results.data.map((img, i) => (
            <div key={i} className='overflow-hidden rounded-lg border'>
              {img.url ? (
                <img
                  src={img.url}
                  alt={img.revised_prompt || `Image ${i + 1}`}
                  className='size-full object-cover'
                />
              ) : img.b64_json ? (
                <img
                  src={`data:image/png;base64,${img.b64_json}`}
                  alt={`Image ${i + 1}`}
                  className='size-full object-cover'
                />
              ) : null}
              {img.revised_prompt && (
                <p className='truncate p-2 text-xs text-muted-foreground'>
                  {img.revised_prompt}
                </p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
