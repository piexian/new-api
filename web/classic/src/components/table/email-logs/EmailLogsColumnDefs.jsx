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
import { Button, Tag, Tooltip, Typography } from '@douyinfe/semi-ui';
import { IconEyeOpened } from '@douyinfe/semi-icons';

const { Text } = Typography;

const renderStatus = (status, t) => {
  switch (status) {
    case 'success':
      return (
        <Tag color='green' shape='circle'>
          {t('成功')}
        </Tag>
      );
    case 'failed':
      return (
        <Tag color='red' shape='circle'>
          {t('失败')}
        </Tag>
      );
    case 'suppressed':
      return (
        <Tag color='orange' shape='circle'>
          {t('已抑制')}
        </Tag>
      );
    default:
      return (
        <Tag color='grey' shape='circle'>
          {status || '-'}
        </Tag>
      );
  }
};

const renderCopyableText = (text, copyText, className = '') => {
  if (!text) return '-';
  return (
    <Text
      ellipsis={{ showTooltip: true }}
      className={className}
      onClick={() => copyText(text)}
      style={{ cursor: 'pointer', display: 'inline-block', maxWidth: 260 }}
    >
      {text}
    </Text>
  );
};

export const getEmailLogsColumns = ({
  t,
  COLUMN_KEYS,
  copyText,
  onPreview,
}) => [
  {
    key: COLUMN_KEYS.SEND_TIME,
    title: t('发送时间'),
    dataIndex: 'send_time_text',
    fixed: true,
    render: (text) => <div className='whitespace-nowrap'>{text || '-'}</div>,
  },
  {
    key: COLUMN_KEYS.STATUS,
    title: t('状态'),
    dataIndex: 'status',
    render: (status) => renderStatus(status, t),
  },
  {
    key: COLUMN_KEYS.RECEIVER,
    title: t('收件人'),
    dataIndex: 'receiver',
    render: (text) => renderCopyableText(text, copyText, 'font-mono'),
  },
  {
    key: COLUMN_KEYS.SUBJECT,
    title: t('主题'),
    dataIndex: 'subject',
    render: (text) => renderCopyableText(text, copyText),
  },
  {
    key: COLUMN_KEYS.PROVIDER,
    title: t('提供商'),
    dataIndex: 'provider',
    render: (text) =>
      text ? (
        <Tag color='blue' shape='circle'>
          {text}
        </Tag>
      ) : (
        '-'
      ),
  },
  {
    key: COLUMN_KEYS.DURATION,
    title: t('耗时'),
    dataIndex: 'duration_ms',
    render: (duration) => {
      if (duration === undefined || duration === null) return '-';
      return (
        <Tag color={duration > 3000 ? 'orange' : 'green'} shape='circle'>
          {duration} ms
        </Tag>
      );
    },
  },
  {
    key: COLUMN_KEYS.ERROR_MESSAGE,
    title: t('错误信息'),
    dataIndex: 'error_message',
    render: (text) => renderCopyableText(text, copyText),
  },
  {
    key: COLUMN_KEYS.ACTIONS,
    title: t('Actions'),
    dataIndex: 'id',
    fixed: 'right',
    render: (_id, record) => (
      <Tooltip content={t('Preview')}>
        <Button
          theme='borderless'
          size='small'
          icon={<IconEyeOpened />}
          aria-label={t('Preview')}
          onClick={() => onPreview?.(record)}
        />
      </Tooltip>
    ),
  },
];
