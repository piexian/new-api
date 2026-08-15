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
import {
  Avatar,
  Button,
  Card,
  Checkbox,
  Input,
  Typography,
} from '@douyinfe/semi-ui';
import { HandCoins, X } from 'lucide-react';
import {
  API,
  renderQuota,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';

// 日利率展示与 default 主题一致（percent，最多两位小数）
const formatDailyRate = (rate) => {
  if (typeof rate !== 'number' || isNaN(rate)) return '-';
  return Intl.NumberFormat(undefined, {
    style: 'percent',
    maximumFractionDigits: 2,
  }).format(rate);
};

const BorrowForm = ({ t, status, onBorrowed, presetOrder, onClearOrder }) => {
  const [amount, setAmount] = useState('');
  const [fieldError, setFieldError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  // 只接官方资金：跳过市场撮合，整笔由平台放款（官方资金无秒结清惩罚条款）
  const [platformOnly, setPlatformOnly] = useState(false);

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
        order_id: presetOrder?.id ?? 0,
        platform_only: platformOnly,
      });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('借款成功'));
        // 借款后即时提示：本次放款含秒结清惩罚条款的资金，说明手动提前结清的
        // 惩罚与签到自动还款的豁免（签到视为正常还款，不触发惩罚）
        const penaltyTotal = (data?.fundings ?? []).reduce(
          (sum, f) =>
            sum +
            (f.fast_repay_penalty_quota > 0 ? f.fast_repay_penalty_quota : 0),
          0,
        );
        if (penaltyTotal > 0) {
          showWarning(
            t(
              '本次借款包含带秒结清惩罚的资金（共 {{amount}}）：在惩罚窗口期内手动全额提前结清将被收取该惩罚；签到自动还款不会触发惩罚，可正常还款。',
              { amount: renderQuota(penaltyTotal) },
            ),
          );
        }
        setAmount('');
        onBorrowed?.(data);
        onClearOrder?.();
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
        {presetOrder ? (
          <div className='rounded-lg border bg-slate-50 dark:bg-slate-800 p-3 text-xs space-y-0.5'>
            <div className='flex items-start justify-between gap-2'>
              <div className='space-y-0.5'>
                <p className='text-gray-500 dark:text-gray-400'>
                  {t('正在从挂单 #{{id}} 借款', { id: presetOrder.id })}
                </p>
                <p>
                  {t('固定利率')}: {formatDailyRate(presetOrder.rate_fixed)} ·{' '}
                  {t('可用')}: {renderQuota(presetOrder.amount_available || 0)}
                </p>
                <p className='text-gray-500 dark:text-gray-400'>
                  {t('若挂单额度不足以覆盖全额，剩余部分由平台兜底。')}
                </p>
              </div>
              <Button
                theme='borderless'
                size='small'
                icon={<X size={14} />}
                onClick={() => onClearOrder?.()}
                aria-label={t('清除挂单选择')}
                className='shrink-0'
              />
            </div>
          </div>
        ) : null}
        {status?.market_enabled && !presetOrder ? (
          <Checkbox
            checked={platformOnly}
            onChange={(e) => setPlatformOnly(!!e.target.checked)}
            extra={t(
              '跳过市场挂单，整笔从官方资金池借款；官方资金不附带秒结清惩罚。',
            )}
          >
            {t('只接官方资金')}
          </Checkbox>
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
