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
import {
  Button,
  Collapse,
  Modal,
  Progress,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';

const { Text } = Typography;

const formatResetAt = (value) => {
  const numericValue = Number(value);
  if (!Number.isFinite(numericValue) || numericValue <= 0) return '-';
  const milliseconds = numericValue < 1e12 ? numericValue * 1000 : numericValue;
  return new Date(milliseconds).toLocaleString();
};

const clampPercent = (value) => {
  const numericValue = Number(value);
  if (!Number.isFinite(numericValue)) return 0;
  return Math.max(0, Math.min(100, numericValue));
};

const QwenTokenPlanUsage = ({ t, record }) => {
  const [loading, setLoading] = useState(true);
  const [payload, setPayload] = useState(null);

  const fetchUsage = useCallback(async () => {
    if (!record?.id) return;
    setLoading(true);
    try {
      const response = await API.get(
        `/api/channel/${record.id}/qwen/token_plan/usage?key_index=0`,
        { skipErrorHandler: true },
      );
      setPayload(response?.data ?? null);
      if (!response?.data?.success) {
        showError(
          response?.data?.message || t('获取 Qwen Token Plan 额度失败'),
        );
      }
    } catch (error) {
      setPayload({ success: false, message: String(error) });
      showError(t('获取 Qwen Token Plan 额度失败'));
    } finally {
      setLoading(false);
    }
  }, [record?.id, t]);

  useEffect(() => {
    fetchUsage().catch(() => {});
  }, [fetchUsage]);

  if (loading) {
    return (
      <div className='flex items-center justify-center py-10'>
        <Spin spinning size='large' tip={t('加载中...')} />
      </div>
    );
  }

  if (!payload?.success) {
    return (
      <div className='flex flex-col gap-3'>
        <Text type='danger'>
          {payload?.message || t('获取 Qwen Token Plan 额度失败')}
        </Text>
        <Button type='primary' theme='outline' onClick={fetchUsage}>
          {t('刷新')}
        </Button>
      </div>
    );
  }

  const usage = payload?.data || {};
  const per5Hour = usage.per_5_hour || {};
  const per1Week = usage.per_1_week || {};

  const renderWindow = (label, window) => {
    const present = window.present === true;
    const usedPercent = clampPercent(window.used_percent);
    return (
      <div className='flex flex-col gap-2'>
        <div className='flex items-center justify-between gap-3'>
          <Text strong>{label}</Text>
          <Tag
            color={
              usedPercent >= 80 ? 'red' : usedPercent >= 50 ? 'orange' : 'green'
            }
          >
            {Math.floor(usedPercent)}% {t('已用')}
          </Tag>
        </div>
        <Progress
          percent={usedPercent}
          showInfo={false}
          stroke={usedPercent >= 80 ? '#ef4444' : '#22c55e'}
        />
        <Text type='tertiary' size='small'>
          {present
            ? `${t('重置时间')}：${formatResetAt(window.reset_at)}`
            : t('该窗口暂无限额数据')}
        </Text>
      </div>
    );
  };

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex items-center justify-between gap-3'>
        <Text strong>{record?.name || 'Qwen Token Plan'}</Text>
        <Tag color={usage.subscribed ? 'green' : 'grey'}>
          {usage.subscribed ? t('有效') : t('未订阅或已失效')}
        </Tag>
      </div>
      {renderWindow(t('5 小时窗口额度'), per5Hour)}
      {renderWindow(t('1 周窗口额度'), per1Week)}
      <Collapse>
        <Collapse.Panel header={t('原始响应')} itemKey='raw'>
          <pre className='overflow-auto whitespace-pre-wrap break-all text-xs'>
            {JSON.stringify(payload, null, 2)}
          </pre>
        </Collapse.Panel>
      </Collapse>
    </div>
  );
};

export const openQwenTokenPlanUsageModal = ({ t, record }) => {
  Modal.info({
    title: t('Qwen Token Plan 额度'),
    centered: false,
    width: 760,
    style: { maxWidth: '95vw', top: 24 },
    content: <QwenTokenPlanUsage t={t} record={record} />,
    footer: (
      <Button type='primary' theme='solid' onClick={() => Modal.destroyAll()}>
        {t('关闭')}
      </Button>
    ),
  });
};
