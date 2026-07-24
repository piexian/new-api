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
import { api } from '@/lib/api'

// ============================================================================
// Types
// ============================================================================

export type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type PageData<T> = {
  page: number
  page_size: number
  total: number
  items: T[]
}

export type RiskLiveSource = 'probe_guard' | 'error_ban'
export type RiskLiveDimension = 'ip' | 'user' | 'both'
export type RiskLiveStatus =
  | 'observing'
  | 'near_threshold'
  | 'threshold_reached'

export type RiskLiveRuleSummary = {
  source: RiskLiveSource
  rule_id: string
  rule_name: string
  enabled: boolean
  parent_enabled: boolean
  system: boolean
  dry_run: boolean
  dimension: RiskLiveDimension
  threshold: number
  window_seconds: number
  active_targets: number
  near_threshold_targets: number
  triggered_targets: number
  max_progress_percent: number
  last_seen_at: number
}

export type RiskLiveTarget = {
  id: string
  source: RiskLiveSource
  rule_id: string
  rule_name: string
  dimension: Exclude<RiskLiveDimension, 'both'>
  target: string
  user_id: number
  username: string
  context: string
  current_count: number
  threshold: number
  progress_percent: number
  window_seconds: number
  remaining_seconds: number
  last_seen_at: number
  expires_at: number
  status: RiskLiveStatus
  members: string[]
}

// Probe Guard types
export type ProbeGuardBanDimension = 'ip' | 'user' | 'both'

export type ProbeGuardConfig = {
  enabled: boolean
  dry_run: boolean
  window_seconds: number
  distinct_model_count: number
  ban_dimension: ProbeGuardBanDimension
  first_ip_ban_minutes: number
  second_ip_ban_minutes: number
  permanent_offense_count: number
  offense_dedupe_seconds: number
  whitelist_user_ids: string
  whitelist_groups: string[]
  user_ban_reason: string
  notify_user_enabled: boolean
  notify_admin_enabled: boolean
  appeal_hint: string
}

function normalizeProbeGuardConfig(data: ProbeGuardConfig): ProbeGuardConfig {
  return {
    ...data,
    ban_dimension: data.ban_dimension || 'ip',
    whitelist_groups: data.whitelist_groups ?? [],
  }
}

export type ProbeIPOffense = {
  id: number
  target_ip: string
  last_user_id: number
  offense_count: number
  last_offense_at: number
  last_models: string
  created_at: number
  updated_at: number
}

export type ProbeUserOffense = {
  id: number
  user_id: number
  offense_count: number
  last_offense_at: number
  last_ip: string
  last_models: string
  created_at: number
  updated_at: number
}

export type ProbeGuardStats = {
  total_ip_states: number
  total_user_states: number
  total_offenses: number
  recent_offenses: number
}

// Error Ban types
export type ErrorBanRule = {
  id: string
  name: string
  pattern: string
  keywords: string[]
  error_codes: string[]
  enabled: boolean
  count_retries: boolean
  dimension: '' | 'ip' | 'user'
  threshold: number
  reason_template: string
  tiers: ErrorBanTier[]
}

export type ErrorBanTier = {
  offense_count: number
  action: 'temp_ip_ban' | 'perm_ip_ban' | 'disable_user' | 'both'
  duration_minutes: number
  reason_suffix: string
}

export type ErrorBanConfig = {
  enabled: boolean
  dry_run: boolean
  window_seconds: number
  default_dimension: 'ip' | 'user'
  default_reason_template: string
  notify_user_enabled: boolean
  notify_admin_enabled: boolean
  appeal_hint: string
  whitelist_user_ids: string
  whitelist_groups: string[]
  exclude_status_codes: number[]
  rules: ErrorBanRule[]
  tiers: ErrorBanTier[]
}

export type ErrorBanIPState = {
  id: number
  target_ip: string
  rule_id: string
  offense_count: number
  window_count: number
  window_start: number
  last_error: string
  last_offense_at: number
  created_at: number
  updated_at: number
}

export type ErrorBanUserState = {
  id: number
  user_id: number
  rule_id: string
  offense_count: number
  window_count: number
  window_start: number
  last_error: string
  last_offense_at: number
  created_at: number
  updated_at: number
}

export type ErrorBanStats = {
  total_ip_states: number
  total_user_states: number
  total_offenses: number
  active_rules: number
}

export type RuleTestResult = {
  valid: boolean
  matched: boolean
  error?: string
}

// Ban Log types
export type RiskBanLog = {
  id: number
  dimension: 'ip' | 'user'
  target_ip: string
  user_id: number
  username: string
  source: 'probe_guard' | 'error_ban' | 'ip_middleware' | 'manual'
  rule_id: string
  rule_name: string
  action: 'temp_ip_ban' | 'perm_ip_ban' | 'disable_user' | 'both'
  duration_minutes: number
  is_permanent: boolean
  unban_at: number
  offense_count: number
  reason: string
  error_sample: string
  models: string
  operator_id: number
  dry_run: boolean
  created_at: number
}

export type BanLogStats = {
  total: number
  dry_run_count: number
  permanent: number
  today: number
  by_dimension: Record<string, number>
  by_source: Record<string, number>
}

export type BanLogFilters = {
  p?: number
  page_size?: number
  dimension?: string
  source?: string
  keyword?: string
  dry_run?: string
  start_at?: number
  end_at?: number
}

const skipBusinessErrorConfig = {
  skipBusinessError: true,
} as Record<string, unknown>

function normalizePageResponse<T>(
  response: ApiResponse<PageData<T>>,
  normalizeItem: (item: T) => T
): ApiResponse<PageData<T>> {
  if (!response.data) return response

  return {
    ...response,
    data: {
      ...response.data,
      items: (response.data.items ?? []).map(normalizeItem),
    },
  }
}

function normalizeProbeIPOffense(item: ProbeIPOffense): ProbeIPOffense {
  return {
    ...item,
    target_ip: item.target_ip ?? '',
    last_models: item.last_models ?? '',
  }
}

function normalizeProbeUserOffense(item: ProbeUserOffense): ProbeUserOffense {
  return {
    ...item,
    last_ip: item.last_ip ?? '',
    last_models: item.last_models ?? '',
  }
}

function normalizeErrorBanIPState(item: ErrorBanIPState): ErrorBanIPState {
  return {
    ...item,
    target_ip: item.target_ip ?? '',
    rule_id: item.rule_id ?? '',
    last_error: item.last_error ?? '',
  }
}

function normalizeErrorBanUserState(
  item: ErrorBanUserState
): ErrorBanUserState {
  return {
    ...item,
    rule_id: item.rule_id ?? '',
    last_error: item.last_error ?? '',
  }
}

function normalizeRiskBanLog(log: RiskBanLog): RiskBanLog {
  return {
    ...log,
    target_ip: log.target_ip ?? '',
    username: log.username ?? '',
    rule_id: log.rule_id ?? '',
    rule_name: log.rule_name ?? '',
    reason: log.reason ?? '',
    error_sample: log.error_sample ?? '',
    models: log.models ?? '',
  }
}

function normalizeRiskLiveTarget(target: RiskLiveTarget): RiskLiveTarget {
  return {
    ...target,
    rule_name: target.rule_name ?? '',
    target: target.target ?? '',
    username: target.username ?? '',
    context: target.context ?? '',
    members: target.members ?? [],
  }
}

// ============================================================================
// Live Progress API
// ============================================================================

export async function getRiskLiveRules(): Promise<
  ApiResponse<RiskLiveRuleSummary[]>
> {
  const res = await api.get('/api/risk/live-progress/rules')
  const response = res.data as ApiResponse<RiskLiveRuleSummary[]>
  return {
    ...response,
    data: response.data ?? [],
  }
}

export async function getRiskLiveTargets(params: {
  source: RiskLiveSource
  rule_id: string
  dimension?: Exclude<RiskLiveDimension, 'both'> | ''
  p?: number
  page_size?: number
  keyword?: string
}): Promise<ApiResponse<PageData<RiskLiveTarget>>> {
  const res = await api.get('/api/risk/live-progress/targets', {
    params: {
      ...params,
      dimension: params.dimension || undefined,
      keyword: params.keyword || undefined,
    },
  })
  return normalizePageResponse(res.data, normalizeRiskLiveTarget)
}

export async function toggleRiskLiveRule(data: {
  source: RiskLiveSource
  rule_id: string
  enabled: boolean
}): Promise<ApiResponse<typeof data>> {
  const res = await api.patch(
    '/api/risk/live-progress/rules/enabled',
    data,
    skipBusinessErrorConfig
  )
  return res.data
}

// ============================================================================
// Probe Guard API
// ============================================================================

export async function getProbeGuardConfig(): Promise<
  ApiResponse<ProbeGuardConfig>
> {
  const res = await api.get('/api/risk/probe-guard/config')
  const response = res.data as ApiResponse<ProbeGuardConfig>
  return response.data
    ? {
        ...response,
        data: normalizeProbeGuardConfig(response.data),
      }
    : response
}

export async function updateProbeGuardConfig(
  data: ProbeGuardConfig
): Promise<ApiResponse<ProbeGuardConfig>> {
  const res = await api.put(
    '/api/risk/probe-guard/config',
    data,
    skipBusinessErrorConfig
  )
  const response = res.data as ApiResponse<ProbeGuardConfig>
  return response.data
    ? {
        ...response,
        data: normalizeProbeGuardConfig(response.data),
      }
    : response
}

export async function getProbeGuardIPOffenses(
  params: { p?: number; page_size?: number; keyword?: string } = {}
): Promise<ApiResponse<PageData<ProbeIPOffense>>> {
  const { p = 1, page_size = 10, keyword = '' } = params
  const res = await api.get('/api/risk/probe-guard/ip-offenses', {
    params: { p, page_size, keyword: keyword || undefined },
  })
  return normalizePageResponse(res.data, normalizeProbeIPOffense)
}

export async function getProbeGuardUserOffenses(
  params: { p?: number; page_size?: number; keyword?: string } = {}
): Promise<ApiResponse<PageData<ProbeUserOffense>>> {
  const { p = 1, page_size = 10, keyword = '' } = params
  const res = await api.get('/api/risk/probe-guard/user-offenses', {
    params: { p, page_size, keyword: keyword || undefined },
  })
  return normalizePageResponse(res.data, normalizeProbeUserOffense)
}

export async function resetProbeGuardIPOffense(
  ip: string
): Promise<ApiResponse<null>> {
  const res = await api.post(`/api/risk/probe-guard/ip-offenses/${ip}/reset`)
  return res.data
}

export async function unbanProbeGuardUser(
  id: number
): Promise<ApiResponse<null>> {
  const res = await api.post(`/api/risk/probe-guard/user-offenses/${id}/unban`)
  return res.data
}

export async function getProbeGuardStats(): Promise<
  ApiResponse<ProbeGuardStats>
> {
  const res = await api.get('/api/risk/probe-guard/stats')
  return res.data
}

// ============================================================================
// Error Ban API
// ============================================================================

function normalizeErrorBanConfig(data: ErrorBanConfig): ErrorBanConfig {
  const legacyTiers = data.tiers ?? []
  return {
    ...data,
    whitelist_groups: data.whitelist_groups ?? [],
    exclude_status_codes: data.exclude_status_codes ?? [],
    rules: (data.rules ?? []).map((rule) => ({
      ...rule,
      name: rule.name ?? '',
      pattern: rule.pattern ?? '',
      keywords: rule.keywords ?? [],
      error_codes: rule.error_codes ?? [],
      count_retries: rule.count_retries ?? false,
      reason_template: rule.reason_template ?? '',
      tiers: (rule.tiers?.length ? rule.tiers : legacyTiers).map((tier) => ({
        ...tier,
        reason_suffix: tier.reason_suffix ?? '',
      })),
    })),
    tiers: legacyTiers,
  }
}

export async function getErrorBanConfig(): Promise<
  ApiResponse<ErrorBanConfig>
> {
  const res = await api.get('/api/risk/error-ban/config')
  const response = res.data as ApiResponse<ErrorBanConfig>
  return response.data
    ? { ...response, data: normalizeErrorBanConfig(response.data) }
    : response
}

export async function updateErrorBanConfig(
  data: ErrorBanConfig
): Promise<ApiResponse<ErrorBanConfig>> {
  const res = await api.put(
    '/api/risk/error-ban/config',
    data,
    skipBusinessErrorConfig
  )
  const response = res.data as ApiResponse<ErrorBanConfig>
  return response.data
    ? { ...response, data: normalizeErrorBanConfig(response.data) }
    : response
}

export async function testErrorBanRule(data: {
  pattern: string
  keywords: string[]
  error_codes: string[]
  sample_text: string
  error_code: string
}): Promise<ApiResponse<RuleTestResult>> {
  const res = await api.post(
    '/api/risk/error-ban/rules/test',
    data,
    skipBusinessErrorConfig
  )
  return res.data
}

export async function getErrorBanIPStates(
  params: { p?: number; page_size?: number; keyword?: string } = {}
): Promise<ApiResponse<PageData<ErrorBanIPState>>> {
  const { p = 1, page_size = 10, keyword = '' } = params
  const res = await api.get('/api/risk/error-ban/ip-states', {
    params: { p, page_size, keyword: keyword || undefined },
  })
  return normalizePageResponse(res.data, normalizeErrorBanIPState)
}

export async function getErrorBanUserStates(
  params: { p?: number; page_size?: number; keyword?: string } = {}
): Promise<ApiResponse<PageData<ErrorBanUserState>>> {
  const { p = 1, page_size = 10, keyword = '' } = params
  const res = await api.get('/api/risk/error-ban/user-states', {
    params: { p, page_size, keyword: keyword || undefined },
  })
  return normalizePageResponse(res.data, normalizeErrorBanUserState)
}

export async function resetErrorBanIPState(
  ip: string
): Promise<ApiResponse<null>> {
  const res = await api.post(`/api/risk/error-ban/ip-states/${ip}/reset`)
  return res.data
}

export async function resetErrorBanUserState(
  id: number
): Promise<ApiResponse<null>> {
  const res = await api.post(`/api/risk/error-ban/user-states/${id}/reset`)
  return res.data
}

export async function getErrorBanStats(): Promise<ApiResponse<ErrorBanStats>> {
  const res = await api.get('/api/risk/error-ban/stats')
  return res.data
}

// ============================================================================
// Ban Logs API
// ============================================================================

export async function getRiskBanLogs(
  filters: BanLogFilters = {}
): Promise<ApiResponse<PageData<RiskBanLog>>> {
  const {
    p = 1,
    page_size = 10,
    dimension = '',
    source = '',
    keyword = '',
    dry_run = '',
    start_at,
    end_at,
  } = filters
  const params: Record<string, string | number | undefined> = {
    p,
    page_size,
  }
  if (dimension) params.dimension = dimension
  if (source) params.source = source
  if (keyword) params.keyword = keyword
  if (dry_run) params.dry_run = dry_run
  if (start_at) params.start_at = start_at
  if (end_at) params.end_at = end_at
  const res = await api.get('/api/risk/ban-logs', { params })
  return normalizePageResponse(res.data, normalizeRiskBanLog)
}

export async function getRiskBanLog(
  id: number
): Promise<ApiResponse<RiskBanLog>> {
  const res = await api.get(`/api/risk/ban-logs/${id}`)
  const response = res.data as ApiResponse<RiskBanLog>
  return response.data
    ? { ...response, data: normalizeRiskBanLog(response.data) }
    : response
}

export async function getRiskBanLogStats(): Promise<ApiResponse<BanLogStats>> {
  const res = await api.get('/api/risk/ban-logs/stats')
  const response = res.data as ApiResponse<BanLogStats>
  return response.data
    ? {
        ...response,
        data: {
          ...response.data,
          by_dimension: response.data.by_dimension ?? {},
          by_source: response.data.by_source ?? {},
        },
      }
    : response
}
