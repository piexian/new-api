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

import React, { useState } from 'react';
import { Avatar, Button, Card, Input, Typography } from '@douyinfe/semi-ui';
import { HandCoins } from 'lucide-react';
import { API, renderQuota, showError, showSuccess } from '../../../helpers';

const BorrowForm = ({ t, status, onBorrowed }) => {
  const [amount, setAmount] = useState('');
  const [fieldError, setFieldError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const termsBlocked = !!status && status.terms_enabled && !status.terms_agreed;
  // quota_per_unit 缺失/非法时跳过本地上限校验，交给后端兜底
  const rawQuotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit'));
  const availableUsd =
    status && Number.isFinite(rawQuotaPerUnit) && rawQuotaPerUnit > 0
      ? status.available / rawQuotaPerUnit
      : null;

  const handleSubmit = async () => {
    setFieldError('');
    const value = amount.trim();
    const num = Number(value);
    if (!value) {
      setFieldError(t('请输入借款金额'));
      return;
    }
    if (!Number.isFinite(num) || num <= 0) {
      setFieldError(t('请输入有效的正数金额'));
      return;
    }
    if (status && availableUsd !== null && num > availableUsd) {
      setFieldError(t('金额超出您的可借额度'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post('/api/user/loan/borrow', {
        amount_usd: value,
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('借款成功'));
        setAmount('');
        onBorrowed?.(data);
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

  return (
    <Card className='!rounded-2xl'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='green' className='mr-3 shadow-md'>
          <HandCoins size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('借款')}
          </Typography.Text>
          <div className='text-xs text-gray-500 dark:text-gray-400'>
            {t('借款额度将立即计入您的账户余额')}
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
          disabled={!status || termsBlocked || submitting}
        />
        {fieldError ? (
          <Typography.Text type='danger' size='small'>
            {fieldError}
          </Typography.Text>
        ) : (
          <Typography.Text type='tertiary' size='small'>
            {t('可借额度')}: {status ? renderQuota(status.available || 0) : '-'}
          </Typography.Text>
        )}
        {termsBlocked ? (
          <div className='text-xs text-gray-500 dark:text-gray-400'>
            {t('请先同意借款条款后再借款')}
          </div>
        ) : null}
        <Button
          type='primary'
          theme='solid'
          onClick={handleSubmit}
          loading={submitting}
          disabled={!status || termsBlocked || submitting || !amount.trim()}
          block
        >
          {submitting ? t('提交中...') : t('借款')}
        </Button>
      </div>
    </Card>
  );
};

export default BorrowForm;
