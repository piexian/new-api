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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Avatar,
  Banner,
  Empty,
  Progress,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { Activity, Gauge, Timer, Zap } from 'lucide-react';
import { API } from '../../../../../helpers';
import {
  average,
  clampPercent,
  formatLatency,
  formatPercent,
  formatThroughput,
  successColor,
  toNumber,
} from '../../shared/performanceFormat';

const { Text } = Typography;

const HOURS = 24;

const aggregateSeries = (groups, field, { includeZero = false } = {}) => {
  const buckets = new Map();

  groups.forEach((group) => {
    const series = Array.isArray(group.series) ? group.series : [];
    series.forEach((point) => {
      const ts = toNumber(point.ts);
      const value = toNumber(point[field]);
      if (ts === null || value === null || (!includeZero && value <= 0)) {
        return;
      }

      const key = String(ts);
      const current = buckets.get(key) || [];
      current.push(value);
      buckets.set(key, current);
    });
  });

  return Array.from(buckets.entries())
    .map(([ts, values]) => ({
      ts: Number(ts),
      value: average(values, { includeZero }),
    }))
    .filter((point) => point.value !== null)
    .sort((a, b) => a.ts - b.ts);
};

const formatPointTime = (ts) => {
  if (!ts) {
    return '';
  }
  return new Date(ts * 1000).toLocaleString();
};

const MetricCard = ({ icon: Icon, label, value, hint, color }) => (
  <div
    className='rounded-xl p-3'
    style={{
      border: '1px solid var(--semi-color-border)',
      background: 'var(--semi-color-bg-0)',
    }}
  >
    <div className='flex items-center gap-2'>
      <span
        className='inline-flex h-8 w-8 items-center justify-center rounded-full'
        style={{ background: `${color}1a`, color }}
      >
        <Icon size={16} />
      </span>
      <Text type='secondary' size='small'>
        {label}
      </Text>
    </div>
    <div className='mt-3 text-xl font-semibold'>{value}</div>
    <div className='mt-1 text-xs text-gray-500'>{hint}</div>
  </div>
);

const MiniBarTrend = ({ series, emptyText, formatValue, color }) => {
  const points = series.slice(-HOURS);
  if (points.length === 0) {
    return (
      <div
        className='flex h-20 items-center justify-center rounded-xl text-sm text-gray-500'
        style={{
          border: '1px dashed var(--semi-color-border)',
          background: 'var(--semi-color-bg-1)',
        }}
      >
        {emptyText}
      </div>
    );
  }

  const maxValue = Math.max(...points.map((point) => point.value), 1);

  return (
    <div
      className='flex h-20 items-end gap-1 rounded-xl p-2'
      style={{
        border: '1px solid var(--semi-color-border)',
        background: 'var(--semi-color-bg-1)',
      }}
    >
      {points.map((point) => (
        <span
          key={point.ts}
          className='min-w-[4px] flex-1 rounded-sm'
          title={`${formatPointTime(point.ts)} ${formatValue(point.value)}`}
          style={{
            height: `${Math.max(10, (point.value / maxValue) * 100)}%`,
            background: color,
          }}
        />
      ))}
    </div>
  );
};

const AvailabilityBlocks = ({ series, emptyText }) => {
  const points = series.slice(-HOURS);
  if (points.length === 0) {
    return (
      <div
        className='flex h-20 items-center justify-center rounded-xl text-sm text-gray-500'
        style={{
          border: '1px dashed var(--semi-color-border)',
          background: 'var(--semi-color-bg-1)',
        }}
      >
        {emptyText}
      </div>
    );
  }

  return (
    <div
      className='flex h-20 items-center gap-1 rounded-xl p-2'
      style={{
        border: '1px solid var(--semi-color-border)',
        background: 'var(--semi-color-bg-1)',
      }}
    >
      {points.map((point) => (
        <span
          key={point.ts}
          className='h-9 min-w-[4px] flex-1 rounded-sm'
          title={`${formatPointTime(point.ts)} ${formatPercent(point.value)}`}
          style={{ background: successColor(point.value) }}
        />
      ))}
    </div>
  );
};

const ModelPerformancePanel = ({ modelData, t }) => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [groups, setGroups] = useState([]);

  useEffect(() => {
    let cancelled = false;
    const modelName = modelData?.model_name;

    if (!modelName) {
      setGroups([]);
      return undefined;
    }

    const loadMetrics = async () => {
      setLoading(true);
      setError('');
      try {
        const res = await API.get('/api/perf-metrics', {
          params: {
            model: modelName,
            hours: HOURS,
          },
          skipErrorHandler: true,
        });

        if (cancelled) {
          return;
        }

        if (!res.data?.success) {
          throw new Error(res.data?.message || t('获取模型监控数据失败'));
        }

        const nextGroups = Array.isArray(res.data?.data?.groups)
          ? res.data.data.groups
          : [];
        setGroups(nextGroups);
      } catch (err) {
        if (cancelled) {
          return;
        }
        setGroups([]);
        setError(
          err?.response?.data?.message ||
            err?.message ||
            t('获取模型监控数据失败'),
        );
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    loadMetrics();

    return () => {
      cancelled = true;
    };
  }, [modelData?.model_name, t]);

  const latencySeries = useMemo(
    () => aggregateSeries(groups, 'avg_latency_ms'),
    [groups],
  );
  const availabilitySeries = useMemo(
    () => aggregateSeries(groups, 'success_rate', { includeZero: true }),
    [groups],
  );

  const summary = useMemo(() => {
    const avgTps = average(groups.map((group) => toNumber(group.avg_tps)));
    const avgLatency = average(
      groups.map((group) => toNumber(group.avg_latency_ms)),
    );
    const successRate = average(
      groups.map((group) => toNumber(group.success_rate)),
      { includeZero: true },
    );
    const incidentBuckets = groups.reduce((total, group) => {
      const series = Array.isArray(group.series) ? group.series : [];
      return (
        total +
        series.filter((point) => {
          const rate = toNumber(point.success_rate);
          return rate !== null && rate < 100;
        }).length
      );
    }, 0);

    return {
      avgTps,
      avgLatency,
      successRate,
      incidentBuckets,
    };
  }, [groups]);

  const tableData = useMemo(
    () =>
      groups.map((group) => ({
        key: group.group || 'default',
        group: group.group || 'default',
        avg_tps: toNumber(group.avg_tps),
        avg_ttft_ms: toNumber(group.avg_ttft_ms),
        avg_latency_ms: toNumber(group.avg_latency_ms),
        success_rate: toNumber(group.success_rate),
      })),
    [groups],
  );

  const columns = [
    {
      title: t('分组'),
      dataIndex: 'group',
      width: 110,
      render: (group) => (
        <Tag color='white' size='small' shape='circle'>
          {group}
          {t('分组')}
        </Tag>
      ),
    },
    {
      title: t('TPS'),
      dataIndex: 'avg_tps',
      width: 92,
      render: (value) => (
        <Text className='font-medium'>{formatThroughput(value)}</Text>
      ),
    },
    {
      title: t('平均 TTFT'),
      dataIndex: 'avg_ttft_ms',
      width: 110,
      render: (value) => formatLatency(value),
    },
    {
      title: t('平均延迟'),
      dataIndex: 'avg_latency_ms',
      width: 110,
      render: (value) => formatLatency(value),
    },
    {
      title: t('成功率'),
      dataIndex: 'success_rate',
      width: 150,
      render: (value) => (
        <div className='min-w-[120px]'>
          <div className='mb-1 flex items-center justify-between gap-2'>
            <Text size='small'>{formatPercent(value)}</Text>
          </div>
          <Progress
            percent={clampPercent(value)}
            stroke={successColor(value)}
            aria-label='model success rate'
            showInfo={false}
            style={{ margin: 0 }}
          />
        </div>
      ),
    },
  ];

  return (
    <div>
      <div className='mb-4 flex items-center'>
        <Avatar size='small' color='green' className='mr-2 shadow-md'>
          <Activity size={16} />
        </Avatar>
        <div>
          <Text className='text-lg font-medium'>{t('性能监控')}</Text>
          <div className='text-xs text-gray-600'>
            {t('最近24小时的模型请求性能')}
          </div>
        </div>
      </div>

      {loading && (
        <div className='flex items-center justify-center py-12'>
          <Spin tip={t('加载模型监控数据...')} />
        </div>
      )}

      {!loading && error && (
        <Banner
          type='danger'
          description={error}
          closeIcon={null}
          className='!rounded-xl'
        />
      )}

      {!loading && !error && groups.length === 0 && (
        <div className='py-8'>
          <Empty
            description={
              <div>
                <div>{t('暂无模型监控数据')}</div>
                <div className='mt-1 text-xs text-gray-500'>
                  {t('该模型最近24小时暂无可用请求样本')}
                </div>
              </div>
            }
          />
        </div>
      )}

      {!loading && !error && groups.length > 0 && (
        <div className='space-y-5'>
          <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
            <MetricCard
              icon={Zap}
              label={t('持续输出吞吐')}
              value={formatThroughput(summary.avgTps)}
              hint={t('按生成耗时聚合')}
              color='#2563eb'
            />
            <MetricCard
              icon={Timer}
              label={t('平均延迟')}
              value={formatLatency(summary.avgLatency)}
              hint={t('请求完成耗时')}
              color='#9333ea'
            />
            <MetricCard
              icon={Gauge}
              label={t('请求成功率')}
              value={formatPercent(summary.successRate)}
              hint={t('按分组平均')}
              color={successColor(summary.successRate)}
            />
            <MetricCard
              icon={Activity}
              label={t('异常桶')}
              value={summary.incidentBuckets}
              hint={
                summary.incidentBuckets > 0
                  ? t('最近24小时 {{count}} 个异常桶', {
                      count: summary.incidentBuckets,
                    })
                  : t('最近24小时无异常桶')
              }
              color={summary.incidentBuckets > 0 ? '#d97706' : '#16a34a'}
            />
          </div>

          <div>
            <div className='mb-3'>
              <Text className='font-medium'>{t('分组性能')}</Text>
              <div className='text-xs text-gray-600'>
                {t('各分组的平均延迟、首 Token 延迟、吞吐和成功率')}
              </div>
            </div>
            <Table
              dataSource={tableData}
              columns={columns}
              pagination={false}
              size='small'
              bordered={false}
              scroll={{ x: 560 }}
              className='!rounded-lg'
            />
          </div>

          <div className='grid grid-cols-1 gap-4'>
            <div>
              <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
                <Text className='font-medium'>{t('延迟趋势（24h）')}</Text>
                <Text type='secondary' size='small'>
                  {t('按小时聚合的平均响应延迟')}
                </Text>
              </div>
              <MiniBarTrend
                series={latencySeries}
                emptyText={t('无延迟趋势数据')}
                formatValue={formatLatency}
                color='#2563eb'
              />
            </div>

            <div>
              <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
                <Text className='font-medium'>{t('可用性（24h）')}</Text>
                <Text type='secondary' size='small'>
                  {t('按小时聚合的请求成功率')}
                </Text>
              </div>
              <AvailabilityBlocks
                series={availabilitySeries}
                emptyText={t('无可用性趋势数据')}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ModelPerformancePanel;
