import type { TFunction } from 'i18next'
import type { SubscriptionPlan, UserSubscription } from '../types'

export function parseAllowedModels(value?: string): string[] {
  if (!value || value.trim() === '') return []
  try {
    const parsed = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed.map((item) => String(item || '').trim()).filter(Boolean)
  } catch {
    return []
  }
}

export function getModelRestrictionMeta(
  plan: Partial<SubscriptionPlan> | null | undefined,
  t: TFunction
): { label: string; tooltip?: string } | null {
  const mode = plan?.model_restrict_mode || ''
  if (!mode) return null

  if (mode === 'group') {
    const restrictGroup = String(plan?.model_restrict_group || '').trim()
    const upgradeGroup = String(plan?.upgrade_group || '').trim()
    const displayGroup = restrictGroup || upgradeGroup
    return {
      label: displayGroup
        ? `${t('Model Restriction')}: ${t('Restrict by model group')} (${displayGroup})`
        : `${t('Model Restriction')}: ${t('Restrict by model group')}`,
      tooltip: restrictGroup
        ? t('Only models in the selected restriction group are allowed.')
        : upgradeGroup
          ? t('When no restriction group is selected, the upgrade group is used.')
          : t(
              'When empty, the upgrade group is used first, otherwise the current user group is used.'
            ),
    }
  }

  const allowedModels = parseAllowedModels(plan?.allowed_models)
  return {
    label: `${t('Model Restriction')}: ${t('Restrict by custom models')}${
      allowedModels.length > 0 ? ` (${allowedModels.length})` : ''
    }`,
    tooltip:
      allowedModels.length > 0
        ? allowedModels.join(', ')
        : t('No allowed models configured'),
  }
}

export function getQuotaWindowItems(
  plan: Partial<SubscriptionPlan> | null | undefined,
  t: TFunction,
  formatQuota: (quota: number) => string,
  subscription?: Partial<UserSubscription> | null
): { label: string }[] {
  const definitions = [
    {
      label: t('Daily Quota Limit'),
      limit: Number(plan?.daily_quota_limit || 0),
      used: Number(subscription?.daily_window_used || 0),
    },
    {
      label: t('Weekly Quota Limit'),
      limit: Number(plan?.weekly_quota_limit || 0),
      used: Number(subscription?.weekly_window_used || 0),
    },
    {
      label: t('Monthly Quota Limit'),
      limit: Number(plan?.monthly_quota_limit || 0),
      used: Number(subscription?.monthly_window_used || 0),
    },
  ]

  return definitions
    .filter((item) => item.limit > 0)
    .map((item) => ({
      label: subscription
        ? `${item.label}: ${formatQuota(item.used)}/${formatQuota(item.limit)}`
        : `${item.label}: ${formatQuota(item.limit)}`,
    }))
}
