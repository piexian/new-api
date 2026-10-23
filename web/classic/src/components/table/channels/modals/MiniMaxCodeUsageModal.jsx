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
import { Banner, Button, Modal, Spin, Switch, Tag } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';
import {
  MetaBlock,
  MiniMaxModelCard,
  resolveModelWindows,
} from './MiniMaxUsageModal';

// 签到格子状态取自官方客户端：2 可领取，3 已领取，4 不可用，其余为未到期。
const SIGNIN_CLAIMABLE = 2;
const SIGNIN_CLAIMED = 3;
const SIGNIN_DISABLED = 4;

const toNumber = (value) => {
  const numericValue = Number(value);
  return Number.isFinite(numericValue) ? numericValue : null;
};

const toRecord = (value) =>
  value && typeof value === 'object' && !Array.isArray(value) ? value : null;

const toRecordList = (value) =>
  Array.isArray(value) ? value.map(toRecord).filter(Boolean) : [];

const formatDateTime = (value) => {
  const numericValue = toNumber(value);
  if (numericValue != null && numericValue > 0) {
    const epochMs = numericValue < 1e12 ? numericValue * 1000 : numericValue;
    return new Date(epochMs).toLocaleString();
  }
  if (typeof value === 'string' && value) {
    const parsed = new Date(value).getTime();
    return Number.isFinite(parsed) ? new Date(parsed).toLocaleString() : value;
  }
  return '-';
};

const signinStatusMeta = (t, status) => {
  const value = toNumber(status);
  if (value === SIGNIN_CLAIMED) return { text: t('已领取'), color: 'green' };
  if (value === SIGNIN_CLAIMABLE) return { text: t('可领取'), color: 'blue' };
  if (value === SIGNIN_DISABLED) return { text: t('不可领取'), color: 'grey' };
  return { text: t('未到'), color: 'grey' };
};

const Section = ({ title, extra, children }) => (
  <div className='rounded-xl border border-semi-color-border bg-semi-color-bg-1 p-4'>
    <div className='mb-3 flex items-center justify-between gap-3'>
      <div className='font-semibold text-semi-color-text-0'>{title}</div>
      {extra}
    </div>
    {children}
  </div>
);

const SectionError = ({ message }) => (
  <Banner type='warning' description={String(message || '-')} />
);

const EmptyText = ({ children }) => (
  <div className='text-sm text-semi-color-text-2'>{children}</div>
);

const MiniMaxCodeUsageLoader = ({ t, record }) => {
  const [loading, setLoading] = useState(true);
  const [payload, setPayload] = useState(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');
  const [fallbackEnabled, setFallbackEnabled] = useState(null);

  const fetchData = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get(`/api/channel/${record.id}/minimaxcode/usage`, {
        skipErrorHandler: true,
      });
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('获取用量失败'));
      }
      const data = res.data.data || {};
      setPayload(data);
      const enabled = toRecord(data.summary)?.credit_fallback_enabled;
      setFallbackEnabled(typeof enabled === 'boolean' ? enabled : null);
    } catch (err) {
      setError(err?.message || t('获取用量失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (record?.id) {
      fetchData();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [record?.id]);

  const runAction = async (key, fn, successText) => {
    if (busy) return false;
    setBusy(key);
    try {
      const res = await fn();
      if (res?.data?.success) {
        showSuccess(successText || t('操作成功'));
        await fetchData();
        return true;
      }
      showError(res?.data?.message || t('操作失败'));
    } catch (err) {
      showError(err?.message || t('操作失败'));
    } finally {
      setBusy('');
    }
    return false;
  };

  const account = toRecord(payload?.account);
  const summary = toRecord(payload?.summary) || {};
  const remains = toRecord(payload?.remains);
  const credit = toRecord(payload?.credit);
  const creditDetailsSection = toRecord(payload?.credit_details);
  const resetCardsSection = toRecord(payload?.reset_cards);
  const signin = toRecord(payload?.signin);

  const remainsData = toRecord(remains?.data);
  const models = useMemo(() => {
    const items = Array.isArray(remainsData?.model_remains)
      ? remainsData.model_remains
      : [];
    return items
      .map((item) => toRecord(item))
      .filter(Boolean)
      .map((item) => ({ item, windows: resolveModelWindows(item, t) }))
      .filter((entry) => entry.windows.length > 0);
  }, [remainsData, t]);

  const cards = toRecordList(summary.reset_cards);
  const creditDetails = toRecordList(summary.credit_details);
  const signinDays = toRecordList(summary.signin_days).sort(
    (a, b) => (toNumber(a.day_no) ?? 0) - (toNumber(b.day_no) ?? 0),
  );
  const claimedToday = signinDays.some(
    (day) => day.is_today === true && toNumber(day.status) === SIGNIN_CLAIMED,
  );
  const claimable = signinDays.some(
    (day) => toNumber(day.status) === SIGNIN_CLAIMABLE,
  );

  const confirmUseCard = (card) => {
    const outBizNo = String(card.out_biz_no || '');
    Modal.confirm({
      title: t('使用这张重置卡？'),
      content: t('套餐用量窗口将立即重置，并消耗这张卡。'),
      onOk: () =>
        runAction(
          `card-${outBizNo}`,
          () =>
            API.post(
              `/api/channel/${record.id}/minimaxcode/reset_card/use`,
              // source_type 原样回传，上游类型未固定（数字或字符串）
              { source_type: card.source_type, out_biz_no: outBizNo },
              { skipErrorHandler: true },
            ),
          t('重置卡已使用'),
        ),
    });
  };

  if (loading && !payload) {
    return (
      <div className='flex items-center justify-center py-12'>
        <Spin />
      </div>
    );
  }

  if (error && !payload) {
    return <Banner type='danger' description={error} />;
  }

  // 上游不一定返回账户名等字段，只展示有值的项。
  const metaItems = [
    { label: t('区域'), value: account?.region },
    { label: t('账户'), value: account?.user_name || account?.email },
    { label: t('套餐分组'), value: account?.group_id },
    {
      label: t('令牌过期时间'),
      value: account?.expires_at ? formatDateTime(account.expires_at) : '',
    },
  ].filter((meta) => meta.value);

  let remainsContent;
  if (remains?.success === false) {
    remainsContent = <SectionError message={remains.message} />;
  } else if (models.length === 0) {
    remainsContent = <EmptyText>{t('暂无数据')}</EmptyText>;
  } else {
    remainsContent = (
      <div className='flex flex-col gap-3'>
        {models.map(({ item, windows }, index) => (
          <MiniMaxModelCard
            key={`${item.model_name || 'model'}-${index}`}
            t={t}
            item={item}
            windows={windows}
          />
        ))}
      </div>
    );
  }

  let creditDetailsContent;
  if (creditDetailsSection?.success === false) {
    creditDetailsContent = (
      <SectionError message={creditDetailsSection.message} />
    );
  } else if (creditDetails.length === 0) {
    creditDetailsContent = <EmptyText>{t('暂无积分明细')}</EmptyText>;
  } else {
    creditDetailsContent = (
      <div className='flex flex-col gap-2'>
        {creditDetails.map((detail) => (
          <div
            key={`${detail.credit_type ?? ''}-${detail.granted_at_ms ?? ''}-${detail.expire_at_ms ?? ''}`}
            className='flex flex-wrap items-center justify-between gap-x-4 gap-y-1 rounded-lg border border-semi-color-border bg-semi-color-bg-0 px-3 py-2 text-xs'
          >
            <span className='text-semi-color-text-0'>
              {t('剩余')} {String(detail.remaining_amount ?? '-')} / {t('发放')}{' '}
              {String(detail.granted_amount ?? '-')}
            </span>
            <span className='text-semi-color-text-2'>
              {t('到期时间')}：{formatDateTime(detail.expire_at_ms)}
            </span>
          </div>
        ))}
      </div>
    );
  }

  let resetCardsContent;
  if (resetCardsSection?.success === false) {
    resetCardsContent = <SectionError message={resetCardsSection.message} />;
  } else if (cards.length === 0) {
    resetCardsContent = <EmptyText>{t('暂无可用重置卡')}</EmptyText>;
  } else {
    resetCardsContent = (
      <div className='flex flex-col gap-2'>
        {cards.map((card, index) => {
          const outBizNo = String(card.out_biz_no || '');
          const usable = card.source_type !== undefined && Boolean(outBizNo);
          return (
            <div
              key={String(card.card_id || outBizNo || index)}
              className='flex flex-wrap items-center justify-between gap-3 rounded-lg border border-semi-color-border bg-semi-color-bg-0 px-3 py-2'
            >
              <div className='min-w-0'>
                <div className='text-sm font-medium text-semi-color-text-0'>
                  {String(card.card_name || card.name || t('完全重置'))}
                </div>
                <div className='text-xs text-semi-color-text-2'>
                  {t('到期时间')}：{formatDateTime(card.expire_time)}
                </div>
              </div>
              <Button
                size='small'
                theme='outline'
                disabled={!usable || Boolean(busy)}
                loading={busy === `card-${outBizNo}`}
                onClick={() => confirmUseCard(card)}
              >
                {t('使用')}
              </Button>
            </div>
          );
        })}
      </div>
    );
  }

  let signinContent;
  if (signin?.success === false) {
    signinContent = <SectionError message={signin.message} />;
  } else if (signinDays.length === 0) {
    signinContent = <EmptyText>{t('暂无签到数据')}</EmptyText>;
  } else {
    signinContent = (
      <div className='grid grid-cols-4 gap-2 sm:grid-cols-7'>
        {signinDays.map((day) => {
          const meta = signinStatusMeta(t, day.status);
          return (
            <div
              key={String(day.day_no)}
              className={`flex flex-col items-center gap-1 rounded-lg border bg-semi-color-bg-0 p-2 text-xs ${
                day.is_today === true
                  ? 'border-semi-color-primary'
                  : 'border-semi-color-border'
              }`}
            >
              <div className='text-semi-color-text-2'>
                {t('第 {{day}} 天', { day: day.day_no ?? '-' })}
              </div>
              <div className='font-semibold text-semi-color-text-0'>
                +{String(day.points ?? 0)}
                {toNumber(day.bonus_points)
                  ? ` +${String(day.bonus_points)}`
                  : ''}
              </div>
              <Tag size='small' color={meta.color}>
                {meta.text}
              </Tag>
            </div>
          );
        })}
      </div>
    );
  }

  return (
    <div className='flex flex-col gap-4 pr-1'>
      <div className='flex justify-end'>
        <Button
          type='primary'
          theme='outline'
          size='small'
          loading={loading}
          onClick={fetchData}
        >
          {t('刷新')}
        </Button>
      </div>

      {metaItems.length > 0 && (
        <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
          {metaItems.map((meta) => (
            <MetaBlock key={meta.label} label={meta.label} value={meta.value} />
          ))}
        </div>
      )}

      <Section title={t('套餐余量')}>{remainsContent}</Section>

      <Section title={t('积分')}>
        {credit?.success === false && summary.credit_balance == null ? (
          <SectionError message={credit.message} />
        ) : (
          <div className='flex items-baseline gap-2'>
            <span className='text-sm text-semi-color-text-2'>
              {t('积分余额')}
            </span>
            <span className='text-lg font-semibold text-semi-color-text-0'>
              {summary.credit_balance ?? '-'}
            </span>
          </div>
        )}
        {fallbackEnabled !== null && (
          <div className='mt-3 flex items-center justify-between gap-3 border-0 border-t border-solid border-semi-color-border pt-3'>
            <span className='text-sm text-semi-color-text-0'>
              {t('套餐用尽后改扣积分')}
            </span>
            <Switch
              checked={fallbackEnabled}
              disabled={Boolean(busy)}
              onChange={async (checked) => {
                setFallbackEnabled(checked);
                const ok = await runAction('fallback', () =>
                  API.post(
                    `/api/channel/${record.id}/minimaxcode/credit_fallback`,
                    { enabled: checked },
                    { skipErrorHandler: true },
                  ),
                );
                if (!ok) setFallbackEnabled(!checked);
              }}
            />
          </div>
        )}
        <div className='mt-3 border-0 border-t border-solid border-semi-color-border pt-3'>
          <div className='mb-2 text-xs text-semi-color-text-2'>
            {t('积分明细')}
          </div>
          {creditDetailsContent}
        </div>
      </Section>

      <Section title={t('重置卡')}>{resetCardsContent}</Section>

      <Section
        title={t('每日签到')}
        extra={
          signin?.success === false ? null : (
            <Button
              size='small'
              theme='outline'
              disabled={Boolean(busy) || claimedToday || !claimable}
              loading={busy === 'signin'}
              onClick={() =>
                runAction(
                  'signin',
                  () =>
                    API.post(
                      `/api/channel/${record.id}/minimaxcode/signin/claim`,
                      {},
                      { skipErrorHandler: true },
                    ),
                  t('签到成功'),
                )
              }
            >
              {claimedToday ? t('今日已签到') : t('签到')}
            </Button>
          )
        }
      >
        {signinContent}
      </Section>
    </div>
  );
};

export const openMiniMaxCodeUsageModal = ({ t, record }) => {
  Modal.info({
    title: t('MiniMax Code 用量与积分'),
    centered: false,
    width: 900,
    style: { maxWidth: '95vw', top: 12 },
    bodyStyle: {
      maxHeight: 'calc(100vh - 160px)',
      overflowY: 'auto',
      WebkitOverflowScrolling: 'touch',
    },
    content: <MiniMaxCodeUsageLoader t={t} record={record} />,
    footer: (
      <div className='flex justify-end gap-2'>
        <Button type='primary' theme='solid' onClick={() => Modal.destroyAll()}>
          {t('关闭')}
        </Button>
      </div>
    ),
  });
};

export default MiniMaxCodeUsageLoader;
