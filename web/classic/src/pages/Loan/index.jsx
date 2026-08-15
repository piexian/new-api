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
import { Button, Card, Spin, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';
import LoanStatusCard from './components/LoanStatusCard';
import BorrowForm from './components/BorrowForm';
import RepayForm from './components/RepayForm';
import LoanRecordsTable from './components/LoanRecordsTable';
import OfficerApplications from './components/OfficerApplications';
import TermsModal from './components/TermsModal';
import LendingMarket from './components/LendingMarket';

const { Text, Title } = Typography;

// 服务器本地日序号（unix/86400），用于黑名单到期判断
const formatBlacklistDate = (dayNumber) => {
  const d = new Date(dayNumber * 86400 * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

const Loan = () => {
  const { t } = useTranslation();
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [recordsRefreshKey, setRecordsRefreshKey] = useState(0);
  const [marketRefreshKey, setMarketRefreshKey] = useState(0);
  const [selectedOrder, setSelectedOrder] = useState(null);
  const [disclaimerAgreed, setDisclaimerAgreed] = useState(false);

  const fetchStatus = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const res = await API.get('/api/user/loan/status');
      const { success, message, data } = res.data;
      if (success) {
        setStatus(data);
      } else {
        setLoadError(message || t('获取借款状态失败'));
        showError(message);
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
      setLoadError(t('获取借款状态失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  // 借款/还款成功后后端返回最新状态，直接复用并触发台账刷新
  const handleBorrowed = (newStatus) => {
    if (newStatus) setStatus(newStatus);
    setRecordsRefreshKey((k) => k + 1);
    setMarketRefreshKey((k) => k + 1);
  };

  // 市场浏览中选中挂单：定位到借款表单，本次借款优先定向匹配该挂单
  const handleBorrowOrder = (offer) => {
    setSelectedOrder(offer);
    const el = document.getElementById('loan-borrow-form');
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  const termsRequired =
    !!status && status.terms_enabled && !status.terms_agreed;

  // 黑名单为服务器本地日序号（unix/86400），仅在未到期时展示
  const todayDay = Math.floor(Date.now() / 1000 / 86400);
  const blacklisted = !!status && status.blacklisted_until_day >= todayDay;

  const renderContent = () => {
    if (loading && !status) {
      return (
        <div className='flex justify-center py-16'>
          <Spin size='large' />
        </div>
      );
    }
    if (!status) {
      return (
        <Card className='!rounded-2xl'>
          <div className='flex flex-col items-center gap-3 py-8'>
            <Text type='secondary'>{loadError || t('获取借款状态失败')}</Text>
            <Button theme='outline' onClick={fetchStatus}>
              {t('重试')}
            </Button>
          </div>
        </Card>
      );
    }
    if (!status.enabled) {
      return (
        <Card className='!rounded-2xl'>
          <div className='py-8 text-center text-sm text-gray-500 dark:text-gray-400'>
            {t('词元贷功能未启用')}
          </div>
        </Card>
      );
    }
    return (
      <div className='flex flex-col gap-6'>
        {status.has_overdue ? (
          <div className='rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-700 dark:text-amber-400'>
            {t('您有逾期借款。利息与罚息会持续累计，直到还清。')}
          </div>
        ) : null}
        {blacklisted ? (
          <div className='rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-700 dark:text-red-400'>
            {t('您已被禁止借款，直至 {{date}}。', {
              date: formatBlacklistDate(status.blacklisted_until_day),
            })}
          </div>
        ) : null}
        <div className='grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]'>
          <LoanStatusCard t={t} status={status} />
          <div className='flex flex-col gap-6'>
            <div id='loan-borrow-form' className='scroll-mt-4'>
              <BorrowForm
                t={t}
                status={status}
                onBorrowed={handleBorrowed}
                presetOrder={selectedOrder}
                onClearOrder={() => setSelectedOrder(null)}
              />
            </div>
            {status.debt > 0 ? (
              <RepayForm t={t} status={status} onRepaid={handleBorrowed} />
            ) : null}
          </div>
        </div>
        <LoanRecordsTable t={t} refreshKey={recordsRefreshKey} />
        {status.ai_enabled ? <OfficerApplications t={t} /> : null}
        {status.market_enabled ? (
          <LendingMarket
            t={t}
            disclaimerAgreed={
              status.lender_disclaimer_agreed || disclaimerAgreed
            }
            onDisclaimerAgreed={() => {
              setDisclaimerAgreed(true);
              fetchStatus();
            }}
            onBorrowOrder={handleBorrowOrder}
            refreshKey={marketRefreshKey}
          />
        ) : null}
      </div>
    );
  };

  return (
    <div className='w-full max-w-7xl mx-auto relative min-h-screen lg:min-h-0 mt-[60px] px-2 pb-6'>
      <div className='mb-6'>
        <Title heading={3} className='!mb-1'>
          {t('词元贷')}
        </Title>
        <Text type='secondary'>{t('借用额度、查看台账与管理申请')}</Text>
      </div>

      {renderContent()}

      <TermsModal
        t={t}
        visible={termsRequired}
        termsText={status?.terms_text || ''}
        onAgreed={fetchStatus}
      />
    </div>
  );
};

export default Loan;
