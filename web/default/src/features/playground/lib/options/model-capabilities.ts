import { CHAT_CAPABLE_ENDPOINTS, type ModelOption, type ModelsDevEntry } from '../../types'

/**
 * 判断模型是否支持对话（按 endpoint types 优先，models.dev 兜底）
 * 规则：
 * 1. 若模型有 supportedEndpointTypes（来自模型管理配置），检查是否含对话类 endpoint
 * 2. 若无 endpoint types 配置，用 models.dev catalog 的 modalities 兜底判断
 * 3. 若 models.dev 也无记录，默认认为可对话（不误杀）
 */
export function isChatCapableModel(
  model: ModelOption,
  catalog?: Record<string, ModelsDevEntry>
): boolean {
  // 优先用 endpoint types
  if (model.supportedEndpointTypes && model.supportedEndpointTypes.length > 0) {
    return model.supportedEndpointTypes.some((et) =>
      CHAT_CAPABLE_ENDPOINTS.has(et)
    )
  }

  // 兜底：models.dev catalog
  if (catalog) {
    const entry = catalog[model.value] || catalog[model.label]
    if (entry) {
      const inputMods = entry.modalities?.input ?? []
      const outputMods = entry.modalities?.output ?? []
      const allMods = new Set([...inputMods, ...outputMods])
      // 纯 embed/rerank/audio-only 模型不支持对话
      // 含 text modality 的模型默认可对话
      if (allMods.size === 0) return true // 未知则不过滤
      return allMods.has('text')
    }
  }

  // 无任何信息，默认可对话（不误杀）
  return true
}

/**
 * 过滤模型列表，只保留支持对话的模型
 */
export function filterChatCapableModels(
  models: ModelOption[],
  catalog?: Record<string, ModelsDevEntry>
): ModelOption[] {
  if (!catalog) return models // catalog 未加载时不过滤
  return models.filter((m) => isChatCapableModel(m, catalog))
}


/**
 * 从 models.dev catalog 获取模型能力信息
 */
export function getModelCapabilities(
  modelValue: string,
  catalog?: Record<string, ModelsDevEntry>
): ModelsDevEntry | undefined {
  if (!catalog) return undefined
  return catalog[modelValue]
}

/**
 * 判断模型是否支持图片生成
 */
export function isImageCapableModel(
  model: ModelOption,
  catalog?: Record<string, ModelsDevEntry>
): boolean {
  if (model.supportedEndpointTypes && model.supportedEndpointTypes.length > 0) {
    return model.supportedEndpointTypes.some((et) =>
      et.includes('image')
    )
  }
  if (catalog) {
    const entry = catalog[model.value] || catalog[model.label]
    if (entry) {
      const outputMods = entry.modalities?.output ?? []
      return outputMods.includes('image')
    }
  }
  return true
}

/**
 * 过滤模型列表，只保留支持图片生成的模型
 */
export function filterImageCapableModels(
  models: ModelOption[],
  catalog?: Record<string, ModelsDevEntry>
): ModelOption[] {
  if (!catalog) return models
  return models.filter((m) => isImageCapableModel(m, catalog))
}

/**
 * 判断模型是否支持视频生成
 */
export function isVideoCapableModel(
  model: ModelOption,
  catalog?: Record<string, ModelsDevEntry>
): boolean {
  if (model.supportedEndpointTypes && model.supportedEndpointTypes.length > 0) {
    return model.supportedEndpointTypes.some((et) =>
      et.includes('video')
    )
  }
  if (catalog) {
    const entry = catalog[model.value] || catalog[model.label]
    if (entry) {
      const outputMods = entry.modalities?.output ?? []
      return outputMods.includes('video')
    }
  }
  return true
}

/**
 * 过滤模型列表，只保留支持视频生成的模型
 */
export function filterVideoCapableModels(
  models: ModelOption[],
  catalog?: Record<string, ModelsDevEntry>
): ModelOption[] {
  if (!catalog) return models
  return models.filter((m) => isVideoCapableModel(m, catalog))
}

/**
 * 判断模型是否支持音频（TTS/转写）
 */
export function isAudioCapableModel(
  model: ModelOption,
  catalog?: Record<string, ModelsDevEntry>
): boolean {
  if (model.supportedEndpointTypes && model.supportedEndpointTypes.length > 0) {
    return model.supportedEndpointTypes.some((et) =>
      et.includes('audio')
    )
  }
  if (catalog) {
    const entry = catalog[model.value] || catalog[model.label]
    if (entry) {
      const inputMods = entry.modalities?.input ?? []
      const outputMods = entry.modalities?.output ?? []
      const allMods = new Set([...inputMods, ...outputMods])
      return allMods.has('audio')
    }
  }
  return true
}

/**
 * 过滤模型列表，只保留支持音频的模型
 */
export function filterAudioCapableModels(
  models: ModelOption[],
  catalog?: Record<string, ModelsDevEntry>
): ModelOption[] {
  if (!catalog) return models
  return models.filter((m) => isAudioCapableModel(m, catalog))
}

/**
 * 判断模型是否支持思考（reasoning）
 */
export function isReasoningModel(
  modelValue: string,
  catalog?: Record<string, ModelsDevEntry>
): boolean {
  const entry = getModelCapabilities(modelValue, catalog)
  if (entry) return entry.reasoning
  return false
}

/**
 * 判断模型是否支持工具调用
 */
export function isToolCapableModel(
  modelValue: string,
  catalog?: Record<string, ModelsDevEntry>
): boolean {
  const entry = getModelCapabilities(modelValue, catalog)
  if (entry) return entry.tool_call
  return false
}
