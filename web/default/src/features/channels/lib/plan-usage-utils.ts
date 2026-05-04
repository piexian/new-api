import type { Channel } from '../types'

const CHANNEL_TYPE_MINIMAX = 35
const CHANNEL_TYPE_ZHIPU = 16
const CHANNEL_TYPE_ZHIPU_V4 = 26
const CHANNEL_TYPE_MOONSHOT = 25

const ZHIPU_CODING_PLAN_SPECIAL_BASE_URLS = new Set([
  'glm-coding-plan',
  'glm-coding-plan-international',
])

const ZHIPU_CODING_PLAN_DOMAINS = [
  'api.z.ai',
  'open.bigmodel.cn',
  'www.bigmodel.cn',
]

const KIMI_CODING_PLAN_BASE_URL = 'kimi-coding-plan'

export function isKimiCodingPlanChannel(channel: Channel): boolean {
  if (channel.type !== CHANNEL_TYPE_MOONSHOT) {
    return false
  }
  const baseURL = String(channel.base_url || '').trim()
  if (baseURL === KIMI_CODING_PLAN_BASE_URL) {
    return true
  }
  const normalized = baseURL.toLowerCase().replace(/\/+$/, '')
  return normalized.endsWith('/coding')
}

export function isMiniMaxTokenPlanChannel(channel: Channel): boolean {
  return channel.type === CHANNEL_TYPE_MINIMAX
}

export function isZhipuCodingPlanChannel(channel: Channel): boolean {
  if (![CHANNEL_TYPE_ZHIPU, CHANNEL_TYPE_ZHIPU_V4].includes(channel.type)) {
    return false
  }

  const baseURL = String(channel.base_url || '').trim()
  if (ZHIPU_CODING_PLAN_SPECIAL_BASE_URLS.has(baseURL)) {
    return true
  }

  const normalized = baseURL.toLowerCase().replace(/\/+$/, '')
  return ZHIPU_CODING_PLAN_DOMAINS.some((domain) =>
    normalized.includes(domain)
  )
}
