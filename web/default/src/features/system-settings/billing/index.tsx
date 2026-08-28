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
import { SettingsPage } from '../components/settings-page'
import type { BillingSettings } from '../types'
import {
  BILLING_DEFAULT_SECTION,
  getBillingSectionContent,
  getBillingSectionMeta,
} from './section-registry.tsx'

const defaultBillingSettings: BillingSettings = {
  QuotaForNewUser: 0,
  PreConsumedQuota: 0,
  QuotaForInviter: 0,
  QuotaForInvitee: 0,
  TopUpLink: '',
  'general_setting.docs_link': '',
  'quota_setting.enable_free_model_pre_consume': true,
  QuotaPerUnit: 500000,
  USDExchangeRate: 7,
  'general_setting.quota_display_type': 'USD',
  'general_setting.custom_currency_symbol': '¤',
  'general_setting.custom_currency_exchange_rate': 1,
  DisplayInCurrencyEnabled: true,
  DisplayTokenStatEnabled: true,
  ModelPrice: '',
  ModelRatio: '',
  CacheRatio: '',
  CreateCacheRatio: '',
  CompletionRatio: '',
  ImageRatio: '',
  AudioRatio: '',
  AudioCompletionRatio: '',
  ExposeRatioEnabled: false,
  'billing_setting.billing_mode': '{}',
  'billing_setting.billing_expr': '{}',
  'tool_price_setting.prices': '{}',
  TopupGroupRatio: '',
  GroupRatio: '',
  UserUsableGroups: '',
  GroupGroupRatio: '',
  AutoGroups: '',
  DefaultUseAutoGroup: false,
  'group_ratio_setting.group_special_usable_group': '{}',
  PayAddress: '',
  EpayId: '',
  EpayKey: '',
  Price: 7.3,
  MinTopUp: 1,
  CustomCallbackAddress: '',
  PayMethods: '',
  'payment_setting.amount_options': '',
  'payment_setting.amount_discount': '',
  'payment_setting.compliance_confirmed': false,
  'payment_setting.compliance_terms_version': '',
  'payment_setting.compliance_confirmed_at': 0,
  'payment_setting.compliance_confirmed_by': 0,
  'payment_setting.compliance_confirmed_ip': '',
  StripeApiSecret: '',
  StripeWebhookSecret: '',
  StripePriceId: '',
  StripeUnitPrice: 8.0,
  StripeMinTopUp: 1,
  StripePromotionCodesEnabled: false,
  CreemApiKey: '',
  CreemWebhookSecret: '',
  CreemTestMode: false,
  CreemProducts: '[]',
  WaffoEnabled: false,
  WaffoApiKey: '',
  WaffoPrivateKey: '',
  WaffoPublicCert: '',
  WaffoSandboxPublicCert: '',
  WaffoSandboxApiKey: '',
  WaffoSandboxPrivateKey: '',
  WaffoSandbox: false,
  WaffoMerchantId: '',
  WaffoCurrency: 'USD',
  WaffoUnitPrice: 1,
  WaffoMinTopUp: 1,
  WaffoNotifyUrl: '',
  WaffoReturnUrl: '',
  WaffoPayMethods: '[]',
  WaffoPancakeMerchantID: '',
  WaffoPancakePrivateKey: '',
  WaffoPancakeReturnURL: '',
  WaffoPancakeStoreID: '',
  WaffoPancakeProductID: '',
  'checkin_setting.enabled': false,
  'checkin_setting.min_quota': 1000,
  'checkin_setting.max_quota': 10000,
  'checkin_setting.special_enabled': false,
  'checkin_setting.special_weekday': '7',
  'checkin_setting.special_quota': 0,
  'checkin_setting.client_check_enabled': false,
  'checkin_setting.decay_enabled': false,
  'checkin_setting.decay_rate': 0.85,
  'checkin_setting.decay_floor': 0,
  'checkin_setting.usage_boost_enabled': false,
  'checkin_setting.usage_boost_days': 3,
  'checkin_setting.high_reward_threshold': 0.8,
  'checkin_setting.base_high_probability': 0.05,
  'checkin_setting.boost_max_probability': 0.8,
  'checkin_setting.makeup_enabled': false,
  'checkin_setting.makeup_max_days': 3,
  'checkin_setting.makeup_counts_toward_progress': true,
  'checkin_setting.makeup_reward_enabled': true,
  'checkin_setting.risk_watch_enabled': false,
  'checkin_setting.risk_watch_days': 14,
  'checkin_setting.risk_min_daily_calls': 1,
  'checkin_setting.risk_min_daily_quota': 100,
  'checkin_setting.expire_enabled': false,
  'checkin_setting.expire_mode': 'unused',
  'loan_setting.enabled': false,
  'loan_setting.max_total': 2500000,
  'loan_setting.daily_rate': 0.001,
  'loan_setting.repay_fee_rate': 0.0001,
  'loan_setting.min_register_days': 0,
  'loan_setting.max_per_borrow': 0,
  'loan_setting.checkin_repay_enabled': true,
  'notify_setting.loan_lender_overflow': true,
  'loan_setting.ai_enabled': false,
  'loan_setting.ai_models': '[]',
  'loan_setting.ai_max_limit': 10000000,
  'loan_setting.ai_min_rate': 0.0005,
  'loan_setting.ai_max_grace_days': 30,
  'loan_setting.ai_max_active_applications': 1,
  'loan_setting.ai_daily_limit': 3,
  'loan_setting.ai_max_rounds': 10,
  'loan_setting.ai_max_output': 2048,
  'loan_setting.ai_prompt': '',
  'loan_setting.credit_tier_limits': '[]',
  'loan_setting.terms_enabled': true,
  'loan_setting.terms_text': '',
}

export function BillingSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/billing/$section'
      defaultSettings={defaultBillingSettings}
      defaultSection={BILLING_DEFAULT_SECTION}
      getSectionContent={getBillingSectionContent}
      getSectionMeta={getBillingSectionMeta}
    />
  )
}
