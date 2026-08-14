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

import React from 'react';
import { Avatar, Card, Typography } from '@douyinfe/semi-ui';
import { Landmark } from 'lucide-react';
import { renderQuota } from '../../../helpers';

const StatusItem = ({ label, value, hint }) => (
  <div>
    <div className='text-xs font-medium tracking-wider uppercase text-gray-500 dark:text-gray-400'>
      {label}
    </div>
    <div className='text-lg font-semibold tabular-nums'>{value}</div>
    {hint ? (
      <div className='text-xs text-gray-400 dark:text-gray-500'>{hint}</div>
    ) : null}
  </div>
);

// 日利率展示与 default 主题一致（percent，最多两位小数）
const formatDailyRate = (rate) => {
  if (typeof rate !== 'number' || isNaN(rate)) return '-';
  return Intl.NumberFormat(undefined, {
    style: 'percent',
    maximumFractionDigits: 2,
  }).format(rate);
};

// interest_free_until 为服务器本地日序号（unix/86400），按本地时间展示日期
const formatGraceDate = (dayNumber) => {
  const d = new Date(dayNumber * 86400 * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

const LoanStatusCard = ({ t, status }) => {
  const graceDay = status?.interest_free_until || 0;
  const graceActive = graceDay > 0 && graceDay * 86400 * 1000 > Date.now();

  return (
    <Card className='!rounded-2xl'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='blue' className='mr-3 shadow-md'>
          <Landmark size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('借款状态')}
          </Typography.Text>
          <div className='text-xs text-gray-500 dark:text-gray-400'>
            {t('利息按当前时间估算，每日累计')}
          </div>
        </div>
      </div>

      <div className='grid grid-cols-2 sm:grid-cols-3 gap-x-4 gap-y-5'>
        <StatusItem
          label={t('未还本金')}
          value={renderQuota(status?.principal || 0)}
        />
        <StatusItem
          label={t('当前利息')}
          value={renderQuota(status?.interest || 0)}
        />
        <StatusItem
          label={t('总欠款')}
          value={renderQuota(status?.debt || 0)}
        />
        <StatusItem
          label={t('可借额度')}
          value={renderQuota(status?.available || 0)}
          hint={`${t('信用额度')}: ${renderQuota(status?.effective_max || 0)}`}
        />
        <StatusItem
          label={t('日利率')}
          value={formatDailyRate(status?.daily_rate)}
        />
        <StatusItem
          label={t('免息期')}
          value={graceActive ? formatGraceDate(graceDay) : t('无')}
          hint={graceActive ? t('此日期前不计利息') : undefined}
        />
        <StatusItem
          label={t('累计借款')}
          value={renderQuota(status?.total_borrowed || 0)}
        />
        <StatusItem
          label={t('累计还款')}
          value={renderQuota(status?.total_repaid || 0)}
        />
      </div>
    </Card>
  );
};

export default LoanStatusCard;
