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
  InputNumber,
  Modal,
  Select,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  renderQuota,
  showError,
  showSuccess,
  timestamp2string,
} from '../../../helpers';
import QueryError from './QueryError';

const PAGE_SIZE = 10;

const REPAY_PLAN_KEYS = [
  'full',
  'no_penalty',
  'interest_freeze',
  'principal_only',
];

const sourceLabel = (t, source) => {
  if (source === 'pool') return t('资金池');
  if (source === 'ai') return t('AI');
  if (source === 'order') return t('订单');
  return t('平台');
};

const statusLabel = (t, status) => {
  if (status === 'active') return t('生效');
  if (status === 'overdue') return t('逾期');
  if (status === 'repaid') return t('已结清');
  return t('已核销');
};

const statusTag = (t, status) => {
  const color =
    status === 'overdue' ? 'red' : status === 'repaid' ? 'green' : 'grey';
  return (
    <Tag color={color} size='small'>
      {statusLabel(t, status)}
    </Tag>
  );
};

const repayPlanLabel = (t, plan) => {
  if (plan === 'full') return t('全额（复利）');
  if (plan === 'no_penalty') return t('免罚息');
  if (plan === 'interest_freeze') return t('停止计息');
  return t('只还本金');
};

// 日利率展示与 default 主题一致（percent，最多两位小数）
const formatDailyRate = (rate) => {
  if (typeof rate !== 'number' || isNaN(rate)) return '-';
  return Intl.NumberFormat(undefined, {
    style: 'percent',
    maximumFractionDigits: 2,
  }).format(rate);
};

// due_day 为服务器本地日序号（unix/86400），按本地时间展示日期
const formatDay = (dayNumber) => {
  if (!dayNumber) return '-';
  const d = new Date(dayNumber * 86400 * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

/**
 * 收益台账：放贷产生的借款明细；逾期记录可延长、核销或设为永续，在贷/逾期行可调整还款计划。
 */
const MyFundings = ({ t, refreshKey }) => {
  const [page, setPage] = useState(1);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [extendTarget, setExtendTarget] = useState(null); // funding
  const [extendDays, setExtendDays] = useState(7);
  const [confirmTarget, setConfirmTarget] = useState(null); // { funding, action }
  const [submitting, setSubmitting] = useState(false);

  const fetchFundings = useCallback(
    async (p) => {
      setLoading(true);
      setError('');
      try {
        const res = await API.get('/api/user/loan/market/fundings', {
          params: { p, page_size: PAGE_SIZE },
        });
        const { success, message, data } = res.data;
        if (success) {
          setItems(data?.items || []);
          setTotal(data?.total || 0);
        } else {
          setError(message || t('获取放贷记录失败'));
        }
      } catch {
        // 网络/HTTP 错误已由 API 拦截器提示
        setError(t('获取放贷记录失败'));
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    fetchFundings(page);
  }, [page, refreshKey, fetchFundings]);

  const runResolve = async (funding, action, days) => {
    setSubmitting(true);
    try {
      const res = await API.post(
        `/api/user/loan/market/fundings/${funding.id}/resolve`,
        { action, extend_days: days },
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('处置成功'));
        fetchFundings(page);
      } else {
        showError(message);
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
    } finally {
      setSubmitting(false);
    }
  };

  const handleExtend = async () => {
    const funding = extendTarget;
    if (!funding) return;
    if (!Number.isInteger(extendDays) || extendDays <= 0) {
      showError(t('请输入有效的天数'));
      return;
    }
    setExtendTarget(null);
    await runResolve(funding, 'extend', extendDays);
  };

  const handleConfirmResolve = async () => {
    const target = confirmTarget;
    if (!target) return;
    setConfirmTarget(null);
    await runResolve(target.funding, target.action, 0);
  };

  const handleRepayPlanChange = async (funding, plan) => {
    if (funding.repay_plan === plan) return;
    try {
      const res = await API.post(
        `/api/user/loan/market/fundings/${funding.id}/repay_plan`,
        { plan },
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('还款计划已更新'));
        fetchFundings(page);
      } else {
        showError(message);
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (v) => (
          <span className='text-xs text-gray-500 dark:text-gray-400'>
            {timestamp2string(v)}
          </span>
        ),
      },
      {
        title: t('来源'),
        dataIndex: 'source_type',
        render: (v) => sourceLabel(t, v),
      },
      {
        title: t('放出金额'),
        dataIndex: 'amount',
        render: (v) => (
          <span className='font-medium tabular-nums'>
            {renderQuota(v || 0)}
          </span>
        ),
      },
      {
        title: t('已回本金'),
        dataIndex: 'repaid_principal',
        render: (v) => (
          <span className='tabular-nums'>{renderQuota(v || 0)}</span>
        ),
      },
      {
        title: t('当前债务'),
        dataIndex: 'debt',
        render: (v) => (
          <span className='font-medium tabular-nums'>
            {renderQuota(v || 0)}
          </span>
        ),
      },
      {
        title: t('利率'),
        dataIndex: 'rate',
        render: (v) => (
          <span className='text-gray-500 dark:text-gray-400'>
            {formatDailyRate(v)}
          </span>
        ),
      },
      {
        title: t('应还日期'),
        dataIndex: 'due_day',
        render: (v) => (
          <span className='text-xs text-gray-500 dark:text-gray-400'>
            {formatDay(v)}
          </span>
        ),
      },
      {
        title: t('借款人信用分'),
        dataIndex: 'borrower_credit_score',
        render: (v) => <span className='tabular-nums'>{v}</span>,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (v) => statusTag(t, v),
      },
      {
        title: t('还款计划'),
        dataIndex: 'repay_plan',
        render: (v, record) => {
          const editable =
            record.status === 'active' || record.status === 'overdue';
          if (!editable) {
            return (
              <span className='text-sm text-gray-500 dark:text-gray-400'>
                {repayPlanLabel(t, v)}
              </span>
            );
          }
          return (
            <Select
              value={v}
              onChange={(next) => handleRepayPlanChange(record, next)}
              style={{ width: 150 }}
              optionList={REPAY_PLAN_KEYS.map((key) => ({
                label: repayPlanLabel(t, key),
                value: key,
              }))}
            />
          );
        },
      },
      {
        title: t('操作'),
        dataIndex: 'id',
        render: (v, record) =>
          record.status === 'overdue' ? (
            <div className='flex flex-wrap items-center justify-end gap-1.5'>
              <Button
                theme='outline'
                size='small'
                onClick={() => {
                  setExtendDays(7);
                  setExtendTarget(record);
                }}
              >
                {t('延长')}
              </Button>
              <Button
                theme='outline'
                size='small'
                onClick={() =>
                  setConfirmTarget({ funding: record, action: 'writeoff' })
                }
              >
                {t('核销')}
              </Button>
              <Button
                theme='outline'
                size='small'
                onClick={() =>
                  setConfirmTarget({ funding: record, action: 'perpetual' })
                }
              >
                {t('永续')}
              </Button>
            </div>
          ) : null,
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, page],
  );

  return (
    <>
      <div className='mb-4'>
        <Typography.Text strong>{t('收益台账')}</Typography.Text>
        <div className='text-xs text-gray-500 dark:text-gray-400'>
          {t('您挂单放出的每一笔借款；逾期记录可延长、核销或设为永续。')}
        </div>
      </div>

      {error ? (
        <QueryError t={t} message={error} onRetry={() => fetchFundings(page)} />
      ) : (
        <Table
          size='small'
          columns={columns}
          dataSource={items}
          rowKey='id'
          loading={loading}
          empty={t('暂无放贷记录')}
          scroll={{ x: true }}
          onRow={(record) =>
            record.status === 'overdue'
              ? { className: 'bg-red-50 dark:bg-red-500/10' }
              : {}
          }
          pagination={{
            currentPage: page,
            pageSize: PAGE_SIZE,
            total,
            onPageChange: (p) => setPage(p),
          }}
        />
      )}

      {/* 延长弹窗 */}
      <Modal
        title={t('延长逾期借款')}
        visible={extendTarget !== null}
        onCancel={() => setExtendTarget(null)}
        footer={
          <>
            <Button theme='outline' onClick={() => setExtendTarget(null)}>
              {t('取消')}
            </Button>
            <Button
              type='primary'
              theme='solid'
              onClick={handleExtend}
              loading={submitting}
              disabled={submitting}
            >
              {t('确认')}
            </Button>
          </>
        }
      >
        <Typography.Text type='secondary' size='small'>
          {t('设置新的应还日期。已累计的罚息保留，该借款恢复为正常在贷。')}
        </Typography.Text>
        <div className='mt-3'>
          <Typography.Text strong className='block mb-1'>
            {t('延长天数')}
          </Typography.Text>
          <InputNumber
            min={1}
            precision={0}
            value={extendDays}
            onChange={(v) =>
              setExtendDays(v === null || v === undefined ? 0 : Number(v))
            }
            style={{ width: '100%' }}
          />
        </div>
      </Modal>

      {/* 核销 / 永续确认弹窗 */}
      <Modal
        title={
          confirmTarget?.action === 'writeoff' ? t('核销借款') : t('设为永续')
        }
        visible={confirmTarget !== null}
        onCancel={() => setConfirmTarget(null)}
        footer={
          <>
            <Button theme='outline' onClick={() => setConfirmTarget(null)}>
              {t('取消')}
            </Button>
            <Button
              type={confirmTarget?.action === 'writeoff' ? 'danger' : 'primary'}
              theme='solid'
              onClick={handleConfirmResolve}
              loading={submitting}
              disabled={submitting}
            >
              {t('确认')}
            </Button>
          </>
        }
      >
        <Typography.Text type='secondary'>
          {confirmTarget?.action === 'writeoff'
            ? t('未偿债务将被销毁，借款人会受到惩罚。此操作不可撤销。')
            : t('该借款保持逾期状态并继续计息，直到还清为止。')}
        </Typography.Text>
      </Modal>
    </>
  );
};

export default MyFundings;
