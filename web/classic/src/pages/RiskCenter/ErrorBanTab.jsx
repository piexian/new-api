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
  Select,
  Space,
  Switch,
  TextArea,
  Typography,
  Modal,
  Tag,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { Plus, Trash2, TestTube } from 'lucide-react';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import CardTable from '../../components/common/ui/CardTable';

const { Title, Text } = Typography;

const defaultConfig = {
  enabled: false,
  dry_run: true,
  window_seconds: 300,
  default_dimension: 'ip',
  default_reason_template: '',
  notify_user_enabled: true,
  notify_admin_enabled: true,
  appeal_hint: '',
  whitelist_user_ids: '',
  exclude_status_codes: [],
  rules: [],
  tiers: [
    { offense_count: 1, action: 'temp_ip_ban', duration_minutes: 30, reason_suffix: '' },
  ],
};

const ACTION_OPTIONS = [
  { value: 'temp_ip_ban', label: '临时IP封禁' },
  { value: 'perm_ip_ban', label: '永久IP封禁' },
  { value: 'disable_user', label: '禁用用户' },
  { value: 'both', label: '同时封禁IP和用户' },
];

const DIMENSION_OPTIONS = [
  { value: '', label: '继承全局' },
  { value: 'ip', label: 'IP' },
  { value: 'user', label: '用户' },
];

const ErrorBanTab = () => {
  const { t } = useTranslation();
  const [config, setConfig] = useState(defaultConfig);
  const [stats, setStats] = useState(null);
  const [ipStates, setIpStates] = useState([]);
  const [userStates, setUserStates] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [ipPage, setIpPage] = useState(1);
  const [ipTotal, setIpTotal] = useState(0);
  const [userPage, setUserPage] = useState(1);
  const [userTotal, setUserTotal] = useState(0);
  const pageSize = 10;

  // Test dialog state
  const [testVisible, setTestVisible] = useState(false);
  const [testPattern, setTestPattern] = useState('');
  const [testSample, setTestSample] = useState('');
  const [testResult, setTestResult] = useState(null);
  const [testLoading, setTestLoading] = useState(false);

  const fetchConfig = useCallback(async () => {
    try {
      const res = await API.get('/api/risk/error-ban/config');
      if (res.data.success) {
        setConfig(res.data.data);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  const fetchStats = useCallback(async () => {
    try {
      const res = await API.get('/api/risk/error-ban/stats');
      if (res.data.success) {
        setStats(res.data.data);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  const fetchIpStates = useCallback(async (page) => {
    try {
      const res = await API.get('/api/risk/error-ban/ip-states', {
        params: { p: page, page_size: 10 },
      });
      if (res.data.success) {
        setIpStates(res.data.data.items);
        setIpTotal(res.data.data.total);
        setIpPage(res.data.data.page);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  const fetchUserStates = useCallback(async (page) => {
    try {
      const res = await API.get('/api/risk/error-ban/user-states', {
        params: { p: page, page_size: 10 },
      });
      if (res.data.success) {
        setUserStates(res.data.data.items);
        setUserTotal(res.data.data.total);
        setUserPage(res.data.data.page);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  useEffect(() => {
    setLoading(true);
    Promise.all([fetchConfig(), fetchStats(), fetchIpStates(1), fetchUserStates(1)])
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [fetchConfig, fetchStats, fetchIpStates, fetchUserStates]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const res = await API.put('/api/risk/error-ban/config', config);
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

  const handleResetIpState = async (ip) => {
    try {
      const res = await API.post(`/api/risk/error-ban/ip-states/${ip}/reset`);
      if (res.data.success) {
        showSuccess(t('重置成功'));
        fetchIpStates(ipPage);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    }
  };

  const handleResetUserState = async (id) => {
    try {
      const res = await API.post(`/api/risk/error-ban/user-states/${id}/reset`);
      if (res.data.success) {
        showSuccess(t('重置成功'));
        fetchUserStates(userPage);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    }
  };

  const addRule = () => {
    const newRule = {
      id: `rule_${Date.now()}`,
      name: '',
      pattern: '',
      enabled: true,
      dimension: '',
      threshold: 3,
      reason_template: '',
    };
    setConfig({ ...config, rules: [...(config.rules || []), newRule] });
  };

  const removeRule = (index) => {
    const rules = [...(config.rules || [])];
    rules.splice(index, 1);
    setConfig({ ...config, rules });
  };

  const updateRule = (index, field, value) => {
    const rules = [...(config.rules || [])];
    rules[index] = { ...rules[index], [field]: value };
    setConfig({ ...config, rules });
  };

  const addTier = () => {
    const newTier = {
      offense_count: (config.tiers?.length || 0) + 1,
      action: 'temp_ip_ban',
      duration_minutes: 30,
      reason_suffix: '',
    };
    setConfig({ ...config, tiers: [...(config.tiers || []), newTier] });
  };

  const removeTier = (index) => {
    const tiers = [...(config.tiers || [])];
    tiers.splice(index, 1);
    setConfig({ ...config, tiers });
  };

  const updateTier = (index, field, value) => {
    const tiers = [...(config.tiers || [])];
    tiers[index] = { ...tiers[index], [field]: value };
    setConfig({ ...config, tiers });
  };

  const handleTestRule = async () => {
    setTestLoading(true);
    setTestResult(null);
    try {
      const res = await API.post('/api/risk/error-ban/rules/test', {
        pattern: testPattern,
        sample_text: testSample,
      });
      if (res.data.success) {
        setTestResult(res.data.data);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setTestLoading(false);
    }
  };

  const ipColumns = useMemo(
    () => [
      { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
      { title: t('IP地址'), dataIndex: 'target_ip', key: 'target_ip' },
      { title: t('规则ID'), dataIndex: 'rule_id', key: 'rule_id' },
      { title: t('违规次数'), dataIndex: 'offense_count', key: 'offense_count', width: 90 },
      { title: t('窗口内次数'), dataIndex: 'window_count', key: 'window_count', width: 100 },
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
            onClick={() => handleResetIpState(record.target_ip)}
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
      { title: t('规则ID'), dataIndex: 'rule_id', key: 'rule_id' },
      { title: t('违规次数'), dataIndex: 'offense_count', key: 'offense_count', width: 90 },
      { title: t('窗口内次数'), dataIndex: 'window_count', key: 'window_count', width: 100 },
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
            onClick={() => handleResetUserState(record.id)}
          >
            {t('重置')}
          </Button>
        ),
      },
    ],
    [t],
  );

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <div className='flex justify-between items-center mb-4'>
          <Title heading={5}>{t('错误封禁配置')}</Title>
          <Button icon={<TestTube size={16} />} onClick={() => setTestVisible(true)}>
            {t('测试规则')}
          </Button>
        </div>
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
            max={86400}
            step={1}
          />
          <Form.Select
            field='default_dimension'
            label={t('默认封禁维度')}
            optionList={[
              { value: 'ip', label: 'IP' },
              { value: 'user', label: '用户' },
            ]}
          />
          <Form.Input field='default_reason_template' label={t('默认封禁原因模板')} />
          <Form.Input field='whitelist_user_ids' label={t('白名单用户ID（逗号分隔）')} />
          <Form.Switch field='notify_user_enabled' label={t('通知用户')} />
          <Form.Switch field='notify_admin_enabled' label={t('通知管理员')} />
          <Form.TextArea field='appeal_hint' label={t('申诉提示')} rows={2} />
          <Form.Slot>
            <Button type='primary' htmlType='submit' loading={saving}>
              {t('保存配置')}
            </Button>
          </Form.Slot>
        </Form>
      </Card>

      {/* Rules section */}
      <Card>
        <div className='flex justify-between items-center mb-4'>
          <Title heading={5}>{t('封禁规则')}</Title>
          <Button icon={<Plus size={16} />} onClick={addRule}>
            {t('添加规则')}
          </Button>
        </div>
        {(config.rules || []).map((rule, index) => (
          <div key={rule.id || index} className='border rounded-lg p-4 mb-3'>
            <div className='flex justify-between items-center mb-2'>
              <Text strong>
                {t('规则')} #{index + 1}
              </Text>
              <Button
                type='danger'
                size='small'
                icon={<Trash2 size={14} />}
                onClick={() => removeRule(index)}
              />
            </div>
            <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
              <Form.Input
                label={t('规则名称')}
                value={rule.name}
                onChange={(v) => updateRule(index, 'name', v)}
              />
              <Form.Input
                label='ID'
                value={rule.id}
                onChange={(v) => updateRule(index, 'id', v)}
              />
              <Form.Input
                label={t('正则表达式')}
                value={rule.pattern}
                onChange={(v) => updateRule(index, 'pattern', v)}
              />
              <Form.Select
                label={t('封禁维度')}
                value={rule.dimension}
                onChange={(v) => updateRule(index, 'dimension', v)}
                optionList={DIMENSION_OPTIONS}
              />
              <Form.InputNumber
                label={t('触发阈值')}
                value={rule.threshold}
                min={1}
                max={100}
                onChange={(v) => updateRule(index, 'threshold', v)}
              />
              <Form.Input
                label={t('封禁原因模板')}
                value={rule.reason_template}
                onChange={(v) => updateRule(index, 'reason_template', v)}
              />
              <Form.Switch
                label={t('启用')}
                checked={rule.enabled}
                onChange={(v) => updateRule(index, 'enabled', v)}
              />
            </div>
          </div>
        ))}
      </Card>

      {/* Tiers section */}
      <Card>
        <div className='flex justify-between items-center mb-4'>
          <Title heading={5}>{t('处罚等级')}</Title>
          <Button icon={<Plus size={16} />} onClick={addTier}>
            {t('添加等级')}
          </Button>
        </div>
        {(config.tiers || []).map((tier, index) => (
          <div key={index} className='border rounded-lg p-4 mb-3'>
            <div className='flex justify-between items-center mb-2'>
              <Text strong>
                {t('等级')} #{index + 1}
              </Text>
              <Button
                type='danger'
                size='small'
                icon={<Trash2 size={14} />}
                onClick={() => removeTier(index)}
              />
            </div>
            <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
              <Form.InputNumber
                label={t('累计违规次数')}
                value={tier.offense_count}
                min={1}
                max={100}
                onChange={(v) => updateTier(index, 'offense_count', v)}
              />
              <Form.Select
                label={t('处罚动作')}
                value={tier.action}
                onChange={(v) => updateTier(index, 'action', v)}
                optionList={ACTION_OPTIONS}
              />
              <Form.InputNumber
                label={t('封禁时长（分钟）')}
                value={tier.duration_minutes}
                min={0}
                max={86400}
                onChange={(v) => updateTier(index, 'duration_minutes', v)}
              />
              <Form.Input
                label={t('原因后缀')}
                value={tier.reason_suffix}
                onChange={(v) => updateTier(index, 'reason_suffix', v)}
              />
            </div>
          </div>
        ))}
      </Card>

      {/* Stats */}
      {stats && (
        <Card>
          <Title heading={5} className='mb-2'>
            {t('统计数据')}
          </Title>
          <div className='flex flex-wrap gap-4'>
            <Text>{t('IP状态数')}: {stats.total_ip_states}</Text>
            <Text>{t('用户状态数')}: {stats.total_user_states}</Text>
            <Text>{t('总违规次数')}: {stats.total_offenses}</Text>
            <Text>{t('活跃规则数')}: {stats.active_rules}</Text>
          </div>
        </Card>
      )}

      <Card>
        <Title heading={5} className='mb-4'>
          {t('IP状态记录')}
        </Title>
        <CardTable
          columns={ipColumns}
          dataSource={ipStates}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: ipPage,
            pageSize: pageSize,
            total: ipTotal,
            onChange: (page) => fetchIpStates(page),
          }}
        />
      </Card>

      <Card>
        <Title heading={5} className='mb-4'>
          {t('用户状态记录')}
        </Title>
        <CardTable
          columns={userColumns}
          dataSource={userStates}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: userPage,
            pageSize: pageSize,
            total: userTotal,
            onChange: (page) => fetchUserStates(page),
          }}
        />
      </Card>

      {/* Test rule dialog */}
      <Modal
        title={t('测试封禁规则')}
        visible={testVisible}
        onCancel={() => {
          setTestVisible(false);
          setTestResult(null);
        }}
        footer={
          <Space>
            <Button onClick={() => setTestVisible(false)}>{t('关闭')}</Button>
            <Button type='primary' loading={testLoading} onClick={handleTestRule}>
              {t('测试')}
            </Button>
          </Space>
        }
      >
        <div className='flex flex-col gap-3'>
          <Form.Input
            label={t('正则表达式')}
            value={testPattern}
            onChange={(v) => setTestPattern(v)}
          />
          <Form.TextArea
            label={t('样本文本')}
            value={testSample}
            onChange={(v) => setTestSample(v)}
            rows={4}
          />
          {testResult && (
            <div className='mt-2 p-3 rounded bg-gray-50 dark:bg-gray-800'>
              <Text>
                {t('正则有效')}:{' '}
                <Tag color={testResult.valid ? 'green' : 'red'}>
                  {testResult.valid ? t('是') : t('否')}
                </Tag>
              </Text>
              {testResult.valid && (
                <div className='mt-1'>
                  <Text>
                    {t('匹配成功')}:{' '}
                    <Tag color={testResult.matched ? 'green' : 'orange'}>
                      {testResult.matched ? t('是') : t('否')}
                    </Tag>
                  </Text>
                </div>
              )}
              {testResult.error && (
                <div className='mt-1'>
                  <Text type='danger'>{testResult.error}</Text>
                </div>
              )}
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
};

export default ErrorBanTab;
