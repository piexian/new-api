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
import {
  Button,
  Empty,
  Modal,
  SideSheet,
  Space,
  Tag,
  Typography,
  Popconfirm,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  showError,
  showSuccess,
  renderQuota,
  timestamp2string,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import CardTable from '../../../common/ui/CardTable';

const { Text } = Typography;

const PAGE_SIZE = 10;

function formatExpiredTime(expiredTime, t) {
  if (expiredTime === -1 || !expiredTime) {
    return t('永不过期');
  }
  return timestamp2string(expiredTime);
}

const UserTokensModal = ({ visible, onCancel, user, t, onSuccess }) => {
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [tokens, setTokens] = useState([]);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [actionLoadingId, setActionLoadingId] = useState(null);

  const loadTokens = async () => {
    if (!user?.id) return;
    setLoading(true);
    try {
      const res = await API.get(
        `/api/user/${user.id}/tokens?p=${currentPage}&size=${PAGE_SIZE}`,
      );
      if (res.data?.success) {
        const data = res.data.data || {};
        setTokens(data.items || []);
        setTotal(data.total || 0);
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) return;
    setCurrentPage(1);
  }, [visible]);

  useEffect(() => {
    if (visible && user?.id) {
      loadTokens();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, currentPage, user?.id]);

  const handlePageChange = (page) => {
    setCurrentPage(page);
  };

  const handleToggleStatus = async (record) => {
    setActionLoadingId(record.id);
    try {
      const next = { ...record, status: record.status === 1 ? 2 : 1 };
      const res = await API.put(`/api/user/${user.id}/tokens/${record.id}`, next);
      if (res.data?.success) {
        showSuccess(
          record.status === 1 ? t('令牌已禁用') : t('令牌已启用'),
        );
        loadTokens();
      } else {
        showError(res.data?.message || t('操作失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setActionLoadingId(null);
    }
  };

  const handleDelete = async (record) => {
    setActionLoadingId(record.id);
    try {
      const res = await API.delete(`/api/user/${user.id}/tokens/${record.id}`);
      if (res.data?.success) {
        showSuccess(t('令牌已删除'));
        // 如果当前页只剩一条且不是第一页，回退一页
        if (tokens.length === 1 && currentPage > 1) {
          setCurrentPage(currentPage - 1);
        } else {
          loadTokens();
        }
        onSuccess?.();
      } else {
        showError(res.data?.message || t('删除失败'));
      }
    } catch (e) {
      showError(t('请求失败'));
    } finally {
      setActionLoadingId(null);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('名称'),
        dataIndex: 'name',
        width: 160,
        render: (text, record) => (
          <Space>
            <Text strong>{text || '-'}</Text>
            {record.status === 2 && (
              <Tag color='red' size='small'>
                {t('已禁用')}
              </Tag>
            )}
          </Space>
        ),
      },
      {
        title: 'Key',
        dataIndex: 'key',
        width: 200,
        render: (text) => (
          <Text
            copyable
            ellipsis={{ showTooltip: true }}
            style={{ maxWidth: 180 }}
          >
            {text}
          </Text>
        ),
      },
      {
        title: t('分组'),
        dataIndex: 'group',
        width: 100,
        render: (text) => text || t('默认'),
      },
      {
        title: t('剩余额度'),
        dataIndex: 'remain_quota',
        width: 120,
        render: (val, record) =>
          record.unlimited_quota ? t('无限') : renderQuota(val),
      },
      {
        title: t('已用额度'),
        dataIndex: 'used_quota',
        width: 120,
        render: (val) => renderQuota(val),
      },
      {
        title: t('过期时间'),
        dataIndex: 'expired_time',
        width: 160,
        render: (val) => formatExpiredTime(val, t),
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_time',
        width: 160,
        render: (val) => (val ? timestamp2string(val) : '-'),
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        fixed: 'right',
        width: 150,
        render: (_, record) => (
          <Space>
            <Popconfirm
              title={t('确认操作')}
              content={
                record.status === 1
                  ? t('确定要禁用此令牌吗？')
                  : t('确定要启用此令牌吗？')
              }
              onConfirm={() => handleToggleStatus(record)}
            >
              <Button
                type={record.status === 1 ? 'danger' : 'primary'}
                size='small'
                loading={actionLoadingId === record.id}
              >
                {record.status === 1 ? t('禁用') : t('启用')}
              </Button>
            </Popconfirm>
            <Popconfirm
              title={t('确认删除')}
              content={t('确定要删除此令牌吗？此操作不可撤销。')}
              onConfirm={() => handleDelete(record)}
            >
              <Button
                type='danger'
                size='small'
                loading={actionLoadingId === record.id}
              >
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [tokens, actionLoadingId, currentPage, t],
  );

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 1100}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('管理')}
          </Tag>
          <Typography.Title heading={4} className='m-0'>
            {t('用户令牌管理')}
          </Typography.Title>
          <Text type='tertiary' className='ml-2'>
            {user?.username || '-'} (ID: {user?.id || '-'})
          </Text>
        </Space>
      }
    >
      <div className='p-4'>
        <CardTable
          columns={columns}
          dataSource={tokens}
          rowKey='id'
          loading={loading}
          scroll={{ x: 'max-content' }}
          hidePagination={false}
          pagination={{
            currentPage,
            pageSize: PAGE_SIZE,
            total,
            pageSizeOpts: [10, 20, 50],
            showSizeChanger: false,
            onPageChange: handlePageChange,
          }}
          empty={
            <Empty
              image={
                <IllustrationNoResult style={{ width: 150, height: 150 }} />
              }
              darkModeImage={
                <IllustrationNoResultDark
                  style={{ width: 150, height: 150 }}
                />
              }
              description={t('暂无令牌')}
              style={{ padding: 30 }}
            />
          }
          size='middle'
        />
      </div>
    </SideSheet>
  );
};

export default UserTokensModal;
