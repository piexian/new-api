/*
Copyright (C) 2025 QuantumNous

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

import React, { useEffect, useState } from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import SettingsGeneral from '../../pages/Setting/Operation/SettingsGeneral';
import SettingsHeaderNavModules from '../../pages/Setting/Operation/SettingsHeaderNavModules';
import SettingsSidebarModulesAdmin from '../../pages/Setting/Operation/SettingsSidebarModulesAdmin';
import SettingsSensitiveWords from '../../pages/Setting/Operation/SettingsSensitiveWords';
import SettingsLog from '../../pages/Setting/Operation/SettingsLog';
import SettingsMonitoring from '../../pages/Setting/Operation/SettingsMonitoring';
import SettingsCreditLimit from '../../pages/Setting/Operation/SettingsCreditLimit';
import SettingsCheckin from '../../pages/Setting/Operation/SettingsCheckin';
import SettingsLoan from '../../pages/Setting/Operation/SettingsLoan';
import { API, showError, toBoolean } from '../../helpers';

const OperationSetting = () => {
  let [inputs, setInputs] = useState({
    /* 额度相关 */
    QuotaForNewUser: 0,
    DefaultSubscriptionPlans: '[]',
    PreConsumedQuota: 0,
    QuotaForInviter: 0,
    QuotaForInvitee: 0,
    'quota_setting.enable_free_model_pre_consume': true,

    /* 通用设置 */
    TopUpLink: '',
    'general_setting.docs_link': '',
    QuotaPerUnit: 0,
    USDExchangeRate: 0,
    RetryTimes: 0,
    'general_setting.quota_display_type': 'USD',
    DisplayTokenStatEnabled: false,
    DefaultCollapseSidebar: false,
    DemoSiteEnabled: false,
    SelfUseModeEnabled: false,

    /* 顶栏模块管理 */
    HeaderNavModules: '',

    /* 左侧边栏模块管理（管理员） */
    SidebarModulesAdmin: '',

    /* 敏感词设置 */
    CheckSensitiveEnabled: false,
    CheckSensitiveOnPromptEnabled: false,
    SensitiveWords: '',

    /* 日志设置 */
    LogConsumeEnabled: false,
    ForceRecordIpLogEnabled: false,

    /* 监控设置 */
    ChannelDisableThreshold: 0,
    AutomaticDisableChannelEnabled: false,
    AutomaticEnableChannelEnabled: false,
    AutomaticDisableKeywords: '',
    AutomaticDisableStatusCodes: '401',
    AutomaticRetryStatusCodes:
      '100-199,300-399,401-407,409-499,500-503,505-523,525-599',
    'monitor_setting.auto_test_channel_enabled': false,
    'monitor_setting.auto_test_channel_minutes': 10,
    'monitor_setting.channel_test_mode': 'scheduled_all',
    /* 管理员通知设置 */
    'notify_setting.channel_auto_disabled': true,
    'notify_setting.channel_auto_enabled': true,
    'notify_setting.channel_quota_cooldown': true,
    'notify_setting.channel_test_result': true,
    /* 签到设置 */
    'checkin_setting.enabled': false,
    'checkin_setting.min_quota': 1000,
    'checkin_setting.max_quota': 10000,
    'checkin_setting.special_enabled': false,
    'checkin_setting.special_weekday': '7',
    'checkin_setting.special_quota': 20000,
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
    'checkin_setting.makeup_counts_toward_progress': false,
    'checkin_setting.makeup_reward_enabled': true,
    'checkin_setting.risk_watch_enabled': false,
    'checkin_setting.risk_watch_days': 14,
    'checkin_setting.risk_min_daily_calls': 1,
    'checkin_setting.risk_min_daily_quota': 100,
    'checkin_setting.expire_enabled': false,
    'checkin_setting.expire_mode': 'unused',

    /* 令牌设置 */
    'token_setting.max_user_tokens': 1000,

    /* 词元贷设置 */
    'loan_setting.enabled': false,
    'loan_setting.max_total': 2500000,
    'loan_setting.daily_rate': 0.001,
    'loan_setting.min_register_days': 0,
    'loan_setting.max_per_borrow': 0,
    'loan_setting.checkin_repay_enabled': true,
    'notify_setting.loan_lender_overflow': true,
    'loan_setting.terms_enabled': true,
    'loan_setting.terms_text': '',
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
  });

  let [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        if (typeof inputs[item.key] === 'boolean') {
          newInputs[item.key] = toBoolean(item.value);
        } else {
          newInputs[item.key] = item.value;
        }
      });

      setInputs(newInputs);
    } else {
      showError(message);
    }
  };
  async function onRefresh() {
    try {
      setLoading(true);
      await getOptions();
      // showSuccess('刷新成功');
    } catch (error) {
      showError('刷新失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    onRefresh();
  }, []);

  return (
    <>
      <Spin spinning={loading} size='large'>
        {/* 通用设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsGeneral options={inputs} refresh={onRefresh} />
        </Card>
        {/* 顶栏模块管理 */}
        <div style={{ marginTop: '10px' }}>
          <SettingsHeaderNavModules options={inputs} refresh={onRefresh} />
        </div>
        {/* 左侧边栏模块管理（管理员） */}
        <div style={{ marginTop: '10px' }}>
          <SettingsSidebarModulesAdmin options={inputs} refresh={onRefresh} />
        </div>
        {/* 屏蔽词过滤设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsSensitiveWords options={inputs} refresh={onRefresh} />
        </Card>
        {/* 日志设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsLog options={inputs} refresh={onRefresh} />
        </Card>
        {/* 监控设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsMonitoring options={inputs} refresh={onRefresh} />
        </Card>
        {/* 额度设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsCreditLimit options={inputs} refresh={onRefresh} />
        </Card>
        {/* 签到设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsCheckin options={inputs} refresh={onRefresh} />
        </Card>
        {/* 词元贷设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsLoan options={inputs} refresh={onRefresh} />
        </Card>
      </Spin>
    </>
  );
};

export default OperationSetting;
