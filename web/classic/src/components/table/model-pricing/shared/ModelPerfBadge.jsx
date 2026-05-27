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
import { Typography } from '@douyinfe/semi-ui';
import {
  formatCompactThroughput,
  formatLatency,
  toNumber,
} from './performanceFormat';

const { Text } = Typography;

const getStatusConfig = (successRate, t) => {
  const rate = toNumber(successRate);
  if (rate === null) {
    return {
      label: t('监控无数据'),
      className: 'bg-gray-100 text-gray-500 ring-gray-200',
    };
  }
  if (rate < 80) {
    return {
      label: t('监控异常'),
      className: 'bg-red-50 text-red-600 ring-red-200',
    };
  }
  if (rate < 99) {
    return {
      label: t('监控偏低'),
      className: 'bg-sky-50 text-sky-700 ring-sky-200',
    };
  }
  if (rate < 99.9) {
    return {
      label: t('监控波动'),
      className: 'bg-amber-50 text-amber-700 ring-amber-200',
    };
  }
  return {
    label: t('监控正常'),
    className: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  };
};

const ModelPerfBadge = ({ perf, t, className = '' }) => {
  if (!perf) {
    return null;
  }

  const successRate = toNumber(perf.success_rate);
  const successRateText =
    successRate === null ? '—' : `${successRate.toFixed(1)}%`;
  const statusConfig = getStatusConfig(successRate, t);

  return (
    <div
      className={`grid w-[154px] grid-cols-[42px_52px_44px] gap-x-2 text-right tabular-nums ${className}`}
    >
      <div title={t('平均延迟')} className='min-w-0'>
        <div className='text-[10px] leading-4 text-gray-400'>{t('延迟')}</div>
        <Text className='block truncate font-mono text-xs leading-4 text-gray-600'>
          {formatLatency(perf.avg_latency_ms)}
        </Text>
      </div>
      <div title={t('持续输出吞吐')} className='min-w-0'>
        <div className='text-[10px] leading-4 text-gray-400'>{t('吞吐')}</div>
        <Text className='block truncate font-mono text-xs leading-4 text-gray-600'>
          {formatCompactThroughput(perf.avg_tps)}
        </Text>
      </div>
      <div
        title={`${t('成功率')}: ${successRateText}`}
        className='min-w-0'
      >
        <div className='text-[10px] leading-4 text-gray-400'>{t('状态')}</div>
        <div className='flex h-4 items-center justify-end'>
          <span
            className={`inline-flex max-w-full items-center rounded-full px-1.5 text-[10px] font-medium leading-4 ring-1 ring-inset ${statusConfig.className}`}
          >
            {statusConfig.label}
          </span>
        </div>
      </div>
    </div>
  );
};

export default ModelPerfBadge;
