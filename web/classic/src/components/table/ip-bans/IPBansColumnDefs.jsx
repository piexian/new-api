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
import { Button, Space, Tag, Tooltip, Typography } from '@douyinfe/semi-ui';
import { timestamp2string } from '../../../helpers';

const { Paragraph } = Typography;

export const getIPBanType = (record) =>
  record.expires_at === 0 ? 'permanent' : 'temporary';

export const isExpiredIPBan = (record) =>
  record.expires_at > 0 && record.expires_at <= Math.floor(Date.now() / 1000);

const renderType = (record, t) => {
  if (isExpiredIPBan(record)) {
    return (
      <Tag color='orange' shape='circle'>
        {t('已过期')}
      </Tag>
    );
  }

  return record.expires_at === 0 ? (
    <Tag color='red' shape='circle'>
      {t('永久封禁')}
    </Tag>
  ) : (
    <Tag color='orange' shape='circle'>
      {t('临时封禁')}
    </Tag>
  );
};

const renderTimestamp = (timestamp, t) => {
  if (!timestamp) {
    return (
      <Tag color='grey' shape='circle'>
        {t('永不过期')}
      </Tag>
    );
  }
  return (
    <span className='font-mono text-xs'>{timestamp2string(timestamp)}</span>
  );
};

export const getIPBansColumns = ({
  t,
  setEditingIPBan,
  setShowEditIPBan,
  showDeleteIPBanModal,
}) => [
  {
    title: t('ID'),
    dataIndex: 'id',
    width: 80,
  },
  {
    title: t('IP / CIDR'),
    dataIndex: 'target',
    render: (text) => (
      <Paragraph copyable={{ content: text }} className='!mb-0 font-mono'>
        {text}
      </Paragraph>
    ),
  },
  {
    title: t('类型'),
    dataIndex: 'expires_at',
    render: (text, record) => renderType(record, t),
  },
  {
    title: t('原因'),
    dataIndex: 'reason',
    render: (text) => (
      <Tooltip content={text || '-'} position='top'>
        <span className='inline-block max-w-[360px] truncate'>
          {text || '-'}
        </span>
      </Tooltip>
    ),
  },
  {
    title: t('过期时间'),
    dataIndex: 'expires_at',
    render: (text) => renderTimestamp(text, t),
  },
  {
    title: t('创建时间'),
    dataIndex: 'created_at',
    render: (text) =>
      text ? (
        <span className='font-mono text-xs'>{timestamp2string(text)}</span>
      ) : (
        '-'
      ),
  },
  {
    title: '',
    dataIndex: 'operate',
    fixed: 'right',
    width: 150,
    render: (text, record) => (
      <Space>
        <Button
          size='small'
          type='tertiary'
          onClick={() => {
            setEditingIPBan(record);
            setShowEditIPBan(true);
          }}
        >
          {t('编辑')}
        </Button>
        <Button
          size='small'
          type='danger'
          onClick={() => showDeleteIPBanModal(record)}
        >
          {t('删除')}
        </Button>
      </Space>
    ),
  },
];
