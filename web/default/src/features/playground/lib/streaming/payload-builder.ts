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
import type {
  ChatCompletionRequest,
  Message,
  PlaygroundConfig,
  ParameterEnabled,
} from '../../types'
import { formatMessageForAPI, isValidMessage } from '../message/message-utils'
import {
  isParameterNumber,
  PLAYGROUND_PARAMETER_CONTROLS,
} from '../parameters/playground-parameters'

/**
 * Build API request payload from messages and config
 */
export function buildChatCompletionPayload(
  messages: Message[],
  config: PlaygroundConfig,
  parameterEnabled: ParameterEnabled
): ChatCompletionRequest {
  // Filter and format valid messages
  const processedMessages = messages
    .filter(isValidMessage)
    .map(formatMessageForAPI)

  const payload: ChatCompletionRequest = {
    model: config.model,
    group: config.group,
    messages: processedMessages,
    stream: config.stream,
  }

  for (const { key } of PLAYGROUND_PARAMETER_CONTROLS) {
    const value = config[key]
    if (parameterEnabled[key] && isParameterNumber(value)) {
      payload[key] = value
    }
  }

  // 思考等级
  if (config.reasoningEffort && config.reasoningEffort !== 'none') {
    payload.reasoning_effort = config.reasoningEffort
  }

  if (config.webSearchEnabled && config.chatInterface === 'openai') {
    payload.web_search_options = {}
  }

  return payload
}
