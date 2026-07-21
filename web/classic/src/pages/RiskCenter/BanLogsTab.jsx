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
  Descriptions,
  Modal,
  Select,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, timestamp2string } from '../../helpers';
import CardTable from '../../components/common/ui/CardTable';

const { Title, Text } = Typography;

const DIMENSION_OPTIONS = [
  { value: '', label: '全部' },
  { value: 'ip', label: 'IP' },
  { value: 'user', label: '用户' },
];

const SOURCE_OPTIONS = [
  { value: '', label: '全部' },
  { value: 'probe_guard', label: '探针防护' },
  { value: 'error_ban', label: '错误封禁' },
  { value: 'ip_middleware', label: 'IP中间件' },
  { value: 'manual', label: '手动' },
];

const DRY_RUN_OPTIONS = [
  { value: '', label: '全部' },
  { value: 'true', label: '仅记录' },
  { value: 'false', label: '实际封禁' },
];

const ACTION_COLORS = {
  temp_ip_ban: 'orange',
  perm_ip_ban: 'red',
  disable_user: 'purple',
  both: 'pink',
};

const BanLogsTab = () => {
  const { t } = useTranslation();
  const [logs, setLogs] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [dimension, setDimension] = useState('');
  const [source, setSource] = useState('');
  const [keyword, setKeyword] = useState('');
  const [dryRun, setDryRun] = useState('');
  const [startAt, setStartAt] = useState('');
  const [endAt, setEndAt] = useState('');
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailLog, setDetailLog] = useState(null);
  const pageSize = 15;

  const fetchLogs = useCallback(async (p) => {
    setLoading(true);
    try {
      const params = { p: p || page, page_size: pageSize };
      if (dimension) params.dimension = dimension;
      if (source) params.source = source;
      if (keyword) params.keyword = keyword;
      if (dryRun) params.dry_run = dryRun;
      if (startAt) params.start_at = Math.floor(new Date(startAt).getTime() / 1000);
      if (endAt) params.end_at = Math.floor(new Date(endAt).getTime() / 1000);
      const res = await API.get('/api/risk/ban-logs', { params });
      if (res.data.success) {
        setLogs(res.data.data.items);
        setTotal(res.data.data.total);
        setPage(res.data.data.page);
      }
    } catch (err) {
      showError(err);
    } finally {
      setLoading(false);
    }
  }, [page, dimension, source, keyword, dryRun, startAt, endAt]);

  const fetchStats = useCallback(async () => {
    try {
      const res = await API.get('/api/risk/ban-logs/stats');
      if (res.data.success) {
        setStats(res.data.data);
      }
    } catch (err) {
      showError(err);
    }
  }, []);

  useEffect(() => {
    fetchLogs(1);
    fetchStats();
  }, []);

  const fetchDetail = async (id) => {
    try {
      const res = await API.get(`/api/risk/ban-logs/${id}`);
      if (res.data.success) {
        setDetailLog(res.data.data);
        setDetailVisible(true);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    }
  };

  const handleSearch = () => {
    fetchLogs(1);
  };

  const renderSource = (source) => {
    const sourceLabels = {
      probe_guard: t('探针防护'),
      error_ban: t('错误封禁'),
      ip_middleware: t('IP中间件'),
      manual: t('手动'),
    };
    return sourceLabels[source] || source;
  };

  const renderAction = (action) => {
    const actionLabels = {
      temp_ip_ban: t('临时IP封禁'),
      perm_ip_ban: t('永久IP封禁'),
      disable_user: t('禁用用户'),
      both: t('封禁IP+用户'),
    };
    return (
      <Tag color={ACTION_COLORS[action] || 'blue'}>
        {actionLabels[action] || action}
      </Tag>
    );
  };

  const columns = useMemo(
    () => [
      { title: 'ID', dataIndex: 'id', key: 'id', width: 80 },
      { title: t('维度'), dataIndex: 'dimension', key: 'dimension', width: 70 },
      { title: t('IP地址'), dataIndex: 'target_ip', key: 'target_ip', width: 140 },
      {
        title: t('用户名'),
        dataIndex: 'username',
        key: 'username',
        width: 120,
        render: (val) => val || '-',
      },
      {
        title: t('来源'),
        dataIndex: 'source',
        key: 'source',
        width: 100,
        render: (val) => renderSource(val),
      },
      {
        title: t('动作'),
        dataIndex: 'action',
        key: 'action',
        width: 120,
        render: (val) => renderAction(val),
      },
      {
        title: t('理由'),
        dataIndex: 'reason',
        key: 'reason',
        ellipsis: true,
        render: (val) => val || '-',
      },
      {
        title: t('违规次数'),
        dataIndex: 'offense_count',
        key: 'offense_count',
        width: 90,
      },
      {
        title: t('仅记录'),
        dataIndex: 'dry_run',
        key: 'dry_run',
        width: 80,
        render: (val) => (
          <Tag color={val ? 'orange' : 'green'}>{val ? t('是') : t('否')}</Tag>
        ),
      },
      {
        title: t('时间'),
        dataIndex: 'created_at',
        key: 'created_at',
        width: 160,
        render: (val) => (val ? timestamp2string(val) : '-'),
      },
      {
        title: t('操作'),
        key: 'action_btn',
        width: 80,
        render: (_, record) => (
          <Button size='small' onClick={() => fetchDetail(record.id)}>
            {t('详情')}
          </Button>
        ),
      },
    ],
    [t],
  );

  return (
    <div className='flex flex-col gap-4'>
      {/* Stats */}
      {stats && (
        <Card>
          <Title heading={5} className='mb-2'>
            {t('统计数据')}
          </Title>
          <div className='flex flex-wrap gap-4 mb-2'>
            <Text>{t('总记录')}: {stats.total}</Text>
            <Text>{t('仅记录')}: {stats.dry_run_count}</Text>
            <Text>{t('永久封禁')}: {stats.permanent}</Text>
            <Text>{t('今日')}: {stats.today}</Text>
          </div>
          {stats.by_dimension && (
            <div className='flex flex-wrap gap-4'>
              {Object.entries(stats.by_dimension).map(([key, val]) => (
                <Text key={key}>
                  {key === 'ip' ? t('IP') : t('用户')}: {val}
                </Text>
              ))}
            </div>
          )}
        </Card>
      )}

      {/* Filters */}
      <Card>
        <div className='flex flex-wrap gap-3 items-end'>
          <div style={{ minWidth: 120 }}>
            <Select
              placeholder={t('维度')}
              value={dimension}
              onChange={(v) => setDimension(v)}
              optionList={DIMENSION_OPTIONS}
              style={{ width: 120 }}
            />
          </div>
          <div style={{ minWidth: 140 }}>
            <Select
              placeholder={t('来源')}
              value={source}
              onChange={(v) => setSource(v)}
              optionList={SOURCE_OPTIONS}
              style={{ width: 140 }}
            />
          </div>
          <div style={{ minWidth: 120 }}>
            <Select
              placeholder={t('类型')}
              value={dryRun}
              onChange={(v) => setDryRun(v)}
              optionList={DRY_RUN_OPTIONS}
              style={{ width: 120 }}
            />
          </div>
          <div>
            <input
              type='text'
              placeholder={t('搜索关键词')}
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              className='px-3 py-2 border rounded'
              style={{
                border: '1px solid var(--semi-color-border)',
                borderRadius: 'var(--semi-border-radius-small)',
                background: 'var(--semi-color-bg-0)',
                color: 'var(--semi-color-text-0)',
                outline: 'none',
              }}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            />
          </div>
          <div>
            <input
              type='date'
              value={startAt}
              onChange={(e) => setStartAt(e.target.value)}
              className='px-3 py-2 border rounded'
              style={{
                border: '1px solid var(--semi-color-border)',
                borderRadius: 'var(--semi-border-radius-small)',
                background: 'var(--semi-color-bg-0)',
                color: 'var(--semi-color-text-0)',
                outline: 'none',
              }}
            />
          </div>
          <div>
            <input
              type='date'
              value={endAt}
              onChange={(e) => setEndAt(e.target.value)}
              className='px-3 py-2 border rounded'
              style={{
                border: '1px solid var(--semi-color-border)',
                borderRadius: 'var(--semi-border-radius-small)',
                background: 'var(--semi-color-bg-0)',
                color: 'var(--semi-color-text-0)',
                outline: 'none',
              }}
            />
          </div>
          <Button type='primary' onClick={handleSearch}>
            {t('搜索')}
          </Button>
          <Button
            onClick={() => {
              setDimension('');
              setSource('');
              setKeyword('');
              setDryRun('');
              setStartAt('');
              setEndAt('');
              setTimeout(() => fetchLogs(1), 0);
            }}
          >
            {t('重置')}
          </Button>
        </div>
      </Card>

      {/* Table */}
      <Card>
        <CardTable
          columns={columns}
          dataSource={logs}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: page,
            pageSize: pageSize,
            total: total,
            onChange: (p) => fetchLogs(p),
          }}
        />
      </Card>

      {/* Detail modal */}
      <Modal
        title={t('封禁日志详情')}
        visible={detailVisible}
        onCancel={() => {
          setDetailVisible(false);
          setDetailLog(null);
        }}
        footer={
          <Button onClick={() => setDetailVisible(false)}>{t('关闭')}</Button>
        }
        style={{ width: 640 }}
      >
        {detailLog && (
          <Descriptions
            data={[
              { key: 'ID', value: detailLog.id },
              { key: t('维度'), value: detailLog.dimension },
              { key: t('IP地址'), value: detailLog.target_ip || '-' },
              { key: t('用户ID'), value: detailLog.user_id || '-' },
              { key: t('用户名'), value: detailLog.username || '-' },
              { key: t('来源'), value: renderSource(detailLog.source) },
              { key: t('规则ID'), value: detailLog.rule_id || '-' },
              { key: t('规则名称'), value: detailLog.rule_name || '-' },
              { key: t('动作'), value: renderAction(detailLog.action) },
              { key: t('时长（分钟）'), value: detailLog.duration_minutes || '-' },
              {
                key: t('永久封禁'),
                value: detailLog.is_permanent ? t('是') : t('否'),
              },
              {
                key: t('解封时间'),
                value: detailLog.unban_at
                  ? timestamp2string(detailLog.unban_at)
                  : '-',
              },
              { key: t('违规次数'), value: detailLog.offense_count },
              { key: t('理由'), value: detailLog.reason || '-' },
              { key: t('仅记录'), value: detailLog.dry_run ? t('是') : t('否') },
              {
                key: t('创建时间'),
                value: timestamp2string(detailLog.created_at),
              },
              { key: t('请求ID'), value: detailLog.request_id || '-' },
            ]}
            column={1}
          />
        )}
      </Modal>
    </div>
  );
};

export default BanLogsTab;
