import type {
  ChatCompletionRequest,
  ChatInterface,
  PlaygroundConfig,
} from '../../types'

export function supportsCodeInterpreter(chatInterface: ChatInterface): boolean {
  return chatInterface !== 'openai'
}

export function buildNativeRequest(
  chat: ChatCompletionRequest,
  config: PlaygroundConfig
): {
  endpoint: string
  payload: ChatCompletionRequest | Record<string, unknown>
} {
  const base = { model: chat.model, group: chat.group, stream: chat.stream }
  const messages = chat.messages.filter((message) => message.role !== 'system')
  const system = chat.messages
    .filter((message) => message.role === 'system')
    .map((message) => message.content)
    .join('\n')
  if (config.chatInterface === 'openai-response') {
    const tools: Record<string, unknown>[] = []
    if (config.webSearchEnabled) tools.push({ type: 'web_search' })
    if (config.codeInterpreterEnabled) {
      tools.push({ type: 'code_interpreter', container: { type: 'auto' } })
    }
    return {
      endpoint: '/pg/responses',
      payload: {
        ...base,
        input: chat.messages.map((message) => ({
          role: message.role,
          content:
            typeof message.content === 'string'
              ? message.content
              : message.content.map((part) =>
                  part.type === 'text'
                    ? {
                        type:
                          message.role === 'assistant'
                            ? 'output_text'
                            : 'input_text',
                        text: part.text,
                      }
                    : { type: 'input_image', image_url: part.image_url?.url }
                ),
        })),
        temperature: chat.temperature,
        top_p: chat.top_p,
        max_output_tokens: chat.max_tokens,
        reasoning: chat.reasoning_effort
          ? { effort: chat.reasoning_effort }
          : undefined,
        tools: tools.length ? tools : undefined,
      },
    }
  }
  if (config.chatInterface === 'anthropic') {
    const tools: Record<string, unknown>[] = []
    if (config.webSearchEnabled) {
      tools.push({ type: 'web_search_20250305', name: 'web_search' })
    }
    if (config.codeInterpreterEnabled) {
      tools.push({ type: 'code_execution_20250825', name: 'code_execution' })
    }
    return {
      endpoint: '/pg/messages',
      payload: {
        ...base,
        system: system || undefined,
        messages: messages.map((message) => ({
          ...message,
          content:
            typeof message.content === 'string'
              ? message.content
              : message.content.map((part) => {
                  if (part.type === 'text') {
                    return { type: 'text', text: part.text }
                  }
                  const url = part.image_url?.url ?? ''
                  const inline = /^data:([^;]+);base64,(.*)$/.exec(url)
                  return {
                    type: 'image',
                    source: inline
                      ? {
                          type: 'base64',
                          media_type: inline[1],
                          data: inline[2],
                        }
                      : { type: 'url', url },
                  }
                }),
        })),
        max_tokens: chat.max_tokens ?? config.max_tokens,
        temperature: chat.temperature,
        top_p: chat.top_p,
        output_config: chat.reasoning_effort
          ? { effort: chat.reasoning_effort }
          : undefined,
        tools: tools.length ? tools : undefined,
      },
    }
  }
  if (config.chatInterface === 'gemini') {
    const tools: Record<string, unknown>[] = []
    if (config.webSearchEnabled) tools.push({ googleSearch: {} })
    if (config.codeInterpreterEnabled) tools.push({ codeExecution: {} })
    const action = chat.stream
      ? 'streamGenerateContent?alt=sse'
      : 'generateContent'
    return {
      endpoint: `/pg/v1beta/models/${encodeURIComponent(chat.model)}:${action}`,
      payload: {
        model: chat.model,
        group: chat.group,
        contents: messages.map((message) => ({
          role: message.role === 'assistant' ? 'model' : 'user',
          parts:
            typeof message.content === 'string'
              ? [{ text: message.content }]
              : message.content.map((part) => {
                  if (part.type === 'text') return { text: part.text }
                  const url = part.image_url?.url ?? ''
                  const inline = /^data:([^;]+);base64,(.*)$/.exec(url)
                  return inline
                    ? { inlineData: { mimeType: inline[1], data: inline[2] } }
                    : { fileData: { fileUri: url } }
                }),
        })),
        systemInstruction: system ? { parts: [{ text: system }] } : undefined,
        generationConfig: {
          temperature: chat.temperature,
          topP: chat.top_p,
          maxOutputTokens: chat.max_tokens,
          presencePenalty: chat.presence_penalty,
          frequencyPenalty: chat.frequency_penalty,
          seed: chat.seed,
        },
        tools: tools.length ? tools : undefined,
      },
    }
  }
  return { endpoint: '/pg/chat/completions', payload: chat }
}
