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

import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Card,
  Table,
  Tabs,
  TabPane,
  Tag,
  Skeleton,
  Empty,
  Spin,
  Typography,
  Tooltip,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { IconArrowUp, IconArrowDown, IconMinus } from '@douyinfe/semi-icons';
import { TrendingUp, TrendingDown, BarChart3, Trophy } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../helpers';
import { renderNumber, getLobeHubIcon } from '../helpers/render';

const { Title, Paragraph, Text } = Typography;

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

function formatTokens(value) {
  if (!Number.isFinite(value) || value <= 0) return '0';
  if (value >= 1_000_000_000_000)
    return `${(value / 1_000_000_000_000).toFixed(2)}T`;
  if (value >= 1_000_000_000)
    return `${(value / 1_000_000_000).toFixed(value >= 10_000_000_000 ? 1 : 2)}B`;
  if (value >= 1_000_000)
    return `${(value / 1_000_000).toFixed(value >= 10_000_000 ? 1 : 2)}M`;
  if (value >= 1_000)
    return `${(value / 1_000).toFixed(value >= 10_000 ? 0 : 1)}K`;
  return value.toLocaleString();
}

function formatShare(share) {
  if (!Number.isFinite(share) || share <= 0) return '0%';
  if (share < 0.001) return '<0.1%';
  return `${(share * 100).toFixed(share < 0.01 ? 2 : 1)}%`;
}

function formatGrowth(pct) {
  if (!Number.isFinite(pct)) return '-';
  const sign = pct > 0 ? '+' : '';
  return `${sign}${pct.toFixed(1)}%`;
}

// ---------------------------------------------------------------------------
// Vendor colour palette
// ---------------------------------------------------------------------------

const VENDOR_COLOURS = {
  OpenAI: '#10a37f',
  Anthropic: '#d97757',
  Google: '#4285f4',
  DeepSeek: '#7c5cff',
  Alibaba: '#ff9900',
  xAI: '#1f2937',
  Meta: '#1877f2',
  Moonshot: '#ec4899',
  Zhipu: '#06b6d4',
  Mistral: '#ff7000',
  ByteDance: '#3b82f6',
  Tencent: '#22c55e',
  MiniMax: '#a855f7',
  Cohere: '#fb923c',
  Baidu: '#ef4444',
  Others: '#94a3b8',
};

const FALLBACK_PALETTE = [
  '#0ea5e9', '#22c55e', '#a855f7', '#f97316', '#14b8a6',
  '#eab308', '#ec4899', '#84cc16', '#6366f1', '#10b981',
  '#f43f5e', '#0891b2', '#94a3b8',
];

function getVendorColour(name, idx) {
  return VENDOR_COLOURS[name] ?? FALLBACK_PALETTE[idx % FALLBACK_PALETTE.length];
}

// ---------------------------------------------------------------------------
// Period options
// ---------------------------------------------------------------------------

const PERIODS = [
  { id: 'today', label: '今天' },
  { id: 'week', label: '本周' },
  { id: 'month', label: '本月' },
  { id: 'year', label: '今年' },
  { id: 'all', label: '全部' },
];

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

/** Rank number with delta arrow */
function RankCell({ rank, previousRank }) {
  const delta =
    previousRank != null ? previousRank - rank : null;
  const isNew = previousRank == null;

  return (
    <div className='flex items-center gap-1'>
      <span className='font-mono font-semibold tabular-nums'>{rank}</span>
      {isNew && rank > 1 && (
        <Tag size='small' color='blue'>
          New
        </Tag>
      )}
      {delta != null && delta > 0 && (
        <span className='flex items-center text-xs text-emerald-500'>
          <IconArrowUp size='small' />
          {delta}
        </span>
      )}
      {delta != null && delta < 0 && (
        <span className='flex items-center text-xs text-rose-500'>
          <IconArrowDown size='small' />
          {Math.abs(delta)}
        </span>
      )}
      {delta === 0 && (
        <span className='text-xs text-gray-400'>
          <IconMinus size='small' />
        </span>
      )}
    </div>
  );
}

/** Growth text with colour */
function GrowthCell({ pct }) {
  if (!Number.isFinite(pct)) return <span className='text-gray-400'>-</span>;
  const isPositive = pct > 0;
  const isNegative = pct < 0;
  const color = isPositive
    ? 'text-emerald-500'
    : isNegative
      ? 'text-rose-500'
      : 'text-gray-500';
  return (
    <span className={`font-mono tabular-nums text-xs ${color}`}>
      {formatGrowth(pct)}
    </span>
  );
}

/** Model leaderboard table */
function ModelLeaderboard({ rows, t }) {
  const columns = useMemo(
    () => [
      {
        title: '#',
        dataIndex: 'rank',
        key: 'rank',
        width: 80,
        render: (rank, row) => <RankCell rank={rank} previousRank={row.previous_rank} />,
      },
      {
        title: t('模型'),
        dataIndex: 'model_name',
        key: 'model_name',
        render: (name, row) => (
          <div className='flex items-center gap-2'>
            <span className='shrink-0'>
              {getLobeHubIcon(row.vendor_icon, 20)}
            </span>
            <div className='min-w-0'>
              <div className='font-mono text-sm font-medium truncate'>{name}</div>
              <div className='text-xs text-gray-500 truncate'>{row.vendor}</div>
            </div>
          </div>
        ),
      },
      {
        title: t('总 Token'),
        dataIndex: 'total_tokens',
        key: 'total_tokens',
        width: 120,
        sorter: (a, b) => a.total_tokens - b.total_tokens,
        render: (val) => (
          <span className='font-mono tabular-nums font-semibold'>
            {formatTokens(val)}
          </span>
        ),
      },
      {
        title: t('份额'),
        dataIndex: 'share',
        key: 'share',
        width: 90,
        sorter: (a, b) => a.share - b.share,
        render: (val) => (
          <span className='font-mono tabular-nums text-sm'>{formatShare(val)}</span>
        ),
      },
      {
        title: t('增长'),
        dataIndex: 'growth_pct',
        key: 'growth_pct',
        width: 90,
        sorter: (a, b) => a.growth_pct - b.growth_pct,
        render: (val) => <GrowthCell pct={val} />,
      },
    ],
    [t],
  );

  return (
    <Table
      columns={columns}
      dataSource={rows}
      rowKey='model_name'
      pagination={{ pageSize: 20, showSizeChanger: false }}
      size='middle'
      empty={
        <Empty
          image={<IllustrationNoResult style={{ width: 100, height: 100 }} />}
          darkModeImage={<IllustrationNoResultDark style={{ width: 100, height: 100 }} />}
          description={t('暂无数据')}
        />
      }
    />
  );
}

/** Vendor share list */
function VendorShareList({ rows, t }) {
  if (!rows || rows.length === 0) {
    return (
      <Empty
        image={<IllustrationNoResult style={{ width: 100, height: 100 }} />}
        darkModeImage={<IllustrationNoResultDark style={{ width: 100, height: 100 }} />}
        description={t('暂无数据')}
      />
    );
  }

  return (
    <div className='grid grid-cols-1 md:grid-cols-2 gap-x-8'>
      {rows.map((vendor, idx) => (
        <div key={vendor.vendor} className='flex items-center gap-3 py-2.5'>
          <span className='text-gray-400 w-6 shrink-0 text-right font-mono text-xs tabular-nums'>
            {vendor.rank}.
          </span>
          <span
            aria-hidden
            className='w-2.5 h-2.5 shrink-0 rounded-full'
            style={{ backgroundColor: getVendorColour(vendor.vendor, idx) }}
          />
          <div className='min-w-0 flex-1 truncate'>
            <span className='font-medium text-sm'>{vendor.vendor}</span>
            <span className='text-gray-400 text-xs ml-2'>
              {vendor.models_count} {t('个模型')}
            </span>
          </div>
          <div className='shrink-0 text-right'>
            <div className='font-mono text-sm font-semibold tabular-nums'>
              {formatTokens(vendor.total_tokens)}
            </div>
            <div className='text-gray-400 font-mono text-xs tabular-nums'>
              {formatShare(vendor.share)}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

/** Vendor share bar chart (simple CSS bars) */
function VendorShareBars({ rows }) {
  if (!rows || rows.length === 0) return null;
  const maxTokens = Math.max(...rows.map((r) => r.total_tokens), 1);
  return (
    <div className='space-y-2'>
      {rows.slice(0, 8).map((vendor, idx) => (
        <div key={vendor.vendor} className='flex items-center gap-2'>
          <span className='w-24 shrink-0 truncate text-xs text-gray-500'>
            {vendor.vendor}
          </span>
          <div className='flex-1 h-5 bg-gray-100 dark:bg-gray-800 rounded overflow-hidden'>
            <div
              className='h-full rounded transition-all duration-500'
              style={{
                width: `${Math.max((vendor.total_tokens / maxTokens) * 100, 1)}%`,
                backgroundColor: getVendorColour(vendor.vendor, idx),
              }}
            />
          </div>
          <span className='w-14 shrink-0 text-right font-mono text-xs tabular-nums'>
            {formatShare(vendor.share)}
          </span>
        </div>
      ))}
    </div>
  );
}

/** Movers / droppers cards */
function MoversCard({ title, icon, items, intent, t }) {
  const emptyLabel =
    intent === 'up'
      ? t('暂无明显上升趋势')
      : t('暂无明显下降趋势');
  return (
    <Card
      className='!rounded-2xl'
      title={
        <div className='flex items-center gap-2'>
          {icon}
          <span className='text-sm font-semibold'>{title}</span>
        </div>
      }
      bodyStyle={{ padding: '8px 16px' }}
    >
      {items.length === 0 ? (
        <div className='text-center text-gray-400 py-6 text-xs'>{emptyLabel}</div>
      ) : (
        <ul>
          {items.map((row) => (
            <li key={row.model_name} className='flex items-center gap-3 py-2'>
              <span className='shrink-0'>
                {getLobeHubIcon(row.vendor_icon, 18)}
              </span>
              <div className='min-w-0 flex-1'>
                <div className='font-mono text-xs font-medium truncate'>
                  {row.model_name}
                </div>
                <div className='text-gray-400 text-xs truncate'>
                  #{row.current_rank} · {row.vendor}
                </div>
              </div>
              <span
                className={`inline-flex shrink-0 items-center gap-0.5 font-mono text-xs font-semibold tabular-nums ${
                  intent === 'up'
                    ? 'text-emerald-500'
                    : 'text-rose-500'
                }`}
              >
                {intent === 'up' ? (
                  <IconArrowUp size='small' />
                ) : (
                  <IconArrowDown size='small' />
                )}
                {Math.abs(row.rank_delta)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Skeleton loading placeholder
// ---------------------------------------------------------------------------

function RankingsSkeleton() {
  return (
    <div className='space-y-4'>
      <Skeleton.Title />
      <Skeleton.Paragraph rows={2} />
      <Card className='!rounded-2xl'>
        <Skeleton.Title />
        <Skeleton.Paragraph rows={8} />
      </Card>
      <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
        <Card className='!rounded-2xl'>
          <Skeleton.Title />
          <Skeleton.Paragraph rows={5} />
        </Card>
        <Card className='!rounded-2xl'>
          <Skeleton.Title />
          <Skeleton.Paragraph rows={5} />
        </Card>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Rankings component
// ---------------------------------------------------------------------------

const Rankings = () => {
  const { t } = useTranslation();
  const [period, setPeriod] = useState('week');
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchData = useCallback(async (p) => {
    setLoading(true);
    setError(null);
    try {
      const res = await API.get('/api/rankings', {
        params: { period: p },
      });
      const { success, message, data: snapshot } = res.data;
      if (success) {
        setData(snapshot);
      } else {
        setError(message || t('加载排行榜数据失败'));
        showError(message);
      }
    } catch (err) {
      setError(t('加载排行榜数据失败'));
      showError(err);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchData(period);
  }, [period, fetchData]);

  const handlePeriodChange = (key) => {
    setPeriod(key);
  };

  return (
    <div className='mt-[60px] px-2 max-w-[1280px] mx-auto'>
      {/* Hero section */}
      <div className='mb-6'>
        <Title heading={2} style={{ marginBottom: 4 }}>
          <Trophy size={24} className='inline mr-2' style={{ verticalAlign: 'middle' }} />
          {t('排行榜')}
        </Title>
        <Paragraph type='tertiary' size='small'>
          {t('发现平台上最受欢迎的模型和供应商，数据基于实时使用情况更新。')}
        </Paragraph>
      </div>

      {/* Period selector */}
      <div className='mb-4'>
        <Tabs
          type='button'
          activeKey={period}
          onChange={handlePeriodChange}
          className='!mb-0'
        >
          {PERIODS.map((p) => (
            <TabPane tab={t(p.label)} itemKey={p.id} key={p.id} />
          ))}
        </Tabs>
      </div>

      {/* Content */}
      {loading && !data ? (
        <RankingsSkeleton />
      ) : error && !data ? (
        <Card className='!rounded-2xl'>
          <Empty
            image={<IllustrationNoResult style={{ width: 120, height: 120 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 120, height: 120 }} />
            }
            description={error}
          />
        </Card>
      ) : data ? (
        <Spin spinning={loading} tip={t('加载中...')}>
          <div className='space-y-4'>
            {/* Model leaderboard */}
            <Card
              className='!rounded-2xl'
              title={
                <div className='flex items-center gap-2'>
                  <BarChart3 size={16} />
                  {t('模型排行榜')}
                </div>
              }
            >
              <ModelLeaderboard rows={data.models || []} t={t} />
            </Card>

            {/* Vendor market share */}
            <Card
              className='!rounded-2xl'
              title={
                <div className='flex items-center gap-2'>
                  <TrendingUp size={16} />
                  {t('市场份额')}
                </div>
              }
            >
              <VendorShareBars rows={data.vendors || []} />
              <div className='mt-4 border-t pt-4'>
                <h4 className='text-sm font-semibold mb-3'>{t('供应商排行')}</h4>
                <VendorShareList rows={data.vendors || []} t={t} />
              </div>
            </Card>

            {/* Movers and droppers */}
            <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
              <MoversCard
                title={t('上升趋势')}
                icon={<TrendingUp size={16} className='text-emerald-500' />}
                items={data.top_movers || []}
                intent='up'
                t={t}
              />
              <MoversCard
                title={t('下降趋势')}
                icon={<TrendingDown size={16} className='text-rose-500' />}
                items={data.top_droppers || []}
                intent='down'
                t={t}
              />
            </div>
          </div>
        </Spin>
      ) : null}
    </div>
  );
};

export default Rankings;
