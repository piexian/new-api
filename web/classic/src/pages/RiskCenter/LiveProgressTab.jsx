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
  Input,
  Progress,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { Eye, RefreshCw, Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import CardTable from '../../components/common/ui/CardTable';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';

const { Title, Text } = Typography;
const PAGE_SIZE = 10;
const REFRESH_STORAGE_KEY = 'new-api:risk-live-progress:refresh:v1';
const REFRESH_OPTIONS = [0, 5, 10, 15, 30, 60];

const loadRefreshSeconds = () => {
  try {
    const value = Number(window.localStorage.getItem(REFRESH_STORAGE_KEY));
    return REFRESH_OPTIONS.includes(value) ? value : 0;
  } catch {
    return 0;
  }
};

const saveRefreshSeconds = (value) => {
  try {
    window.localStorage.setItem(REFRESH_STORAGE_KEY, String(value));
  } catch {
    // Storage can be unavailable in private browsing.
  }
};

const formatTimestamp = (value) => (value ? timestamp2string(value) : '-');

const LiveProgressTab = () => {
  const { t } = useTranslation();
  const [rules, setRules] = useState([]);
  const [rulesLoading, setRulesLoading] = useState(false);
  const [selectedKey, setSelectedKey] = useState('');
  const [dimension, setDimension] = useState('');
  const [targets, setTargets] = useState([]);
  const [targetsLoading, setTargetsLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [searchKeyword, setSearchKeyword] = useState('');
  const [refreshSeconds, setRefreshSeconds] = useState(loadRefreshSeconds);
  const [togglingKey, setTogglingKey] = useState('');

  const selectedRule =
    rules.find((rule) => `${rule.source}:${rule.rule_id}` === selectedKey) ||
    rules[0];
  const selectedRuleKey = selectedRule
    ? `${selectedRule.source}:${selectedRule.rule_id}`
    : '';
  const selectedSource = selectedRule?.source || '';
  const selectedRuleId = selectedRule?.rule_id || '';
  const targetDimension = selectedRule
    ? selectedRule.dimension === 'both'
      ? dimension
      : selectedRule.dimension
    : '';

  const fetchRules = useCallback(async () => {
    setRulesLoading(true);
    try {
      const res = await API.get('/api/risk/live-progress/rules');
      if (res.data.success) {
        setRules(res.data.data || []);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setRulesLoading(false);
    }
  }, []);

  const fetchTargets = useCallback(async () => {
    if (!selectedSource || !selectedRuleId) {
      setTargets([]);
      setTotal(0);
      return;
    }
    setTargetsLoading(true);
    try {
      const res = await API.get('/api/risk/live-progress/targets', {
        params: {
          source: selectedSource,
          rule_id: selectedRuleId,
          dimension: targetDimension || undefined,
          p: page,
          page_size: PAGE_SIZE,
          keyword: searchKeyword || undefined,
        },
      });
      if (res.data.success) {
        setTargets(res.data.data.items || []);
        setTotal(res.data.data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setTargetsLoading(false);
    }
  }, [selectedSource, selectedRuleId, targetDimension, page, searchKeyword]);

  const refresh = useCallback(() => {
    fetchRules();
    fetchTargets();
  }, [fetchRules, fetchTargets]);

  useEffect(() => {
    fetchRules();
  }, [fetchRules]);

  useEffect(() => {
    fetchTargets();
  }, [fetchTargets]);

  useEffect(() => {
    if (refreshSeconds === 0) return undefined;
    const refreshWhenVisible = () => {
      if (document.visibilityState === 'visible') refresh();
    };
    const timer = window.setInterval(refreshWhenVisible, refreshSeconds * 1000);
    document.addEventListener('visibilitychange', refreshWhenVisible);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', refreshWhenVisible);
    };
  }, [refresh, refreshSeconds]);

  const selectRule = (rule) => {
    setSelectedKey(`${rule.source}:${rule.rule_id}`);
    setDimension('');
    setPage(1);
    setKeyword('');
    setSearchKeyword('');
  };

  const toggleRule = async (rule, enabled) => {
    const key = `${rule.source}:${rule.rule_id}`;
    setTogglingKey(key);
    try {
      const res = await API.patch('/api/risk/live-progress/rules/enabled', {
        source: rule.source,
        rule_id: rule.rule_id,
        enabled,
      });
      if (res.data.success) {
        setRules((current) =>
          current.map((item) =>
            item.source === rule.source && item.rule_id === rule.rule_id
              ? { ...item, enabled }
              : item,
          ),
        );
        showSuccess(t('Rule status updated'));
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setTogglingKey('');
    }
  };

  const ruleColumns = useMemo(
    () => [
      {
        title: t('Rule'),
        key: 'rule',
        render: (_, rule) => (
          <Space spacing='tight' wrap>
            <Text strong>
              {rule.system ? t('Probe Guard') : rule.rule_name || rule.rule_id}
            </Text>
            {rule.system && <Tag color='blue'>{t('System Rule')}</Tag>}
            {rule.dry_run && <Tag>{t('Dry Run')}</Tag>}
          </Space>
        ),
      },
      {
        title: t('Dimension'),
        dataIndex: 'dimension',
        key: 'dimension',
        render: (value) =>
          value === 'both' ? t('IP + User') : value === 'ip' ? 'IP' : t('User'),
      },
      { title: t('Threshold'), dataIndex: 'threshold', key: 'threshold' },
      {
        title: t('Active Targets'),
        dataIndex: 'active_targets',
        key: 'active_targets',
      },
      {
        title: t('Near Threshold'),
        dataIndex: 'near_threshold_targets',
        key: 'near_threshold_targets',
      },
      {
        title: t('Max Progress'),
        key: 'max_progress_percent',
        render: (_, rule) => (
          <div className='min-w-32'>
            <Progress
              percent={rule.max_progress_percent}
              aria-label={t('Max Progress')}
            />
          </div>
        ),
      },
      {
        title: t('Last Activity'),
        dataIndex: 'last_seen_at',
        key: 'last_seen_at',
        render: formatTimestamp,
      },
      {
        title: t('Enabled'),
        key: 'enabled',
        render: (_, rule) => {
          const key = `${rule.source}:${rule.rule_id}`;
          return (
            <Switch
              checked={rule.enabled}
              loading={togglingKey === key}
              disabled={Boolean(togglingKey)}
              aria-label={t('Toggle rule {{name}}', {
                name: rule.system
                  ? t('Probe Guard')
                  : rule.rule_name || rule.rule_id,
              })}
              onChange={(enabled) => toggleRule(rule, enabled)}
            />
          );
        },
      },
      {
        title: t('Actions'),
        key: 'actions',
        render: (_, rule) => (
          <Button
            theme='borderless'
            size='small'
            icon={<Eye size={15} />}
            aria-label={t('Details')}
            onClick={() => selectRule(rule)}
          />
        ),
      },
    ],
    [t, togglingKey],
  );

  const targetColumns = useMemo(() => {
    const columns = [
      {
        title: t('Target'),
        key: 'target',
        render: (_, target) => (
          <div className='flex flex-col'>
            <Text code>{target.target}</Text>
            {target.username && <Text type='secondary'>{target.username}</Text>}
          </div>
        ),
      },
      {
        title: t('Dimension'),
        dataIndex: 'dimension',
        key: 'dimension',
        render: (value) => (value === 'ip' ? 'IP' : t('User')),
      },
      {
        title: t('Context'),
        dataIndex: 'context',
        key: 'context',
        render: (value) => value || '-',
      },
      {
        title: t('Current Progress'),
        key: 'progress_percent',
        render: (_, target) => (
          <div className='min-w-36'>
            <Progress
              percent={target.progress_percent}
              format={() => `${target.current_count} / ${target.threshold}`}
            />
          </div>
        ),
      },
    ];
    if (selectedRule?.system) {
      columns.push({
        title: t('Current Models'),
        key: 'members',
        render: (_, target) => target.members?.join(', ') || '-',
      });
    }
    columns.push(
      {
        title: t('Window Remaining'),
        dataIndex: 'remaining_seconds',
        key: 'remaining_seconds',
        render: (value) => t('{{seconds}} seconds', { seconds: value }),
      },
      {
        title: t('Last Activity'),
        dataIndex: 'last_seen_at',
        key: 'last_seen_at',
        render: formatTimestamp,
      },
      {
        title: t('Status'),
        dataIndex: 'status',
        key: 'status',
        render: (value) => {
          if (value === 'threshold_reached') {
            return <Tag color='red'>{t('Threshold reached')}</Tag>;
          }
          if (value === 'near_threshold') {
            return <Tag color='orange'>{t('Near threshold')}</Tag>;
          }
          return <Tag color='blue'>{t('Observing')}</Tag>;
        },
      },
    );
    return columns;
  }, [selectedRule?.system, t]);

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <div className='mb-4 flex flex-wrap items-end justify-between gap-3'>
          <Title heading={5}>{t('Live Progress')}</Title>
          <Space wrap>
            <div className='flex flex-col gap-1'>
              <Text type='secondary'>{t('Auto refresh')}</Text>
              <Select
                value={refreshSeconds}
                style={{ width: 140 }}
                onChange={(value) => {
                  setRefreshSeconds(value);
                  saveRefreshSeconds(value);
                }}
                optionList={REFRESH_OPTIONS.map((seconds) => ({
                  value: seconds,
                  label:
                    seconds === 0
                      ? t('Off')
                      : t('{{seconds}} seconds', { seconds }),
                }))}
              />
            </div>
            <Button
              icon={<RefreshCw size={16} />}
              loading={rulesLoading || targetsLoading}
              aria-label={t('Refresh')}
              onClick={refresh}
            >
              {t('Refresh')}
            </Button>
          </Space>
        </div>
        <CardTable
          columns={ruleColumns}
          dataSource={rules}
          loading={rulesLoading}
          rowKey={(rule) => `${rule.source}:${rule.rule_id}`}
          hidePagination
          rowClassName={(rule) =>
            `${rule.source}:${rule.rule_id}` === selectedRuleKey
              ? 'bg-semi-color-fill-0'
              : ''
          }
        />
      </Card>

      {selectedRule && (
        <Card>
          <div className='mb-4 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between'>
            <div>
              <Title heading={5}>
                {selectedRule.system
                  ? t('Probe Guard')
                  : selectedRule.rule_name || selectedRule.rule_id}
              </Title>
              <Text type='secondary'>
                {t('Window {{window}}s / Threshold {{threshold}}', {
                  window: selectedRule.window_seconds,
                  threshold: selectedRule.threshold,
                })}
              </Text>
            </div>
            <Space wrap>
              {selectedRule.dimension === 'both' && (
                <Select
                  value={dimension || 'all'}
                  style={{ width: 120 }}
                  onChange={(value) => {
                    setDimension(value === 'all' ? '' : value);
                    setPage(1);
                  }}
                  optionList={[
                    { value: 'all', label: t('All') },
                    { value: 'ip', label: 'IP' },
                    { value: 'user', label: t('User') },
                  ]}
                />
              )}
              <Input
                prefix={<Search size={16} />}
                value={keyword}
                onChange={setKeyword}
                onEnterPress={() => {
                  setPage(1);
                  setSearchKeyword(keyword.trim());
                }}
                showClear
                placeholder={t('Search...')}
                style={{ width: 260 }}
              />
              <Button
                onClick={() => {
                  setPage(1);
                  setSearchKeyword(keyword.trim());
                }}
              >
                {t('Search')}
              </Button>
            </Space>
          </div>
          <CardTable
            columns={targetColumns}
            dataSource={targets}
            loading={targetsLoading}
            rowKey='id'
            empty={
              <div className='p-8 text-center'>
                <Text type='tertiary'>{t('No active progress')}</Text>
              </div>
            }
            pagination={{
              currentPage: page,
              pageSize: PAGE_SIZE,
              total,
              onChange: setPage,
            }}
          />
        </Card>
      )}
    </div>
  );
};

export default LiveProgressTab;
