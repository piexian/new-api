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

import React, { useCallback, useEffect, useState } from 'react';
import { Button, Card, Tag, Typography } from '@douyinfe/semi-ui';
import { API, renderQuota } from '../../../helpers';
import QueryError from './QueryError';

// 日利率展示与 default 主题一致（percent，最多两位小数）
const formatDailyRate = (rate) => {
  if (typeof rate !== 'number' || isNaN(rate)) return '-';
  return Intl.NumberFormat(undefined, {
    style: 'percent',
    maximumFractionDigits: 2,
  }).format(rate);
};

// 秒结清惩罚条款展示：0 = 不收；窗口 0 = 仅当天
const penaltyText = (t, offer) => {
  if (!offer.fast_repay_penalty_quota) return t('不收');
  const windowText =
    offer.fast_repay_window_days === 0
      ? t('仅当天')
      : t('{{days}} 天', { days: offer.fast_repay_window_days });
  return `${renderQuota(offer.fast_repay_penalty_quota)} · ${windowText}`;
};

const OfferStat = ({ label, value, strong }) => (
  <div>
    <div className='text-xs text-gray-500 dark:text-gray-400'>{label}</div>
    <div className={`tabular-nums ${strong ? 'font-medium' : ''}`}>{value}</div>
  </div>
);

/**
 * 市场浏览：其他放贷人的公开订单挂单列表；点击"借这笔"选中挂单并跳转到借款表单。
 */
const MarketBrowse = ({ t, onBorrow, refreshKey }) => {
  const [offers, setOffers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const fetchOffers = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/user/loan/market/list');
      const { success, message, data } = res.data;
      if (success) {
        setOffers(data?.offers || []);
      } else {
        setError(message || t('获取市场挂单失败'));
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
      setError(t('获取市场挂单失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchOffers();
  }, [fetchOffers, refreshKey]);

  return (
    <>
      <div className='mb-4'>
        <Typography.Text strong>{t('市场浏览')}</Typography.Text>
        <div className='text-xs text-gray-500 dark:text-gray-400'>
          {t('其他放贷人的公开订单挂单；可直接向其中一笔借款。')}
        </div>
      </div>

      {error ? (
        <QueryError t={t} message={error} onRetry={fetchOffers} />
      ) : loading && offers.length === 0 ? (
        <div className='py-8 text-center text-sm text-gray-500'>
          {t('加载中...')}
        </div>
      ) : offers.length === 0 ? (
        <div className='py-8 text-center text-sm text-gray-500'>
          {t('当前没有公开挂单')}
        </div>
      ) : (
        <div className='space-y-2'>
          {offers.map((offer) => (
            <Card
              key={offer.id}
              className='!rounded-xl'
              bodyStyle={{ padding: '12px' }}
            >
              <div className='grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'>
                <div className='grid gap-2 text-sm sm:grid-cols-5'>
                  <OfferStat
                    label={t('可用')}
                    value={renderQuota(offer.amount_available || 0)}
                    strong
                  />
                  <OfferStat
                    label={t('固定利率')}
                    value={formatDailyRate(offer.rate_fixed)}
                  />
                  <OfferStat
                    label={t('信用门槛')}
                    value={
                      offer.min_credit_score === -50
                        ? t('不限')
                        : offer.min_credit_score
                    }
                  />
                  <OfferStat
                    label={t('放贷人信用分')}
                    value={offer.lender_credit_score}
                  />
                  <OfferStat
                    label={t('秒结清惩罚')}
                    value={penaltyText(t, offer)}
                  />
                </div>
                <div className='flex items-center justify-end gap-2'>
                  <Tag size='small'>{t('订单')}</Tag>
                  <Button
                    type='primary'
                    theme='solid'
                    size='small'
                    onClick={() => onBorrow(offer)}
                  >
                    {t('借这笔')}
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </>
  );
};

export default MarketBrowse;
