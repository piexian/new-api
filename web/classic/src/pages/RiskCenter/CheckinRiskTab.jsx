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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Input,
  Modal,
  Radio,
  RadioGroup,
  Space,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  API,
  renderQuota,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import CardTable from '../../components/common/ui/CardTable';

const { Title, Text } = Typography;
const PAGE_SIZE = 10;

const CheckinRiskTab = () => {
  const { t } = useTranslation();
  const [status, setStatus] = useState('');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);

  // 逐日对比弹窗
  const [contrastVisible, setContrastVisible] = useState(false);
  const [contrastLoading, setContrastLoading] = useState(false);
  const [contrastData, setContrastData] = useState([]);
  const [contrastUser, setContrastUser] = useState(null);

  // 解除弹窗
  const [releaseVisible, setReleaseVisible] = useState(false);
  const [releaseNote, setReleaseNote] = useState('');
  const [releaseLoading, setReleaseLoading] = useState(false);
  const [releaseUser, setReleaseUser] = useState(null);

  const fetchItems = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/checkin_risk/', {
        params: {
          p: page,
          page_size: PAGE_SIZE,
          status: status || undefined,
        },
      });
      if (res.data.success) {
        setItems(res.data.data.items || []);
        setTotal(res.data.data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setLoading(false);
    }
  }, [page, status]);

  useEffect(() => {
    fetchItems();
  }, [fetchItems]);

  const openContrast = async (record) => {
    setContrastUser(record);
    setContrastVisible(true);
    setContrastLoading(true);
    setContrastData([]);
    try {
      const res = await API.get(`/api/checkin_risk/${record.user_id}/contrast`, {
        params: { days: 30 },
      });
      if (res.data.success) {
        setContrastData(res.data.data || []);
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setContrastLoading(false);
    }
  };

  const openRelease = (record) => {
    setReleaseUser(record);
    setReleaseNote('');
    setReleaseVisible(true);
  };

  const doRelease = async () => {
    setReleaseLoading(true);
    try {
      const res = await API.post(
        `/api/checkin_risk/${releaseUser.user_id}/release`,
        { note: releaseNote },
      );
      if (res.data.success) {
        showSuccess(t('解除成功'));
        setReleaseVisible(false);
        fetchItems();
      } else {
        showError(res.data.message);
      }
    } catch (err) {
      showError(err);
    } finally {
      setReleaseLoading(false);
    }
  };

  const renderTime = (value) => {
    if (!value) return '-';
    return typeof value === 'number' ? timestamp2string(value) : String(value);
  };

  const renderStatus = (value) => {
    switch (value) {
      case 'watching':
        return <Tag color='orange'>{t('观察中')}</Tag>;
      case 'locked':
        return <Tag color='red'>{t('已锁定')}</Tag>;
      case 'released':
        return <Tag color='green'>{t('已解除')}</Tag>;
      default:
        return <Tag>{value || '-'}</Tag>;
    }
  };

  const columns = useMemo(
    () => [
      { title: t('用户ID'), dataIndex: 'user_id', key: 'user_id' },
      { title: t('用户名'), dataIndex: 'username', key: 'username' },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        render: renderStatus,
      },
      {
        title: t('连续签到天数'),
        dataIndex: 'streak_days',
        key: 'streak_days',
      },
      { title: t('日均调用'), dataIndex: 'avg_calls', key: 'avg_calls' },
      {
        title: t('日均消费'),
        dataIndex: 'avg_quota',
        key: 'avg_quota',
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('日均签到所得'),
        dataIndex: 'avg_awarded',
        key: 'avg_awarded',
        render: (value) => renderQuota(value || 0),
      },
      { title: t('触发原因'), dataIndex: 'reason', key: 'reason' },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        key: 'created_at',
        render: renderTime,
      },
      {
        title: t('更新时间'),
        dataIndex: 'updated_at',
        key: 'updated_at',
        render: renderTime,
      },
      {
        title: t('操作'),
        key: 'action',
        render: (_, record) => (
          <Space>
            <Button size='small' onClick={() => openContrast(record)}>
              {t('逐日对比')}
            </Button>
            {record.status !== 'released' && (
              <Button
                size='small'
                type='primary'
                onClick={() => openRelease(record)}
              >
                {t('解除')}
              </Button>
            )}
          </Space>
        ),
      },
    ],
    [t],
  );

  const contrastColumns = useMemo(
    () => [
      { title: t('日期'), dataIndex: 'date', key: 'date' },
      {
        title: t('签到所得'),
        dataIndex: 'quota_awarded',
        key: 'quota_awarded',
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('是否补签'),
        dataIndex: 'is_makeup',
        key: 'is_makeup',
        render: (value) => (value ? t('是') : t('否')),
      },
      { title: t('调用次数'), dataIndex: 'calls', key: 'calls' },
      {
        title: t('消费额度'),
        dataIndex: 'quota',
        key: 'quota',
        render: (value) => renderQuota(value || 0),
      },
    ],
    [t],
  );

  return (
    <div className='flex flex-col gap-4'>
      <Card>
        <div className='flex flex-wrap items-end justify-between gap-3'>
          <Space wrap>
            <div>
              <Text type='secondary'>{t('状态筛选')}</Text>
              <div className='mt-2'>
                <RadioGroup
                  type='button'
                  value={status}
                  onChange={(event) => {
                    setStatus(event.target.value);
                    setPage(1);
                  }}
                >
                  <Radio value=''>{t('全部')}</Radio>
                  <Radio value='watching'>{t('观察中')}</Radio>
                  <Radio value='locked'>{t('已锁定')}</Radio>
                  <Radio value='released'>{t('已解除')}</Radio>
                </RadioGroup>
              </div>
            </div>
          </Space>
        </div>
      </Card>

      <Card>
        <Title heading={5} className='mb-4'>
          {t('签到风控')}
        </Title>
        <CardTable
          columns={columns}
          dataSource={items}
          loading={loading}
          rowKey='id'
          pagination={{
            currentPage: page,
            pageSize: PAGE_SIZE,
            total,
            onChange: setPage,
          }}
        />
      </Card>

      {/* 逐日对比弹窗 */}
      <Modal
        title={t('逐日对比') + (contrastUser ? ` - ${contrastUser.username}` : '')}
        visible={contrastVisible}
        onCancel={() => setContrastVisible(false)}
        footer={null}
        fullScreen
      >
        <Table
          columns={contrastColumns}
          dataSource={contrastData}
          loading={contrastLoading}
          rowKey='date'
          pagination={false}
        />
      </Modal>

      {/* 解除弹窗 */}
      <Modal
        title={t('解除风控') + (releaseUser ? ` - ${releaseUser.username}` : '')}
        visible={releaseVisible}
        onOk={doRelease}
        onCancel={() => setReleaseVisible(false)}
        okText={t('确认解除')}
        cancelText={t('取消')}
        confirmLoading={releaseLoading}
        centered
      >
        <Input
          value={releaseNote}
          onChange={setReleaseNote}
          placeholder={t('解除备注（可选）')}
        />
      </Modal>
    </div>
  );
};

export default CheckinRiskTab;
