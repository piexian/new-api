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

const getStatusColorClass = (successRate) => {
  const rate = toNumber(successRate);
  if (rate === null || rate < 99) {
    return 'bg-red-500';
  }
  if (rate < 99.9) {
    return 'bg-amber-500';
  }
  return 'bg-emerald-500';
};

const ModelPerfBadge = ({ perf, t, className = '' }) => {
  if (!perf) {
    return null;
  }

  const successRate = toNumber(perf.success_rate);

  return (
    <div
      className={`grid w-[136px] grid-cols-[42px_52px_30px] gap-x-2 text-right tabular-nums ${className}`}
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
        title={`${t('成功率')}: ${
          successRate === null ? '—' : `${successRate.toFixed(1)}%`
        }`}
        className='min-w-0'
      >
        <div className='text-[10px] leading-4 text-gray-400'>{t('状态')}</div>
        <div className='flex h-4 items-center justify-end gap-0.5'>
          <span className='h-2 w-1 rounded-full bg-gray-200' />
          <span className='h-2.5 w-1 rounded-full bg-gray-300' />
          <span
            className={`h-3 w-1 rounded-full ${getStatusColorClass(successRate)}`}
          />
        </div>
      </div>
    </div>
  );
};

export default ModelPerfBadge;
