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
import { Button, Modal, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { Plus } from 'lucide-react';
import {
  API,
  renderQuota,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../../../helpers';
import QueryError from './QueryError';
import CreateOfferDialog from './CreateOfferDialog';
import LenderDisclaimerModal from './LenderDisclaimerModal';

const modeLabel = (t, mode) => {
  if (mode === 'pool') return t('资金池');
  if (mode === 'ai') return t('AI');
  return t('订单');
};

const statusLabel = (t, status) => {
  if (status === 'active') return t('生效');
  if (status === 'paused') return t('已暂停');
  return t('已关闭');
};

const statusTag = (t, status) => {
  const color =
    status === 'active' ? 'green' : status === 'paused' ? 'orange' : 'grey';
  return (
    <Tag color={color} size='small'>
      {statusLabel(t, status)}
    </Tag>
  );
};

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
  return `${renderQuota(offer.fast_repay_penalty_quota)} / ${windowText}`;
};

const actionSuccessMessage = (t, action) => {
  if (action === 'pause') return t('挂单已暂停');
  if (action === 'resume') return t('挂单已恢复');
  if (action === 'close') return t('挂单已关闭');
  return t('挂单额度已撤回');
};

const confirmTitle = (t, action) => {
  if (action === 'pause') return t('暂停挂单');
  if (action === 'resume') return t('恢复挂单');
  if (action === 'close') return t('关闭挂单');
  return t('撤回挂单');
};

const confirmDescription = (t, offer, action) => {
  const available = renderQuota(offer.amount_available || 0);
  if (action === 'pause') {
    return t('暂停该挂单？暂停后不再参与新的撮合。');
  }
  if (action === 'resume') {
    return t('恢复该挂单？恢复后重新参与撮合。');
  }
  if (action === 'close') {
    return t('关闭该挂单？此操作不可撤销，闲置额度将退回余额。');
  }
  return t('将 {{available}} 闲置额度撤回余额？挂单保持当前状态。', {
    available,
  });
};

/**
 * 我的供给：用户自己的放贷挂单列表 + 创建挂单 / 暂停 / 恢复 / 撤回 / 关闭。
 */
const MyOffers = ({ t, disclaimerAgreed, onDisclaimerAgreed, refreshKey }) => {
  const [offers, setOffers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [createVisible, setCreateVisible] = useState(false);
  const [disclaimerVisible, setDisclaimerVisible] = useState(false);
  const [confirmTarget, setConfirmTarget] = useState(null); // { offer, action }

  const fetchOffers = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/user/loan/market/offers');
      const { success, message, data } = res.data;
      if (success) {
        setOffers(data?.offers || []);
      } else {
        setError(message || t('获取挂单列表失败'));
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
      setError(t('获取挂单列表失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchOffers();
  }, [fetchOffers, refreshKey]);

  const handleCreateClick = () => {
    if (!disclaimerAgreed) {
      setDisclaimerVisible(true);
      return;
    }
    setCreateVisible(true);
  };

  const runAction = async (offer, action, onSuccess) => {
    try {
      let res;
      if (action === 'pause') {
        res = await API.post(`/api/user/loan/market/offers/${offer.id}/pause`);
      } else if (action === 'resume') {
        res = await API.post(`/api/user/loan/market/offers/${offer.id}/resume`);
      } else if (action === 'close') {
        res = await API.post(`/api/user/loan/market/offers/${offer.id}/close`);
      } else {
        res = await API.post(
          `/api/user/loan/market/offers/${offer.id}/withdraw`,
        );
      }
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(actionSuccessMessage(t, action));
        onSuccess?.(data);
        fetchOffers();
        return;
      }
      showError(message);
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
    }
  };

  const handleConfirm = async () => {
    const target = confirmTarget;
    if (!target) return;
    setConfirmTarget(null);
    const { offer, action } = target;
    if (action === 'withdraw') {
      await runAction(offer, action, (result) => {
        if (result?.refunded !== undefined) {
          showInfo(
            t('{{amount}} 已退回您的余额', {
              amount: renderQuota(result.refunded),
            }),
          );
        }
      });
      return;
    }
    await runAction(offer, action);
  };

  const columns = useMemo(
    () => [
      {
        title: t('模式'),
        dataIndex: 'mode',
        render: (v, record) => (
          <div className='flex flex-col gap-0.5'>
            <Tag size='small'>{modeLabel(t, v)}</Tag>
            {v === 'ai' && record.per_loan_cap > 0 ? (
              <span className='text-xs text-gray-500 dark:text-gray-400'>
                {t('单笔上限')}: {renderQuota(record.per_loan_cap)}
              </span>
            ) : null}
          </div>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (v) => statusTag(t, v),
      },
      {
        title: t('挂出总额'),
        dataIndex: 'amount_total',
        render: (v) => (
          <span className='font-medium tabular-nums'>
            {renderQuota(v || 0)}
          </span>
        ),
      },
      {
        title: t('可用'),
        dataIndex: 'amount_available',
        render: (v) => (
          <span className='tabular-nums'>{renderQuota(v || 0)}</span>
        ),
      },
      {
        title: t('累计放出'),
        dataIndex: 'total_lent',
        render: (v) => (
          <span className='tabular-nums'>{renderQuota(v || 0)}</span>
        ),
      },
      {
        title: t('累计利息'),
        dataIndex: 'total_interest_earned',
        render: (v) => (
          <span className='tabular-nums'>{renderQuota(v || 0)}</span>
        ),
      },
      {
        title: t('利率'),
        dataIndex: 'rate_fixed',
        render: (v, record) => (
          <span className='text-gray-500 dark:text-gray-400'>
            {record.mode === 'ai'
              ? `${formatDailyRate(record.rate_min)} – ${formatDailyRate(record.rate_max)}`
              : formatDailyRate(v)}
          </span>
        ),
      },
      {
        title: t('秒结清惩罚'),
        dataIndex: 'fast_repay_penalty_quota',
        render: (v, record) => (
          <span className='tabular-nums'>{penaltyText(t, record)}</span>
        ),
      },
      {
        title: t('信用门槛'),
        dataIndex: 'min_credit_score',
        render: (v) => (
          <span className='tabular-nums text-gray-500 dark:text-gray-400'>
            {v === -50 ? t('不限') : v}
          </span>
        ),
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        render: (v) => (
          <span className='text-xs text-gray-500 dark:text-gray-400'>
            {timestamp2string(v)}
          </span>
        ),
      },
      {
        title: t('操作'),
        dataIndex: 'id',
        render: (v, record) => (
          <div className='flex flex-wrap items-center justify-end gap-1.5'>
            {record.status === 'active' ? (
              <Button
                theme='outline'
                size='small'
                onClick={() =>
                  setConfirmTarget({ offer: record, action: 'pause' })
                }
              >
                {t('暂停')}
              </Button>
            ) : null}
            {record.status === 'paused' ? (
              <Button
                theme='outline'
                size='small'
                onClick={() =>
                  setConfirmTarget({ offer: record, action: 'resume' })
                }
              >
                {t('恢复')}
              </Button>
            ) : null}
            {record.status !== 'closed' ? (
              <>
                <Button
                  theme='outline'
                  size='small'
                  disabled={record.amount_available <= 0}
                  onClick={() =>
                    setConfirmTarget({ offer: record, action: 'withdraw' })
                  }
                >
                  {t('撤回')}
                </Button>
                <Button
                  theme='outline'
                  size='small'
                  onClick={() =>
                    setConfirmTarget({ offer: record, action: 'close' })
                  }
                >
                  {t('关闭')}
                </Button>
              </>
            ) : null}
          </div>
        ),
      },
    ],
    [t],
  );

  return (
    <>
      <div className='flex items-center justify-between gap-3 mb-4'>
        <div className='min-w-0'>
          <Typography.Text strong>{t('我的供给')}</Typography.Text>
          <div className='text-xs text-gray-500 dark:text-gray-400'>
            {t('您发布的挂单；闲置额度可撤回，也可整体关闭。')}
          </div>
        </div>
        <Button
          type='primary'
          theme='solid'
          size='small'
          icon={<Plus size={14} />}
          onClick={handleCreateClick}
          className='shrink-0'
        >
          {t('创建挂单')}
        </Button>
      </div>

      {error ? (
        <QueryError t={t} message={error} onRetry={fetchOffers} />
      ) : (
        <Table
          size='small'
          columns={columns}
          dataSource={offers}
          rowKey='id'
          loading={loading}
          empty={t('暂无挂单，创建一个开始放贷。')}
          scroll={{ x: true }}
        />
      )}

      <CreateOfferDialog
        t={t}
        visible={createVisible}
        onClose={() => setCreateVisible(false)}
        onCreated={fetchOffers}
      />

      <LenderDisclaimerModal
        t={t}
        visible={disclaimerVisible}
        onAgreed={() => {
          setDisclaimerVisible(false);
          setCreateVisible(true);
          onDisclaimerAgreed?.();
        }}
        onCancel={() => setDisclaimerVisible(false)}
      />

      <Modal
        title={
          confirmTarget ? confirmTitle(t, confirmTarget.action) : t('确认')
        }
        visible={confirmTarget !== null}
        onCancel={() => setConfirmTarget(null)}
        footer={
          <>
            <Button theme='outline' onClick={() => setConfirmTarget(null)}>
              {t('取消')}
            </Button>
            <Button type='primary' theme='solid' onClick={handleConfirm}>
              {t('确认')}
            </Button>
          </>
        }
      >
        {confirmTarget ? (
          <Typography.Text type='secondary'>
            {confirmDescription(t, confirmTarget.offer, confirmTarget.action)}
          </Typography.Text>
        ) : null}
      </Modal>
    </>
  );
};

export default MyOffers;
