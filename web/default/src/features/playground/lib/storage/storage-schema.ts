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
import { z } from 'zod'

export const STORAGE_VERSION = 1
export const MAX_STORED_MESSAGES = 100
export const MAX_STORED_MESSAGES_BYTES = 1024 * 1024
export const MAX_LOADED_MESSAGES_CHARS = 120_000
export const MAX_LOADED_MESSAGE_CHARS = 40_000

// Classic stored number inputs as strings; an empty input is never zero.
const optionalNumberSchema = z
  .preprocess((value) => {
    if (typeof value !== 'string') return value
    if (value.trim() === '') return null
    return Number(value)
  }, z.number().nullable().optional())
  .catch(undefined)

const optionalBooleanSchema = z.boolean().optional().catch(undefined)

export const playgroundConfigSchema = z.object({
  model: z.string().optional().catch(undefined),
  group: z.string().optional().catch(undefined),
  temperature: optionalNumberSchema,
  top_p: optionalNumberSchema,
  max_tokens: optionalNumberSchema,
  frequency_penalty: optionalNumberSchema,
  presence_penalty: optionalNumberSchema,
  seed: optionalNumberSchema,
  stream: optionalBooleanSchema,
  webSearchEnabled: optionalBooleanSchema,
  codeInterpreterEnabled: optionalBooleanSchema,
  chatInterface: z
    .enum(['openai', 'openai-response', 'anthropic', 'gemini'])
    .optional()
    .catch(undefined),
  reasoningEffort: z
    .enum(['none', 'low', 'medium', 'high', 'max'])
    .optional()
    .catch(undefined),
})

export const parameterEnabledSchema = z.object({
  temperature: optionalBooleanSchema,
  top_p: optionalBooleanSchema,
  max_tokens: optionalBooleanSchema,
  frequency_penalty: optionalBooleanSchema,
  presence_penalty: optionalBooleanSchema,
  seed: optionalBooleanSchema,
})

const messageRoleSchema = z.enum(['user', 'assistant', 'system'])
const messageStatusSchema = z.enum([
  'loading',
  'streaming',
  'complete',
  'error',
])

const messageVersionSchema = z.object({
  id: z.string(),
  content: z.string(),
})

const sourceSchema = z.object({
  href: z.string(),
  title: z.string(),
})

const reasoningSchema = z.object({
  content: z.string(),
  duration: z.number(),
  startedAt: z.number().optional(),
  completedAt: z.number().optional(),
  durationMs: z.number().optional(),
})

const messageSchema = z.object({
  key: z.string(),
  from: messageRoleSchema,
  versions: z.array(messageVersionSchema).min(1),
  createdAt: z.number().optional(),
  startedAt: z.number().optional(),
  completedAt: z.number().optional(),
  durationMs: z.number().optional(),
  sources: z.array(sourceSchema).optional(),
  reasoning: reasoningSchema.optional(),
  isReasoningStreaming: z.boolean().optional(),
  isReasoningComplete: z.boolean().optional(),
  isContentComplete: z.boolean().optional(),
  status: messageStatusSchema.optional(),
  errorCode: z.string().nullable().optional(),
})

export const messagesSchema = z.array(messageSchema)
