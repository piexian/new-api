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
import type { LogOtherData, TaskLog } from '../types'

export type GenerationParams = Record<string, unknown>

export interface GenerationParamRow {
  key: string
  label: string
  value: string
  mono?: boolean
}

const PARAM_ORDER = [
  'model',
  'prompt',
  'size',
  'quality',
  'aspect_ratio',
  'resolution',
  'duration',
  'n',
  'response_format',
  'background',
  'moderation',
  'partial_images',
  'watermark',
  'seed',
  'negative_prompt',
  'style_preset',
  'image',
  'images',
  'reference_images',
  'video',
]

const PARAM_LABELS: Record<string, string> = {
  model: 'Model',
  prompt: 'Prompt',
  size: 'Size',
  quality: 'Quality',
  aspect_ratio: 'Aspect Ratio',
  resolution: 'Resolution',
  duration: 'Duration',
  n: 'Count',
  response_format: 'Response Format',
  background: 'Background',
  moderation: 'Moderation',
  partial_images: 'Partial Images',
  watermark: 'Watermark',
  seed: 'Seed',
  negative_prompt: 'Negative Prompt',
  style_preset: 'Style Preset',
  image: 'Source Image',
  images: 'Source Images',
  reference_images: 'Reference Images',
  video: 'Source Video',
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function parseJSONRecord(value: unknown): GenerationParams | null {
  if (isRecord(value)) return value
  if (typeof value !== 'string' || value.trim() === '') return null
  try {
    const parsed = JSON.parse(value)
    return isRecord(parsed) ? parsed : null
  } catch {
    return null
  }
}

function taskProperties(log: TaskLog): GenerationParams | null {
  return parseJSONRecord(log.properties)
}

export function usageGenerationParams(
  other: LogOtherData | null | undefined
): GenerationParams | null {
  if (!other) return null

  const merged: GenerationParams = {
    ...(isRecord(other.image_request) ? other.image_request : {}),
    ...(isRecord(other.image_generation_call_detail)
      ? other.image_generation_call_detail
      : {}),
    ...(isRecord(other.generation_params) ? other.generation_params : {}),
  }

  return Object.keys(merged).length > 0 ? merged : null
}

export function taskGenerationParams(log: TaskLog): GenerationParams | null {
  const props = taskProperties(log)
  const input = props ? parseJSONRecord(props.input) : null
  if (input) return input

  const other = parseJSONRecord(log.other)
  if (other && isRecord(other.generation_params)) return other.generation_params

  return null
}

function paramLabel(key: string): string {
  return PARAM_LABELS[key] ?? key.replaceAll('_', ' ')
}

function formatScalar(value: unknown, t: (key: string) => string): string {
  if (typeof value === 'boolean') return value ? t('Yes') : t('No')
  if (typeof value === 'number') {
    return Number.isInteger(value) ? String(value) : String(value)
  }
  if (typeof value === 'string') return value
  return String(value)
}

function formatMediaValue(value: unknown, t: (key: string) => string): string {
  if (typeof value === 'string') return value
  if (isRecord(value)) {
    if (typeof value.url === 'string') return value.url
    return JSON.stringify(value)
  }
  if (Array.isArray(value)) {
    if (value.length === 0) return ''
    const rendered = value
      .map((item) => formatMediaValue(item, t))
      .filter(Boolean)
    if (rendered.length === 0) return ''
    return rendered.length === 1
      ? rendered[0]
      : `${value.length} ${t('items')}: ${rendered.join(', ')}`
  }
  return ''
}

function formatParamValue(
  key: string,
  value: unknown,
  t: (key: string) => string
): string {
  if (value == null || value === '') return ''
  if (['image', 'images', 'reference_images', 'video'].includes(key)) {
    return formatMediaValue(value, t)
  }
  if (Array.isArray(value)) {
    const rendered = value
      .map((item) => formatParamValue(key, item, t))
      .filter(Boolean)
    return rendered.join(', ')
  }
  if (isRecord(value)) return JSON.stringify(value)
  return formatScalar(value, t)
}

export function buildGenerationParamRows(
  params: GenerationParams | null | undefined,
  t: (key: string) => string
): GenerationParamRow[] {
  if (!params) return []

  const keys = [
    ...PARAM_ORDER.filter((key) => Object.hasOwn(params, key)),
    ...Object.keys(params)
      .filter(
        (key) =>
          !PARAM_ORDER.includes(key) && key !== 'provider' && key !== 'type'
      )
      .sort((a, b) => a.localeCompare(b)),
  ]

  return keys
    .map((key) => {
      const value = formatParamValue(key, params[key], t)
      if (!value) return null
      return {
        key,
        label: t(paramLabel(key)),
        value,
        mono: [
          'model',
          'size',
          'aspect_ratio',
          'resolution',
          'response_format',
        ].includes(key),
      }
    })
    .filter(Boolean) as GenerationParamRow[]
}

export function generationParamsSummary(
  rows: GenerationParamRow[],
  t: (key: string) => string
): string {
  const wanted = rows.filter((row) =>
    ['duration', 'aspect_ratio', 'resolution', 'size', 'quality'].includes(
      row.key
    )
  )
  if (wanted.length > 0) {
    return wanted
      .slice(0, 3)
      .map((row) => `${row.label}: ${row.value}`)
      .join(' · ')
  }
  return rows.length > 0 ? t('View parameters') : ''
}
