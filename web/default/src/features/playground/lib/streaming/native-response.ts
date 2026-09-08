import type { ChatCompletionResponse, ChatInterface } from '../../types'
import type { StreamMessageUpdate } from './stream-utils'

type ContentBlock = {
  type?: string
  text?: string
  thinking?: string
  refusal?: string
  thought?: boolean
  id?: string
  name?: string
  executableCode?: { language?: string; code?: string }
  codeExecutionResult?: { output?: string }
}

export type NativeResponse = {
  id?: string
  model?: string
  type?: string
  status?: string
  incomplete_details?: { reason?: string }
  index?: number
  delta?:
    | string
    | { type?: string; text?: string; thinking?: string; partial_json?: string }
  content_block?: ContentBlock
  content?: ContentBlock[]
  output?: Array<{
    type: string
    content?: ContentBlock[]
    summary?: ContentBlock[]
  }>
  candidates?: Array<{
    content?: { parts?: ContentBlock[] }
    finishReason?: string
  }>
  error?: { message?: string }
  response?: {
    error?: { message?: string }
    incomplete_details?: { reason?: string }
  }
  promptFeedback?: { blockReason?: string }
}

function blockUpdates(blocks: ContentBlock[]): StreamMessageUpdate[] {
  return blocks.flatMap((block): StreamMessageUpdate[] => {
    if (block.thinking) return [{ type: 'reasoning', chunk: block.thinking }]
    if (block.text) {
      return [
        { type: block.thought ? 'reasoning' : 'content', chunk: block.text },
      ]
    }
    if (block.refusal) return [{ type: 'content', chunk: block.refusal }]
    if (block.executableCode?.code) {
      return [
        {
          type: 'content',
          chunk: `\n\n\`\`\`${block.executableCode.language?.toLowerCase() ?? ''}\n${block.executableCode.code}\n\`\`\`\n`,
        },
      ]
    }
    if (block.codeExecutionResult?.output) {
      return [
        {
          type: 'content',
          chunk: `\n\n\`\`\`text\n${block.codeExecutionResult.output}\n\`\`\`\n`,
        },
      ]
    }
    return []
  })
}

export const NATIVE_STREAM_EVENTS = [
  'content_block_start',
  'content_block_delta',
  'message_stop',
  'response.output_text.delta',
  'response.reasoning_summary_text.delta',
  'response.refusal.delta',
  'response.completed',
  'response.failed',
  'response.incomplete',
] as const

export function parseNativeStreamResponse(response: NativeResponse): {
  updates: StreamMessageUpdate[]
  done: boolean
  error?: string
} {
  const error =
    response.error?.message ??
    response.response?.error?.message ??
    response.promptFeedback?.blockReason
  if (error) return { updates: [], done: false, error }
  if (
    response.type === 'response.failed' ||
    response.type === 'response.incomplete'
  ) {
    return {
      updates: [],
      done: false,
      error: response.response?.incomplete_details?.reason ?? response.type,
    }
  }
  if (
    response.type === 'message_stop' ||
    response.type === 'response.completed'
  ) {
    return { updates: [], done: true }
  }
  if (response.candidates) {
    const candidate = response.candidates[0]
    return {
      updates: blockUpdates(candidate?.content?.parts ?? []),
      done: Boolean(candidate?.finishReason),
    }
  }
  if (typeof response.delta === 'string') {
    if (
      response.type === 'response.output_text.delta' ||
      response.type === 'response.refusal.delta'
    ) {
      return {
        updates: [{ type: 'content', chunk: response.delta }],
        done: false,
      }
    }
    if (response.type === 'response.reasoning_summary_text.delta') {
      return {
        updates: [{ type: 'reasoning', chunk: response.delta }],
        done: false,
      }
    }
  }
  if (response.type === 'content_block_start' && response.content_block) {
    return { updates: blockUpdates([response.content_block]), done: false }
  }
  if (
    response.type === 'content_block_delta' &&
    typeof response.delta === 'object'
  ) {
    return { updates: blockUpdates([response.delta]), done: false }
  }
  return { updates: [], done: false }
}

export function normalizeNativeResponse(
  response: NativeResponse,
  format: ChatInterface
): ChatCompletionResponse {
  const error = response.error?.message ?? response.promptFeedback?.blockReason
  if (error) throw new Error(error)
  if (response.status === 'failed' || response.status === 'incomplete') {
    throw new Error(response.incomplete_details?.reason ?? response.status)
  }
  let updates: StreamMessageUpdate[] = []
  if (format === 'anthropic') updates = blockUpdates(response.content ?? [])
  if (format === 'gemini') {
    updates = blockUpdates(response.candidates?.[0]?.content?.parts ?? [])
  }
  if (format === 'openai-response') {
    updates = (response.output ?? []).flatMap((item) => {
      if (item.type === 'reasoning') {
        return (item.summary ?? []).flatMap((part): StreamMessageUpdate[] =>
          part.text ? [{ type: 'reasoning', chunk: part.text }] : []
        )
      }
      return blockUpdates(item.content ?? [])
    })
  }
  return {
    id: response.id ?? '',
    model: response.model ?? '',
    object: 'chat.completion',
    created: 0,
    choices: [
      {
        index: 0,
        finish_reason: 'stop',
        message: {
          role: 'assistant',
          content: updates
            .filter((update) => update.type === 'content')
            .map((update) => update.chunk)
            .join(''),
          reasoning_content: updates
            .filter((update) => update.type === 'reasoning')
            .map((update) => update.chunk)
            .join(''),
        },
      },
    ],
  }
}
