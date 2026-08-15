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

import React, { useEffect, useState } from 'react';
import {
  Button,
  InputNumber,
  Modal,
  Select,
  Typography,
} from '@douyinfe/semi-ui';
import { API, getQuotaPerUnit, showError, showSuccess } from '../../../helpers';

const MODE_KEYS = ['pool', 'ai', 'order'];

// 利率输入采用"日利率百分比"（如 0.1 = 0.1%/天），提交时换算为小数
const percentToRate = (percent) => {
  const n = Number(percent);
  return Number.isFinite(n) ? n / 100 : 0;
};

const isValidPositive = (v) => {
  const n = Number(v);
  return Number.isFinite(n) && n > 0;
};

const isValidRate = (v) => isValidPositive(v) && Number(v) <= 100;

const isValidCreditScore = (v) => {
  const n = Number(v);
  return Number.isInteger(n) && n >= -50 && n <= 100;
};

const modeLabel = (t, mode) => {
  if (mode === 'pool') return t('资金池（固定利率）');
  if (mode === 'ai') return t('AI（利率区间）');
  return t('订单（公开挂牌）');
};

const modeDescription = (t, mode) => {
  if (mode === 'pool') {
    return t('您的额度汇入共享资金池，按您的固定利率自动匹配给借款人。');
  }
  if (mode === 'ai') {
    return t('AI 审核员会在您设定的利率区间与单笔上限内撮合每笔借款。');
  }
  return t('您的挂单会公开挂牌，借款人可直接借取。');
};

const MODE_DEFAULT_VALUES = {
  pool: { rateFixed: '0.1' },
  ai: { rateMin: '0.1', rateMax: '0.3', perLoanCap: '' },
  order: { rateFixed: '0.1' },
};

/**
 * 创建放贷挂单弹窗：三种模式（资金池 / AI / 订单），金额为美元，额度按 quota_per_unit 换算。
 */
const CreateOfferDialog = ({ t, visible, onClose, onCreated }) => {
  const [mode, setMode] = useState('pool');
  const [amount, setAmount] = useState('');
  const [rateFixed, setRateFixed] = useState('0.1');
  const [rateMin, setRateMin] = useState('0.1');
  const [rateMax, setRateMax] = useState('0.3');
  const [perLoanCap, setPerLoanCap] = useState('');
  const [minCreditScore, setMinCreditScore] = useState('-50');
  const [fieldError, setFieldError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // 关闭弹窗时重置本地状态，避免下次打开残留旧值
  useEffect(() => {
    if (!visible) {
      setMode('pool');
      setAmount('');
      setRateFixed('0.1');
      setRateMin('0.1');
      setRateMax('0.3');
      setPerLoanCap('');
      setMinCreditScore('-50');
      setFieldError('');
      setSubmitting(false);
    }
  }, [visible]);

  const switchMode = (next) => {
    setMode(next);
    setFieldError('');
    // 切换模式时回填该模式的默认利率，避免残留值误导
    const defaults = MODE_DEFAULT_VALUES[next] || {};
    if (defaults.rateFixed !== undefined) setRateFixed(defaults.rateFixed);
    if (defaults.rateMin !== undefined) setRateMin(defaults.rateMin);
    if (defaults.rateMax !== undefined) setRateMax(defaults.rateMax);
    if (defaults.perLoanCap !== undefined) setPerLoanCap(defaults.perLoanCap);
  };

  const validate = () => {
    if (!amount.trim() || !isValidPositive(amount)) {
      return t('请输入有效的正数金额');
    }
    if (mode === 'ai') {
      if (!rateMin.trim() || !isValidRate(rateMin)) {
        return t('请输入 0 到 100 之间的最低日利率');
      }
      if (!rateMax.trim() || !isValidRate(rateMax)) {
        return t('请输入 0 到 100 之间的最高日利率');
      }
      if (Number(rateMin) > Number(rateMax)) {
        return t('最低利率不能高于最高利率');
      }
      if (perLoanCap.trim() && !isValidPositive(perLoanCap)) {
        return t('请输入有效的正数单笔上限');
      }
    } else if (!rateFixed.trim() || !isValidRate(rateFixed)) {
      return t('请输入 0 到 100 之间的日利率');
    }
    if (!isValidCreditScore(minCreditScore)) {
      return t('信用分必须在 -50 到 100 之间');
    }
    return '';
  };

  const handleSubmit = async () => {
    const err = validate();
    if (err) {
      setFieldError(err);
      return;
    }
    setSubmitting(true);
    try {
      const payload = {
        mode,
        amount_usd: amount.trim(),
        rate_fixed: mode === 'ai' ? '0' : String(percentToRate(rateFixed)),
        rate_min: mode === 'ai' ? percentToRate(rateMin) : 0,
        rate_max: mode === 'ai' ? percentToRate(rateMax) : 0,
        per_loan_cap:
          mode === 'ai'
            ? Math.round(Number(perLoanCap || 0) * getQuotaPerUnit())
            : 0,
        min_credit_score: Number(minCreditScore),
      };
      const res = await API.post('/api/user/loan/market/offers', payload);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('挂单创建成功'));
        onClose();
        onCreated?.();
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
    <Modal
      title={t('创建放贷挂单')}
      visible={visible}
      onCancel={onClose}
      footer={
        <>
          <Button theme='outline' onClick={onClose}>
            {t('取消')}
          </Button>
          <Button
            type='primary'
            theme='solid'
            onClick={handleSubmit}
            loading={submitting}
            disabled={submitting}
          >
            {submitting ? t('提交中...') : t('创建挂单')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <Typography.Text type='secondary' size='small'>
          {t('将闲置额度出借到词元贷市场赚取日利息。')}
        </Typography.Text>

        <div>
          <Typography.Text strong className='block mb-1'>
            {t('放贷模式')}
          </Typography.Text>
          <Select
            value={mode}
            onChange={(v) => switchMode(v)}
            style={{ width: '100%' }}
            optionList={MODE_KEYS.map((key) => ({
              label: modeLabel(t, key),
              value: key,
            }))}
          />
          <div className='mt-1 text-xs text-gray-500 dark:text-gray-400'>
            {modeDescription(t, mode)}
          </div>
        </div>

        <div>
          <Typography.Text strong className='block mb-1'>
            {t('金额（美元）')}
          </Typography.Text>
          <InputNumber
            min={0}
            precision={2}
            value={amount === '' ? undefined : Number(amount)}
            onChange={(v) =>
              setAmount(v === null || v === undefined ? '' : String(v))
            }
            style={{ width: '100%' }}
            placeholder='0.00'
          />
          <div className='mt-1 text-xs text-gray-500 dark:text-gray-400'>
            {t('额度将从您的余额中扣除并锁定到该挂单。')}
          </div>
        </div>

        {mode === 'ai' ? (
          <>
            <div className='grid grid-cols-2 gap-3'>
              <div>
                <Typography.Text strong className='block mb-1'>
                  {t('最低日利率（%）')}
                </Typography.Text>
                <InputNumber
                  min={0}
                  precision={2}
                  value={rateMin === '' ? undefined : Number(rateMin)}
                  onChange={(v) =>
                    setRateMin(v === null || v === undefined ? '' : String(v))
                  }
                  style={{ width: '100%' }}
                />
              </div>
              <div>
                <Typography.Text strong className='block mb-1'>
                  {t('最高日利率（%）')}
                </Typography.Text>
                <InputNumber
                  min={0}
                  precision={2}
                  value={rateMax === '' ? undefined : Number(rateMax)}
                  onChange={(v) =>
                    setRateMax(v === null || v === undefined ? '' : String(v))
                  }
                  style={{ width: '100%' }}
                />
              </div>
            </div>
            <div>
              <Typography.Text strong className='block mb-1'>
                {t('单笔上限（USD）')}
              </Typography.Text>
              <InputNumber
                min={0}
                precision={2}
                value={perLoanCap === '' ? undefined : Number(perLoanCap)}
                onChange={(v) =>
                  setPerLoanCap(v === null || v === undefined ? '' : String(v))
                }
                style={{ width: '100%' }}
                placeholder='0.00'
              />
              <div className='mt-1 text-xs text-gray-500 dark:text-gray-400'>
                {t('单笔 AI 审核借款可从该挂单获取的最大额度。')}
              </div>
            </div>
          </>
        ) : (
          <div>
            <Typography.Text strong className='block mb-1'>
              {t('固定日利率（%）')}
            </Typography.Text>
            <InputNumber
              min={0}
              precision={2}
              value={rateFixed === '' ? undefined : Number(rateFixed)}
              onChange={(v) =>
                setRateFixed(v === null || v === undefined ? '' : String(v))
              }
              style={{ width: '100%' }}
            />
            <div className='mt-1 text-xs text-gray-500 dark:text-gray-400'>
              {t('例如 0.1 表示每天 0.1% 利息')}
            </div>
          </div>
        )}

        <div>
          <Typography.Text strong className='block mb-1'>
            {t('最低信用分')}
          </Typography.Text>
          <InputNumber
            min={-50}
            max={100}
            precision={0}
            value={minCreditScore === '' ? undefined : Number(minCreditScore)}
            onChange={(v) =>
              setMinCreditScore(v === null || v === undefined ? '' : String(v))
            }
            style={{ width: '100%' }}
          />
          <div className='mt-1 text-xs text-gray-500 dark:text-gray-400'>
            {t('-50 表示不限，0 到 100 表示要求的最低分数')}
          </div>
        </div>

        {fieldError ? (
          <Typography.Text type='danger' size='small'>
            {fieldError}
          </Typography.Text>
        ) : null}
      </div>
    </Modal>
  );
};

export default CreateOfferDialog;
