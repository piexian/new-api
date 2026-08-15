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
import { Avatar, Card, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { ReceiptText } from 'lucide-react';
import { API, renderQuota, timestamp2string } from '../../../helpers';
import QueryError from './QueryError';

const PAGE_SIZE = 10;

const LoanRecordsTable = ({ t, refreshKey }) => {
  const [page, setPage] = useState(1);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const fetchRecords = async (p) => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/user/loan/records', {
        params: { p, page_size: PAGE_SIZE },
      });
      const { success, message, data } = res.data;
      if (success) {
        setItems(data?.items || []);
        setTotal(data?.total || 0);
      } else {
        // 查询失败展示后端 message + 重试，不伪装成空数据
        setError(message || t('获取借款记录失败'));
      }
    } catch {
      // 网络/HTTP 错误已由 API 拦截器提示
      setError(t('获取借款记录失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRecords(page);
  }, [page, refreshKey]);

  const typeLabel = (type) => (type === 'borrow' ? t('借款') : t('还款'));
  const sourceLabel = (source) => {
    if (source === 'checkin') return t('签到');
    if (source === 'ai') return t('AI 专员');
    return t('手动');
  };

  const columns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        render: (v) => timestamp2string(v),
      },
      {
        title: t('类型'),
        dataIndex: 'type',
        render: (v) => (
          <Tag color={v === 'borrow' ? 'orange' : 'green'} size='small'>
            {typeLabel(v)}
          </Tag>
        ),
      },
      {
        title: t('金额'),
        dataIndex: 'amount',
        render: (v) => renderQuota(v || 0),
      },
      {
        title: t('利息部分'),
        dataIndex: 'interest_part',
        render: (v) => renderQuota(v || 0),
      },
      {
        title: t('本金部分'),
        dataIndex: 'principal_part',
        render: (v) => renderQuota(v || 0),
      },
      {
        title: t('手续费'),
        dataIndex: 'fee_part',
        render: (v) => renderQuota(v || 0),
      },
      {
        title: t('欠款余额'),
        dataIndex: 'debt_after',
        render: (v) => renderQuota(v || 0),
      },
      {
        title: t('来源'),
        dataIndex: 'source',
        render: (v) => sourceLabel(v),
      },
    ],
    [t],
  );

  return (
    <Card className='!rounded-2xl'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='violet' className='mr-3 shadow-md'>
          <ReceiptText size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('借款台账')}
          </Typography.Text>
          <div className='text-xs text-gray-500 dark:text-gray-400'>
            {t('借款与还款历史')}
          </div>
        </div>
      </div>

      {error ? (
        <QueryError t={t} message={error} onRetry={() => fetchRecords(page)} />
      ) : (
        <Table
          size='small'
          columns={columns}
          dataSource={items}
          rowKey='id'
          loading={loading}
          empty={t('暂无借款记录')}
          scroll={{ x: true }}
          pagination={{
            currentPage: page,
            pageSize: PAGE_SIZE,
            total,
            onPageChange: (p) => setPage(p),
          }}
        />
      )}
    </Card>
  );
};

export default LoanRecordsTable;
