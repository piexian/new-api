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
import { HeartPulse, Timer, Zap } from 'lucide-react';
import {
  formatLatency,
  formatPercent,
  formatThroughput,
  successColor,
} from '../../shared/performanceFormat';

const SummaryItem = ({ icon: Icon, label, value, color }) => (
  <div className='flex min-w-0 items-center gap-2 px-3 py-2'>
    <Icon size={14} className='shrink-0 text-gray-400' />
    <div className='min-w-0 flex-1'>
      <div className='truncate text-[10px] font-medium uppercase tracking-wide text-gray-500'>
        {label}
      </div>
      <div
        className='truncate font-mono text-sm font-semibold tabular-nums'
        style={{ color }}
      >
        {value}
      </div>
    </div>
  </div>
);

const ModelPerformanceSummary = ({ perfSummary, t }) => {
  if (!perfSummary) {
    return null;
  }

  return (
    <div
      className='grid overflow-hidden rounded-xl sm:grid-cols-3 sm:divide-x'
      style={{
        border: '1px solid var(--semi-color-border)',
        background: 'var(--semi-color-bg-1)',
      }}
    >
      <SummaryItem
        icon={Zap}
        label={t('TPS')}
        value={formatThroughput(perfSummary.avg_tps)}
        color='#2563eb'
      />
      <SummaryItem
        icon={Timer}
        label={t('平均延迟')}
        value={formatLatency(perfSummary.avg_latency_ms)}
        color='#9333ea'
      />
      <SummaryItem
        icon={HeartPulse}
        label={t('成功率')}
        value={formatPercent(perfSummary.success_rate)}
        color={successColor(perfSummary.success_rate)}
      />
    </div>
  );
};

export default ModelPerformanceSummary;
