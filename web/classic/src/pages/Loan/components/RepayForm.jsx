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
import { Avatar, Button, Card, Input, Typography } from '@douyinfe/semi-ui';
import { Wallet } from 'lucide-react';
import { API, renderQuota, showError, showSuccess } from '../../../helpers';

const RepayForm = ({ t, status, onRepaid }) => {
  const [amount, setAmount] = useState('');
  const [fieldError, setFieldError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [balance, setBalance] = useState(null);
  // 最近一次还款的结果拆分（后端 repay 字段），有惩罚时高亮展示
  const [lastRepay, setLastRepay] = useState(null);
  // 手续费说明"不再显示"标记（localStorage，按浏览器记住）
  const [feeNoticeHidden, setFeeNoticeHidden] = useState(
    () => localStorage.getItem('loan-repay-fee-notice-hidden') === '1',
  );

  const debt = status?.debt ?? 0;
  const feeRate = status?.repay_fee_rate ?? 0;
  const feeRateText = `${Number((feeRate * 100).toFixed(6))}%`;

  // quota_per_unit 缺失/非法时跳过本地余额校验，交给后端兜底
  const rawQuotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit'));
  const balanceUsd =
    balance !== null && Number.isFinite(rawQuotaPerUnit) && rawQuotaPerUnit > 0
      ? balance / rawQuotaPerUnit
      : null;

  const fetchBalance = useCallback(async () => {
    try {
      const res = await API.get('/api/user/self');
      const { success, data } = res.data;
      if (success) {
        setBalance(data.quota ?? 0);
      }
    } catch {
      // 余额展示失败不阻断还款，后端会兜底校验
    }
  }, []);

  useEffect(() => {
    fetchBalance();
  }, [fetchBalance]);

  const submit = async (amountUsd) => {
    setSubmitting(true);
    try {
      const res = await API.post('/api/user/loan/repay', {
        amount_usd: amountUsd,
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('还款成功'));
        setAmount('');
        setLastRepay(data?.repay ?? null);
        fetchBalance();
        onRepaid?.(data);
      } else {
        // 后端错误信息已 i18n，直接展示
        showError(message);
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
    } finally {
      setSubmitting(false);
    }
  };

  const handleSubmit = async () => {
    setFieldError('');
    const value = amount.trim();
    const num = Number(value);
    if (!value) {
      setFieldError(t('请输入还款金额'));
      return;
    }
    if (!Number.isFinite(num) || num <= 0) {
      setFieldError(t('请输入有效的正数金额'));
      return;
    }
    if (balanceUsd !== null && num > balanceUsd) {
      setFieldError(t('金额超出您的钱包余额'));
      return;
    }
    await submit(value);
  };

  return (
    <Card className='!rounded-2xl'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='orange' className='mr-3 shadow-md'>
          <Wallet size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('提前还款')}
          </Typography.Text>
          <div className='text-xs text-gray-500 dark:text-gray-400'>
            {t('使用钱包余额还款，先抵利息后抵本金')}
          </div>
        </div>
      </div>

      <div className='space-y-2'>
        <Typography.Text strong>{t('金额（美元）')}</Typography.Text>
        <Input
          type='number'
          min={0}
          step='any'
          placeholder='0.00'
          value={amount}
          onChange={(v) => setAmount(v)}
          disabled={!status || debt <= 0 || submitting}
        />
        {fieldError ? (
          <Typography.Text type='danger' size='small'>
            {fieldError}
          </Typography.Text>
        ) : (
          <Typography.Text type='tertiary' size='small'>
            {t('钱包余额')}: {balance !== null ? renderQuota(balance) : '-'}
          </Typography.Text>
        )}
        {feeRate > 0 && !feeNoticeHidden && (
          <div className='rounded-lg border p-3 text-xs space-y-2 bg-gray-50 dark:bg-gray-800'>
            <Typography.Text type='tertiary' size='small'>
              {t(
                '提前还款将按抵本部分收取 {{rate}} 手续费，与利息一并从钱包余额扣除',
                { rate: feeRateText },
              )}
            </Typography.Text>
            <div>
              <Button
                size='small'
                theme='borderless'
                onClick={() => {
                  localStorage.setItem('loan-repay-fee-notice-hidden', '1');
                  setFeeNoticeHidden(true);
                }}
              >
                {t('不再显示')}
              </Button>
            </div>
          </div>
        )}
        {lastRepay ? (
          <div className='rounded-lg border p-3 text-xs space-y-1.5 bg-gray-50 dark:bg-gray-800'>
            <Typography.Text type='tertiary' size='small'>
              <span className='font-medium'>{t('还款结果')}</span>
            </Typography.Text>
            <div className='flex items-center justify-between gap-3'>
              <Typography.Text type='tertiary' size='small'>
                {t('还款')}
              </Typography.Text>
              <Typography.Text size='small' className='tabular-nums'>
                {renderQuota(lastRepay.amount || 0)}
              </Typography.Text>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <Typography.Text type='tertiary' size='small'>
                {t('利息部分')}
              </Typography.Text>
              <Typography.Text size='small' className='tabular-nums'>
                {renderQuota(lastRepay.interest_part || 0)}
              </Typography.Text>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <Typography.Text type='tertiary' size='small'>
                {t('本金部分')}
              </Typography.Text>
              <Typography.Text size='small' className='tabular-nums'>
                {renderQuota(lastRepay.principal_part || 0)}
              </Typography.Text>
            </div>
            {lastRepay.fee_part > 0 ? (
              <div className='flex items-center justify-between gap-3'>
                <Typography.Text type='tertiary' size='small'>
                  {t('手续费')}
                </Typography.Text>
                <Typography.Text size='small' className='tabular-nums'>
                  {renderQuota(lastRepay.fee_part)}
                </Typography.Text>
              </div>
            ) : null}
            {lastRepay.penalty_part > 0 ? (
              <div className='flex items-center justify-between gap-3'>
                <Typography.Text type='tertiary' size='small'>
                  {t('秒结清惩罚')}
                </Typography.Text>
                <Typography.Text
                  size='small'
                  type='danger'
                  className='tabular-nums'
                >
                  {renderQuota(lastRepay.penalty_part)}
                </Typography.Text>
              </div>
            ) : null}
            <div className='flex items-center justify-between gap-3'>
              <Typography.Text type='tertiary' size='small'>
                {t('欠款余额')}
              </Typography.Text>
              <Typography.Text size='small' className='tabular-nums'>
                {renderQuota(lastRepay.debt_after || 0)}
              </Typography.Text>
            </div>
          </div>
        ) : null}
        <div className='flex gap-2'>
          <Button
            type='primary'
            theme='solid'
            onClick={handleSubmit}
            loading={submitting}
            disabled={!status || debt <= 0 || submitting || !amount.trim()}
            block
          >
            {submitting ? t('提交中...') : t('还款')}
          </Button>
          <Button
            theme='outline'
            onClick={() => submit('all')}
            loading={submitting}
            disabled={!status || debt <= 0 || submitting}
            block
          >
            {t('全部还清')}
          </Button>
        </div>
      </div>
    </Card>
  );
};

export default RepayForm;
