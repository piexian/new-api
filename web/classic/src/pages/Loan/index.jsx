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
import LoanRecordsTable from './components/LoanRecordsTable';
import OfficerApplications from './components/OfficerApplications';
import TermsModal from './components/TermsModal';

const { Text, Title } = Typography;

const Loan = () => {
  const { t } = useTranslation();
  const [status, setStatus] = useState(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [recordsRefreshKey, setRecordsRefreshKey] = useState(0);

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

  // 借款成功后后端返回最新状态，直接复用并触发台账刷新
  const handleBorrowed = (newStatus) => {
    if (newStatus) setStatus(newStatus);
    setRecordsRefreshKey((k) => k + 1);
  };

  const termsRequired =
    !!status && status.terms_enabled && !status.terms_agreed;

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
        <div className='grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]'>
          <LoanStatusCard t={t} status={status} />
          <BorrowForm t={t} status={status} onBorrowed={handleBorrowed} />
        </div>
        <LoanRecordsTable t={t} refreshKey={recordsRefreshKey} />
        {status.ai_enabled ? <OfficerApplications t={t} /> : null}
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
