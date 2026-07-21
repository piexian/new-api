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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
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
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import CardTable from '../../components/common/ui/CardTable';

const { Title, Text } = Typography;

const defaultConfig = {
  enabled: false,
  dry_run: true,
  window_seconds: 60,
  distinct_model_count: 5,
  first_ip_ban_minutes: 10,
  second_ip_ban_minutes: 60,
  permanent_offense_count: 3,
  offense_dedupe_seconds: 60,
  whitelist_user_ids: '',
  user_ban_enabled: false,
  user_ban_threshold: 2,
  user_ban_reason: '',
  notify_user_enabled: true,
  notify_admin_enabled: true,
  appeal_hint: '',
};

const ProbeGuardTab = () => {
  const { t } = useTranslation();
  const [config, setConfig] = useState(defaultConfig);
  const [stats, setStats] = useState(null);
  const [ipOffenses, setIpOffenses] = useState([]);
  const [userOffenses, setUserOffenses] = useState([]);
  const [loading, setLoading] = useState(false);
  const [ipPage, setIpPage] = useState(1);
  const [ipTotal, setIpTotal] = useState(0);
  const [userPage, setUserPage] = useState(1);
  const [userTotal, setUserTotal] = useState(0);
  const [saving, setSaving] = useState(false);
  const pageSize = 10;

  const fetchConfig = useCallback(async () => {
    try {
      const res = await API.get('/api/risk/probe-guard/config');
      if (res.data.success) {
        setConfig(res.data.data);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  const fetchStats = useCallback(async () => {
    try {
      const res = await API.get('/api/risk/probe-guard/stats');
      if (res.data.success) {
        setStats(res.data.data);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  const fetchIpOffenses = useCallback(async (page) => {
    try {
      const res = await API.get('/api/risk/probe-guard/ip-offenses', {
        params: { p: page, page_size: 10 },
      });
      if (res.data.success) {
        setIpOffenses(res.data.data.items);
        setIpTotal(res.data.data.total);
        setIpPage(res.data.data.page);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  const fetchUserOffenses = useCallback(async (page) => {
    try {
      const res = await API.get('/api/risk/probe-guard/user-offenses', {
        params: { p: page, page_size: 10 },
      });
      if (res.data.success) {
        setUserOffenses(res.data.data.items);
        setUserTotal(res.data.data.total);
        setUserPage(res.data.data.page);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  useEffect(() => {
    setLoading(true);
    Promise.all([fetchConfig(), fetchStats(), fetchIpOffenses(1), fetchUserOffenses(1)])
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [fetchConfig, fetchStats, fetchIpOffenses, fetchUserOffenses]);

  const handleSave = async (values) => {
    setSaving(true);
    try {
      const res = await API.put('/api/risk/probe-guard/config', values);
      if (res.data.success) {
        setConfig(res.data.data);
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

  const handleResetIp = async (ip) => {
    try {
      const res = await API.post(`/api/risk/probe-guard/ip-offenses/${ip}/reset`);
      if (res.data.success) {
        showSuccess(t('重置成功'));
        fetchIpOffenses(ipPage);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    }
  };

  const handleUnbanUser = async (id) => {
    try {
      const res = await API.post(`/api/risk/probe-guard/user-offenses/${id}/unban`);
      if (res.data.success) {
        showSuccess(t('解封成功'));
        fetchUserOffenses(userPage);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    }
  };

  const ipColumns = useMemo(
    () => [
      { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
      { title: t('IP地址'), dataIndex: 'target_ip', key: 'target_ip' },
      { title: t('最近用户ID'), dataIndex: 'last_user_id', key: 'last_user_id', width: 120 },
      { title: t('违规次数'), dataIndex: 'offense_count', key: 'offense_count', width: 100 },
      {
        title: t('最近违规'),
        dataIndex: 'last_offense_at',
        key: 'last_offense_at',
        render: (val) => (val ? timestamp2string(val) : '-'),
      },
      {
        title: t('操作'),
        key: 'action',
        render: (_, record) => (
          <Button
            size='small'
            type='danger'
            onClick={() => handleResetIp(record.target_ip)}
          >
            {t('重置')}
          </Button>
        ),
      },
    ],
    [t],
  );

  const userColumns = useMemo(
    () => [
      { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
      { title: t('用户ID'), dataIndex: 'user_id', key: 'user_id', width: 100 },
      { title: t('违规次数'), dataIndex: 'offense_count', key: 'offense_count', width: 100 },
      {
        title: t('最近违规'),
        dataIndex: 'last_offense_at',
        key: 'last_offense_at',
        render: (val) => (val ? timestamp2string(val) : '-'),
      },
      { title: t('最近IP'), dataIndex: 'last_ip', key: 'last_ip' },
      {
        title: t('操作'),
        key: 'action',
        render: (_, record) => (
          <Button
            size='small'
            type='danger'
            onClick={() => handleUnbanUser(record.id)}
          >
            {t('解封')}
          </Button>
        ),
      },
    ],
    [t],
  );

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <Title heading={5} className='mb-4'>
          {t('探针防护配置')}
        </Title>
        <Form
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
          <Form.InputNumber
            field='first_ip_ban_minutes'
            label={t('首次封禁时长（分钟）')}
            min={1}
            max={1440}
            step={1}
          />
          <Form.InputNumber
            field='second_ip_ban_minutes'
            label={t('再次封禁时长（分钟）')}
            min={1}
            max={1440}
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
          <Form.Input field='whitelist_user_ids' label={t('白名单用户ID（逗号分隔）')} />
          <Form.Switch field='user_ban_enabled' label={t('自动封禁用户')} />
          <Form.InputNumber
            field='user_ban_threshold'
            label={t('用户封禁阈值')}
            min={1}
            max={100}
            step={1}
          />
          <Form.Input field='user_ban_reason' label={t('用户封禁原因')} />
          <Form.Switch field='notify_user_enabled' label={t('通知用户')} />
          <Form.Switch field='notify_admin_enabled' label={t('通知管理员')} />
          <Form.TextArea field='appeal_hint' label={t('申诉提示')} rows={2} />
          <Form.Slot>
            <Button type='primary' htmlType='submit' loading={saving}>
              {t('保存')}
            </Button>
          </Form.Slot>
        </Form>
      </Card>

      {stats && (
        <Card>
          <Title heading={5} className='mb-2'>
            {t('统计数据')}
          </Title>
          <div className='flex flex-wrap gap-4'>
            <Text>{t('IP状态数')}: {stats.total_ip_states}</Text>
            <Text>{t('用户状态数')}: {stats.total_user_states}</Text>
            <Text>{t('总违规次数')}: {stats.total_offenses}</Text>
            <Text>{t('近期违规')}: {stats.recent_offenses}</Text>
          </div>
        </Card>
      )}

      <Card>
        <Title heading={5} className='mb-4'>
          {t('IP违规记录')}
        </Title>
        <CardTable
          columns={ipColumns}
          dataSource={ipOffenses}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: ipPage,
            pageSize: pageSize,
            total: ipTotal,
            onChange: (page) => fetchIpOffenses(page),
          }}
        />
      </Card>

      <Card>
        <Title heading={5} className='mb-4'>
          {t('用户违规记录')}
        </Title>
        <CardTable
          columns={userColumns}
          dataSource={userOffenses}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: userPage,
            pageSize: pageSize,
            total: userTotal,
            onChange: (page) => fetchUserOffenses(page),
          }}
        />
      </Card>
    </div>
  );
};

export default ProbeGuardTab;
