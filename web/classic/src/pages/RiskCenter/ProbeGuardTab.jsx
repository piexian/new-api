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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Button,
  Card,
  Form,
  InputNumber,
  Space,
  Switch,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import RiskWhitelistGroupsField from './RiskWhitelistGroupsField';

const { Title } = Typography;

const defaultConfig = {
  enabled: false,
  dry_run: true,
  window_seconds: 60,
  distinct_model_count: 5,
  ban_dimension: 'ip',
  first_ip_ban_minutes: 10,
  second_ip_ban_minutes: 60,
  permanent_offense_count: 3,
  offense_dedupe_seconds: 60,
  whitelist_user_ids: '',
  whitelist_groups: [],
  user_ban_reason: '',
  notify_user_enabled: true,
  notify_admin_enabled: true,
  appeal_hint: '',
};

const ProbeGuardTab = () => {
  const { t } = useTranslation();
  const [config, setConfig] = useState(defaultConfig);
  const [saving, setSaving] = useState(false);
  const [banDimension, setBanDimension] = useState('ip');
  const formApiRef = useRef(null);

  const normalizeConfig = useCallback((data) => {
    return {
      ...data,
      ban_dimension: data.ban_dimension || 'ip',
      whitelist_groups: data.whitelist_groups || [],
    };
  }, []);

  const fetchConfig = useCallback(async () => {
    try {
      const res = await API.get('/api/risk/probe-guard/config');
      if (res.data.success) {
        const nextConfig = normalizeConfig(res.data.data);
        setConfig(nextConfig);
        setBanDimension(nextConfig.ban_dimension);
        formApiRef.current?.setValues(nextConfig);
      }
    } catch (err) {
      showError(err);
    }
  }, [normalizeConfig]);

  useEffect(() => {
    fetchConfig();
  }, [fetchConfig]);

  const handleSave = async (values) => {
    setSaving(true);
    try {
      const res = await API.put('/api/risk/probe-guard/config', values);
      if (res.data.success) {
        const nextConfig = normalizeConfig(res.data.data);
        setConfig(nextConfig);
        setBanDimension(nextConfig.ban_dimension);
        formApiRef.current?.setValues(nextConfig);
        showSuccess(t('保存成功'));
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <Title heading={5} className='mb-4'>
          {t('探针防护配置')}
        </Title>
        <Form
          getFormApi={(api) => (formApiRef.current = api)}
          initValues={config}
          onSubmit={handleSave}
          labelPosition='left'
          labelAlign='right'
          labelWidth={180}
        >
          <Form.Switch field='enabled' label={t('启用')} />
          <Form.Switch field='dry_run' label={t('仅记录（不实际封禁）')} />
          <Form.InputNumber
            field='window_seconds'
            label={t('窗口时间（秒）')}
            min={1}
            max={3600}
            step={1}
          />
          <Form.InputNumber
            field='distinct_model_count'
            label={t('触发模型数')}
            min={2}
            max={100}
            step={1}
          />
          <Form.Select
            field='ban_dimension'
            label={t('封禁维度')}
            optionList={[
              { value: 'ip', label: t('IP') },
              { value: 'user', label: t('用户') },
              { value: 'both', label: t('同时封禁IP和用户') },
            ]}
            onChange={setBanDimension}
          />
          <Form.InputNumber
            field='first_ip_ban_minutes'
            label={t('首次封禁时长（分钟）')}
            min={1}
            max={525600}
            step={1}
          />
          <Form.InputNumber
            field='second_ip_ban_minutes'
            label={t('再次封禁时长（分钟）')}
            min={1}
            max={525600}
            step={1}
          />
          <Form.InputNumber
            field='permanent_offense_count'
            label={t('永久封禁违规次数')}
            min={1}
            max={100}
            step={1}
          />
          <Form.InputNumber
            field='offense_dedupe_seconds'
            label={t('违规去重时间（秒）')}
            min={0}
            max={3600}
            step={1}
          />
          <Form.Input
            field='whitelist_user_ids'
            label={t('白名单用户ID（逗号分隔）')}
          />
          <RiskWhitelistGroupsField selectedGroups={config.whitelist_groups} />
          {banDimension !== 'ip' && (
            <>
              <Form.Input field='user_ban_reason' label={t('用户封禁原因')} />
              <Form.Switch field='notify_user_enabled' label={t('通知用户')} />
            </>
          )}
          <Form.Switch field='notify_admin_enabled' label={t('通知管理员')} />
          <Form.TextArea field='appeal_hint' label={t('申诉提示')} rows={2} />
          <Form.Slot>
            <Button type='primary' htmlType='submit' loading={saving}>
              {t('保存')}
            </Button>
          </Form.Slot>
        </Form>
      </Card>
    </div>
  );
};

export default ProbeGuardTab;
