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
  Input,
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
import { Pencil, Plus, Save, Trash2, TestTube } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';
import RiskWhitelistGroupsField from './RiskWhitelistGroupsField';

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
  whitelist_groups: [],
  exclude_status_codes: [],
  rules: [],
  tiers: [
    {
      offense_count: 1,
      action: 'temp_ip_ban',
      duration_minutes: 30,
      reason_suffix: '',
    },
  ],
};

const createDefaultTier = (offenseCount = 1) => ({
  offense_count: offenseCount,
  action: 'temp_ip_ban',
  duration_minutes: 30,
  reason_suffix: '',
});

const normalizeRule = (rule, legacyTiers) => ({
  ...rule,
  pattern: rule.pattern || '',
  keywords: rule.keywords || [],
  error_codes: rule.error_codes || [],
  tiers:
    rule.tiers && rule.tiers.length
      ? rule.tiers
      : legacyTiers && legacyTiers.length
        ? legacyTiers.map((tier) => ({ ...tier }))
        : [createDefaultTier()],
});

const normalizeConfig = (data) => ({
  ...data,
  whitelist_groups: data.whitelist_groups || [],
  exclude_status_codes: data.exclude_status_codes || [],
  rules: (data.rules || []).map((rule) => normalizeRule(rule, data.tiers)),
});

const buildSaveConfig = (config, values) => ({
  ...config,
  ...values,
  // The rule editor is state-controlled while Semi Form retains its old snapshot.
  rules: config.rules || [],
  tiers: config.tiers || [],
  whitelist_groups: values.whitelist_groups ?? config.whitelist_groups ?? [],
  exclude_status_codes: config.exclude_status_codes || [],
});

const ControlField = ({ label, children }) => (
  <div className='flex min-w-0 flex-col gap-1.5'>
    <Text type='secondary'>{label}</Text>
    {children}
  </div>
);

const ErrorBanTab = () => {
  const { t } = useTranslation();
  const actionOptions = [
    { value: 'temp_ip_ban', label: t('临时IP封禁') },
    { value: 'perm_ip_ban', label: t('永久IP封禁') },
    { value: 'disable_user', label: t('禁用用户') },
    { value: 'both', label: t('同时封禁IP和用户') },
  ];
  const dimensionOptions = [
    { value: '', label: t('继承全局') },
    { value: 'ip', label: 'IP' },
    { value: 'user', label: t('用户') },
  ];
  const [config, setConfig] = useState(defaultConfig);
  const [saving, setSaving] = useState(false);
  const formApiRef = useRef(null);
  const [ruleVisible, setRuleVisible] = useState(false);
  const [editingRuleIndex, setEditingRuleIndex] = useState(null);
  const [ruleDraft, setRuleDraft] = useState(null);

  // Test dialog state
  const [testVisible, setTestVisible] = useState(false);
  const [testPattern, setTestPattern] = useState('');
  const [testKeywords, setTestKeywords] = useState('');
  const [testErrorCodes, setTestErrorCodes] = useState('');
  const [testSample, setTestSample] = useState('');
  const [testErrorCode, setTestErrorCode] = useState('');
  const [testResult, setTestResult] = useState(null);
  const [testLoading, setTestLoading] = useState(false);

  const fetchConfig = useCallback(async () => {
    try {
      const res = await API.get('/api/risk/error-ban/config');
      if (res.data.success) {
        const nextConfig = normalizeConfig(res.data.data);
        setConfig(nextConfig);
        formApiRef.current?.setValues(nextConfig);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  useEffect(() => {
    fetchConfig();
  }, [fetchConfig]);

  const handleSave = async (values) => {
    setSaving(true);
    try {
      const nextConfig = buildSaveConfig(config, values);
      const res = await API.put('/api/risk/error-ban/config', nextConfig);
      if (res.data.success) {
        const savedConfig = normalizeConfig(res.data.data);
        setConfig(savedConfig);
        formApiRef.current?.setValues(savedConfig);
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

  const addRule = () => {
    setEditingRuleIndex(null);
    setRuleDraft({
      id: `rule_${Date.now()}`,
      name: '',
      pattern: '',
      keywords: [],
      error_codes: [],
      enabled: true,
      dimension: '',
      threshold: 3,
      reason_template: '',
      tiers: [createDefaultTier()],
    });
    setRuleVisible(true);
  };

  const editRule = (index) => {
    setEditingRuleIndex(index);
    setRuleDraft(normalizeRule(config.rules[index], config.tiers));
    setRuleVisible(true);
  };

  const saveRule = () => {
    if (!ruleDraft) return;
    const rules = [...(config.rules || [])];
    if (editingRuleIndex === null) rules.push(ruleDraft);
    else rules[editingRuleIndex] = ruleDraft;
    setConfig({ ...config, rules });
    setRuleVisible(false);
  };

  const removeRule = (index) => {
    const rules = [...(config.rules || [])];
    rules.splice(index, 1);
    setConfig({ ...config, rules });
  };

  const toggleRule = (index, enabled) => {
    const rules = [...(config.rules || [])];
    rules[index] = { ...rules[index], enabled };
    setConfig({ ...config, rules });
  };

  const updateDraftTier = (index, field, value) => {
    const tiers = [...(ruleDraft.tiers || [])];
    tiers[index] = { ...tiers[index], [field]: value };
    if (
      field === 'action' &&
      value === 'temp_ip_ban' &&
      tiers[index].duration_minutes <= 0
    ) {
      tiers[index].duration_minutes = 1;
    }
    setRuleDraft({ ...ruleDraft, tiers });
  };

  const handleTestRule = async () => {
    setTestLoading(true);
    setTestResult(null);
    try {
      const res = await API.post('/api/risk/error-ban/rules/test', {
        pattern: testPattern,
        keywords: testKeywords
          .split('\n')
          .map((value) => value.trim())
          .filter(Boolean),
        error_codes: testErrorCodes
          .split('\n')
          .map((value) => value.trim())
          .filter(Boolean),
        sample_text: testSample,
        error_code: testErrorCode,
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

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <div className='flex justify-between items-center mb-4'>
          <Title heading={5}>{t('错误封禁配置')}</Title>
          <Button
            icon={<TestTube size={16} />}
            onClick={() => setTestVisible(true)}
          >
            {t('测试规则')}
          </Button>
        </div>
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
          <Form.Input
            field='default_reason_template'
            label={t('默认封禁原因模板')}
          />
          <Form.Input
            field='whitelist_user_ids'
            label={t('白名单用户ID（逗号分隔）')}
          />
          <RiskWhitelistGroupsField selectedGroups={config.whitelist_groups} />
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
          <Space>
            <Button
              icon={<Save size={16} />}
              loading={saving}
              onClick={() =>
                handleSave(formApiRef.current?.getValues?.() || config)
              }
            >
              {t('保存配置')}
            </Button>
            <Button
              icon={<Plus size={16} />}
              onClick={addRule}
              disabled={(config.rules || []).length >= 20}
            >
              {t('添加规则')}
            </Button>
          </Space>
        </div>
        <div className='overflow-hidden rounded-lg border border-semi-color-border bg-semi-color-bg-0 divide-y divide-semi-color-border'>
          {(config.rules || []).length === 0 && (
            <div className='p-4 text-center'>
              <Text type='tertiary'>{t('暂无规则')}</Text>
            </div>
          )}
          {(config.rules || []).map((rule, index) => (
            <div
              key={rule.id}
              className='flex items-center gap-3 min-h-14 px-4 py-2 transition-colors hover:bg-semi-color-fill-0'
            >
              <Text strong ellipsis={{ showTooltip: true }} className='flex-1'>
                {rule.name || rule.id}
              </Text>
              <Text type='secondary'>
                {t('触发阈值')}: {rule.threshold}
              </Text>
              <Switch
                checked={rule.enabled}
                aria-label={t('Toggle rule {{name}}', {
                  name: rule.name || rule.id,
                })}
                onChange={(enabled) => toggleRule(index, enabled)}
              />
              <Button
                theme='borderless'
                size='small'
                icon={<Pencil size={14} />}
                aria-label={t('编辑')}
                onClick={() => editRule(index)}
              />
              <Button
                theme='borderless'
                size='small'
                icon={<Trash2 size={14} />}
                aria-label={t('删除')}
                onClick={() => removeRule(index)}
              />
            </div>
          ))}
        </div>
      </Card>

      <Modal
        title={editingRuleIndex === null ? t('添加规则') : t('编辑规则')}
        visible={ruleVisible}
        width={760}
        onCancel={() => setRuleVisible(false)}
        onOk={saveRule}
        okButtonProps={{
          disabled:
            !ruleDraft?.name?.trim() ||
            !ruleDraft?.id?.trim() ||
            ruleDraft?.threshold < 1 ||
            (ruleDraft?.enabled &&
              !ruleDraft?.pattern?.trim() &&
              !ruleDraft?.keywords?.length &&
              !ruleDraft?.error_codes?.length) ||
            !ruleDraft?.tiers?.length ||
            ruleDraft?.tiers?.some(
              (tier) =>
                tier.offense_count < 1 ||
                tier.duration_minutes < 0 ||
                (tier.action === 'temp_ip_ban' && tier.duration_minutes < 1),
            ),
        }}
      >
        {ruleDraft && (
          <div className='flex flex-col gap-4'>
            <Text>{t('所有已配置的匹配条件必须同时满足')}</Text>
            <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
              <ControlField label={t('规则名称')}>
                <Input
                  value={ruleDraft.name || ''}
                  onChange={(name) => setRuleDraft({ ...ruleDraft, name })}
                />
              </ControlField>
              <ControlField label={t('规则ID')}>
                <Input
                  value={ruleDraft.id || ''}
                  onChange={(id) => setRuleDraft({ ...ruleDraft, id })}
                />
              </ControlField>
              <ControlField label={t('触发阈值')}>
                <InputNumber
                  value={ruleDraft.threshold}
                  min={1}
                  max={100000}
                  onChange={(threshold) =>
                    setRuleDraft({ ...ruleDraft, threshold })
                  }
                />
              </ControlField>
              <ControlField label={t('封禁维度')}>
                <Select
                  value={ruleDraft.dimension || ''}
                  onChange={(dimension) =>
                    setRuleDraft({ ...ruleDraft, dimension })
                  }
                  optionList={dimensionOptions}
                />
              </ControlField>
            </div>
            <ControlField label={t('正则表达式（可选）')}>
              <Input
                value={ruleDraft.pattern || ''}
                onChange={(pattern) => setRuleDraft({ ...ruleDraft, pattern })}
              />
            </ControlField>
            <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
              <ControlField label={t('错误关键词（每行一个，全部匹配）')}>
                <TextArea
                  value={(ruleDraft.keywords || []).join('\n')}
                  onChange={(value) =>
                    setRuleDraft({
                      ...ruleDraft,
                      keywords: value
                        .split('\n')
                        .map((item) => item.trim())
                        .filter(Boolean),
                    })
                  }
                  rows={3}
                />
              </ControlField>
              <ControlField label={t('错误码（每行一个，任一匹配）')}>
                <TextArea
                  value={(ruleDraft.error_codes || []).join('\n')}
                  onChange={(value) =>
                    setRuleDraft({
                      ...ruleDraft,
                      error_codes: value
                        .split('\n')
                        .map((item) => item.trim())
                        .filter(Boolean),
                    })
                  }
                  rows={3}
                  placeholder='*'
                />
              </ControlField>
            </div>
            <ControlField label={t('封禁原因模板')}>
              <Input
                value={ruleDraft.reason_template || ''}
                onChange={(reason_template) =>
                  setRuleDraft({ ...ruleDraft, reason_template })
                }
              />
            </ControlField>
            <div className='border-t border-semi-color-border pt-3'>
              <div className='flex justify-between items-center mb-3'>
                <Text strong>{t('处罚等级')}</Text>
                <Button
                  size='small'
                  icon={<Plus size={14} />}
                  onClick={() =>
                    setRuleDraft({
                      ...ruleDraft,
                      tiers: [
                        ...(ruleDraft.tiers || []),
                        createDefaultTier(
                          (ruleDraft.tiers?.at(-1)?.offense_count || 0) + 1,
                        ),
                      ],
                    })
                  }
                >
                  {t('添加等级')}
                </Button>
              </div>
              {(ruleDraft.tiers || []).map((tier, index) => (
                <div
                  key={index}
                  className='border-t border-semi-color-border py-3 first:border-t-0'
                >
                  <div className='flex justify-between items-center mb-2'>
                    <Text strong>
                      {t('等级')} #{index + 1}
                    </Text>
                    <Button
                      theme='borderless'
                      size='small'
                      icon={<Trash2 size={14} />}
                      onClick={() =>
                        setRuleDraft({
                          ...ruleDraft,
                          tiers: ruleDraft.tiers.filter(
                            (_, tierIndex) => tierIndex !== index,
                          ),
                        })
                      }
                    />
                  </div>
                  <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
                    <ControlField label={t('累计违规次数')}>
                      <InputNumber
                        value={tier.offense_count}
                        min={1}
                        max={100000}
                        onChange={(value) =>
                          updateDraftTier(index, 'offense_count', value)
                        }
                      />
                    </ControlField>
                    <ControlField label={t('处罚动作')}>
                      <Select
                        value={tier.action}
                        onChange={(value) =>
                          updateDraftTier(index, 'action', value)
                        }
                        optionList={actionOptions}
                      />
                    </ControlField>
                    {tier.action !== 'perm_ip_ban' && (
                      <ControlField
                        label={
                          tier.action === 'temp_ip_ban'
                            ? t('IP封禁时长（分钟）')
                            : t('账号封禁时长（分钟，0为永久）')
                        }
                      >
                        <InputNumber
                          value={tier.duration_minutes}
                          min={tier.action === 'temp_ip_ban' ? 1 : 0}
                          max={525600}
                          onChange={(value) =>
                            updateDraftTier(index, 'duration_minutes', value)
                          }
                        />
                      </ControlField>
                    )}
                    <ControlField label={t('原因后缀')}>
                      <Input
                        value={tier.reason_suffix || ''}
                        onChange={(value) =>
                          updateDraftTier(index, 'reason_suffix', value)
                        }
                      />
                    </ControlField>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </Modal>

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
            <Button
              type='primary'
              loading={testLoading}
              onClick={handleTestRule}
            >
              {t('测试')}
            </Button>
          </Space>
        }
      >
        <div className='flex flex-col gap-3'>
          <ControlField label={t('正则表达式')}>
            <Input value={testPattern} onChange={(v) => setTestPattern(v)} />
          </ControlField>
          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            <ControlField label={t('错误关键词（每行一个，全部匹配）')}>
              <TextArea
                value={testKeywords}
                onChange={(value) => setTestKeywords(value)}
                rows={3}
              />
            </ControlField>
            <ControlField label={t('允许的错误码（每行一个）')}>
              <TextArea
                value={testErrorCodes}
                onChange={(value) => setTestErrorCodes(value)}
                rows={3}
                placeholder='*'
              />
            </ControlField>
          </div>
          <ControlField label={t('样本文本')}>
            <TextArea
              value={testSample}
              onChange={(v) => setTestSample(v)}
              rows={4}
            />
          </ControlField>
          <ControlField label={t('样本错误码')}>
            <Input
              value={testErrorCode}
              onChange={(value) => setTestErrorCode(value)}
            />
          </ControlField>
          {testResult && (
            <div className='mt-2 p-3 rounded bg-gray-50 dark:bg-gray-800'>
              <Text>
                {t('规则有效')}:{' '}
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
