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
import { useCallback, useRef, useState } from 'react'
import { SSE } from 'sse.js'

import { getCommonHeaders } from '@/lib/api'

import { API_ENDPOINTS, ERROR_MESSAGES } from '../constants'
import {
  getStreamReadyStateError,
  isStreamClosedReadyState,
  isStreamDoneMessage,
  parseStreamErrorDetails,
  parseStreamMessageUpdates,
} from '../lib'
import {
  NATIVE_STREAM_EVENTS,
  parseNativeStreamResponse,
} from '../lib/streaming/native-response'
import type { ChatCompletionRequest, ChatInterface } from '../types'

/**
 * Hook for handling streaming chat completion requests
 */
export function useStreamRequest() {
  const sseSourceRef = useRef<SSE | null>(null)
  const isStreamCompleteRef = useRef(false)
  const [isStreaming, setIsStreaming] = useState(false)

  const closeActiveStream = useCallback((source?: SSE) => {
    const streamSource = source ?? sseSourceRef.current
    streamSource?.close()

    if (!source || sseSourceRef.current === source) {
      sseSourceRef.current = null
      setIsStreaming(false)
    }
  }, [])

  const sendStreamRequest = useCallback(
    (
      payload: ChatCompletionRequest | Record<string, unknown>,
      onUpdate: (
        type: 'reasoning' | 'content' | 'tool_calls',
        chunk: string
      ) => void,
      onComplete: () => void,
      onError: (error: string, errorCode?: string) => void,
      endpoint?: string,
      format: ChatInterface = 'openai'
    ) => {
      sseSourceRef.current?.close()

      const url = endpoint ?? API_ENDPOINTS.CHAT_COMPLETIONS
      const source = new SSE(url, {
        headers: getCommonHeaders(),
        method: 'POST',
        payload: JSON.stringify(payload),
      })

      sseSourceRef.current = source
      isStreamCompleteRef.current = false
      setIsStreaming(true)

      const handleError = (errorMessage: string, errorCode?: string) => {
        if (!isStreamCompleteRef.current) {
          onError(errorMessage, errorCode)
          closeActiveStream(source)
        }
      }

      const handleMessage = (e: MessageEvent) => {
        if (isStreamCompleteRef.current) return
        if (isStreamDoneMessage(e.data)) {
          isStreamCompleteRef.current = true
          closeActiveStream(source)
          onComplete()
          return
        }

        try {
          if (format !== 'openai') {
            const result = parseNativeStreamResponse(JSON.parse(e.data))
            if (result.error) {
              handleError(result.error)
              return
            }
            for (const update of result.updates) {
              onUpdate(update.type, update.chunk)
            }
            if (result.done) {
              isStreamCompleteRef.current = true
              closeActiveStream(source)
              onComplete()
            }
            return
          }
          const updates = parseStreamMessageUpdates(e.data)

          for (const update of updates) {
            onUpdate(update.type, update.chunk)
          }
        } catch {
          handleError(ERROR_MESSAGES.PARSE_ERROR)
        }
      }
      source.addEventListener('message', handleMessage)
      if (format !== 'openai') {
        for (const event of NATIVE_STREAM_EVENTS) {
          source.addEventListener(event, handleMessage)
        }
      }

      source.addEventListener('error', (e: Event & { data?: string }) => {
        // Only handle errors if stream didn't complete normally
        if (!isStreamClosedReadyState(source.readyState)) {
          const { errorCode, errorMessage } = parseStreamErrorDetails(e.data)
          handleError(errorMessage, errorCode)
        }
      })

      source.addEventListener(
        'readystatechange',
        (e: Event & { readyState?: number }) => {
          const errorMessage = getStreamReadyStateError(e.readyState, source)

          if (errorMessage) {
            handleError(errorMessage)
          }
        }
      )

      try {
        source.stream()
      } catch {
        onError(ERROR_MESSAGES.STREAM_START_ERROR)
        closeActiveStream(source)
      }
    },
    [closeActiveStream]
  )

  const stopStream = useCallback(() => {
    closeActiveStream()
  }, [closeActiveStream])

  return {
    sendStreamRequest,
    stopStream,
    isStreaming,
  }
}
